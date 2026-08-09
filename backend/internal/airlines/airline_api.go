package airlines

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// AirlineAPI tries the carrier HTTP search page, then falls back to the simulator.
type AirlineAPI struct {
	sim    *airlineProvider
	client *http.Client
}

func NewAirlineAPI(sim airlineProvider) *AirlineAPI {
	s := sim
	return &AirlineAPI{
		sim: &s,
		client: &http.Client{
			Timeout: 6 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 3 {
					return http.ErrUseLastResponse
				}
				return nil
			},
		},
	}
}

func (a *AirlineAPI) Code() string { return a.sim.Code() }
func (a *AirlineAPI) Name() string { return a.sim.Name() }
func (a *AirlineAPI) Kind() string { return "airline" }

func (a *AirlineAPI) Fetch(ctx context.Context, origin, dest, departDate string, returnDate *string, cabin string) ([]Offer, error) {
	if offers, err := a.fetchLive(ctx, origin, dest, departDate, returnDate, cabin); err == nil && len(offers) > 0 {
		return offers, nil
	}
	off, err := a.sim.fetchOne(ctx, origin, dest, departDate, returnDate, cabin)
	if err != nil {
		return nil, err
	}
	off.Source = "simulator:" + a.Code()
	return []Offer{off}, nil
}

func (a *AirlineAPI) fetchLive(ctx context.Context, origin, dest, departDate string, returnDate *string, cabin string) ([]Offer, error) {
	endpoint := airlineSearchURL(a.Code(), origin, dest, departDate, returnDate)
	if endpoint == "" {
		return nil, fmt.Errorf("no public endpoint for %s", a.Code())
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "FareWatch/1.0 (+https://github.com/dn-cuong/Farewatch; fare-tracker)")
	req.Header.Set("Accept", "text/html,application/json;q=0.9,*/*;q=0.8")

	res, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 512*1024))
	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("%s http %d", a.Code(), res.StatusCode)
	}

	if _, ok := extractPublicPrice(body); !ok {
		return nil, fmt.Errorf("%s: no parseable fare in response", a.Code())
	}

	// Price alone isn't enough to build a real itinerary.
	return nil, fmt.Errorf("%s: price-only response cannot be normalized safely", a.Code())
}

