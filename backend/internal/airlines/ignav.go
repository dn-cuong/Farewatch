package airlines

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// IgnavClient fetches live fares from Ignav (self-serve flight price API).
// Docs: https://ignav.com/docs/quickstart
type IgnavClient struct {
	apiKey string
	base   string
	http   *http.Client

	mu    sync.Mutex
	cache map[string]ignavCacheEntry
}

type ignavCacheEntry struct {
	offers []Offer
	until  time.Time
}

func NewIgnav(apiKey string) *IgnavClient {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil
	}
	return &IgnavClient{
		apiKey: apiKey,
		base:   "https://ignav.com",
		http:   &http.Client{Timeout: 30 * time.Second},
		cache:  make(map[string]ignavCacheEntry),
	}
}

func (c *IgnavClient) Enabled() bool { return c != nil }

type ignavOneWayResp struct {
	Itineraries []ignavItinerary `json:"itineraries"`
}

type ignavItinerary struct {
	Price struct {
		Amount   float64 `json:"amount"`
		Currency string  `json:"currency"`
	} `json:"price"`
	Outbound struct {
		Carrier         string `json:"carrier"`
		DurationMinutes int    `json:"duration_minutes"`
		Segments        []struct {
			MarketingCarrierCode string `json:"marketing_carrier_code"`
			FlightNumber         string `json:"flight_number"`
			OperatingCarrierName string `json:"operating_carrier_name"`
			DepartureAirport     string `json:"departure_airport"`
			DepartureTimeUTC     string `json:"departure_time_utc"`
			ArrivalAirport       string `json:"arrival_airport"`
			ArrivalTimeUTC       string `json:"arrival_time_utc"`
			DurationMinutes      int    `json:"duration_minutes"`
			Aircraft             string `json:"aircraft"`
		} `json:"segments"`
	} `json:"outbound"`
	CabinClass string `json:"cabin_class"`
	IgnavID    string `json:"ignav_id"`
}

func (c *IgnavClient) Search(ctx context.Context, origin, dest, departDate string, returnDate *string) ([]Offer, error) {
	from := LookupAirport(strings.ToUpper(origin))
	to := LookupAirport(strings.ToUpper(dest))
	distanceKm := haversineKm(from.Lat, from.Lon, to.Lat, to.Lon)
	key := strings.ToUpper(origin) + "|" + strings.ToUpper(dest) + "|" + departDate + "|" + derefStr(returnDate)
	c.mu.Lock()
	if ent, ok := c.cache[key]; ok && time.Now().Before(ent.until) {
		offers := ent.offers
		c.mu.Unlock()
		return offers, nil
	}
	c.mu.Unlock()

	endpoint := "/api/fares/one-way"
	body := map[string]any{
		"origin":         strings.ToUpper(origin),
		"destination":    strings.ToUpper(dest),
		"departure_date": departDate,
	}
	if returnDate != nil && *returnDate != "" {
		endpoint = "/api/fares/round-trip"
		body["return_date"] = *returnDate
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", c.apiKey)

	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	respBody, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("ignav %d: %s", res.StatusCode, truncate(string(respBody), 300))
	}

	var parsed ignavOneWayResp
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, err
	}

	offers := make([]Offer, 0, len(parsed.Itineraries))
	for _, it := range parsed.Itineraries {
		if len(it.Outbound.Segments) == 0 {
			continue
		}
		first := it.Outbound.Segments[0]
		last := it.Outbound.Segments[len(it.Outbound.Segments)-1]
		departAt, _ := time.Parse(time.RFC3339, first.DepartureTimeUTC)
		arriveAt, _ := time.Parse(time.RFC3339, last.ArrivalTimeUTC)
		if departAt.IsZero() {
			departAt, _ = time.Parse("2006-01-02T15:04:05Z", first.DepartureTimeUTC)
		}
		if arriveAt.IsZero() {
			arriveAt, _ = time.Parse("2006-01-02T15:04:05Z", last.ArrivalTimeUTC)
		}
		if !plausibleItineraryPrice(distanceKm, len(it.Outbound.Segments)-1, it.CabinClass, it.Price.Amount) {
			continue
		}
		code := strings.ToUpper(first.MarketingCarrierCode)
		airline := it.Outbound.Carrier
		if airline == "" {
			airline = first.OperatingCarrierName
		}
		if airline == "" {
			airline = code
		}
		flightNum := strings.TrimSpace(first.FlightNumber)
		if !strings.HasPrefix(strings.ToUpper(flightNum), code) {
			flightNum = fmt.Sprintf("%s %s", code, flightNum)
		}
		cabin := it.CabinClass
		if cabin == "" {
			cabin = "economy"
		}
		currency := it.Price.Currency
		if currency == "" {
			currency = "USD"
		}
		offers = append(offers, Offer{
			OfferID:         it.IgnavID,
			Airline:         airline,
			AirlineCode:     code,
			FlightNumber:    flightNum,
			Origin:          first.DepartureAirport,
			OriginCity:      from.City,
			Destination:     last.ArrivalAirport,
			DestinationCity: to.City,
			DepartAt:        departAt,
			ArriveAt:        arriveAt,
			DurationMinutes: it.Outbound.DurationMinutes,
			Stops:           len(it.Outbound.Segments) - 1,
			Cabin:           cabin,
			Aircraft:        first.Aircraft,
			Price:           it.Price.Amount,
			Currency:        currency,
			DeepLink:        googleFlightsURL(first.DepartureAirport, last.ArrivalAirport, departDate, returnDate),
			Source:          "ignav",
			Segments:        buildSegments(it),
			LayoverAirports: layoverAirports(it.Outbound.Segments),
		})
	}

	c.mu.Lock()
	c.cache[key] = ignavCacheEntry{offers: offers, until: time.Now().Add(2 * time.Minute)}
	c.mu.Unlock()
	return offers, nil
}

