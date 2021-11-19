package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/util"
	"github.com/evcc-io/evcc/vehicle/audi"
	"github.com/evcc-io/evcc/vehicle/seat"
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

func main() {
	// parameters
	flag.StringVar(&brand, "brand", "", "vehicle brand (audi|seat|vw)")
	flag.StringVar(&vin, "vin", "", "vehicle identification number")
	flag.StringVar(&user, "user", "", "user name")
	flag.StringVar(&password, "password", "", "password")

	flag.Parse()

	if brand == "" || vin == "" || user == "" || password == "" {
		fatalf("Usage: -brand=audi|skoda|vw -vin=<vin> -user=<user name> -password=<password>")
	}

	vin = strings.ToUpper(vin)

	// create the token source
	var identity *vw.Identity
	log := util.NewLogger("ident")

	switch strings.ToLower(brand) {
	case "audi":
		identity = vw.NewIdentity(log, audi.AuthClientID, audi.AuthParams, user, password)
		if err := identity.Login(); err != nil {
			fatalf("login failed: %w", err)
		}

	case "seat":
		identity = vw.NewIdentity(log, seat.AuthClientID, seat.AuthParams, user, password)
		if err := identity.Login(); err != nil {
			fatalf("login failed: %w", err)
		}

	case "vw":
		identity = vw.NewIdentity(log, vw.AuthClientID, vw.AuthParams, user, password)
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
	case "audi":
		api := vw.NewAPI(log, ts, audi.Brand, audi.Country)
		if err := api.HomeRegion(vin); err != nil {
			fatalf("home region failed: %w", err)
		}
		vehicle = vw.NewProvider(api, vin, cache)

	case "seat":
		api := vw.NewAPI(log, ts, seat.Brand, seat.Country)
		if err := api.HomeRegion(vin); err != nil {
			fatalf("home region failed: %w", err)
		}
		vehicle = vw.NewProvider(api, vin, cache)

	case "vw":
		api := vw.NewAPI(log, ts, vw.Brand, vw.Country)
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
