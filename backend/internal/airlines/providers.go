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

var aircraftRangesKm = map[string]float64{
	"A220":        6150,
	"A220-300":    6150,
	"A319":        6950,
	"A320":        6100,
	"A320neo":     6500,
	"A321":        5950,
	"A321neo":     7400,
	"A330-200":    13450,
	"A330-300":    11750,
	"A330-900neo": 13330,
	"A340-600":    13900,
	"A350-900":    15000,
	"A350-1000":   16000,
	"A380-800":    15200,
	"B717":        3800,
	"B737-700":    5600,
	"B737-800":    5660,
	"B737-900":    5900,
	"B737 MAX 8":  6570,
	"B737 MAX 9":  6550,
	"B747-8":      14800,
	"B757-200":    7220,
	"B777-200":    9700,
	"B777-200ER":  14300,
	"B777-300ER":  13650,
	"B787-8":      13620,
	"B787-9":      14140,
	"B787-10":     11900,
	"E175":        3700,
	"E190":        4450,
	"E195-E2":     5300,
}

// Offer is a normalized full flight offer from an airline provider API.
type Offer struct {
	OfferID         string    `json:"offerId"`
	Airline         string    `json:"airline"`
	AirlineCode     string    `json:"airlineCode"`
	FlightNumber    string    `json:"flightNumber"`
	Origin          string    `json:"origin"`
	OriginCity      string    `json:"originCity"`
	Destination     string    `json:"destination"`
	DestinationCity string    `json:"destinationCity"`
	DepartAt        time.Time `json:"departAt"`
	ArriveAt        time.Time `json:"arriveAt"`
	DurationMinutes int       `json:"durationMinutes"`
	Stops           int       `json:"stops"`
	Cabin           string    `json:"cabin"`
	Aircraft        string    `json:"aircraft"`
	Price           float64   `json:"price"`
	Currency        string    `json:"currency"`
	DeepLink        string    `json:"deepLink"`
	Source          string    `json:"source"`
	Segments        []Segment `json:"segments"`
	LayoverAirports []string  `json:"layoverAirports"`
}

// Segment is one flight leg inside an offer (used for layover display).
type Segment struct {
	AirlineCode     string    `json:"airlineCode"`
	Airline         string    `json:"airline"`
	FlightNumber    string    `json:"flightNumber"`
	Origin          string    `json:"origin"`
	OriginCity      string    `json:"originCity"`
	Destination     string    `json:"destination"`
	DestinationCity string    `json:"destinationCity"`
	DepartAt        time.Time `json:"departAt"`
	ArriveAt        time.Time `json:"arriveAt"`
	DurationMinutes int       `json:"durationMinutes"`
	Aircraft        string    `json:"aircraft"`
}

