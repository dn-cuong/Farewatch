// Package ratelimit provides a small per-client token-bucket limiter for
// public HTTP endpoints. FareWatch's /graphql endpoint allows unauthenticated
// operations (searchFares, createEmailWatch) that fan out to 30+ upstream
// providers per call, so it needs its own throttle independent of the
// scanner's worker-pool rate limit.
package ratelimit

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type bucket struct {
	tokens  float64
	updated time.Time
}

// Limiter is a per-key token bucket, keyed by client IP by default.
type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    float64 // tokens replenished per second
	burst   float64 // maximum tokens a key can hold
}

// New creates a limiter allowing ratePerSec sustained requests per key, with
// bursts up to burst requests. Non-positive values fall back to sane
// defaults so a bad config value cannot accidentally disable throttling.
func New(ratePerSec, burst float64) *Limiter {
	if ratePerSec <= 0 {
		ratePerSec = 5
	}
	if burst <= 0 {
		burst = ratePerSec * 3
	}
	l := &Limiter{buckets: make(map[string]*bucket), rate: ratePerSec, burst: burst}
	go l.cleanupLoop()
	return l
}

// Allow reports whether a request for key should proceed, consuming a token
// if so.
func (l *Limiter) Allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[key]
	if !ok {
		l.buckets[key] = &bucket{tokens: l.burst - 1, updated: now}
		return true
	}
	elapsed := now.Sub(b.updated).Seconds()
	b.tokens += elapsed * l.rate
	if b.tokens > l.burst {
		b.tokens = l.burst
	}
	b.updated = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// cleanupLoop evicts idle buckets so long-running processes don't accumulate
// one entry per distinct IP forever.
func (l *Limiter) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().Add(-10 * time.Minute)
		l.mu.Lock()
		for k, b := range l.buckets {
			if b.updated.Before(cutoff) {
				delete(l.buckets, k)
			}
		}
		l.mu.Unlock()
	}
}

// Middleware rejects requests over the per-key rate with HTTP 429.
func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.Allow(clientKey(r)) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"errors":[{"message":"too many requests — slow down and try again"}]}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientKey(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		if first := strings.TrimSpace(strings.Split(fwd, ",")[0]); first != "" {
			return first
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