func airlineSearchURL(code, origin, dest, depart string, returnDate *string) string {
	origin = strings.ToUpper(origin)
	dest = strings.ToUpper(dest)
	ret := ""
	if returnDate != nil {
		ret = *returnDate
	}
	switch strings.ToUpper(code) {
	case "B6":
		u := fmt.Sprintf("https://www.jetblue.com/booking/flights?from=%s&to=%s&depart=%s", origin, dest, depart)
		if ret != "" {
			u += "&return=" + ret
		}
		return u
	case "WN":
		q := url.Values{}
		q.Set("originationAirportCode", origin)
		q.Set("destinationAirportCode", dest)
		q.Set("departureDate", depart)
		q.Set("adultPassengersCount", "1")
		if ret != "" {
			q.Set("returnDate", ret)
		}
		return "https://www.southwest.com/air/booking/select.html?" + q.Encode()
	case "NK":
		return fmt.Sprintf("https://www.spirit.com/book/flights?origin=%s&destination=%s&date=%s", origin, dest, depart)
	case "F9":
		return fmt.Sprintf("https://booking.flyfrontier.com/Flight/InternalSelect?o1=%s&d1=%s&dd1=%s", origin, dest, depart)
	case "AS":
		return fmt.Sprintf("https://www.alaskaair.com/search/results?A1=%s&A2=%s&D1=%s", origin, dest, depart)
	case "HA":
		return fmt.Sprintf("https://www.hawaiianairlines.com/book/flights?origin=%s&destination=%s&departure=%s", origin, dest, depart)
	case "DL":
		return fmt.Sprintf("https://www.delta.com/flightsearch/book-a-flight?originCity=%s&destinationCity=%s&departureDate=%s", origin, dest, depart)
	case "UA":
		return fmt.Sprintf("https://www.united.com/en/us/fsr/choose-flights?f=%s&t=%s&d=%s", origin, dest, depart)
	case "AA":
		return fmt.Sprintf("https://www.aa.com/booking/find-flights?origin=%s&destination=%s&departDate=%s", origin, dest, depart)
	case "AC":
		return fmt.Sprintf("https://www.aircanada.com/aeroplan/redeem/availability/outbound?org0=%s&dest0=%s&departureDate0=%s", origin, dest, depart)
	case "BA":
		return fmt.Sprintf("https://www.britishairways.com/travel/book/public/en_us?departurePoint=%s&destinationPoint=%s&departureDate=%s", origin, dest, depart)
	case "LH":
		return fmt.Sprintf("https://www.lufthansa.com/us/en/book-a-flight?origin=%s&destination=%s&outbound=%s", origin, dest, depart)
	case "AF":
		return fmt.Sprintf("https://wwws.airfrance.us/search/offers?origin=%s&destination=%s&departureDate=%s", origin, dest, depart)
	case "KL":
		return fmt.Sprintf("https://www.klm.com/search/offers?origin=%s&destination=%s&departureDate=%s", origin, dest, depart)
	case "EK":
		return fmt.Sprintf("https://www.emirates.com/us/english/book/featured-fares/?origin=%s&destination=%s&departure=%s", origin, dest, depart)
	case "QR":
		return fmt.Sprintf("https://www.qatarairways.com/app/booking/flight-selection?widget=QR&from=%s&to=%s&departing=%s", origin, dest, depart)
	case "EY":
		return fmt.Sprintf("https://www.etihad.com/en-us/book/flights?origin=%s&destination=%s&departureDate=%s", origin, dest, depart)
	case "SQ":
		return fmt.Sprintf("https://www.singaporeair.com/en_UK/plan-and-book/book-a-flight/?from=%s&to=%s&depart=%s", origin, dest, depart)
	case "CX":
		return fmt.Sprintf("https://www.cathaypacific.com/booking-trip/flight-search?origin=%s&destination=%s&departureDate=%s", origin, dest, depart)
	case "QF":
		return fmt.Sprintf("https://www.qantas.com/travel/airlines/flight-search/global/en?origin=%s&destination=%s&departureDate=%s", origin, dest, depart)
	case "NZ":
		return fmt.Sprintf("https://www.airnewzealand.com/booking/select-flights?origin=%s&destination=%s&departureDate=%s", origin, dest, depart)
	case "NH":
		return fmt.Sprintf("https://www.ana.co.jp/en/us/plan-book/search/?origin=%s&destination=%s&departureDate=%s", origin, dest, depart)
	case "JL":
		return fmt.Sprintf("https://www.jal.co.jp/flights/en-us/?origin=%s&destination=%s&departureDate=%s", origin, dest, depart)
	case "KE":
		return fmt.Sprintf("https://www.koreanair.com/booking/search?origin=%s&destination=%s&departureDate=%s", origin, dest, depart)
	case "BR":
		return fmt.Sprintf("https://booking.evaair.com/flyeva/eva/b2c/booking-online.aspx?origin=%s&destination=%s&departureDate=%s", origin, dest, depart)
	case "TK":
		return fmt.Sprintf("https://www.turkishairlines.com/en-int/flights/booking/?from=%s&to=%s&departure=%s", origin, dest, depart)
	case "VS":
		return fmt.Sprintf("https://flights.virginatlantic.com/en-us/flights-from-%s-to-%s?departure=%s", strings.ToLower(origin), strings.ToLower(dest), depart)
	case "IB":
		return fmt.Sprintf("https://www.iberia.com/flights/?market=US&origin=%s&destination=%s&departureDate=%s", origin, dest, depart)
	case "LX":
		return fmt.Sprintf("https://www.swiss.com/us/en/book-and-manage/booking?origin=%s&destination=%s&departureDate=%s", origin, dest, depart)
	case "AI":
		return fmt.Sprintf("https://www.airindia.com/in/en/book-and-manage/book-flights.html?origin=%s&destination=%s&departureDate=%s", origin, dest, depart)
	case "SV":
		return fmt.Sprintf("https://www.saudia.com/booking/search?origin=%s&destination=%s&departureDate=%s", origin, dest, depart)
	case "ET":
		return fmt.Sprintf("https://www.ethiopianairlines.com/us/book/booking/search?origin=%s&destination=%s&departureDate=%s", origin, dest, depart)
	default:
		return ""
	}
}

var (
	priceJSONRe = regexp.MustCompile(`(?i)"(?:total|amount|price|fare|lowestFare|minPrice)"\s*:\s*"?\$?([0-9]{2,5}(?:\.[0-9]{1,2})?)"?`)
	priceMetaRe = regexp.MustCompile(`(?i)(?:starting at|from)\s*\$([0-9]{2,5}(?:\.[0-9]{1,2})?)`)
)

func extractPublicPrice(body []byte) (float64, bool) {
	s := string(body)
	if m := priceJSONRe.FindStringSubmatch(s); len(m) == 2 {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil && v >= 29 && v <= 20000 {
			return v, true
		}
	}
	if m := priceMetaRe.FindStringSubmatch(s); len(m) == 2 {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil && v >= 29 && v <= 20000 {
			return v, true
		}
	}
	return 0, false
}
