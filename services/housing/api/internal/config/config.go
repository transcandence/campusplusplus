// Package config loads and validates the housing service's runtime
// configuration from the environment.
package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPAddr    string
	Env         string
	DatabaseURL string
	DBMaxConns  int
	LogLevel    string
}

func Load() (*Config, error) {
	if err := loadDotenv(); err != nil {
		return nil, err
	}

	c := &Config{
		HTTPAddr:    getenv("HTTP_ADDR", ":8080"),
		Env:         getenv("ENV", "dev"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		DBMaxConns:  getenvInt("DB_MAX_CONNS", 10),
		LogLevel:    getenv("LOG_LEVEL", "info"),
	}

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
