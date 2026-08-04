package airlines

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"
	"math/rand"
	"strings"
	"time"
)

// Offer is a normalized full flight offer from an airline provider API.
type Offer struct {
	Airline         string
	AirlineCode     string
	FlightNumber    string
	Origin          string
	OriginCity      string
	Destination     string
	DestinationCity string
	DepartAt        time.Time
	ArriveAt        time.Time
	DurationMinutes int
	Stops           int
	Cabin           string
	Aircraft        string
	Price           float64
	Currency        string
	DeepLink        string
	Source          string
}

// Provider is one airline pricing API adapter (12 carriers = 12 APIs in the worker pool).
type Provider interface {
	Code() string
	Name() string
	Fetch(ctx context.Context, origin, dest, departDate string, returnDate *string, cabin string) (Offer, error)
}

type airlineProvider struct {
	code       string
	name       string
	baseFare   float64
	volatility float64
	latencyMs  int
	aircraft   []string
	hubBias    map[string]float64
}

func (m *airlineProvider) Code() string { return m.code }
func (m *airlineProvider) Name() string { return m.name }

func (m *airlineProvider) Fetch(ctx context.Context, origin, dest, departDate string, returnDate *string, cabin string) (Offer, error) {
	select {
	case <-ctx.Done():
		return Offer{}, ctx.Err()
	case <-time.After(time.Duration(m.latencyMs+rand.Intn(35)) * time.Millisecond):
	}

	if cabin == "" {
		cabin = "economy"
	}
	origin = strings.ToUpper(origin)
	dest = strings.ToUpper(dest)
	from := LookupAirport(origin)
	to := LookupAirport(dest)

	seed := hashSeed(m.code, origin, dest, departDate, cabin)
	// Price drifts slowly (~every 5 minutes) so scans can detect drops.
	r := rand.New(rand.NewSource(seed + time.Now().Unix()/300))

	distanceKm := haversineKm(from.Lat, from.Lon, to.Lat, to.Lon)
	blockMin := int(distanceKm/12.5) + 45 // ~750km/h cruise + taxi
	if blockMin < 75 {
		blockMin = 75
	}

	stops := 0
	if distanceKm > 5500 && r.Float64() < 0.35 {
		stops = 1
		blockMin += 75 + r.Intn(90)
	} else if distanceKm > 2500 && r.Float64() < 0.18 {
		stops = 1
		blockMin += 55 + r.Intn(60)
	}

	departHour := 6 + r.Intn(14)
	departMin := []int{0, 5, 10, 15, 20, 25, 30, 35, 40, 45, 50, 55}[r.Intn(12)]
	departDay, err := time.Parse("2006-01-02", departDate)
	if err != nil {
		return Offer{}, fmt.Errorf("invalid depart date: %w", err)
	}
	departAt := time.Date(departDay.Year(), departDay.Month(), departDay.Day(), departHour, departMin, 0, 0, time.UTC)
	arriveAt := departAt.Add(time.Duration(blockMin) * time.Minute)

	flightNum := 100 + int(seed%800)
	if flightNum < 100 {
		flightNum = 100 + r.Intn(700)
	}

	base := m.baseFare * (0.55 + distanceKm/4500)
	if bias, ok := m.hubBias[origin]; ok {
		base *= bias
	}
	if bias, ok := m.hubBias[dest]; ok {
		base *= bias
	}
	noise := (r.Float64()*2 - 1) * m.volatility
	price := math.Round(base*(1+noise)*100) / 100
	if returnDate != nil && *returnDate != "" {
		price = math.Round(price*1.78*100) / 100
	}
	switch strings.ToLower(cabin) {
	case "premium_economy":
		price *= 1.45
	case "business":
		price *= 2.8
	case "first":
		price *= 4.2
	}
	if price < 59 {
		price = 59
	}

	ac := m.aircraft[r.Intn(len(m.aircraft))]
	link := fmt.Sprintf("https://www.%s.com/booking?from=%s&to=%s&date=%s&flight=%s%d",
		bookingHost(m.code), origin, dest, departDate, m.code, flightNum)

	return Offer{
		Airline:         m.name,
		AirlineCode:     m.code,
		FlightNumber:    fmt.Sprintf("%s %d", m.code, flightNum),
		Origin:          origin,
		OriginCity:      from.City,
		Destination:     dest,
		DestinationCity: to.City,
		DepartAt:        departAt,
		ArriveAt:        arriveAt,
		DurationMinutes: blockMin,
		Stops:           stops,
		Cabin:           cabin,
		Aircraft:        ac,
		Price:           price,
		Currency:        "USD",
		DeepLink:        link,
		Source:          "airline-api:" + m.code,
	}, nil
}

