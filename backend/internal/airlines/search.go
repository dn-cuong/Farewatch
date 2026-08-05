package airlines

import (
	"context"
	"log"
	"strings"
	"sync"
)

// SearchRoute concurrently polls all airline + search providers, then normalizes and dedupes.
func SearchRoute(ctx context.Context, ignav *IgnavClient, origin, dest, departDate string, returnDate *string, cabin string) ([]Offer, error) {
	return SearchProviders(ctx, AllProviders(ignav, nil), origin, dest, departDate, returnDate, cabin)
}

// SearchProviders fans out to the configured providers and merges their normalized offers.
func SearchProviders(
	ctx context.Context,
	providers []Provider,
	origin, dest, departDate string,
	returnDate *string,
	cabin string,
) ([]Offer, error) {
	origin = strings.ToUpper(strings.TrimSpace(origin))
	dest = strings.ToUpper(strings.TrimSpace(dest))
	if cabin == "" {
		cabin = "economy"
	}

	type result struct {
		offers []Offer
		err    error
		code   string
	}
	ch := make(chan result, len(providers))
	// Bound fan-out: the registry is global, but a browser search should not
	// open dozens of upstream connections at exactly the same instant.
	slots := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for _, p := range providers {
		p := p
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case slots <- struct{}{}:
				defer func() { <-slots }()
			case <-ctx.Done():
				ch <- result{err: ctx.Err(), code: p.Code()}
				return
			}
			offers, err := p.Fetch(ctx, origin, dest, departDate, returnDate, cabin)
			ch <- result{offers: offers, err: err, code: p.Code()}
		}()
	}
	go func() {
		wg.Wait()
		close(ch)
	}()

	merged := make([]Offer, 0, 64)
	for r := range ch {
		if r.err != nil {
			log.Printf("search provider=%s err=%v", r.code, r.err)
			continue
		}
		merged = append(merged, r.offers...)
	}
	return DeduplicateCheapest(merged), nil
}
