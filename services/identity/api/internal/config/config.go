// Package config loads and validates the identity service's runtime
// configuration from the environment.
//
// It is the single place .env is loaded: cmd/server, cmd/migrate, and tests
// all go through Load(), so the behavior is identical in every entry point.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config is the identity service's runtime configuration.
// Every field is sourced from an environment variable (see .env.example).
type Config struct {
	// HTTP
	HTTPAddr string // HTTP_ADDR, default ":8080"
	Env      string // ENV, "dev" | "prod"

	// Database
	DatabaseURL string // DATABASE_URL, required
	DBMaxConns  int    // DB_MAX_CONNS, default 10

	// JWT
	//
	// TODO: replace with a key reference once platform/go/auth/keys.go is
	// reviewed. When auth.LoadKeys exists, JWTSecret becomes JWTKeyPath
	// (or a KMS reference) and Load() calls into auth instead of holding
	// the raw secret.
	JWTSecret     string        // JWT_SECRET, required
	JWTAccessTTL  time.Duration // JWT_ACCESS_TTL, default 10m
	JWTRefreshTTL time.Duration // JWT_REFRESH_TTL, default 720h (30d)
	JWTIssuer     string        // JWT_ISSUER, default "campus-identity"
	JWTAudience   string        // JWT_AUDIENCE, default "campus-api"

	// Rate limiting (loose — middleware reads these, spec doesn't promise numbers)
	RateLimitLoginPerMin int // RATE_LIMIT_LOGIN_PER_MIN, default 10

	// Observability
	LogLevel string // LOG_LEVEL, "debug" | "info" | "warn" | "error", default "info"
}

// Load reads the environment and returns a validated Config.
//
// It loads .env from the current directory if present (best-effort: a
// missing file is not an error, because in production there is no .env and
// in dev the Makefile may have already sourced it). A malformed .env is an
// error, so typos fail loudly instead of masquerading as "DATABASE_URL not
// set".
//
// Load fails fast: a missing required variable is an error, not a zero
// value. This is intentional — a zero-valued Config would fail later,
// somewhere less obvious.
func Load() (*Config, error) {
	if err := loadDotenv(); err != nil {
		return nil, err
	}

	c := &Config{
		HTTPAddr:    getenv("HTTP_ADDR", ":8080"),
		Env:         getenv("ENV", "dev"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		DBMaxConns:  getenvInt("DB_MAX_CONNS", 10),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		JWTIssuer:   getenv("JWT_ISSUER", "campus-identity"),
		JWTAudience: getenv("JWT_AUDIENCE", "campus-api"),
		LogLevel:    getenv("LOG_LEVEL", "info"),
	}

	var err error
	if c.JWTAccessTTL, err = getenvDuration("JWT_ACCESS_TTL", 10*time.Minute); err != nil {
		return nil, fmt.Errorf("JWT_ACCESS_TTL: %w", err)
	}
	if c.JWTRefreshTTL, err = getenvDuration("JWT_REFRESH_TTL", 30*24*time.Hour); err != nil {
		return nil, fmt.Errorf("JWT_REFRESH_TTL: %w", err)
	}
	c.RateLimitLoginPerMin = getenvInt("RATE_LIMIT_LOGIN_PER_MIN", 10)

	// Required fields.
	if c.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if c.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	if len(c.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 bytes (got %d)", len(c.JWTSecret))
	}

	return c, nil
}

// loadDotenv loads .env from the current directory if present.
// godotenv does not override variables that are already set, so an explicit
// `export DATABASE_URL=...` always wins over .env.
func loadDotenv() error {
	err := godotenv.Load(".env")
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	return fmt.Errorf("loading .env: %w", err)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getenvDuration(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", v, err)
	}
	return d, nil
}
