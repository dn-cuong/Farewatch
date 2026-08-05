package airlines

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// FreeSearchProviders returns only sources whose credentials are configured.
// Both services offer free developer access, but require the user to create a key.
func FreeSearchProviders(travelpayoutsToken, rapidAPIKey string) []Provider {
	var out []Provider
	if token := strings.TrimSpace(travelpayoutsToken); token != "" {
		out = append(out, NewTravelpayoutsProvider(token))
	}
	if key := strings.TrimSpace(rapidAPIKey); key != "" {
		out = append(out, NewSkyScrapperProvider(key))
	}
	return out
}

type sourceCacheEntry struct {
	offers []Offer
	until  time.Time
}

// TravelpayoutsProvider reads cached market fares from the free Flight Data API.
type TravelpayoutsProvider struct {
	token string
	http  *http.Client
	mu    sync.Mutex
	cache map[string]sourceCacheEntry
}

func NewTravelpayoutsProvider(token string) *TravelpayoutsProvider {
	return &TravelpayoutsProvider{
		token: strings.TrimSpace(token),
		http:  &http.Client{Timeout: 12 * time.Second},
		cache: make(map[string]sourceCacheEntry),
	}
}

func (p *TravelpayoutsProvider) Code() string { return "TP" }
func (p *TravelpayoutsProvider) Name() string { return "Travelpayouts Data API" }
func (p *TravelpayoutsProvider) Kind() string { return "search" }

type travelpayoutsResponse struct {
	Success  bool                `json:"success"`
	Currency string              `json:"currency"`
	Data     []travelpayoutsFare `json:"data"`
}

type travelpayoutsFare struct {
	Origin       string  `json:"origin"`
	Destination  string  `json:"destination"`
	DepartureAt  string  `json:"departure_at"`
	ReturnAt     string  `json:"return_at"`
	Airline      string  `json:"airline"`
	FlightNumber string  `json:"flight_number"`
	Price        float64 `json:"price"`
	Transfers    int     `json:"transfers"`
	Duration     int     `json:"duration"`
	Link         string  `json:"link"`
}

func (p *TravelpayoutsProvider) Fetch(
	ctx context.Context,
	origin, dest, departDate string,
	returnDate *string,
	cabin string,
) ([]Offer, error) {
	key := routeCacheKey(origin, dest, departDate, returnDate, cabin)
	if offers, ok := p.cached(key); ok {
		return offers, nil
	}

	q := url.Values{}
	q.Set("origin", strings.ToUpper(origin))
	q.Set("destination", strings.ToUpper(dest))
	q.Set("departure_at", monthOf(departDate))
	q.Set("one_way", strconv.FormatBool(returnDate == nil || strings.TrimSpace(*returnDate) == ""))
	q.Set("direct", "false")
	q.Set("sorting", "price")
	q.Set("limit", "30")
	q.Set("page", "1")
	q.Set("currency", "usd")
	q.Set("token", p.token)

	endpoint := "https://api.travelpayouts.com/aviasales/v3/prices_for_dates?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Access-Token", p.token)

	res, err := p.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("travelpayouts %d: %s", res.StatusCode, truncate(string(body), 240))
	}

	var parsed travelpayoutsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("travelpayouts decode: %w", err)
	}
	offers := travelpayoutsOffers(parsed, origin, dest, departDate, returnDate, cabin)
	if len(offers) == 0 {
		return nil, fmt.Errorf("travelpayouts: no matching fares")
	}
	p.store(key, offers, 30*time.Minute)
	return offers, nil
}

