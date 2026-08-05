package airlines

import (
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

// AmadeusClient optionally pulls live Flight Offers from Amadeus Self-Service.
// Set AMADEUS_CLIENT_ID / AMADEUS_CLIENT_SECRET to enable.
type AmadeusClient struct {
	baseURL      string
	clientID     string
	clientSecret string
	http         *http.Client

	mu          sync.Mutex
	accessToken string
	expiresAt   time.Time
}

func NewAmadeus(clientID, clientSecret, baseURL string) *AmadeusClient {
	if clientID == "" || clientSecret == "" {
		return nil
	}
	if baseURL == "" {
		baseURL = "https://test.api.amadeus.com"
	}
	return &AmadeusClient{
		baseURL:      strings.TrimRight(baseURL, "/"),
		clientID:     clientID,
		clientSecret: clientSecret,
		http:         &http.Client{Timeout: 20 * time.Second},
	}
}

func (a *AmadeusClient) Enabled() bool { return a != nil }

func (a *AmadeusClient) token(ctx context.Context) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.accessToken != "" && time.Now().Before(a.expiresAt) {
		return a.accessToken, nil
	}
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", a.clientID)
	form.Set("client_secret", a.clientSecret)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/v1/security/oauth2/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := a.http.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return "", fmt.Errorf("amadeus auth %d: %s", res.StatusCode, string(body))
	}
	var parsed struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	a.accessToken = parsed.AccessToken
	a.expiresAt = time.Now().Add(time.Duration(parsed.ExpiresIn-60) * time.Second)
	return a.accessToken, nil
}

type amadeusOfferResp struct {
	Data []struct {
		ID    string `json:"id"`
		Price struct {
			Total    string `json:"total"`
			Currency string `json:"currency"`
		} `json:"price"`
		Itineraries []struct {
			Duration string `json:"duration"`
			Segments []struct {
				Departure struct {
					IataCode string `json:"iataCode"`
					At       string `json:"at"`
				} `json:"departure"`
				Arrival struct {
					IataCode string `json:"iataCode"`
					At       string `json:"at"`
				} `json:"arrival"`
				CarrierCode string `json:"carrierCode"`
				Number      string `json:"number"`
				Aircraft    struct {
					Code string `json:"code"`
				} `json:"aircraft"`
			} `json:"segments"`
		} `json:"itineraries"`
		ValidatingAirlineCodes []string `json:"validatingAirlineCodes"`
	} `json:"data"`
	Dictionaries struct {
		Carriers map[string]string `json:"carriers"`
	} `json:"dictionaries"`
}

// SearchCarrier returns the cheapest Amadeus offer for a validating carrier, if any.
func (a *AmadeusClient) SearchCarrier(ctx context.Context, carrier, origin, dest, departDate, cabin string) (Offer, bool, error) {
	tok, err := a.token(ctx)
	if err != nil {
		return Offer{}, false, err
	}
	from := LookupAirport(strings.ToUpper(origin))
	to := LookupAirport(strings.ToUpper(dest))
	distanceKm := haversineKm(from.Lat, from.Lon, to.Lat, to.Lon)
	if cabin == "" {
		cabin = "ECONOMY"
	}
	q := url.Values{}
	q.Set("originLocationCode", strings.ToUpper(origin))
	q.Set("destinationLocationCode", strings.ToUpper(dest))
	q.Set("departureDate", departDate)
	q.Set("adults", "1")
	q.Set("currencyCode", "USD")
	q.Set("max", "15")
	q.Set("travelClass", strings.ToUpper(cabin))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+"/v2/shopping/flight-offers?"+q.Encode(), nil)
	if err != nil {
		return Offer{}, false, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	res, err := a.http.Do(req)
	if err != nil {
		return Offer{}, false, err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return Offer{}, false, fmt.Errorf("amadeus search %d: %s", res.StatusCode, string(body))
	}
	var parsed amadeusOfferResp
	if err := json.Unmarshal(body, &parsed); err != nil {
		return Offer{}, false, err
	}

	var best *Offer
	for _, d := range parsed.Data {
		code := ""
		if len(d.ValidatingAirlineCodes) > 0 {
			code = d.ValidatingAirlineCodes[0]
		}
		if code != carrier {
			continue
		}
		if len(d.Itineraries) == 0 || len(d.Itineraries[0].Segments) == 0 {
			continue
		}
		segs := d.Itineraries[0].Segments
		first, last := segs[0], segs[len(segs)-1]
		departAt := parseProviderTime(first.Departure.At)
		arriveAt := parseProviderTime(last.Arrival.At)
		var price float64
		_, _ = fmt.Sscanf(d.Price.Total, "%f", &price)
		if !plausibleItineraryPrice(distanceKm, len(segs)-1, cabin, price) {
			continue
		}
		name := parsed.Dictionaries.Carriers[code]
		if name == "" {
			name = code
		}
		off := Offer{
			Airline:         name,
			AirlineCode:     code,
			FlightNumber:    fmt.Sprintf("%s %s", first.CarrierCode, first.Number),
			Origin:          first.Departure.IataCode,
			OriginCity:      from.City,
			Destination:     last.Arrival.IataCode,
			DestinationCity: to.City,
			DepartAt:        departAt,
			ArriveAt:        arriveAt,
			DurationMinutes: int(arriveAt.Sub(departAt).Minutes()),
			Stops:           len(segs) - 1,
			Cabin:           strings.ToLower(cabin),
			Aircraft:        first.Aircraft.Code,
			Price:           price,
			Currency:        d.Price.Currency,
			DeepLink:        fmt.Sprintf("https://www.amadeus.com/offer/%s", d.ID),
			Source:          "amadeus",
		}
		if best == nil || off.Price < best.Price {
			cp := off
			best = &cp
		}
	}
	if best == nil {
		return Offer{}, false, nil
	}
	return *best, true, nil
}

// AmadeusBackedProvider tries Amadeus first, then falls back to the airline simulator.
type AmadeusBackedProvider struct {
	inner   Provider
	amadeus *AmadeusClient
}

func (p *AmadeusBackedProvider) Code() string { return p.inner.Code() }
func (p *AmadeusBackedProvider) Name() string { return p.inner.Name() }
func (p *AmadeusBackedProvider) Kind() string {
	if k, ok := p.inner.(interface{ Kind() string }); ok {
		return k.Kind()
	}
	return "airline"
}

func (p *AmadeusBackedProvider) Fetch(ctx context.Context, origin, dest, departDate string, returnDate *string, cabin string) ([]Offer, error) {
	if p.amadeus != nil {
		if off, ok, err := p.amadeus.SearchCarrier(ctx, p.Code(), origin, dest, departDate, cabin); err == nil && ok {
			off.Source = "search:amadeus"
			return []Offer{off}, nil
		}
	}
	return p.inner.Fetch(ctx, origin, dest, departDate, returnDate, cabin)
}

func AllWithAmadeus(client *AmadeusClient) []Provider {
	return AllProviders(nil, client)
}