// BestForCarrier returns the cheapest Ignav offer for a marketing carrier.
func (c *IgnavClient) BestForCarrier(ctx context.Context, carrier, origin, dest, departDate string, returnDate *string) (Offer, bool, error) {
	offers, err := c.Search(ctx, origin, dest, departDate, returnDate)
	if err != nil {
		return Offer{}, false, err
	}
	carrier = strings.ToUpper(carrier)
	var best *Offer
	for i := range offers {
		o := offers[i]
		if o.AirlineCode != carrier {
			continue
		}
		if best == nil || o.Price < best.Price {
			cp := o
			best = &cp
		}
	}
	if best == nil {
		return Offer{}, false, nil
	}
	return *best, true, nil
}

// IgnavBackedProvider tries Ignav live fares first, then the local airline simulator.
type IgnavBackedProvider struct {
	inner Provider
	ignav *IgnavClient
}

func (p *IgnavBackedProvider) Code() string { return p.inner.Code() }
func (p *IgnavBackedProvider) Name() string { return p.inner.Name() }
func (p *IgnavBackedProvider) Kind() string {
	if k, ok := p.inner.(interface{ Kind() string }); ok {
		return k.Kind()
	}
	return "airline"
}

func (p *IgnavBackedProvider) Fetch(ctx context.Context, origin, dest, departDate string, returnDate *string, cabin string) ([]Offer, error) {
	if p.ignav != nil {
		if off, ok, err := p.ignav.BestForCarrier(ctx, p.Code(), origin, dest, departDate, returnDate); err == nil && ok {
			off.Source = "search:ignav"
			return []Offer{off}, nil
		}
	}
	return p.inner.Fetch(ctx, origin, dest, departDate, returnDate, cabin)
}