func travelpayoutsOffers(
	resp travelpayoutsResponse,
	origin, dest, departDate string,
	returnDate *string,
	cabin string,
) []Offer {
	from := LookupAirport(strings.ToUpper(origin))
	to := LookupAirport(strings.ToUpper(dest))
	distanceKm := haversineKm(from.Lat, from.Lon, to.Lat, to.Lon)
	currency := strings.ToUpper(resp.Currency)
	if currency == "" {
		currency = "USD"
	}
	out := make([]Offer, 0, len(resp.Data))
	for i, fare := range resp.Data {
		departAt := parseProviderTime(fare.DepartureAt)
		if departAt.IsZero() || departAt.UTC().Format("2006-01-02") != departDate || fare.Price <= 0 {
			continue
		}
		if !plausibleItineraryPrice(distanceKm, fare.Transfers, cabin, fare.Price) {
			continue
		}
		arriveAt := departAt.Add(time.Duration(fare.Duration) * time.Minute)
		if fare.Duration <= 0 {
			arriveAt = departAt
		}
		code := strings.ToUpper(strings.TrimSpace(fare.Airline))
		flight := normalizeFlightNumber(code, fare.FlightNumber)
		if flight == "" {
			flight = fmt.Sprintf("%s TP%d", code, i+1)
		}
		deepLink := strings.TrimSpace(fare.Link)
		if deepLink != "" && !strings.HasPrefix(deepLink, "http") {
			deepLink = "https://www.aviasales.com" + deepLink
		}
		if deepLink == "" {
			deepLink = googleFlightsURL(origin, dest, departDate, returnDate)
		}
		out = append(out, Offer{
			OfferID:         fmt.Sprintf("tp-%s-%s-%d", code, flight, departAt.Unix()),
			Airline:         airlineName(code),
			AirlineCode:     code,
			FlightNumber:    flight,
			Origin:          strings.ToUpper(origin),
			OriginCity:      from.City,
			Destination:     strings.ToUpper(dest),
			DestinationCity: to.City,
			DepartAt:        departAt,
			ArriveAt:        arriveAt,
			DurationMinutes: fare.Duration,
			Stops:           fare.Transfers,
			Cabin:           normalizedCabin(cabin),
			Price:           fare.Price,
			Currency:        currency,
			DeepLink:        deepLink,
			Source:          "search:travelpayouts",
		})
	}
	return out
}

func (p *TravelpayoutsProvider) cached(key string) ([]Offer, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	entry, ok := p.cache[key]
	if !ok || time.Now().After(entry.until) {
		delete(p.cache, key)
		return nil, false
	}
	return cloneOffers(entry.offers), true
}

func (p *TravelpayoutsProvider) store(key string, offers []Offer, ttl time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cache[key] = sourceCacheEntry{offers: cloneOffers(offers), until: time.Now().Add(ttl)}
}

// SkyScrapperProvider uses Sky Scrapper on RapidAPI. Its Basic plan is suitable
// for demos; the long cache protects the small free monthly quota.
type SkyScrapperProvider struct {
	apiKey       string
	http         *http.Client
	mu           sync.Mutex
	cache        map[string]sourceCacheEntry
	airportCache map[string]skyAirport
}

func NewSkyScrapperProvider(apiKey string) *SkyScrapperProvider {
	return &SkyScrapperProvider{
		apiKey:       strings.TrimSpace(apiKey),
		http:         &http.Client{Timeout: 20 * time.Second},
		cache:        make(map[string]sourceCacheEntry),
		airportCache: make(map[string]skyAirport),
	}
}

func (p *SkyScrapperProvider) Code() string { return "SKY" }
func (p *SkyScrapperProvider) Name() string { return "Sky Scrapper API" }
func (p *SkyScrapperProvider) Kind() string { return "search" }

type skyAirport struct {
	SkyID    string `json:"skyId"`
	EntityID string `json:"entityId"`
}

type skyAirportResponse struct {
	Data []skyAirport `json:"data"`
}

type skySearchResponse struct {
	Data struct {
		Itineraries []skyItinerary `json:"itineraries"`
	} `json:"data"`
}

type skyItinerary struct {
	ID    string `json:"id"`
	Price struct {
		Raw       float64 `json:"raw"`
		Formatted string  `json:"formatted"`
	} `json:"price"`
	Legs []skyLeg `json:"legs"`
}

type skyLeg struct {
	Origin struct {
		DisplayCode string `json:"displayCode"`
		City        string `json:"city"`
	} `json:"origin"`
	Destination struct {
		DisplayCode string `json:"displayCode"`
		City        string `json:"city"`
	} `json:"destination"`
	Departure        string `json:"departure"`
	Arrival          string `json:"arrival"`
	DurationInMinute int    `json:"durationInMinutes"`
	StopCount        int    `json:"stopCount"`
	Carriers         struct {
		Marketing []struct {
			Name        string `json:"name"`
			AlternateID string `json:"alternateId"`
		} `json:"marketing"`
	} `json:"carriers"`
	Segments []struct {
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
	} `json:"segments"`
}

