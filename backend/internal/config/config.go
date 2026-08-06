package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// DefaultJWTSecret is the insecure dev fallback. Production boots must
// override JWT_SECRET — see (Config).IsProduction and its use in cmd/api.
const DefaultJWTSecret = "farewatch-dev-secret-change-me"

type Config struct {
	Env                 string
	Port                string
	DatabaseURL         string
	RedisURL            string
	SMTPHost            string
	SMTPPort            string
	SMTPFrom            string
	SMTPUser            string
	SMTPPass            string
	WorkerCount         int
	RateLimitPerSec     int
	CacheTTL            time.Duration
	AlertFromEmail      string
	FrontendOrigin      string
	JWTSecret           string
	AmadeusClientID     string
	AmadeusClientSecret string
	AmadeusBaseURL      string
	IgnavAPIKey         string
	TravelpayoutsToken  string
	RapidAPIKey         string
	FirebaseProjectID   string
	HTTPRateLimitPerSec float64
	HTTPRateLimitBurst  float64
	FareRetention       time.Duration
}

func Load() Config {
	return Config{
		Env:                 getEnv("APP_ENV", "development"),
		Port:                getEnv("PORT", "8080"),
		DatabaseURL:         getEnv("DATABASE_URL", "postgres://farewatch:farewatch@localhost:5432/farewatch?sslmode=disable"),
		RedisURL:            getEnv("REDIS_URL", "redis://localhost:6379/0"),
		SMTPHost:            getEnv("SMTP_HOST", "localhost"),
		SMTPPort:            getEnv("SMTP_PORT", "1025"),
		SMTPFrom:            getEnv("SMTP_FROM", "alerts@farewatch.app"),
		SMTPUser:            getEnv("SMTP_USER", ""),
		SMTPPass:            getEnv("SMTP_PASS", ""),
		WorkerCount:         getEnvInt("WORKER_COUNT", 8),
		RateLimitPerSec:     getEnvInt("RATE_LIMIT_PER_SEC", 20),
		CacheTTL:            time.Duration(getEnvInt("CACHE_TTL_SECONDS", 900)) * time.Second,
		AlertFromEmail:      getEnv("SMTP_FROM", "alerts@farewatch.app"),
		FrontendOrigin:      getEnv("FRONTEND_ORIGIN", "http://localhost:5173"),
		JWTSecret:           getEnv("JWT_SECRET", DefaultJWTSecret),
		AmadeusClientID:     getEnv("AMADEUS_CLIENT_ID", ""),
		AmadeusClientSecret: getEnv("AMADEUS_CLIENT_SECRET", ""),
		AmadeusBaseURL:      getEnv("AMADEUS_BASE_URL", "https://test.api.amadeus.com"),
		IgnavAPIKey:         getEnv("IGNAV_API_KEY", ""),
		TravelpayoutsToken:  getEnv("TRAVELPAYOUTS_TOKEN", ""),
		RapidAPIKey:         getEnv("RAPIDAPI_KEY", ""),
		FirebaseProjectID:   getEnv("FIREBASE_PROJECT_ID", ""),
		HTTPRateLimitPerSec: getEnvFloat("HTTP_RATE_LIMIT_PER_SEC", 5),
		HTTPRateLimitBurst:  getEnvFloat("HTTP_RATE_LIMIT_BURST", 15),
		FareRetention:       time.Duration(getEnvInt("FARE_RETENTION_DAYS", 90)) * 24 * time.Hour,
	}
}

// IsProduction reports whether APP_ENV names a production-like environment.
// Gates GraphiQL, permissive localhost CORS, and the default JWT secret.
func (c Config) IsProduction() bool {
	return strings.EqualFold(c.Env, "production") || strings.EqualFold(c.Env, "prod")
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			return n
		}
	}
	return fallback
}