// Provider is one pricing API adapter polled by the worker pool.
// Airline adapters return one (or few) offers; search adapters may return many.
type Provider interface {
	Code() string
	Name() string
	Kind() string // "airline" or "search"
	Fetch(ctx context.Context, origin, dest, departDate string, returnDate *string, cabin string) ([]Offer, error)
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
func (m *airlineProvider) Kind() string { return "airline" }

func (m *airlineProvider) Fetch(ctx context.Context, origin, dest, departDate string, returnDate *string, cabin string) ([]Offer, error) {
	off, err := m.fetchOne(ctx, origin, dest, departDate, returnDate, cabin)
	if err != nil {
		return nil, err
	}
	return []Offer{off}, nil
}

func (m *airlineProvider) fetchOne(ctx context.Context, origin, dest, departDate string, returnDate *string, cabin string) (Offer, error) {
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
	if !routeIsFeasible(distanceKm, m.aircraft) {
		return Offer{}, fmt.Errorf("route %s-%s exceeds %s fleet range", origin, dest, m.code)
	}
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

	ac := pickAircraftForDistance(distanceKm, m.aircraft, r)
	if ac == "" {
		return Offer{}, fmt.Errorf("route %s-%s exceeds %s fleet range", origin, dest, m.code)
	}
	link := googleFlightsURL(origin, dest, departDate, returnDate)
	flightLabel := fmt.Sprintf("%s %d", m.code, flightNum)
	segments := []Segment{{
		AirlineCode:     m.code,
		Airline:         m.name,
		FlightNumber:    flightLabel,
		Origin:          origin,
		OriginCity:      from.City,
		Destination:     dest,
		DestinationCity: to.City,
		DepartAt:        departAt,
		ArriveAt:        arriveAt,
		DurationMinutes: blockMin,
		Aircraft:        ac,
	}}
	var layovers []string
	if stops > 0 {
		hub := layoverHub(origin, dest)
		hubCity := LookupAirport(hub).City
		leg1 := blockMin / 2
		midArrive := departAt.Add(time.Duration(leg1) * time.Minute)
		midDepart := midArrive.Add(55 * time.Minute)
		segments = []Segment{
			{
				AirlineCode: m.code, Airline: m.name, FlightNumber: flightLabel,
				Origin: origin, OriginCity: from.City, Destination: hub, DestinationCity: hubCity,
				DepartAt: departAt, ArriveAt: midArrive, DurationMinutes: leg1, Aircraft: ac,
			},
			{
				AirlineCode: m.code, Airline: m.name, FlightNumber: fmt.Sprintf("%s %d", m.code, flightNum+1),
				Origin: hub, OriginCity: hubCity, Destination: dest, DestinationCity: to.City,
				DepartAt: midDepart, ArriveAt: arriveAt, DurationMinutes: blockMin - leg1, Aircraft: ac,
			},
		}
		layovers = []string{hub}
	}

	return Offer{
		OfferID:         fmt.Sprintf("sim-%s-%s-%s-%d", m.code, origin, dest, flightNum),
		Airline:         m.name,
		AirlineCode:     m.code,
		FlightNumber:    flightLabel,
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
		Source:          "airline:" + m.code,
		Segments:        segments,
		LayoverAirports: layovers,
	}, nil
}

func routeIsFeasible(distanceKm float64, aircraft []string) bool {
	for _, model := range aircraft {
		if max, ok := aircraftRangesKm[model]; ok && max >= distanceKm*1.02 {
			return true
		}
	}
	return false
}

func plausibleItineraryPrice(distanceKm float64, stops int, cabin string, price float64) bool {
	if price <= 0 {
		return false
	}
	if stops <= 0 || distanceKm < 2500 {
		return true
	}
	minPrice := 48.0 + distanceKm/42.0 + float64(stops)*35.0
	switch strings.ToLower(strings.TrimSpace(cabin)) {
	case "premium_economy":
		minPrice *= 1.25
	case "business":
		minPrice *= 1.9
	case "first":
		minPrice *= 2.8
	}
	return price >= minPrice
}

func pickAircraftForDistance(distanceKm float64, aircraft []string, r *rand.Rand) string {
	choices := make([]string, 0, len(aircraft))
	for _, model := range aircraft {
		max, ok := aircraftRangesKm[model]
		if !ok {
			continue
		}
		if max >= distanceKm*1.02 {
			choices = append(choices, model)
		}
	}
	if len(choices) == 0 {
		return ""
	}
	return choices[r.Intn(len(choices))]
}

func layoverHub(origin, dest string) string {
	hubs := []string{"ORD", "ATL", "DFW", "DEN", "CLT"}
	seed := hashSeed(origin, dest, "hub")
	for _, h := range hubs {
		if h != origin && h != dest {
			if seed%int64(len(hubs)) == 0 {
				return h
			}
			seed++
		}
	}
	if origin != "ORD" && dest != "ORD" {
		return "ORD"
	}
	return "ATL"
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

func airlineDefs() []airlineProvider {
	return []airlineProvider{
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
		{code: "AF", name: "Air France", baseFare: 470, volatility: 0.17, latencyMs: 105, aircraft: []string{"A350-900", "B777-300ER", "A220-300", "A320"}, hubBias: map[string]float64{"CDG": 0.87}},
		{code: "KL", name: "KLM Royal Dutch Airlines", baseFare: 465, volatility: 0.17, latencyMs: 105, aircraft: []string{"B787-10", "B777-200ER", "A330-300", "E195-E2"}, hubBias: map[string]float64{"AMS": 0.87}},
		{code: "EK", name: "Emirates", baseFare: 560, volatility: 0.16, latencyMs: 115, aircraft: []string{"A380-800", "B777-300ER", "A350-900"}, hubBias: map[string]float64{"DXB": 0.84}},
		{code: "QR", name: "Qatar Airways", baseFare: 550, volatility: 0.16, latencyMs: 115, aircraft: []string{"A350-1000", "B777-300ER", "B787-9", "A380-800"}, hubBias: map[string]float64{"DOH": 0.84}},
		{code: "EY", name: "Etihad Airways", baseFare: 535, volatility: 0.16, latencyMs: 110, aircraft: []string{"A350-1000", "B787-9", "B777-300ER", "A320"}, hubBias: map[string]float64{"AUH": 0.85}},
		{code: "SQ", name: "Singapore Airlines", baseFare: 590, volatility: 0.15, latencyMs: 120, aircraft: []string{"A350-900", "B787-10", "B777-300ER", "A380-800"}, hubBias: map[string]float64{"SIN": 0.84}},
		{code: "CX", name: "Cathay Pacific", baseFare: 555, volatility: 0.16, latencyMs: 115, aircraft: []string{"A350-1000", "B777-300ER", "A330-300"}, hubBias: map[string]float64{"HKG": 0.85}},
		{code: "QF", name: "Qantas", baseFare: 610, volatility: 0.15, latencyMs: 120, aircraft: []string{"A380-800", "B787-9", "A330-300", "B737-800"}, hubBias: map[string]float64{"SYD": 0.84, "MEL": 0.88}},
		{code: "NZ", name: "Air New Zealand", baseFare: 590, volatility: 0.15, latencyMs: 120, aircraft: []string{"B787-9", "B777-300ER", "A321neo"}, hubBias: map[string]float64{"AKL": 0.84}},
		{code: "NH", name: "All Nippon Airways", baseFare: 540, volatility: 0.14, latencyMs: 110, aircraft: []string{"B787-9", "B777-300ER", "A380-800", "A321neo"}, hubBias: map[string]float64{"HND": 0.85, "NRT": 0.88}},
		{code: "JL", name: "Japan Airlines", baseFare: 535, volatility: 0.14, latencyMs: 110, aircraft: []string{"A350-1000", "B787-9", "B777-300ER", "B737-800"}, hubBias: map[string]float64{"HND": 0.85, "NRT": 0.88}},
		{code: "KE", name: "Korean Air", baseFare: 525, volatility: 0.16, latencyMs: 110, aircraft: []string{"B787-9", "B777-300ER", "A380-800", "A330-300"}, hubBias: map[string]float64{"ICN": 0.85}},
		{code: "BR", name: "EVA Air", baseFare: 515, volatility: 0.16, latencyMs: 110, aircraft: []string{"B787-10", "B777-300ER", "A330-300"}, hubBias: map[string]float64{"TPE": 0.85}},
		{code: "TK", name: "Turkish Airlines", baseFare: 445, volatility: 0.18, latencyMs: 105, aircraft: []string{"A350-900", "B787-9", "B777-300ER", "A321neo"}, hubBias: map[string]float64{"IST": 0.84}},
		{code: "VS", name: "Virgin Atlantic", baseFare: 475, volatility: 0.18, latencyMs: 105, aircraft: []string{"A350-1000", "B787-9", "A330-900neo"}, hubBias: map[string]float64{"LHR": 0.87}},
		{code: "IB", name: "Iberia", baseFare: 430, volatility: 0.18, latencyMs: 100, aircraft: []string{"A350-900", "A330-300", "A321neo", "A320neo"}, hubBias: map[string]float64{"MAD": 0.86}},
		{code: "LX", name: "SWISS", baseFare: 485, volatility: 0.16, latencyMs: 105, aircraft: []string{"A330-300", "B777-300ER", "A220-300", "A320neo"}, hubBias: map[string]float64{"ZRH": 0.86}},
		{code: "AI", name: "Air India", baseFare: 430, volatility: 0.20, latencyMs: 110, aircraft: []string{"A350-900", "B787-8", "B777-300ER", "A320neo"}, hubBias: map[string]float64{"DEL": 0.84, "BOM": 0.88}},
		{code: "SV", name: "Saudia", baseFare: 455, volatility: 0.18, latencyMs: 110, aircraft: []string{"B787-10", "B777-300ER", "A330-300", "A320neo"}, hubBias: map[string]float64{"JED": 0.85, "RUH": 0.88}},
		{code: "ET", name: "Ethiopian Airlines", baseFare: 420, volatility: 0.19, latencyMs: 110, aircraft: []string{"A350-900", "B787-9", "B777-300ER", "B737 MAX 8"}, hubBias: map[string]float64{"ADD": 0.84}},
	}
}

// AirlineProviderCount returns the number of per-carrier adapters in the registry.
func AirlineProviderCount() int { return len(airlineDefs()) }

// All returns global airline pricing adapters (best-effort HTTP + simulator fallback).
func All() []Provider {
	defs := airlineDefs()
	out := make([]Provider, len(defs))
	for i := range defs {
		out[i] = NewAirlineAPI(defs[i])
	}
	return out
}

// AllProviders returns global airline adapters plus configured search APIs.
func AllProviders(ignav *IgnavClient, amadeus *AmadeusClient, extras ...Provider) []Provider {
	base := All()
	out := make([]Provider, 0, len(base)+3+len(extras))
	for _, provider := range base {
		wrapped := Provider(provider)
		if amadeus != nil {
			wrapped = &AmadeusBackedProvider{inner: wrapped, amadeus: amadeus}
		}
		if ignav != nil {
			wrapped = &IgnavBackedProvider{inner: wrapped, ignav: ignav}
		}
		out = append(out, wrapped)
	}
	if ignav != nil {
		out = append(out, NewIgnavSearchProvider(ignav))
	}
	out = append(out, NewGoogleFlightsSearchProvider())
	if amadeus != nil {
		out = append(out, NewAmadeusSearchProvider(amadeus))
	}
	out = append(out, extras...)
	return out
}