func (p *SkyScrapperProvider) Fetch(
	ctx context.Context,
	origin, dest, departDate string,
	returnDate *string,
	cabin string,
) ([]Offer, error) {
	key := routeCacheKey(origin, dest, departDate, returnDate, cabin)
	if offers, ok := p.cached(key); ok {
		return offers, nil
	}
	from, err := p.lookupAirport(ctx, origin)
	if err != nil {
		return nil, fmt.Errorf("sky airport %s: %w", origin, err)
	}
	to, err := p.lookupAirport(ctx, dest)
	if err != nil {
		return nil, fmt.Errorf("sky airport %s: %w", dest, err)
	}

	q := url.Values{}
	q.Set("originSkyId", from.SkyID)
	q.Set("destinationSkyId", to.SkyID)
	q.Set("originEntityId", from.EntityID)
	q.Set("destinationEntityId", to.EntityID)
	q.Set("date", departDate)
	if returnDate != nil && *returnDate != "" {
		q.Set("returnDate", *returnDate)
	}
	q.Set("cabinClass", skyCabin(cabin))
	q.Set("adults", "1")
	q.Set("sortBy", "best")
	q.Set("currency", "USD")
	q.Set("market", "en-US")
	q.Set("countryCode", "US")

	var parsed skySearchResponse
	if err := p.getJSON(ctx, "/api/v2/flights/searchFlights?"+q.Encode(), &parsed); err != nil {
		// Some RapidAPI subscriptions expose the v1 route instead.
		if errV1 := p.getJSON(ctx, "/api/v1/flights/searchFlights?"+q.Encode(), &parsed); errV1 != nil {
			return nil, fmt.Errorf("sky search: %v; v1: %v", err, errV1)
		}
	}
	offers := skyOffers(parsed, origin, dest, departDate, returnDate, cabin)
	if len(offers) == 0 {
		return nil, fmt.Errorf("sky scrapper: no itineraries")
	}
	p.store(key, offers, 2*time.Hour)
	return offers, nil
}

func (p *SkyScrapperProvider) lookupAirport(ctx context.Context, code string) (skyAirport, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	p.mu.Lock()
	if airport, ok := p.airportCache[code]; ok {
		p.mu.Unlock()
		return airport, nil
	}
	p.mu.Unlock()

	var parsed skyAirportResponse
	q := url.Values{"query": {code}, "locale": {"en-US"}}
	if err := p.getJSON(ctx, "/api/v1/flights/searchAirport?"+q.Encode(), &parsed); err != nil {
		return skyAirport{}, err
	}
	if len(parsed.Data) == 0 {
		return skyAirport{}, fmt.Errorf("not found")
	}
	airport := parsed.Data[0]
	p.mu.Lock()
	p.airportCache[code] = airport
	p.mu.Unlock()
	return airport, nil
}

func (p *SkyScrapperProvider) getJSON(ctx context.Context, path string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://sky-scrapper.p.rapidapi.com"+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-RapidAPI-Key", p.apiKey)
	req.Header.Set("X-RapidAPI-Host", "sky-scrapper.p.rapidapi.com")
	req.Header.Set("Accept", "application/json")
	res, err := p.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if res.StatusCode >= 300 {
		return fmt.Errorf("http %d: %s", res.StatusCode, truncate(string(body), 240))
	}
	if err := json.Unmarshal(body, dst); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	return nil
}

