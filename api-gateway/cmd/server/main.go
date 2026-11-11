package main

import (
	"log"
	"net/http"

	"github.com/transcandence/campusplusplus/api-gateway/internal/config"
	"github.com/transcandence/campusplusplus/api-gateway/internal/proxy"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	proxy, err := proxy.NewProxy(cfg.IdentityURL, cfg.HousingURL, cfg.TransportURL)
	if err != nil {
		log.Fatalf("failed to create proxy: %v", err)
	}

	server := &http.Server{
		Addr:    cfg.Port,
		Handler: proxy,
	}

	log.Printf("Starting API Gateway on port %s", cfg.Port)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
