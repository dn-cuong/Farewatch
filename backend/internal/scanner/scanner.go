package scanner

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/farewatch/farewatch/internal/airlines"
	"github.com/farewatch/farewatch/internal/alerts"
	"github.com/farewatch/farewatch/internal/cache"
	"github.com/farewatch/farewatch/internal/config"
	"github.com/farewatch/farewatch/internal/models"
	"github.com/farewatch/farewatch/internal/store"
	"github.com/farewatch/farewatch/internal/worker"
	"github.com/jackc/pgx/v5"
)

type Scanner struct {
	cfg       config.Config
	store     *store.Store
	cache     *cache.Cache
	mailer    *alerts.Mailer
	providers []airlines.Provider
}

func New(cfg config.Config, st *store.Store, c *cache.Cache, mailer *alerts.Mailer) *Scanner {
	ignav := airlines.NewIgnav(cfg.IgnavAPIKey)
	amadeus := airlines.NewAmadeus(cfg.AmadeusClientID, cfg.AmadeusClientSecret, cfg.AmadeusBaseURL)
	extras := airlines.FreeSearchProviders(cfg.TravelpayoutsToken, cfg.RapidAPIKey)
	providers := airlines.AllProviders(ignav, amadeus, extras...)
	airlineCount := airlines.AirlineProviderCount()
	log.Printf("fare providers: %d (%d airline adapters + %d configured search sources)",
		len(providers), airlineCount, len(providers)-airlineCount)
	return &Scanner{
		cfg:       cfg,
		store:     st,
		cache:     c,
		mailer:    mailer,
		providers: providers,
	}
}

func (s *Scanner) Run(ctx context.Context) (*models.ScanStats, error) {
	start := time.Now()
	s.cache.ResetStats()

	routes, err := s.store.ListRoutesWithActiveWatches(ctx)
	if err != nil {
		return nil, err
	}

	stats := &models.ScanStats{
		RoutesScanned:   len(routes),
		AirlinesQueried: len(s.providers),
	}
	if len(routes) == 0 {
		stats.DurationMs = time.Since(start).Milliseconds()
		return stats, nil
	}

	s.runProviders(ctx, routes, stats)

	hits, misses, hitRate := s.cache.Stats()
	stats.CacheHits = int(hits)
	stats.CacheMisses = int(misses)
	stats.CacheHitRate = hitRate
	stats.DurationMs = time.Since(start).Milliseconds()
	log.Printf("scan complete routes=%d fares=%d providers=%d cache_hit_rate=%.1f%% alerts=%d duration_ms=%d",
		stats.RoutesScanned, stats.FaresFound, stats.AirlinesQueried, stats.CacheHitRate, stats.AlertsSent, stats.DurationMs)
	return stats, nil
}

func (s *Scanner) runProviders(ctx context.Context, routes []models.Route, stats *models.ScanStats) {
	pool := worker.NewPool(s.cfg.WorkerCount, s.cfg.RateLimitPerSec, s.cache)
	results := pool.Run(ctx, worker.BuildJobs(routes, s.providers))

	byRoute := map[string][]airlines.Offer{}
	queried := map[string]struct{}{}
	for _, res := range results {
		if res.Err != nil {
			continue
		}
		queried[res.Provider] = struct{}{}
		for _, o := range res.Offers {
			o := o
			if res.Cached {
				// keep source from cache
			}
			byRoute[res.RouteID] = append(byRoute[res.RouteID], o)
		}
	}
	if len(queried) > 0 {
		stats.AirlinesQueried = len(queried)
	}

	for routeID, offers := range byRoute {
		deduped := airlines.DeduplicateCheapest(offers)
		for _, o := range deduped {
			fare := offerToFare(routeID, o, false)
			_ = s.cache.SetFare(ctx, fare)

			prev, prevErr := s.store.LatestFareForSelection(ctx, routeID, o.AirlineCode, o.FlightNumber)
			saved, err := s.store.InsertFare(ctx, fare)
			if err != nil {
				log.Printf("insert fare: %v", err)
				continue
			}
			stats.FaresFound++
			if prevErr != nil {
				if !errors.Is(prevErr, pgx.ErrNoRows) {
					log.Printf("prev fare lookup: %v", prevErr)
				}
				continue
			}
			if saved.Price < prev.Price {
				n, err := s.maybeAlert(ctx, saved, prev.Price)
				if err != nil {
					log.Printf("alert: %v", err)
				}
				stats.AlertsSent += n
			}
		}
	}
}

func offerToFare(routeID string, o airlines.Offer, cached bool) models.Fare {
	return models.Fare{
		RouteID: routeID, Airline: o.Airline, AirlineCode: o.AirlineCode, FlightNumber: o.FlightNumber,
		Origin: o.Origin, OriginCity: o.OriginCity, Destination: o.Destination, DestinationCity: o.DestinationCity,
		DepartAt: o.DepartAt.UTC().Format(time.RFC3339), ArriveAt: o.ArriveAt.UTC().Format(time.RFC3339),
		DurationMinutes: o.DurationMinutes, Stops: o.Stops, Cabin: o.Cabin, Aircraft: o.Aircraft,
		Price: o.Price, Currency: o.Currency, DeepLink: o.DeepLink, Source: o.Source,
		Cached: cached, ObservedAt: time.Now().UTC(),
	}
}

func (s *Scanner) maybeAlert(ctx context.Context, fare *models.Fare, oldPrice float64) (int, error) {
	watches, err := s.store.ActiveWatchesForRoute(ctx, fare.RouteID)
	if err != nil {
		return 0, err
	}
	route, err := s.store.GetRoute(ctx, fare.RouteID)
	if err != nil {
		return 0, err
	}
	sent := 0
	for _, w := range watches {
		if w.AirlineCode != "" && w.AirlineCode != fare.AirlineCode {
			continue
		}
		if w.FlightNumber != "" && w.FlightNumber != fare.FlightNumber {
			continue
		}
		dropPct := 0.0
		if oldPrice > 0 {
			dropPct = (oldPrice - fare.Price) / oldPrice * 100
		}
		hitTarget := w.TargetPrice != nil && fare.Price <= *w.TargetPrice
		hitDrop := w.NotifyOnDrop && dropPct >= w.DropPercent
		if !hitTarget && !hitDrop {
			continue
		}
		detectAt := time.Now()
		elapsed, err := s.mailer.SendDropAlert(alerts.DropEmail{
			To: w.Email, Origin: route.Origin, Destination: route.Destination, DepartDate: route.DepartDate,
			Airline: fare.Airline, OldPrice: oldPrice, NewPrice: fare.Price, DeepLink: fare.DeepLink,
		})
		if err != nil {
			log.Printf("email to %s failed: %v", w.Email, err)
			continue
		}
		delivered := elapsed.Milliseconds()
		if delivered == 0 {
			delivered = time.Since(detectAt).Milliseconds()
		}
		if _, err := s.store.InsertAlert(ctx, models.Alert{
			WatchID: w.ID, FareID: fare.ID, OldPrice: oldPrice, NewPrice: fare.Price,
			Airline: fare.Airline, DeliveredIn: delivered,
		}); err != nil {
			log.Printf("persist alert: %v", err)
		}
		sent++
	}
	return sent, nil
}
