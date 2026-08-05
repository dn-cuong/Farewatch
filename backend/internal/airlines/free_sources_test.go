package airlines

import "testing"

func TestTravelpayoutsOffersNormalizeFare(t *testing.T) {
	resp := travelpayoutsResponse{
		Success:  true,
		Currency: "usd",
		Data: []travelpayoutsFare{{
			Origin:       "JFK",
			Destination:  "LAX",
			DepartureAt:  "2026-09-15T08:30:00-04:00",
			Airline:      "B6",
			FlightNumber: "123",
			Price:        179,
			Transfers:    0,
			Duration:     360,
			Link:         "/search/JFK1509LAX1",
		}},
	}

	offers := travelpayoutsOffers(resp, "JFK", "LAX", "2026-09-15", nil, "economy")
	if len(offers) != 1 {
		t.Fatalf("expected one offer, got %d", len(offers))
	}
	offer := offers[0]
	if offer.FlightNumber != "B6123" || offer.Source != "search:travelpayouts" {
		t.Fatalf("unexpected normalized offer: %+v", offer)
	}
	if offer.Currency != "USD" || offer.Price != 179 {
		t.Fatalf("unexpected money: %.2f %s", offer.Price, offer.Currency)
	}
}

func TestSkyOffersNormalizeItinerary(t *testing.T) {
	var resp skySearchResponse
	itinerary := skyItinerary{ID: "abc"}
	itinerary.Price.Raw = 205.50
	leg := skyLeg{
		Departure:        "2026-09-15T09:00:00",
		Arrival:          "2026-09-15T12:00:00",
		DurationInMinute: 360,
		StopCount:        0,
	}
	leg.Carriers.Marketing = append(leg.Carriers.Marketing, struct {
		Name        string `json:"name"`
		AlternateID string `json:"alternateId"`
	}{Name: "JetBlue", AlternateID: "B6"})
	leg.Segments = append(leg.Segments, struct {
		FlightNumber string `json:"flightNumber"`
		Departure    string `json:"departure"`
		Arrival      string `json:"arrival"`
		Origin       struct {
			DisplayCode string `json:"displayCode"`
			Name        string `json:"name"`
		} `json:"origin"`
		Destination struct {
			DisplayCode string `json:"displayCode"`
			Name        string `json:"name"`
		} `json:"destination"`
		MarketingCarrier struct {
			Name        string `json:"name"`
			AlternateID string `json:"alternateId"`
		} `json:"marketingCarrier"`
	}{FlightNumber: "123"})
	itinerary.Legs = []skyLeg{leg}
	resp.Data.Itineraries = []skyItinerary{itinerary}

	offers := skyOffers(resp, "JFK", "LAX", "2026-09-15", nil, "economy")
	if len(offers) != 1 {
		t.Fatalf("expected one offer, got %d", len(offers))
	}
	if offers[0].AirlineCode != "B6" || offers[0].FlightNumber != "B6123" {
		t.Fatalf("unexpected carrier: %+v", offers[0])
	}
	if offers[0].Source != "search:sky_scrapper" {
		t.Fatalf("unexpected source: %s", offers[0].Source)
	}
}

func TestFreeSearchProvidersRequireKeys(t *testing.T) {
	if got := FreeSearchProviders("", ""); len(got) != 0 {
		t.Fatalf("expected no providers without credentials, got %d", len(got))
	}
	if got := FreeSearchProviders("tp", "rapid"); len(got) != 2 {
		t.Fatalf("expected two configured providers, got %d", len(got))
	}
}
