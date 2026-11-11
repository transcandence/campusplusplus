package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port         string
	IdentityURL  string
	HousingURL   string
	TransportURL string
	JWTPublicKeyPath string
}

func Load() (*Config, error) {
	_ = godotenv.Load("../../.env")
	return LoadFromEnv()
}

func LoadFromEnv() (*Config, error) {
	cfg := &Config{
		Port:         getEnv("PORT", ":8080"),
		IdentityURL:  os.Getenv("IDENTITY_URL"),
		HousingURL:   os.Getenv("HOUSING_URL"),
		TransportURL: os.Getenv("TRANSPORT_URL"),
		JWTPublicKeyPath: os.Getenv("JWT_PUBLIC_KEY_PATH"),
	}

	if !strings.HasPrefix(cfg.Port, ":") {
		cfg.Port = ":" + cfg.Port
	}

	if cfg.IdentityURL == "" {
		return nil, fmt.Errorf("missing required environment variable: IDENTITY_URL")
	}
	if cfg.HousingURL == "" {
		return nil, fmt.Errorf("missing required environment variable: HOUSING_URL")
	}
	if cfg.TransportURL == "" {
		return nil, fmt.Errorf("missing required environment variable: TRANSPORT_URL")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}