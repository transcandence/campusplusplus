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

type Config struct {
	HTTPAddr             string
	Env                  string
	DatabaseURL          string
	DBMaxConns           int
	JWTPrivateKeyPath    string
	JWTPublicKeyPath     string
	JWTAccessTTL         time.Duration
	JWTRefreshTTL        time.Duration
	JWTIssuer            string
	JWTAudience          string
	RateLimitLoginPerMin int
	LogLevel             string
}

func Load() (*Config, error) {
	if err := loadDotenv(); err != nil {
		return nil, err
	}

	c := &Config{
		HTTPAddr:          getenv("HTTP_ADDR", ":8080"),
		Env:               getenv("ENV", "dev"),
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		DBMaxConns:        getenvInt("DB_MAX_CONNS", 10),
		JWTPrivateKeyPath: getenv("JWT_PRIVATE_KEY_PATH", "private.pem"),
		JWTPublicKeyPath:  getenv("JWT_PUBLIC_KEY_PATH", "public.pem"),
		JWTIssuer:         getenv("JWT_ISSUER", "campus-identity"),
		JWTAudience:       getenv("JWT_AUDIENCE", "campus-api"),
		LogLevel:          getenv("LOG_LEVEL", "info"),
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

	return c, nil
}

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
