package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/farewatch/farewatch/internal/alerts"
	"github.com/farewatch/farewatch/internal/cache"
	"github.com/farewatch/farewatch/internal/config"
	"github.com/farewatch/farewatch/internal/scanner"
	"github.com/farewatch/farewatch/internal/store"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	cfg := config.Load()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
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

	stats, err := sc.Run(ctx)
	if err != nil {
		log.Fatalf("scan: %v", err)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(stats)

	if stats.CacheHitRate < 0 {
		os.Exit(1)
	}
}
