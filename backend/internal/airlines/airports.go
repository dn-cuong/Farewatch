package airlines

import "sort"

type Airport struct {
	Code    string
	City    string
	Country string
	Lat     float64
	Lon     float64
	Tz      string
}

var airports = map[string]Airport{
	// United States
	"JFK": {Code: "JFK", City: "New York", Country: "United States", Lat: 40.6413, Lon: -73.7781, Tz: "America/New_York"},
	"LGA": {Code: "LGA", City: "New York", Country: "United States", Lat: 40.7769, Lon: -73.8740, Tz: "America/New_York"},
	"EWR": {Code: "EWR", City: "Newark", Country: "United States", Lat: 40.6895, Lon: -74.1745, Tz: "America/New_York"},
	"LAX": {Code: "LAX", City: "Los Angeles", Country: "United States", Lat: 33.9416, Lon: -118.4085, Tz: "America/Los_Angeles"},
	"SFO": {Code: "SFO", City: "San Francisco", Country: "United States", Lat: 37.6213, Lon: -122.3790, Tz: "America/Los_Angeles"},
	"SJC": {Code: "SJC", City: "San Jose", Country: "United States", Lat: 37.3639, Lon: -121.9289, Tz: "America/Los_Angeles"},
	"SAN": {Code: "SAN", City: "San Diego", Country: "United States", Lat: 32.7338, Lon: -117.1933, Tz: "America/Los_Angeles"},
	"ORD": {Code: "ORD", City: "Chicago", Country: "United States", Lat: 41.9742, Lon: -87.9073, Tz: "America/Chicago"},
	"MDW": {Code: "MDW", City: "Chicago", Country: "United States", Lat: 41.7868, Lon: -87.7522, Tz: "America/Chicago"},
	"BOS": {Code: "BOS", City: "Boston", Country: "United States", Lat: 42.3656, Lon: -71.0096, Tz: "America/New_York"},
	"SEA": {Code: "SEA", City: "Seattle", Country: "United States", Lat: 47.4502, Lon: -122.3088, Tz: "America/Los_Angeles"},
	"PDX": {Code: "PDX", City: "Portland", Country: "United States", Lat: 45.5898, Lon: -122.5951, Tz: "America/Los_Angeles"},
	"MIA": {Code: "MIA", City: "Miami", Country: "United States", Lat: 25.7959, Lon: -80.2870, Tz: "America/New_York"},
	"FLL": {Code: "FLL", City: "Fort Lauderdale", Country: "United States", Lat: 26.0742, Lon: -80.1506, Tz: "America/New_York"},
	"MCO": {Code: "MCO", City: "Orlando", Country: "United States", Lat: 28.4312, Lon: -81.3081, Tz: "America/New_York"},
	"DEN": {Code: "DEN", City: "Denver", Country: "United States", Lat: 39.8561, Lon: -104.6737, Tz: "America/Denver"},
	"ATL": {Code: "ATL", City: "Atlanta", Country: "United States", Lat: 33.6407, Lon: -84.4277, Tz: "America/New_York"},
	"DFW": {Code: "DFW", City: "Dallas", Country: "United States", Lat: 32.8998, Lon: -97.0403, Tz: "America/Chicago"},
	"IAH": {Code: "IAH", City: "Houston", Country: "United States", Lat: 29.9902, Lon: -95.3368, Tz: "America/Chicago"},
	"AUS": {Code: "AUS", City: "Austin", Country: "United States", Lat: 30.1975, Lon: -97.6664, Tz: "America/Chicago"},
	"PHX": {Code: "PHX", City: "Phoenix", Country: "United States", Lat: 33.4352, Lon: -112.0101, Tz: "America/Phoenix"},
	"LAS": {Code: "LAS", City: "Las Vegas", Country: "United States", Lat: 36.0840, Lon: -115.1537, Tz: "America/Los_Angeles"},
	"SLC": {Code: "SLC", City: "Salt Lake City", Country: "United States", Lat: 40.7899, Lon: -111.9791, Tz: "America/Denver"},
	"MSP": {Code: "MSP", City: "Minneapolis", Country: "United States", Lat: 44.8848, Lon: -93.2223, Tz: "America/Chicago"},
	"DTW": {Code: "DTW", City: "Detroit", Country: "United States", Lat: 42.2162, Lon: -83.3554, Tz: "America/New_York"},
	"PHL": {Code: "PHL", City: "Philadelphia", Country: "United States", Lat: 39.8744, Lon: -75.2424, Tz: "America/New_York"},
	"IAD": {Code: "IAD", City: "Washington", Country: "United States", Lat: 38.9531, Lon: -77.4565, Tz: "America/New_York"},
	"DCA": {Code: "DCA", City: "Washington", Country: "United States", Lat: 38.8512, Lon: -77.0402, Tz: "America/New_York"},
	"CLT": {Code: "CLT", City: "Charlotte", Country: "United States", Lat: 35.2144, Lon: -80.9473, Tz: "America/New_York"},
	"HNL": {Code: "HNL", City: "Honolulu", Country: "United States", Lat: 21.3187, Lon: -157.9225, Tz: "Pacific/Honolulu"},
	"ANC": {Code: "ANC", City: "Anchorage", Country: "United States", Lat: 61.1743, Lon: -149.9962, Tz: "America/Anchorage"},

	// Canada
	"YYZ": {Code: "YYZ", City: "Toronto", Country: "Canada", Lat: 43.6777, Lon: -79.6248, Tz: "America/Toronto"},
	"YVR": {Code: "YVR", City: "Vancouver", Country: "Canada", Lat: 49.1967, Lon: -123.1815, Tz: "America/Vancouver"},
	"YUL": {Code: "YUL", City: "Montreal", Country: "Canada", Lat: 45.4657, Lon: -73.7455, Tz: "America/Toronto"},
	"YYC": {Code: "YYC", City: "Calgary", Country: "Canada", Lat: 51.1315, Lon: -114.0106, Tz: "America/Edmonton"},

	// Latin America
	"MEX": {Code: "MEX", City: "Mexico City", Country: "Mexico", Lat: 19.4363, Lon: -99.0721, Tz: "America/Mexico_City"},
	"CUN": {Code: "CUN", City: "Cancun", Country: "Mexico", Lat: 21.0365, Lon: -86.8771, Tz: "America/Cancun"},
	"GRU": {Code: "GRU", City: "Sao Paulo", Country: "Brazil", Lat: -23.4356, Lon: -46.4731, Tz: "America/Sao_Paulo"},
	"GIG": {Code: "GIG", City: "Rio de Janeiro", Country: "Brazil", Lat: -22.8090, Lon: -43.2506, Tz: "America/Sao_Paulo"},
	"EZE": {Code: "EZE", City: "Buenos Aires", Country: "Argentina", Lat: -34.8222, Lon: -58.5358, Tz: "America/Argentina/Buenos_Aires"},
	"SCL": {Code: "SCL", City: "Santiago", Country: "Chile", Lat: -33.3930, Lon: -70.7858, Tz: "America/Santiago"},
	"BOG": {Code: "BOG", City: "Bogota", Country: "Colombia", Lat: 4.7016, Lon: -74.1469, Tz: "America/Bogota"},
	"LIM": {Code: "LIM", City: "Lima", Country: "Peru", Lat: -12.0219, Lon: -77.1143, Tz: "America/Lima"},

	// Europe
	"LHR": {Code: "LHR", City: "London", Country: "United Kingdom", Lat: 51.4700, Lon: -0.4543, Tz: "Europe/London"},
	"LGW": {Code: "LGW", City: "London", Country: "United Kingdom", Lat: 51.1537, Lon: -0.1821, Tz: "Europe/London"},
	"MAN": {Code: "MAN", City: "Manchester", Country: "United Kingdom", Lat: 53.3650, Lon: -2.2728, Tz: "Europe/London"},
	"DUB": {Code: "DUB", City: "Dublin", Country: "Ireland", Lat: 53.4213, Lon: -6.2701, Tz: "Europe/Dublin"},
	"CDG": {Code: "CDG", City: "Paris", Country: "France", Lat: 49.0097, Lon: 2.5479, Tz: "Europe/Paris"},
	"ORY": {Code: "ORY", City: "Paris", Country: "France", Lat: 48.7233, Lon: 2.3794, Tz: "Europe/Paris"},
	"NCE": {Code: "NCE", City: "Nice", Country: "France", Lat: 43.6584, Lon: 7.2159, Tz: "Europe/Paris"},
	"FRA": {Code: "FRA", City: "Frankfurt", Country: "Germany", Lat: 50.0379, Lon: 8.5622, Tz: "Europe/Berlin"},
	"MUC": {Code: "MUC", City: "Munich", Country: "Germany", Lat: 48.3538, Lon: 11.7861, Tz: "Europe/Berlin"},
	"BER": {Code: "BER", City: "Berlin", Country: "Germany", Lat: 52.3667, Lon: 13.5033, Tz: "Europe/Berlin"},
	"AMS": {Code: "AMS", City: "Amsterdam", Country: "Netherlands", Lat: 52.3105, Lon: 4.7683, Tz: "Europe/Amsterdam"},
	"BRU": {Code: "BRU", City: "Brussels", Country: "Belgium", Lat: 50.9014, Lon: 4.4844, Tz: "Europe/Brussels"},
	"ZRH": {Code: "ZRH", City: "Zurich", Country: "Switzerland", Lat: 47.4647, Lon: 8.5492, Tz: "Europe/Zurich"},
	"GVA": {Code: "GVA", City: "Geneva", Country: "Switzerland", Lat: 46.2381, Lon: 6.1090, Tz: "Europe/Zurich"},
	"VIE": {Code: "VIE", City: "Vienna", Country: "Austria", Lat: 48.1103, Lon: 16.5697, Tz: "Europe/Vienna"},
	"MAD": {Code: "MAD", City: "Madrid", Country: "Spain", Lat: 40.4936, Lon: -3.5668, Tz: "Europe/Madrid"},
	"BCN": {Code: "BCN", City: "Barcelona", Country: "Spain", Lat: 41.2974, Lon: 2.0833, Tz: "Europe/Madrid"},
	"LIS": {Code: "LIS", City: "Lisbon", Country: "Portugal", Lat: 38.7756, Lon: -9.1354, Tz: "Europe/Lisbon"},
	"FCO": {Code: "FCO", City: "Rome", Country: "Italy", Lat: 41.8003, Lon: 12.2389, Tz: "Europe/Rome"},
	"MXP": {Code: "MXP", City: "Milan", Country: "Italy", Lat: 45.6301, Lon: 8.7255, Tz: "Europe/Rome"},
	"ATH": {Code: "ATH", City: "Athens", Country: "Greece", Lat: 37.9364, Lon: 23.9445, Tz: "Europe/Athens"},
	"CPH": {Code: "CPH", City: "Copenhagen", Country: "Denmark", Lat: 55.6180, Lon: 12.6508, Tz: "Europe/Copenhagen"},
	"ARN": {Code: "ARN", City: "Stockholm", Country: "Sweden", Lat: 59.6519, Lon: 17.9186, Tz: "Europe/Stockholm"},
	"OSL": {Code: "OSL", City: "Oslo", Country: "Norway", Lat: 60.1976, Lon: 11.1004, Tz: "Europe/Oslo"},
	"HEL": {Code: "HEL", City: "Helsinki", Country: "Finland", Lat: 60.3172, Lon: 24.9633, Tz: "Europe/Helsinki"},
	"WAW": {Code: "WAW", City: "Warsaw", Country: "Poland", Lat: 52.1657, Lon: 20.9671, Tz: "Europe/Warsaw"},
	"PRG": {Code: "PRG", City: "Prague", Country: "Czechia", Lat: 50.1008, Lon: 14.2600, Tz: "Europe/Prague"},
	"IST": {Code: "IST", City: "Istanbul", Country: "Turkey", Lat: 41.2753, Lon: 28.7519, Tz: "Europe/Istanbul"},

	// Middle East and Africa
	"DXB": {Code: "DXB", City: "Dubai", Country: "United Arab Emirates", Lat: 25.2532, Lon: 55.3657, Tz: "Asia/Dubai"},
	"AUH": {Code: "AUH", City: "Abu Dhabi", Country: "United Arab Emirates", Lat: 24.4330, Lon: 54.6511, Tz: "Asia/Dubai"},
	"DOH": {Code: "DOH", City: "Doha", Country: "Qatar", Lat: 25.2731, Lon: 51.6081, Tz: "Asia/Qatar"},
	"JED": {Code: "JED", City: "Jeddah", Country: "Saudi Arabia", Lat: 21.6796, Lon: 39.1565, Tz: "Asia/Riyadh"},
	"RUH": {Code: "RUH", City: "Riyadh", Country: "Saudi Arabia", Lat: 24.9576, Lon: 46.6988, Tz: "Asia/Riyadh"},
	"TLV": {Code: "TLV", City: "Tel Aviv", Country: "Israel", Lat: 32.0114, Lon: 34.8867, Tz: "Asia/Jerusalem"},
	"CAI": {Code: "CAI", City: "Cairo", Country: "Egypt", Lat: 30.1219, Lon: 31.4056, Tz: "Africa/Cairo"},
	"ADD": {Code: "ADD", City: "Addis Ababa", Country: "Ethiopia", Lat: 8.9779, Lon: 38.7993, Tz: "Africa/Addis_Ababa"},
	"NBO": {Code: "NBO", City: "Nairobi", Country: "Kenya", Lat: -1.3192, Lon: 36.9278, Tz: "Africa/Nairobi"},
	"JNB": {Code: "JNB", City: "Johannesburg", Country: "South Africa", Lat: -26.1392, Lon: 28.2460, Tz: "Africa/Johannesburg"},
	"CPT": {Code: "CPT", City: "Cape Town", Country: "South Africa", Lat: -33.9715, Lon: 18.6021, Tz: "Africa/Johannesburg"},

	// Asia
	"NRT": {Code: "NRT", City: "Tokyo", Country: "Japan", Lat: 35.7720, Lon: 140.3929, Tz: "Asia/Tokyo"},
	"HND": {Code: "HND", City: "Tokyo", Country: "Japan", Lat: 35.5494, Lon: 139.7798, Tz: "Asia/Tokyo"},
	"KIX": {Code: "KIX", City: "Osaka", Country: "Japan", Lat: 34.4342, Lon: 135.2328, Tz: "Asia/Tokyo"},
	"ICN": {Code: "ICN", City: "Seoul", Country: "South Korea", Lat: 37.4602, Lon: 126.4407, Tz: "Asia/Seoul"},
	"PEK": {Code: "PEK", City: "Beijing", Country: "China", Lat: 40.0799, Lon: 116.6031, Tz: "Asia/Shanghai"},
	"PVG": {Code: "PVG", City: "Shanghai", Country: "China", Lat: 31.1443, Lon: 121.8083, Tz: "Asia/Shanghai"},
	"CAN": {Code: "CAN", City: "Guangzhou", Country: "China", Lat: 23.3924, Lon: 113.2988, Tz: "Asia/Shanghai"},
	"HKG": {Code: "HKG", City: "Hong Kong", Country: "Hong Kong", Lat: 22.3080, Lon: 113.9185, Tz: "Asia/Hong_Kong"},
	"TPE": {Code: "TPE", City: "Taipei", Country: "Taiwan", Lat: 25.0777, Lon: 121.2328, Tz: "Asia/Taipei"},
	"SIN": {Code: "SIN", City: "Singapore", Country: "Singapore", Lat: 1.3644, Lon: 103.9915, Tz: "Asia/Singapore"},
	"BKK": {Code: "BKK", City: "Bangkok", Country: "Thailand", Lat: 13.6900, Lon: 100.7501, Tz: "Asia/Bangkok"},
	"KUL": {Code: "KUL", City: "Kuala Lumpur", Country: "Malaysia", Lat: 2.7456, Lon: 101.7099, Tz: "Asia/Kuala_Lumpur"},
	"CGK": {Code: "CGK", City: "Jakarta", Country: "Indonesia", Lat: -6.1256, Lon: 106.6559, Tz: "Asia/Jakarta"},
	"DPS": {Code: "DPS", City: "Bali", Country: "Indonesia", Lat: -8.7482, Lon: 115.1672, Tz: "Asia/Makassar"},
	"MNL": {Code: "MNL", City: "Manila", Country: "Philippines", Lat: 14.5086, Lon: 121.0198, Tz: "Asia/Manila"},
	"SGN": {Code: "SGN", City: "Ho Chi Minh City", Country: "Vietnam", Lat: 10.8188, Lon: 106.6520, Tz: "Asia/Ho_Chi_Minh"},
	"HAN": {Code: "HAN", City: "Hanoi", Country: "Vietnam", Lat: 21.2212, Lon: 105.8072, Tz: "Asia/Ho_Chi_Minh"},
	"DAD": {Code: "DAD", City: "Da Nang", Country: "Vietnam", Lat: 16.0439, Lon: 108.1994, Tz: "Asia/Ho_Chi_Minh"},
	"CXR": {Code: "CXR", City: "Nha Trang", Country: "Vietnam", Lat: 11.9982, Lon: 109.2193, Tz: "Asia/Ho_Chi_Minh"},
	"PQC": {Code: "PQC", City: "Phu Quoc", Country: "Vietnam", Lat: 10.1698, Lon: 103.9931, Tz: "Asia/Ho_Chi_Minh"},
	"DEL": {Code: "DEL", City: "Delhi", Country: "India", Lat: 28.5562, Lon: 77.1000, Tz: "Asia/Kolkata"},
	"BOM": {Code: "BOM", City: "Mumbai", Country: "India", Lat: 19.0896, Lon: 72.8656, Tz: "Asia/Kolkata"},
	"BLR": {Code: "BLR", City: "Bengaluru", Country: "India", Lat: 13.1986, Lon: 77.7066, Tz: "Asia/Kolkata"},

	// Oceania
	"SYD": {Code: "SYD", City: "Sydney", Country: "Australia", Lat: -33.9399, Lon: 151.1753, Tz: "Australia/Sydney"},
	"MEL": {Code: "MEL", City: "Melbourne", Country: "Australia", Lat: -37.6690, Lon: 144.8410, Tz: "Australia/Melbourne"},
	"BNE": {Code: "BNE", City: "Brisbane", Country: "Australia", Lat: -27.3842, Lon: 153.1175, Tz: "Australia/Brisbane"},
	"PER": {Code: "PER", City: "Perth", Country: "Australia", Lat: -31.9403, Lon: 115.9669, Tz: "Australia/Perth"},
	"AKL": {Code: "AKL", City: "Auckland", Country: "New Zealand", Lat: -37.0082, Lon: 174.7850, Tz: "Pacific/Auckland"},
}

func LookupAirport(code string) Airport {
	if a, ok := airports[code]; ok {
		return a
	}
	return Airport{Code: code, City: code, Country: "", Tz: "UTC"}
}

// AirportOptions returns the selectable airports sorted by country then city.
func AirportOptions() []Airport {
	out := make([]Airport, 0, len(airports))
	for _, a := range airports {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Country != out[j].Country {
			return out[i].Country < out[j].Country
		}
		if out[i].City != out[j].City {
			return out[i].City < out[j].City
		}
		return out[i].Code < out[j].Code
	})
	return out
}