// AllWithLive is kept for compatibility; prefer AllProviders.
func AllWithLive(ignav *IgnavClient) []Provider {
	return AllProviders(ignav, nil)
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// BookingLink is one purchase option resolved from Ignav.
type BookingLink struct {
	ProviderName string  `json:"providerName"`
	ProviderType string  `json:"providerType"`
	FareName     string  `json:"fareName"`
	Price        float64 `json:"price"`
	Currency     string  `json:"currency"`
	URL          string  `json:"url"`
}

type ignavBookingResp struct {
	BookingOptions []struct {
		Links []struct {
			ProviderName string `json:"provider_name"`
			ProviderType string `json:"provider_type"`
			FareName     string `json:"fare_name"`
			Price        struct {
				Amount   float64 `json:"amount"`
				Currency string  `json:"currency"`
			} `json:"price"`
			URL string `json:"url"`
		} `json:"links"`
	} `json:"booking_options"`
}

// BookingLinks resolves real airline/OTA checkout URLs for an Ignav itinerary.
func (c *IgnavClient) BookingLinks(ctx context.Context, ignavID string) ([]BookingLink, error) {
	if c == nil {
		return nil, fmt.Errorf("ignav is not configured")
	}
	ignavID = strings.TrimSpace(ignavID)
	if ignavID == "" {
		return nil, fmt.Errorf("missing offer id")
	}
	raw, err := json.Marshal(map[string]any{"ignav_id": ignavID})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/api/fares/booking-links", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", c.apiKey)

	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	respBody, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("ignav booking-links %d: %s", res.StatusCode, truncate(string(respBody), 300))
	}

	var parsed ignavBookingResp
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, err
	}
	out := make([]BookingLink, 0, 8)
	for _, opt := range parsed.BookingOptions {
		for _, link := range opt.Links {
			u := strings.TrimSpace(link.URL)
			if u == "" {
				continue
			}
			if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
				u = "https://" + u
			}
			currency := link.Price.Currency
			if currency == "" {
				currency = "USD"
			}
			out = append(out, BookingLink{
				ProviderName: link.ProviderName,
				ProviderType: link.ProviderType,
				FareName:     link.FareName,
				Price:        link.Price.Amount,
				Currency:     currency,
				URL:          u,
			})
		}
	}
	return out, nil
}

func googleFlightsURL(origin, dest, depart string, returnDate *string) string {
	q := fmt.Sprintf("Flights to %s from %s on %s", strings.ToUpper(dest), strings.ToUpper(origin), depart)
	if returnDate != nil && *returnDate != "" {
		q += " through " + *returnDate
	}
	return "https://www.google.com/travel/flights?q=" + strings.ReplaceAll(url.QueryEscape(q), "+", "%20")
}

// GoogleFlightsFallback builds a consumer Google Flights search URL when Ignav booking links are unavailable.
func GoogleFlightsFallback(origin, dest, depart string, returnDate *string) string {
	if origin == "" || dest == "" || depart == "" {
		return "https://www.google.com/travel/flights"
	}
	return googleFlightsURL(origin, dest, depart, returnDate)
}

func parseIgnavTime(raw string) time.Time {
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t, _ = time.Parse("2006-01-02T15:04:05Z", raw)
	}
	return t
}

func buildSegments(it ignavItinerary) []Segment {
	out := make([]Segment, 0, len(it.Outbound.Segments))
	for _, seg := range it.Outbound.Segments {
		code := strings.ToUpper(seg.MarketingCarrierCode)
		fn := strings.TrimSpace(seg.FlightNumber)
		if code != "" && !strings.HasPrefix(strings.ToUpper(fn), code) {
			fn = fmt.Sprintf("%s %s", code, fn)
		}
		airline := seg.OperatingCarrierName
		if airline == "" {
			airline = it.Outbound.Carrier
		}
		if airline == "" {
			airline = code
		}
		from := LookupAirport(seg.DepartureAirport)
		to := LookupAirport(seg.ArrivalAirport)
		out = append(out, Segment{
			AirlineCode:     code,
			Airline:         airline,
			FlightNumber:    fn,
			Origin:          seg.DepartureAirport,
			OriginCity:      from.City,
			Destination:     seg.ArrivalAirport,
			DestinationCity: to.City,
			DepartAt:        parseIgnavTime(seg.DepartureTimeUTC),
			ArriveAt:        parseIgnavTime(seg.ArrivalTimeUTC),
			DurationMinutes: seg.DurationMinutes,
			Aircraft:        seg.Aircraft,
		})
	}
	return out
}

func layoverAirports(segs []struct {
	MarketingCarrierCode string `json:"marketing_carrier_code"`
	FlightNumber         string `json:"flight_number"`
	OperatingCarrierName string `json:"operating_carrier_name"`
	DepartureAirport     string `json:"departure_airport"`
	DepartureTimeUTC     string `json:"departure_time_utc"`
	ArrivalAirport       string `json:"arrival_airport"`
	ArrivalTimeUTC       string `json:"arrival_time_utc"`
	DurationMinutes      int    `json:"duration_minutes"`
	Aircraft             string `json:"aircraft"`
}) []string {
	if len(segs) < 2 {
		return nil
	}
	out := make([]string, 0, len(segs)-1)
	for i := 0; i < len(segs)-1; i++ {
		out = append(out, segs[i].ArrivalAirport)
	}
	return out
}