func bookingHost(code string) string {
	hosts := map[string]string{
		"DL": "delta", "UA": "united", "AA": "aa", "B6": "jetblue",
		"WN": "southwest", "AS": "alaskaair", "NK": "spirit", "F9": "flyfrontier",
		"HA": "hawaiianairlines", "AC": "aircanada", "BA": "britishairways", "LH": "lufthansa",
	}
	if h, ok := hosts[code]; ok {
		return h
	}
	return "flights.example"
}

func haversineKm(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371.0
	toRad := func(d float64) float64 { return d * math.Pi / 180 }
	dLat := toRad(lat2 - lat1)
	dLon := toRad(lon2 - lon1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * R * math.Asin(math.Sqrt(a))
}

func hashSeed(parts ...string) int64 {
	h := fnv.New64a()
	for _, p := range parts {
		_, _ = h.Write([]byte(p))
		_, _ = h.Write([]byte("|"))
	}
	return int64(h.Sum64())
}

// All returns 12 airline API providers polled by the worker pool.
func All() []Provider {
	defs := []airlineProvider{
		{code: "DL", name: "Delta Air Lines", baseFare: 280, volatility: 0.16, latencyMs: 80, aircraft: []string{"A321", "B737-900", "A330-300", "B757-200"}, hubBias: map[string]float64{"ATL": 0.9, "JFK": 0.95}},
		{code: "UA", name: "United Airlines", baseFare: 275, volatility: 0.17, latencyMs: 90, aircraft: []string{"B737-800", "B777-300ER", "B787-9", "A320"}, hubBias: map[string]float64{"ORD": 0.92, "SFO": 0.94, "EWR": 0.93}},
		{code: "AA", name: "American Airlines", baseFare: 270, volatility: 0.15, latencyMs: 85, aircraft: []string{"A321", "B737-800", "B777-200", "A319"}, hubBias: map[string]float64{"DFW": 0.9, "MIA": 0.93, "JFK": 0.96}},
		{code: "B6", name: "JetBlue", baseFare: 210, volatility: 0.20, latencyMs: 70, aircraft: []string{"A320", "A321neo", "E190"}, hubBias: map[string]float64{"JFK": 0.88, "BOS": 0.9}},
		{code: "WN", name: "Southwest Airlines", baseFare: 190, volatility: 0.14, latencyMs: 60, aircraft: []string{"B737-700", "B737-800", "B737 MAX 8"}, hubBias: map[string]float64{"DEN": 0.9, "LAX": 0.95}},
		{code: "AS", name: "Alaska Airlines", baseFare: 245, volatility: 0.15, latencyMs: 75, aircraft: []string{"B737-900", "B737 MAX 9", "E175"}, hubBias: map[string]float64{"SEA": 0.88, "SFO": 0.94}},
		{code: "NK", name: "Spirit Airlines", baseFare: 120, volatility: 0.26, latencyMs: 55, aircraft: []string{"A320neo", "A321neo"}, hubBias: map[string]float64{"FLL": 0.9, "MIA": 0.92}},
		{code: "F9", name: "Frontier Airlines", baseFare: 115, volatility: 0.28, latencyMs: 50, aircraft: []string{"A320neo", "A321neo"}, hubBias: map[string]float64{"DEN": 0.88}},
		{code: "HA", name: "Hawaiian Airlines", baseFare: 360, volatility: 0.13, latencyMs: 100, aircraft: []string{"A330-200", "B717", "A321neo"}, hubBias: map[string]float64{"HNL": 0.85}},
		{code: "AC", name: "Air Canada", baseFare: 300, volatility: 0.17, latencyMs: 95, aircraft: []string{"A220", "B787-9", "A321", "B777-300ER"}, hubBias: map[string]float64{"YYZ": 0.9}},
		{code: "BA", name: "British Airways", baseFare: 480, volatility: 0.18, latencyMs: 110, aircraft: []string{"B777-300ER", "A350-1000", "B787-9", "A320"}, hubBias: map[string]float64{"LHR": 0.88}},
		{code: "LH", name: "Lufthansa", baseFare: 490, volatility: 0.17, latencyMs: 105, aircraft: []string{"A350-900", "B747-8", "A320neo", "A340-600"}, hubBias: map[string]float64{"FRA": 0.88}},
	}
	out := make([]Provider, len(defs))
	for i := range defs {
		p := defs[i]
		out[i] = &p
	}
	return out
}
