package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/util"
	"github.com/evcc-io/evcc/vehicle/audi"
	"github.com/evcc-io/evcc/vehicle/seat"
	"github.com/evcc-io/evcc/vehicle/skoda"
	"github.com/evcc-io/evcc/vehicle/vw"
	"golang.org/x/oauth2"
)

const cache = time.Minute

var brand, vin, user, password string

func fatalf(format string, a ...interface{}) {
	fmt.Printf(format, a...)
	os.Exit(1)
}

type RemoteTokenSource struct {
	mu    sync.Mutex
	ts    oauth2.TokenSource
	token *oauth2.Token
}

func (s *RemoteTokenSource) Token() (*oauth2.Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.token != nil && time.Until(s.token.Expiry) > time.Minute {
		return s.token, nil
	}

	// implement the remote API call to retrieve the token here
	token, err := s.ts.Token()
	if err == nil {
		s.token = token
	}

	return token, err
}

type params struct {
	AuthClientID string
	AuthParams   url.Values
	Brand        string
	Country      string
}

var brands = map[string]params{
	"audi":  {audi.AuthClientID, audi.AuthParams, audi.Brand, audi.Country},
	"seat":  {seat.AuthClientID, seat.AuthParams, seat.Brand, seat.Country},
	"skoda": {skoda.AuthClientID, skoda.AuthParams, skoda.Brand, skoda.Country},
	"vw":    {vw.AuthClientID, vw.AuthParams, vw.Brand, vw.Country},
}

func main() {
	// parameters
	flag.StringVar(&brand, "brand", "", "vehicle brand (audi|seat|skoda|vw)")
	flag.StringVar(&vin, "vin", "", "vehicle identification number")
	flag.StringVar(&user, "user", "", "user name")
	flag.StringVar(&password, "password", "", "password")

	flag.Parse()

	if brand == "" || vin == "" || user == "" || password == "" {
		fatalf("Usage: -brand=audi|seat|skoda|vw -vin=<vin> -user=<user name> -password=<password>")
	}

	vin = strings.ToUpper(vin)

	// create the token source
	var identity *vw.Identity
	log := util.NewLogger("ident")

	switch strings.ToLower(brand) {
	case "audi", "seat", "skoda", "vw":
		params := brands[strings.ToLower(brand)]
		identity = vw.NewIdentity(log, params.AuthClientID, params.AuthParams, user, password)
		if err := identity.Login(); err != nil {
			fatalf("login failed: %w", err)
		}
	}

	// couple the token source with the API
	ts := &RemoteTokenSource{ts: identity}

	// create the actual api
	var vehicle api.Battery
	log = util.NewLogger("api")

	switch strings.ToLower(brand) {
	case "audi", "seat", "skoda", "vw":
		params := brands[strings.ToLower(brand)]
		api := vw.NewAPI(log, ts, params.Brand, params.Country)
		if err := api.HomeRegion(vin); err != nil {
			fatalf("home region failed: %w", err)
		}
		vehicle = vw.NewProvider(api, vin, cache)
	}

	_, ok := vehicle.(api.VehicleStartCharge)
	fmt.Printf("api has start/stop: %v\n", ok)

	soc, _ := vehicle.SoC()
	fmt.Printf("soc: %.1f\n", soc)
}
