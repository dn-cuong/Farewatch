package airlines

type Airport struct {
	Code string
	City string
	Lat  float64
	Lon  float64
	Tz   string
}

var airports = map[string]Airport{
	"JFK": {Code: "JFK", City: "New York", Lat: 40.6413, Lon: -73.7781, Tz: "America/New_York"},
	"LGA": {Code: "LGA", City: "New York", Lat: 40.7769, Lon: -73.8740, Tz: "America/New_York"},
	"EWR": {Code: "EWR", City: "Newark", Lat: 40.6895, Lon: -74.1745, Tz: "America/New_York"},
	"LAX": {Code: "LAX", City: "Los Angeles", Lat: 33.9416, Lon: -118.4085, Tz: "America/Los_Angeles"},
	"SFO": {Code: "SFO", City: "San Francisco", Lat: 37.6213, Lon: -122.3790, Tz: "America/Los_Angeles"},
	"ORD": {Code: "ORD", City: "Chicago", Lat: 41.9742, Lon: -87.9073, Tz: "America/Chicago"},
	"BOS": {Code: "BOS", City: "Boston", Lat: 42.3656, Lon: -71.0096, Tz: "America/New_York"},
	"SEA": {Code: "SEA", City: "Seattle", Lat: 47.4502, Lon: -122.3088, Tz: "America/Los_Angeles"},
	"MIA": {Code: "MIA", City: "Miami", Lat: 25.7959, Lon: -80.2870, Tz: "America/New_York"},
	"DEN": {Code: "DEN", City: "Denver", Lat: 39.8561, Lon: -104.6737, Tz: "America/Denver"},
	"ATL": {Code: "ATL", City: "Atlanta", Lat: 33.6407, Lon: -84.4277, Tz: "America/New_York"},
	"DFW": {Code: "DFW", City: "Dallas", Lat: 32.8998, Lon: -97.0403, Tz: "America/Chicago"},
	"HNL": {Code: "HNL", City: "Honolulu", Lat: 21.3187, Lon: -157.9225, Tz: "Pacific/Honolulu"},
	"NRT": {Code: "NRT", City: "Tokyo", Lat: 35.7720, Lon: 140.3929, Tz: "Asia/Tokyo"},
	"HND": {Code: "HND", City: "Tokyo", Lat: 35.5494, Lon: 139.7798, Tz: "Asia/Tokyo"},
	"LHR": {Code: "LHR", City: "London", Lat: 51.4700, Lon: -0.4543, Tz: "Europe/London"},
	"CDG": {Code: "CDG", City: "Paris", Lat: 49.0097, Lon: 2.5479, Tz: "Europe/Paris"},
	"FRA": {Code: "FRA", City: "Frankfurt", Lat: 50.0379, Lon: 8.5622, Tz: "Europe/Berlin"},
	"AMS": {Code: "AMS", City: "Amsterdam", Lat: 52.3105, Lon: 4.7683, Tz: "Europe/Amsterdam"},
	"YYZ": {Code: "YYZ", City: "Toronto", Lat: 43.6777, Lon: -79.6248, Tz: "America/Toronto"},
	"ICN": {Code: "ICN", City: "Seoul", Lat: 37.4602, Lon: 126.4407, Tz: "Asia/Seoul"},
	"SIN": {Code: "SIN", City: "Singapore", Lat: 1.3644, Lon: 103.9915, Tz: "Asia/Singapore"},
	"DXB": {Code: "DXB", City: "Dubai", Lat: 25.2532, Lon: 55.3657, Tz: "Asia/Dubai"},
	"SYD": {Code: "SYD", City: "Sydney", Lat: -33.9399, Lon: 151.1753, Tz: "Australia/Sydney"},
}

func LookupAirport(code string) Airport {
	if a, ok := airports[code]; ok {
		return a
	}
	return Airport{Code: code, City: code, Tz: "UTC"}
}

func AirportOptions() []Airport {
	out := make([]Airport, 0, len(airports))
	for _, a := range airports {
		out = append(out, a)
	}
	return out
}
