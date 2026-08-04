package worker

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/farewatch/farewatch/internal/airlines"
	"github.com/farewatch/farewatch/internal/cache"
	"github.com/farewatch/farewatch/internal/models"
)

type Job struct {
	Route    models.Route
	Provider airlines.Provider
}

type Result struct {
	RouteID string
	Offer   airlines.Offer
	Cached  bool
	Err     error
}

type Pool struct {
	workers int
	limiter <-chan time.Time
	cache   *cache.Cache
}

func NewPool(workers, ratePerSec int, c *cache.Cache) *Pool {
	if workers < 1 {
		workers = 4
	}
	if ratePerSec < 1 {
		ratePerSec = 10
	}
	ticker := time.NewTicker(time.Second / time.Duration(ratePerSec))
	return &Pool{workers: workers, limiter: ticker.C, cache: c}
}

func offerFromFare(f *models.Fare) airlines.Offer {
	departAt, _ := time.Parse(time.RFC3339, f.DepartAt)
	arriveAt, _ := time.Parse(time.RFC3339, f.ArriveAt)
	return airlines.Offer{
		Airline: f.Airline, AirlineCode: f.AirlineCode, FlightNumber: f.FlightNumber,
		Origin: f.Origin, OriginCity: f.OriginCity, Destination: f.Destination, DestinationCity: f.DestinationCity,
		DepartAt: departAt, ArriveAt: arriveAt, DurationMinutes: f.DurationMinutes, Stops: f.Stops,
		Cabin: f.Cabin, Aircraft: f.Aircraft, Price: f.Price, Currency: f.Currency, DeepLink: f.DeepLink, Source: f.Source,
	}
}

func fareFromOffer(routeID string, o airlines.Offer, cached bool) models.Fare {
	return models.Fare{
		RouteID: routeID, Airline: o.Airline, AirlineCode: o.AirlineCode, FlightNumber: o.FlightNumber,
		Origin: o.Origin, OriginCity: o.OriginCity, Destination: o.Destination, DestinationCity: o.DestinationCity,
		DepartAt: o.DepartAt.UTC().Format(time.RFC3339), ArriveAt: o.ArriveAt.UTC().Format(time.RFC3339),
		DurationMinutes: o.DurationMinutes, Stops: o.Stops, Cabin: o.Cabin, Aircraft: o.Aircraft,
		Price: o.Price, Currency: o.Currency, DeepLink: o.DeepLink, Source: o.Source,
		Cached: cached, ObservedAt: time.Now().UTC(),
	}
}

func (p *Pool) Run(ctx context.Context, jobs []Job) []Result {
	jobCh := make(chan Job, len(jobs))
	resCh := make(chan Result, len(jobs))
	var wg sync.WaitGroup

	for i := 0; i < p.workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for job := range jobCh {
				if ctx.Err() != nil {
					resCh <- Result{RouteID: job.Route.ID, Err: ctx.Err()}
					continue
				}
				if fare, ok, err := p.cache.GetFare(ctx, job.Route.ID, job.Provider.Code()); err == nil && ok {
					resCh <- Result{RouteID: job.Route.ID, Offer: offerFromFare(fare), Cached: true}
					continue
				}
				select {
				case <-ctx.Done():
					resCh <- Result{RouteID: job.Route.ID, Err: ctx.Err()}
					continue
				case <-p.limiter:
				}
				o, err := job.Provider.Fetch(ctx, job.Route.Origin, job.Route.Destination, job.Route.DepartDate, job.Route.ReturnDate, job.Route.Cabin)
				if err != nil {
					log.Printf("worker=%d airline=%s route=%s err=%v", workerID, job.Provider.Code(), job.Route.ID, err)
					resCh <- Result{RouteID: job.Route.ID, Err: err}
					continue
				}
				_ = p.cache.SetFare(ctx, fareFromOffer(job.Route.ID, o, false))
				resCh <- Result{RouteID: job.Route.ID, Offer: o, Cached: false}
			}
		}(i + 1)
	}

	for _, j := range jobs {
		jobCh <- j
	}
	close(jobCh)
	go func() { wg.Wait(); close(resCh) }()

	results := make([]Result, 0, len(jobs))
	for r := range resCh {
		results = append(results, r)
	}
	return results
}

func BuildJobs(routes []models.Route, providers []airlines.Provider) []Job {
	jobs := make([]Job, 0, len(routes)*len(providers))
	for _, r := range routes {
		for _, p := range providers {
			jobs = append(jobs, Job{Route: r, Provider: p})
		}
	}
	return jobs
}
