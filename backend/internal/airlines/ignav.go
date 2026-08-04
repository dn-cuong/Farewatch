package airlines

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
			MarketingCarrierCode  string `json:"marketing_carrier_code"`
			FlightNumber          string `json:"flight_number"`
			OperatingCarrierName  string `json:"operating_carrier_name"`
			DepartureAirport      string `json:"departure_airport"`
			DepartureTimeUTC      string `json:"departure_time_utc"`
			ArrivalAirport        string `json:"arrival_airport"`
			ArrivalTimeUTC        string `json:"arrival_time_utc"`
			DurationMinutes       int    `json:"duration_minutes"`
			Aircraft              string `json:"aircraft"`
		} `json:"segments"`
	} `json:"outbound"`
	CabinClass string `json:"cabin_class"`
	IgnavID    string `json:"ignav_id"`
}

func (c *IgnavClient) Search(ctx context.Context, origin, dest, departDate string, returnDate *string) ([]Offer, error) {
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
		from := LookupAirport(first.DepartureAirport)
		to := LookupAirport(last.ArrivalAirport)
		cabin := it.CabinClass
		if cabin == "" {
			cabin = "economy"
		}
		currency := it.Price.Currency
		if currency == "" {
			currency = "USD"
		}
		offers = append(offers, Offer{
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
			DeepLink:        fmt.Sprintf("https://ignav.com/book/%s", it.IgnavID),
			Source:          "ignav",
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

func (p *IgnavBackedProvider) Fetch(ctx context.Context, origin, dest, departDate string, returnDate *string, cabin string) (Offer, error) {
	if p.ignav != nil {
		if off, ok, err := p.ignav.BestForCarrier(ctx, p.Code(), origin, dest, departDate, returnDate); err == nil && ok {
			return off, nil
		}
	}
	return p.inner.Fetch(ctx, origin, dest, departDate, returnDate, cabin)
}

// AllWithLive wraps airline providers with Ignav (preferred free live source).
func AllWithLive(ignav *IgnavClient) []Provider {
	base := All()
	if ignav == nil {
		return base
	}
	out := make([]Provider, len(base))
	for i, p := range base {
		out[i] = &IgnavBackedProvider{inner: p, ignav: ignav}
	}
	return out
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
