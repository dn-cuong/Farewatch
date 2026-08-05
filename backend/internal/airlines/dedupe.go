package airlines

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Fingerprint builds a stable key for the same marketed itinerary across sources.
// Used to dedupe Ignav/search hits against per-airline API quotes.
func Fingerprint(o Offer) string {
	cabin := strings.ToLower(strings.TrimSpace(o.Cabin))
	if cabin == "" {
		cabin = "economy"
	}
	departDay := o.DepartAt.UTC().Format("2006-01-02")
	if departDay == "" || departDay == "0001-01-01" {
		departDay = "unknown"
	}
	fn := normalizeFlightNumber(o.AirlineCode, o.FlightNumber)
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s",
		strings.ToUpper(o.AirlineCode),
		fn,
		strings.ToUpper(o.Origin),
		strings.ToUpper(o.Destination),
		departDay,
		cabin,
	)
}

func normalizeFlightNumber(code, flight string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	flight = strings.ToUpper(strings.TrimSpace(flight))
	flight = strings.ReplaceAll(flight, " ", "")
	if code != "" && strings.HasPrefix(flight, code) {
		return flight
	}
	if code != "" && flight != "" {
		return code + flight
	}
	return flight
}

// DeduplicateCheapest keeps the lowest price per fingerprint and sorts by price.
func DeduplicateCheapest(offers []Offer) []Offer {
	if len(offers) == 0 {
		return offers
	}
	best := make(map[string]Offer, len(offers))
	for _, o := range offers {
		if o.Currency == "" {
			o.Currency = "USD"
		}
		if o.Cabin == "" {
			o.Cabin = "economy"
		}
		if o.DepartAt.IsZero() {
			o.DepartAt = time.Now().UTC()
		}
		fp := Fingerprint(o)
		prev, ok := best[fp]
		if !ok || preferOffer(o, prev) {
			best[fp] = o
		}
	}
	out := make([]Offer, 0, len(best))
	for _, o := range best {
		out = append(out, o)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Price == out[j].Price {
			return out[i].AirlineCode < out[j].AirlineCode
		}
		return out[i].Price < out[j].Price
	})
	return out
}

func preferOffer(a, b Offer) bool {
	aRank := sourceRank(a.Source)
	bRank := sourceRank(b.Source)
	if aRank != bRank {
		return aRank < bRank
	}
	if a.Price != b.Price {
		return a.Price < b.Price
	}
	return preferSource(a.Source, b.Source)
}

func sourceRank(source string) int {
	switch {
	case strings.HasPrefix(source, "search:"):
		return 0
	case strings.HasPrefix(source, "airline:"):
		return 1
	case strings.HasPrefix(source, "simulator:"):
		return 2
	default:
		return 3
	}
}

func preferSource(a, b string) bool {
	// Prefer real search sources over airline or simulator fallbacks on ties.
	aSearch := strings.HasPrefix(a, "search:")
	bSearch := strings.HasPrefix(b, "search:")
	if aSearch != bSearch {
		return aSearch
	}
	aAirline := strings.HasPrefix(a, "airline:")
	bAirline := strings.HasPrefix(b, "airline:")
	if aAirline != bAirline {
		return aAirline
	}
	return false
}
