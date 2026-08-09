package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/farewatch/farewatch/internal/airlines"
	"github.com/farewatch/farewatch/internal/alerts"
	"github.com/farewatch/farewatch/internal/auth"
	"github.com/farewatch/farewatch/internal/cache"
	"github.com/farewatch/farewatch/internal/config"
	"github.com/farewatch/farewatch/internal/graph"
	"github.com/farewatch/farewatch/internal/ratelimit"
	"github.com/farewatch/farewatch/internal/scanner"
	"github.com/farewatch/farewatch/internal/store"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/graphql-go/handler"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	cfg := config.Load()

	if cfg.IsProduction() && cfg.JWTSecret == config.DefaultJWTSecret {
		log.Fatal("JWT_SECRET must be set in production")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer st.Close()

	c, err := cache.New(cfg.RedisURL, cfg.CacheTTL)
	if err != nil {
		log.Fatalf("redis: %v", err)
	}
	defer c.Close()

	mailer := alerts.NewMailer(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPFrom, cfg.SMTPUser, cfg.SMTPPass)
	sc := scanner.New(cfg, st, c, mailer)
	authSvc := auth.New(cfg.JWTSecret)
	ignav := airlines.NewIgnav(cfg.IgnavAPIKey)
	amadeus := airlines.NewAmadeus(cfg.AmadeusClientID, cfg.AmadeusClientSecret, cfg.AmadeusBaseURL)
	freeSources := airlines.FreeSearchProviders(cfg.TravelpayoutsToken, cfg.RapidAPIKey)
	fareProviders := airlines.AllProviders(ignav, amadeus, freeSources...)

	schema, err := graph.NewSchema(st, sc, authSvc, cfg.FirebaseProjectID, ignav, fareProviders)
	if err != nil {
		log.Fatalf("graphql schema: %v", err)
	}

	gh := handler.New(&handler.Config{
		Schema:   &schema,
		Pretty:   true,
		GraphiQL: !cfg.IsProduction(),
	})

	corsOrigins := []string{cfg.FrontendOrigin}
	if !cfg.IsProduction() {
		// localhost origins for local UI only
		corsOrigins = append(corsOrigins, "http://localhost:5173", "http://localhost:3000", "http://localhost")
	}

	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(90 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   corsOrigins,
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	}))
	r.Use(authSvc.Middleware)

	r.Get("/healthz", func(w http.ResponseWriter, req *http.Request) {
		hctx, hcancel := context.WithTimeout(req.Context(), 2*time.Second)
		defer hcancel()
		dbErr := st.Ping(hctx)
		cacheErr := c.Ping(hctx)

		w.Header().Set("Content-Type", "application/json")
		if dbErr != nil || cacheErr != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":   "degraded",
				"service":  "farewatch-api",
				"postgres": errStatus(dbErr),
				"redis":    errStatus(cacheErr),
			})
			return
		}
		_, _ = w.Write([]byte(`{"status":"ok","service":"farewatch-api"}`))
	})

	limiter := ratelimit.New(cfg.HTTPRateLimitPerSec, cfg.HTTPRateLimitBurst)
	r.Group(func(gr chi.Router) {
		gr.Use(limiter.Middleware)
		gr.Handle("/graphql", gh)
		if !cfg.IsProduction() {
			gr.Handle("/graphiql", gh)
		}
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      100 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Printf("FareWatch API listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
}

func errStatus(err error) string {
	if err == nil {
		return "ok"
	}
	return err.Error()
}
