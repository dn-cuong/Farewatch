package scanner

import (
	"context"
	"errors"
	"log"
	"strings"
	"sync"
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

// minScanInterval bounds how often a full route×provider scan can run,
// whether triggered by the CronJob, the dashboard's "Run scan" button, or a
// newly created watch. Without it, concurrent createWatch/runScan calls each
// spawn their own full scan and can pile up unbounded upstream requests.
const minScanInterval = 10 * time.Second

// alertCooldown prevents re-sending a drop email for the same watch when the
// price has not meaningfully improved since the last alert (e.g. it wiggles
// a cent below the previous alerted price every scan).
const alertCooldown = 6 * time.Hour

type Scanner struct {
	cfg       config.Config
	store     *store.Store
	cache     *cache.Cache
	mailer    *alerts.Mailer
	providers []airlines.Provider

	mu          sync.Mutex
	running     bool
	lastRunEnd  time.Time
	rerunQueued bool
}

// ErrScanCooldown is returned when a scan is requested too soon after the
// previous one finished (or one is already in flight).
var ErrScanCooldown = errors.New("a scan is already running or just finished - try again shortly")

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

// Run executes one full route×provider scan. Concurrent/rapid callers are
// rejected with ErrScanCooldown instead of piling up overlapping scans.
func (s *Scanner) Run(ctx context.Context) (*models.ScanStats, error) {
	if err := s.claim(); err != nil {
		return nil, err
	}
	defer s.release()

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

	go s.pruneStaleFares()

	return stats, nil
}

func (s *Scanner) claim() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running || time.Since(s.lastRunEnd) < minScanInterval {
		return ErrScanCooldown
	}
	s.running = true
	return nil
}

func (s *Scanner) release() {
	s.mu.Lock()
	s.running = false
	s.lastRunEnd = time.Now()
	s.mu.Unlock()
}

// RequestScan asks for an out-of-band scan, e.g. right after a watch is
// created. Concurrent requests coalesce into a single trailing re-run
// instead of each spawning their own full route×provider scan.
func (s *Scanner) RequestScan() {
	s.mu.Lock()
	if s.running {
		s.rerunQueued = true
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	go func() {
		for {
			scanCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			if _, err := s.Run(scanCtx); err != nil && !errors.Is(err, ErrScanCooldown) {
				log.Printf("triggered scan: %v", err)
			}
			cancel()

			s.mu.Lock()
			if !s.rerunQueued {
				s.mu.Unlock()
				return
			}
			s.rerunQueued = false
			s.mu.Unlock()
		}
	}()
}

// pruneStaleFares deletes fare history past the configured retention window.
// Runs detached from the scan's own context/timeout since it is best-effort
// housekeeping, not part of the scan result.
func (s *Scanner) pruneStaleFares() {
	if s.cfg.FareRetention <= 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	n, err := s.store.PruneFares(ctx, s.cfg.FareRetention)
	if err != nil {
		log.Printf("prune fares: %v", err)
		return
	}
	if n > 0 {
		log.Printf("pruned %d fare rows older than %s", n, s.cfg.FareRetention)
	}
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
		byRoute[res.RouteID] = append(byRoute[res.RouteID], res.Offers...)
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
			// Skip simulator:* (local fallback, not a real market price).
			if saved.Price < prev.Price && !strings.HasPrefix(saved.Source, "simulator:") {
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

		if last, err := s.store.LastAlertForWatch(ctx, w.ID); err == nil {
			if time.Since(last.SentAt) < alertCooldown && last.NewPrice <= fare.Price {
				continue // already alerted at an equal-or-better price recently
			}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			log.Printf("last alert lookup: %v", err)
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
