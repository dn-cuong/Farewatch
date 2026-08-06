package graph

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/farewatch/farewatch/internal/airlines"
	"github.com/farewatch/farewatch/internal/auth"
	"github.com/farewatch/farewatch/internal/scanner"
	"github.com/farewatch/farewatch/internal/store"
	"github.com/graphql-go/graphql"
	"github.com/jackc/pgx/v5"
)

func asMap(v any) (map[string]any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func asMaps(v any) ([]map[string]any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m []map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	if m == nil {
		return []map[string]any{}, nil
	}
	return m, nil
}

func requireUser(p graphql.ResolveParams) (string, error) {
	uid, ok := auth.UserIDFromContext(p.Context)
	if !ok {
		return "", errors.New("authentication required")
	}
	return uid, nil
}

func NewSchema(
	st *store.Store,
	sc *scanner.Scanner,
	authSvc *auth.Service,
	firebaseProjectID string,
	ignav *airlines.IgnavClient,
	fareProviders []airlines.Provider,
) (graphql.Schema, error) {
	userType := graphql.NewObject(graphql.ObjectConfig{
		Name: "User",
		Fields: graphql.Fields{
			"id":        &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"email":     &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"name":      &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"createdAt": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
		},
	})

	authPayloadType := graphql.NewObject(graphql.ObjectConfig{
		Name: "AuthPayload",
		Fields: graphql.Fields{
			"token": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"user":  &graphql.Field{Type: userType},
		},
	})

	routeType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Route",
		Fields: graphql.Fields{
			"id":          &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"origin":      &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"destination": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"departDate":  &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"returnDate":  &graphql.Field{Type: graphql.String},
			"cabin":       &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"active":      &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
			"createdAt":   &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
		},
	})

	fareType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Fare",
		Fields: graphql.Fields{
			"id":              &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"routeId":         &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"airline":         &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"airlineCode":     &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"flightNumber":    &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"origin":          &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"originCity":      &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"destination":     &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"destinationCity": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"departAt":        &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"arriveAt":        &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"durationMinutes": &graphql.Field{Type: graphql.NewNonNull(graphql.Int)},
			"stops":           &graphql.Field{Type: graphql.NewNonNull(graphql.Int)},
			"cabin":           &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"aircraft":        &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"price":           &graphql.Field{Type: graphql.NewNonNull(graphql.Float)},
			"currency":        &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"deepLink":        &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"source":          &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"cached":          &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
			"observedAt":      &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
		},
	})

	watchType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Watch",
		Fields: graphql.Fields{
			"id":           &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"userId":       &graphql.Field{Type: graphql.ID},
			"email":        &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"routeId":      &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"airlineCode":  &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"flightNumber": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"targetPrice":  &graphql.Field{Type: graphql.Float},
			"notifyOnDrop": &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
			"dropPercent":  &graphql.Field{Type: graphql.NewNonNull(graphql.Float)},
			"active":       &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
			"createdAt":    &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"change24h":    &graphql.Field{Type: graphql.Float},
			"route":        &graphql.Field{Type: routeType},
			"latestFare":   &graphql.Field{Type: fareType},
		},
	})

	segmentType := graphql.NewObject(graphql.ObjectConfig{
		Name: "FlightSegment",
		Fields: graphql.Fields{
			"airlineCode":     &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"airline":         &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"flightNumber":    &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"origin":          &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"originCity":      &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"destination":     &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"destinationCity": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"departAt":        &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"arriveAt":        &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"durationMinutes": &graphql.Field{Type: graphql.NewNonNull(graphql.Int)},
			"aircraft":        &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
		},
	})

	flightOfferType := graphql.NewObject(graphql.ObjectConfig{
		Name: "FlightOffer",
		Fields: graphql.Fields{
			"offerId":         &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"airline":         &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"airlineCode":     &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"flightNumber":    &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"origin":          &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"originCity":      &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"destination":     &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"destinationCity": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"departAt":        &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"arriveAt":        &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"durationMinutes": &graphql.Field{Type: graphql.NewNonNull(graphql.Int)},
			"stops":           &graphql.Field{Type: graphql.NewNonNull(graphql.Int)},
			"cabin":           &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"aircraft":        &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"price":           &graphql.Field{Type: graphql.NewNonNull(graphql.Float)},
			"currency":        &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"deepLink":        &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"source":          &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"layoverAirports": &graphql.Field{Type: graphql.NewList(graphql.NewNonNull(graphql.String))},
			"segments":        &graphql.Field{Type: graphql.NewList(graphql.NewNonNull(segmentType))},
		},
	})

	bookingLinkType := graphql.NewObject(graphql.ObjectConfig{
		Name: "BookingLink",
		Fields: graphql.Fields{
			"providerName": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"providerType": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"fareName":     &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"price":        &graphql.Field{Type: graphql.NewNonNull(graphql.Float)},
			"currency":     &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"url":          &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
		},
	})

	alertType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Alert",
		Fields: graphql.Fields{
			"id":            &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"watchId":       &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"fareId":        &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"oldPrice":      &graphql.Field{Type: graphql.NewNonNull(graphql.Float)},
			"newPrice":      &graphql.Field{Type: graphql.NewNonNull(graphql.Float)},
			"airline":       &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"sentAt":        &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"deliveredInMs": &graphql.Field{Type: graphql.NewNonNull(graphql.Int)},
		},
	})

	scanStatsType := graphql.NewObject(graphql.ObjectConfig{
		Name: "ScanStats",
		Fields: graphql.Fields{
			"routesScanned":   &graphql.Field{Type: graphql.NewNonNull(graphql.Int)},
			"faresFound":      &graphql.Field{Type: graphql.NewNonNull(graphql.Int)},
			"cacheHits":       &graphql.Field{Type: graphql.NewNonNull(graphql.Int)},
			"cacheMisses":     &graphql.Field{Type: graphql.NewNonNull(graphql.Int)},
			"cacheHitRate":    &graphql.Field{Type: graphql.NewNonNull(graphql.Float)},
			"alertsSent":      &graphql.Field{Type: graphql.NewNonNull(graphql.Int)},
			"durationMs":      &graphql.Field{Type: graphql.NewNonNull(graphql.Int)},
			"airlinesQueried": &graphql.Field{Type: graphql.NewNonNull(graphql.Int)},
		},
	})

	airportType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Airport",
		Fields: graphql.Fields{
			"code":    &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"city":    &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"country": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
		},
	})

	queryType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"health": &graphql.Field{
				Type:    graphql.String,
				Resolve: func(p graphql.ResolveParams) (any, error) { return "ok", nil },
			},
			"me": &graphql.Field{
				Type: userType,
				Resolve: func(p graphql.ResolveParams) (any, error) {
					uid, err := requireUser(p)
					if err != nil {
						return nil, err
					}
					u, err := st.GetUserByID(p.Context, uid)
					if err != nil {
						return nil, err
					}
					return asMap(u)
				},
			},
			"airports": &graphql.Field{
				Type: graphql.NewList(airportType),
				Resolve: func(p graphql.ResolveParams) (any, error) {
					opts := airlines.AirportOptions()
					out := make([]map[string]any, 0, len(opts))
					for _, a := range opts {
						out = append(out, map[string]any{"code": a.Code, "city": a.City, "country": a.Country})
					}
					return out, nil
				},
			},
			"searchFares": &graphql.Field{
				Type: graphql.NewList(flightOfferType),
				Args: graphql.FieldConfigArgument{
					"origin":      &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
					"destination": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
					"departDate":  &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
					"returnDate":  &graphql.ArgumentConfig{Type: graphql.String},
					"cabin":       &graphql.ArgumentConfig{Type: graphql.String},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					origin := strings.ToUpper(strings.TrimSpace(p.Args["origin"].(string)))
					dest := strings.ToUpper(strings.TrimSpace(p.Args["destination"].(string)))
					if origin == dest {
						return nil, errors.New("origin and destination must differ")
					}
					depart := p.Args["departDate"].(string)
					var ret *string
					if v, ok := p.Args["returnDate"].(string); ok && v != "" {
						ret = &v
					}
					cabin, _ := p.Args["cabin"].(string)
					offers, err := airlines.SearchProviders(p.Context, fareProviders, origin, dest, depart, ret, cabin)
					if err != nil {
						return nil, fmt.Errorf("search failed: %w", err)
					}
					return asMaps(offers)
				},
			},
			"bookingLinks": &graphql.Field{
				Type: graphql.NewList(bookingLinkType),
				Args: graphql.FieldConfigArgument{
					"offerId":     &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
					"origin":      &graphql.ArgumentConfig{Type: graphql.String},
					"destination": &graphql.ArgumentConfig{Type: graphql.String},
					"departDate":  &graphql.ArgumentConfig{Type: graphql.String},
					"returnDate":  &graphql.ArgumentConfig{Type: graphql.String},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					offerID := strings.TrimSpace(p.Args["offerId"].(string))
					origin, _ := p.Args["origin"].(string)
					dest, _ := p.Args["destination"].(string)
					depart, _ := p.Args["departDate"].(string)
					var ret *string
					if v, ok := p.Args["returnDate"].(string); ok && v != "" {
						ret = &v
					}
					fallback := airlines.GoogleFlightsFallback(origin, dest, depart, ret)

					if ignav == nil || strings.HasPrefix(offerID, "sim-") {
						return []map[string]any{{
							"providerName": "Google Flights",
							"providerType": "metasearch",
							"fareName":     "",
							"price":        0,
							"currency":     "USD",
							"url":          fallback,
						}}, nil
					}

					links, err := ignav.BookingLinks(p.Context, offerID)
					if err != nil || len(links) == 0 {
						return []map[string]any{{
							"providerName": "Google Flights",
							"providerType": "metasearch",
							"fareName":     "",
							"price":        0,
							"currency":     "USD",
							"url":          fallback,
						}}, nil
					}
					out, err := asMaps(links)
					if err != nil {
						return nil, err
					}
					out = append(out, map[string]any{
						"providerName": "Google Flights",
						"providerType": "metasearch",
						"fareName":     "",
						"price":        0,
						"currency":     "USD",
						"url":          fallback,
					})
					return out, nil
				},
			},
			"myWatches": &graphql.Field{
				Type: graphql.NewList(watchType),
				Resolve: func(p graphql.ResolveParams) (any, error) {
					uid, err := requireUser(p)
					if err != nil {
						return nil, err
					}
					watches, err := st.ListWatchesByUser(p.Context, uid)
					if err != nil {
						return nil, err
					}
					return asMaps(watches)
				},
			},
			"fares": &graphql.Field{
				Type: graphql.NewList(fareType),
				Args: graphql.FieldConfigArgument{
					"routeId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
					"limit":   &graphql.ArgumentConfig{Type: graphql.Int},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if _, err := requireUser(p); err != nil {
						return nil, err
					}
					limit := 50
					if v, ok := p.Args["limit"].(int); ok {
						limit = v
					}
					fares, err := st.ListFares(p.Context, p.Args["routeId"].(string), limit)
					if err != nil {
						return nil, err
					}
					return asMaps(fares)
				},
			},
			"myAlerts": &graphql.Field{
				Type: graphql.NewList(alertType),
				Args: graphql.FieldConfigArgument{
					"limit": &graphql.ArgumentConfig{Type: graphql.Int},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					uid, err := requireUser(p)
					if err != nil {
						return nil, err
					}
					limit := 20
					if v, ok := p.Args["limit"].(int); ok {
						limit = v
					}
					alerts, err := st.ListAlertsForUser(p.Context, uid, limit)
					if err != nil {
						return nil, err
					}
					return asMaps(alerts)
				},
			},
		},
	})

	mutationType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Mutation",
		Fields: graphql.Fields{
			"register": &graphql.Field{
				Type: authPayloadType,
				Args: graphql.FieldConfigArgument{
					"email":    &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
					"password": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
					"name":     &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					email := strings.TrimSpace(strings.ToLower(p.Args["email"].(string)))
					password := p.Args["password"].(string)
					name := strings.TrimSpace(p.Args["name"].(string))
					if len(password) < 6 {
						return nil, errors.New("password must be at least 6 characters")
					}
					if _, err := st.GetUserByEmail(p.Context, email); err == nil {
						return nil, errors.New("email already registered")
					} else if !errors.Is(err, pgx.ErrNoRows) {
						return nil, err
					}
					hash, err := authSvc.HashPassword(password)
					if err != nil {
						return nil, err
					}
					u, err := st.CreateUser(p.Context, email, name, hash)
					if err != nil {
						return nil, err
					}
					token, err := authSvc.IssueToken(u)
					if err != nil {
						return nil, err
					}
					um, _ := asMap(u)
					return map[string]any{"token": token, "user": um}, nil
				},
			},
			"login": &graphql.Field{
				Type: authPayloadType,
				Args: graphql.FieldConfigArgument{
					"email":    &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
					"password": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					email := strings.TrimSpace(strings.ToLower(p.Args["email"].(string)))
					password := p.Args["password"].(string)
					u, err := st.GetUserByEmail(p.Context, email)
					if err != nil {
						return nil, errors.New("invalid email or password")
					}
					if !authSvc.CheckPassword(u.PasswordHash, password) {
						return nil, errors.New("invalid email or password")
					}
					token, err := authSvc.IssueToken(u)
					if err != nil {
						return nil, err
					}
					um, _ := asMap(u)
					return map[string]any{"token": token, "user": um}, nil
				},
			},
			"loginWithFirebase": &graphql.Field{
				Type: authPayloadType,
				Args: graphql.FieldConfigArgument{
					"idToken": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if firebaseProjectID == "" {
						return nil, errors.New("Firebase sign-in is not configured")
					}
					idToken := p.Args["idToken"].(string)
					ident, err := authSvc.VerifyFirebaseToken(p.Context, idToken, firebaseProjectID)
					if err != nil {
						return nil, err
					}
					hash, err := authSvc.HashPassword("firebase-managed-" + ident.UID)
					if err != nil {
						return nil, err
					}
					u, err := st.UpsertFirebaseUser(p.Context, ident.UID, ident.Email, ident.Name, hash)
					if err != nil {
						return nil, err
					}
					token, err := authSvc.IssueToken(u)
					if err != nil {
						return nil, err
					}
					um, _ := asMap(u)
					return map[string]any{"token": token, "user": um}, nil
				},
			},
			"createWatch": &graphql.Field{
				Type: watchType,
				Args: graphql.FieldConfigArgument{
					"origin":       &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
					"destination":  &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
					"departDate":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
					"returnDate":   &graphql.ArgumentConfig{Type: graphql.String},
					"cabin":        &graphql.ArgumentConfig{Type: graphql.String},
					"airlineCode":  &graphql.ArgumentConfig{Type: graphql.String},
					"flightNumber": &graphql.ArgumentConfig{Type: graphql.String},
					"targetPrice":  &graphql.ArgumentConfig{Type: graphql.Float},
					"dropPercent":  &graphql.ArgumentConfig{Type: graphql.Float},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					uid, err := requireUser(p)
					if err != nil {
						return nil, err
					}
					user, err := st.GetUserByID(p.Context, uid)
					if err != nil {
						return nil, err
					}
					var ret *string
					if v, ok := p.Args["returnDate"].(string); ok && v != "" {
						ret = &v
					}
					cabin, _ := p.Args["cabin"].(string)
					airlineCode, _ := p.Args["airlineCode"].(string)
					flightNumber, _ := p.Args["flightNumber"].(string)
					route, err := st.CreateRoute(p.Context,
						p.Args["origin"].(string),
						p.Args["destination"].(string),
						p.Args["departDate"].(string),
						ret,
						cabin,
					)
					if err != nil {
						return nil, err
					}
					var target *float64
					if v, ok := p.Args["targetPrice"].(float64); ok {
						target = &v
					}
					drop := 5.0
					if v, ok := p.Args["dropPercent"].(float64); ok {
						drop = v
					}
					w, err := st.CreateWatch(p.Context, uid, user.Email, route.ID, airlineCode, flightNumber, target, drop)
					if err != nil {
						return nil, err
					}
					w.Route = route
					// Detached + coalesced: RequestScan will not spawn a
					// second full scan if one is already in flight.
					sc.RequestScan()
					return asMap(w)
				},
			},
			"createEmailWatch": &graphql.Field{
				Type: watchType,
				Args: graphql.FieldConfigArgument{
					"email":        &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
					"origin":       &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
					"destination":  &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
					"departDate":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
					"returnDate":   &graphql.ArgumentConfig{Type: graphql.String},
					"cabin":        &graphql.ArgumentConfig{Type: graphql.String},
					"airlineCode":  &graphql.ArgumentConfig{Type: graphql.String},
					"flightNumber": &graphql.ArgumentConfig{Type: graphql.String},
					"targetPrice":  &graphql.ArgumentConfig{Type: graphql.Float},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					email := strings.ToLower(strings.TrimSpace(p.Args["email"].(string)))
					if _, err := mail.ParseAddress(email); err != nil {
						return nil, errors.New("enter a valid email address")
					}
					var ret *string
					if v, ok := p.Args["returnDate"].(string); ok && v != "" {
						ret = &v
					}
					cabin, _ := p.Args["cabin"].(string)
					airlineCode, _ := p.Args["airlineCode"].(string)
					flightNumber, _ := p.Args["flightNumber"].(string)
					route, err := st.CreateRoute(
						p.Context,
						p.Args["origin"].(string),
						p.Args["destination"].(string),
						p.Args["departDate"].(string),
						ret,
						cabin,
					)
					if err != nil {
						return nil, err
					}
					var target *float64
					if v, ok := p.Args["targetPrice"].(float64); ok {
						target = &v
					}
					w, err := st.CreateWatch(
						p.Context,
						"",
						email,
						route.ID,
						airlineCode,
						flightNumber,
						target,
						5,
					)
					if err != nil {
						return nil, err
					}
					w.Route = route
					sc.RequestScan()
					return asMap(w)
				},
			},
			"updateWatch": &graphql.Field{
				Type: watchType,
				Args: graphql.FieldConfigArgument{
					"id":           &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
					"notifyOnDrop": &graphql.ArgumentConfig{Type: graphql.Boolean},
					"targetPrice":  &graphql.ArgumentConfig{Type: graphql.Float},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					uid, err := requireUser(p)
					if err != nil {
						return nil, err
					}
					var notify *bool
					if v, ok := p.Args["notifyOnDrop"].(bool); ok {
						notify = &v
					}
					var target *float64
					if v, ok := p.Args["targetPrice"].(float64); ok {
						target = &v
					}
					if notify == nil && target == nil {
						return nil, errors.New("provide notifyOnDrop and/or targetPrice")
					}
					w, err := st.UpdateWatch(p.Context, uid, p.Args["id"].(string), notify, target)
					if err != nil {
						return nil, err
					}
					return asMap(w)
				},
			},
			"removeWatch": &graphql.Field{
				Type: graphql.Boolean,
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					uid, err := requireUser(p)
					if err != nil {
						return nil, err
					}
					if err := st.DeactivateWatch(p.Context, uid, p.Args["id"].(string)); err != nil {
						return nil, err
					}
					return true, nil
				},
			},
			"runScan": &graphql.Field{
				Type: scanStatsType,
				Resolve: func(p graphql.ResolveParams) (any, error) {
					if _, err := requireUser(p); err != nil {
						return nil, err
					}
					stats, err := sc.Run(p.Context)
					if err != nil {
						return nil, fmt.Errorf("scan failed: %w", err)
					}
					return asMap(stats)
				},
			},
		},
	})

	return graphql.NewSchema(graphql.SchemaConfig{Query: queryType, Mutation: mutationType})
}