func skyOffers(
	resp skySearchResponse,
	origin, dest, departDate string,
	returnDate *string,
	cabin string,
) []Offer {
	from := LookupAirport(strings.ToUpper(origin))
	to := LookupAirport(strings.ToUpper(dest))
	distanceKm := haversineKm(from.Lat, from.Lon, to.Lat, to.Lon)
	out := make([]Offer, 0, len(resp.Data.Itineraries))
	for _, itinerary := range resp.Data.Itineraries {
		if len(itinerary.Legs) == 0 || itinerary.Price.Raw <= 0 {
			continue
		}
		leg := itinerary.Legs[0]
		departAt := parseProviderTime(leg.Departure)
		arriveAt := parseProviderTime(leg.Arrival)
		if departAt.IsZero() || departAt.UTC().Format("2006-01-02") != departDate {
			continue
		}
		if !plausibleItineraryPrice(distanceKm, leg.StopCount, cabin, itinerary.Price.Raw) {
			continue
		}
		code, airline := "", ""
		if len(leg.Carriers.Marketing) > 0 {
			code = strings.ToUpper(leg.Carriers.Marketing[0].AlternateID)
			airline = leg.Carriers.Marketing[0].Name
		}
		flight := ""
		if len(leg.Segments) > 0 {
			if code == "" {
				code = strings.ToUpper(leg.Segments[0].MarketingCarrier.AlternateID)
			}
			if airline == "" {
				airline = leg.Segments[0].MarketingCarrier.Name
			}
			flight = normalizeFlightNumber(code, leg.Segments[0].FlightNumber)
		}
		if flight == "" {
			flight = code + " SKY"
		}
		if airline == "" {
			airline = airlineName(code)
		}
		out = append(out, Offer{
			OfferID:         "sky-" + itinerary.ID,
			Airline:         airline,
			AirlineCode:     code,
			FlightNumber:    flight,
			Origin:          strings.ToUpper(origin),
			OriginCity:      chooseNonEmpty(leg.Origin.City, from.City),
			Destination:     strings.ToUpper(dest),
			DestinationCity: chooseNonEmpty(leg.Destination.City, to.City),
			DepartAt:        departAt,
			ArriveAt:        arriveAt,
			DurationMinutes: leg.DurationInMinute,
			Stops:           leg.StopCount,
			Cabin:           normalizedCabin(cabin),
			Price:           itinerary.Price.Raw,
			Currency:        "USD",
			DeepLink:        googleFlightsURL(origin, dest, departDate, returnDate),
			Source:          "search:sky_scrapper",
			Segments:        skySegments(leg),
		})
	}
	return out
}

func skySegments(leg skyLeg) []Segment {
	out := make([]Segment, 0, len(leg.Segments))
	for _, item := range leg.Segments {
		departAt := parseProviderTime(item.Departure)
		arriveAt := parseProviderTime(item.Arrival)
		duration := int(arriveAt.Sub(departAt).Minutes())
		if duration < 0 {
			duration = 0
		}
		code := strings.ToUpper(item.MarketingCarrier.AlternateID)
		out = append(out, Segment{
			AirlineCode:     code,
			Airline:         item.MarketingCarrier.Name,
			FlightNumber:    normalizeFlightNumber(code, item.FlightNumber),
			Origin:          item.Origin.DisplayCode,
			OriginCity:      item.Origin.Name,
			Destination:     item.Destination.DisplayCode,
			DestinationCity: item.Destination.Name,
			DepartAt:        departAt,
			ArriveAt:        arriveAt,
			DurationMinutes: duration,
		})
	}
	return out
}

func (p *SkyScrapperProvider) cached(key string) ([]Offer, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	entry, ok := p.cache[key]
	if !ok || time.Now().After(entry.until) {
		delete(p.cache, key)
		return nil, false
	}
	return cloneOffers(entry.offers), true
}

func (p *SkyScrapperProvider) store(key string, offers []Offer, ttl time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cache[key] = sourceCacheEntry{offers: cloneOffers(offers), until: time.Now().Add(ttl)}
}

func routeCacheKey(origin, dest, depart string, returnDate *string, cabin string) string {
	ret := ""
	if returnDate != nil {
		ret = *returnDate
	}
	return strings.Join([]string{
		strings.ToUpper(origin),
		strings.ToUpper(dest),
		depart,
		ret,
		normalizedCabin(cabin),
	}, "|")
}

func monthOf(date string) string {
	if len(date) >= 7 {
		return date[:7]
	}
	return date
}

func normalizedCabin(cabin string) string {
	cabin = strings.ToLower(strings.TrimSpace(cabin))
	if cabin == "" {
		return "economy"
	}
	return cabin
}

func skyCabin(cabin string) string {
	switch normalizedCabin(cabin) {
	case "premium_economy":
		return "premium_economy"
	case "business":
		return "business"
	case "first":
		return "first"
	default:
		return "economy"
	}
}

func parseProviderTime(raw string) time.Time {
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func airlineName(code string) string {
	code = strings.ToUpper(code)
	for _, def := range airlineDefs() {
		if def.code == code {
			return def.name
		}
	}
	if code == "" {
		return "Unknown airline"
	}
	return code
}

func cloneOffers(in []Offer) []Offer {
	out := make([]Offer, len(in))
	copy(out, in)
	return out
}

func chooseNonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}
