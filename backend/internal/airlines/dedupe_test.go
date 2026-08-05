package airlines

import "testing"

func TestDeduplicateCheapest(t *testing.T) {
	offers := []Offer{
		{AirlineCode: "DL", FlightNumber: "DL 100", Origin: "JFK", Destination: "LAX", Cabin: "economy", Price: 320, Source: "search:ignav"},
		{AirlineCode: "DL", FlightNumber: "DL100", Origin: "JFK", Destination: "LAX", Cabin: "economy", Price: 299, Source: "airline:DL"},
		{AirlineCode: "UA", FlightNumber: "UA 200", Origin: "JFK", Destination: "LAX", Cabin: "economy", Price: 310, Source: "airline:UA"},
		{AirlineCode: "UA", FlightNumber: "UA 200", Origin: "JFK", Destination: "LAX", Cabin: "economy", Price: 310, Source: "search:ignav"},
	}
	out := DeduplicateCheapest(offers)
	if len(out) != 2 {
		t.Fatalf("expected 2 offers after dedupe, got %d", len(out))
	}
	if out[0].AirlineCode != "UA" || out[0].Price != 310 || out[0].Source != "search:ignav" {
		t.Fatalf("expected cheapest real search offer first, got %+v", out[0])
	}
	if out[1].AirlineCode != "DL" || out[1].Price != 320 || out[1].Source != "search:ignav" {
		t.Fatalf("expected search source to outrank cheaper simulator on same itinerary, got %+v", out[1])
	}
}

func TestDeduplicatePrefersSearchOverCheaperSimulator(t *testing.T) {
	offers := []Offer{
		{AirlineCode: "F9", FlightNumber: "F9123", Origin: "HAN", Destination: "LAX", Cabin: "economy", Price: 240, Source: "simulator:F9"},
		{AirlineCode: "F9", FlightNumber: "F9123", Origin: "HAN", Destination: "LAX", Cabin: "economy", Price: 420, Source: "search:amadeus"},
	}
	out := DeduplicateCheapest(offers)
	if len(out) != 1 {
		t.Fatalf("expected 1 offer after dedupe, got %d", len(out))
	}
	if out[0].Source != "search:amadeus" || out[0].Price != 420 {
		t.Fatalf("expected search offer to win over cheaper simulator, got %+v", out[0])
	}
}

func TestAllProvidersCount(t *testing.T) {
	providers := AllProviders(nil, nil)
	// Global airline registry + Google Flights search adapter.
	if len(providers) < 30 {
		t.Fatalf("expected at least 30 providers, got %d", len(providers))
	}
	airlines := 0
	search := 0
	for _, p := range providers {
		switch p.Kind() {
		case "airline":
			airlines++
		case "search":
			search++
		}
	}
	if airlines != AirlineProviderCount() || airlines < 25 {
		t.Fatalf("expected global airline registry, got %d", airlines)
	}
	if search < 1 {
		t.Fatalf("expected at least 1 search provider, got %d", search)
	}
}

func TestAirlineRegistryHasUniqueCodesAndSearchURLs(t *testing.T) {
	seen := map[string]bool{}
	for _, def := range airlineDefs() {
		if seen[def.code] {
			t.Fatalf("duplicate airline code %s", def.code)
		}
		seen[def.code] = true
		if def.name == "" || len(def.aircraft) == 0 {
			t.Fatalf("incomplete airline definition: %+v", def)
		}
		if got := airlineSearchURL(def.code, "JFK", "LAX", "2026-09-15", nil); got == "" {
			t.Fatalf("missing public search URL for %s", def.code)
		}
	}
}
