package airlines

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// IgnavSearchProvider is the Ignav multi-airline search API adapter.
type IgnavSearchProvider struct {
	client *IgnavClient
}

func NewIgnavSearchProvider(c *IgnavClient) *IgnavSearchProvider {
	return &IgnavSearchProvider{client: c}
}

func (p *IgnavSearchProvider) Code() string { return "IGN" }
func (p *IgnavSearchProvider) Name() string { return "Ignav Search API" }
func (p *IgnavSearchProvider) Kind() string { return "search" }

func (p *IgnavSearchProvider) Fetch(ctx context.Context, origin, dest, departDate string, returnDate *string, cabin string) ([]Offer, error) {
	if p.client == nil {
		return nil, fmt.Errorf("ignav not configured")
	}
	offers, err := p.client.Search(ctx, origin, dest, departDate, returnDate)
	if err != nil {
		return nil, err
	}
	out := make([]Offer, 0, len(offers))
	for _, o := range offers {
		if cabin != "" && !strings.EqualFold(o.Cabin, cabin) && o.Cabin != "" {
			// Keep offers; cabin filters are soft - Ignav may use different labels.
		}
		o.Source = "search:ignav"
		out = append(out, o)
	}
	return out, nil
}

// GoogleFlightsSearchProvider polls Google Flights HTML; empty parse returns no offers.
type GoogleFlightsSearchProvider struct {
	client *http.Client
}

func NewGoogleFlightsSearchProvider() *GoogleFlightsSearchProvider {
	return &GoogleFlightsSearchProvider{
		client: &http.Client{Timeout: 8 * time.Second},
	}
}

func (p *GoogleFlightsSearchProvider) Code() string { return "GFL" }
func (p *GoogleFlightsSearchProvider) Name() string { return "Google Flights Search" }
func (p *GoogleFlightsSearchProvider) Kind() string { return "search" }

func (p *GoogleFlightsSearchProvider) Fetch(ctx context.Context, origin, dest, departDate string, returnDate *string, cabin string) ([]Offer, error) {
	u := googleFlightsURL(origin, dest, departDate, returnDate)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "FareWatch/1.0 (+https://github.com/dn-cuong/Farewatch; fare-tracker)")
	req.Header.Set("Accept", "text/html")

	res, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 512*1024))
	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("google flights http %d", res.StatusCode)
	}

	price, ok := extractPublicPrice(body)
	if !ok {
		// Soft miss - Google often serves JS shells without fares.
		return nil, fmt.Errorf("google flights: no parseable fare")
	}

	// Google Flights pages rarely expose structured carrier rows to simple HTTP clients.
	// When a price is visible we still surface a metasearch deep link via Ignav/airline offers;
	// avoid inventing a fake GFL "flight" that users could watch.
	_ = price
	return nil, fmt.Errorf("google flights: structured itineraries unavailable over plain HTTP")
}

// AmadeusSearchProvider wraps Amadeus Self-Service as a search API when credentials exist.
type AmadeusSearchProvider struct {
	client *AmadeusClient
}

func NewAmadeusSearchProvider(c *AmadeusClient) *AmadeusSearchProvider {
	return &AmadeusSearchProvider{client: c}
}

func (p *AmadeusSearchProvider) Code() string { return "AMA" }
func (p *AmadeusSearchProvider) Name() string { return "Amadeus Search API" }
func (p *AmadeusSearchProvider) Kind() string { return "search" }

func (p *AmadeusSearchProvider) Fetch(ctx context.Context, origin, dest, departDate string, returnDate *string, cabin string) ([]Offer, error) {
	if p.client == nil {
		return nil, fmt.Errorf("amadeus not configured")
	}
	// Reuse Best-per-carrier over major US carriers from a single Amadeus search when possible.
	offers := make([]Offer, 0, 8)
	for _, code := range []string{"DL", "UA", "AA", "B6", "AS", "WN"} {
		off, ok, err := p.client.SearchCarrier(ctx, code, origin, dest, departDate, cabin)
		if err != nil || !ok {
			continue
		}
		off.Source = "search:amadeus"
		offers = append(offers, off)
	}
	if len(offers) == 0 {
		return nil, fmt.Errorf("amadeus: no offers")
	}
	return offers, nil
}
