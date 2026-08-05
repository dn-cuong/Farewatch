package models

import "time"

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"createdAt"`
}

type Route struct {
	ID          string    `json:"id"`
	Origin      string    `json:"origin"`
	Destination string    `json:"destination"`
	DepartDate  string    `json:"departDate"`
	ReturnDate  *string   `json:"returnDate,omitempty"`
	Cabin       string    `json:"cabin"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Fare is a full flight offer snapshot (airline-API shaped).
type Fare struct {
	ID              string    `json:"id"`
	RouteID         string    `json:"routeId"`
	Airline         string    `json:"airline"`
	AirlineCode     string    `json:"airlineCode"`
	FlightNumber    string    `json:"flightNumber"`
	Origin          string    `json:"origin"`
	OriginCity      string    `json:"originCity"`
	Destination     string    `json:"destination"`
	DestinationCity string    `json:"destinationCity"`
	DepartAt        string    `json:"departAt"`
	ArriveAt        string    `json:"arriveAt"`
	DurationMinutes int       `json:"durationMinutes"`
	Stops           int       `json:"stops"`
	Cabin           string    `json:"cabin"`
	Aircraft        string    `json:"aircraft"`
	Price           float64   `json:"price"`
	Currency        string    `json:"currency"`
	DeepLink        string    `json:"deepLink"`
	Source          string    `json:"source"`
	Cached          bool      `json:"cached"`
	ObservedAt      time.Time `json:"observedAt"`
}

type Watch struct {
	ID           string    `json:"id"`
	UserID       string    `json:"userId"`
	Email        string    `json:"email"`
	RouteID      string    `json:"routeId"`
	AirlineCode  string    `json:"airlineCode,omitempty"`
	FlightNumber string    `json:"flightNumber,omitempty"`
	TargetPrice  *float64  `json:"targetPrice,omitempty"`
	NotifyOnDrop bool      `json:"notifyOnDrop"`
	DropPercent  float64   `json:"dropPercent"`
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"createdAt"`
	Route        *Route    `json:"route,omitempty"`
	LatestFare   *Fare     `json:"latestFare,omitempty"`
	Change24h    *float64  `json:"change24h,omitempty"`
}

type Alert struct {
	ID          string    `json:"id"`
	WatchID     string    `json:"watchId"`
	FareID      string    `json:"fareId"`
	OldPrice    float64   `json:"oldPrice"`
	NewPrice    float64   `json:"newPrice"`
	Airline     string    `json:"airline"`
	SentAt      time.Time `json:"sentAt"`
	DeliveredIn int64     `json:"deliveredInMs"`
}

type ScanStats struct {
	RoutesScanned   int     `json:"routesScanned"`
	FaresFound      int     `json:"faresFound"`
	CacheHits       int     `json:"cacheHits"`
	CacheMisses     int     `json:"cacheMisses"`
	CacheHitRate    float64 `json:"cacheHitRate"`
	AlertsSent      int     `json:"alertsSent"`
	DurationMs      int64   `json:"durationMs"`
	AirlinesQueried int     `json:"airlinesQueried"`
}

type AuthPayload struct {
	Token string `json:"token"`
	User  *User  `json:"user"`
}
