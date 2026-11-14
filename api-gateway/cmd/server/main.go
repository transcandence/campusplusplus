package main

import (
	"log"
	"net/http"

	"github.com/transcandence/campusplusplus/api-gateway/internal/config"
	"github.com/transcandence/campusplusplus/api-gateway/internal/middleware"
	"github.com/transcandence/campusplusplus/api-gateway/internal/proxy"
	"github.com/transcandence/campusplusplus/platform/go/auth"
)

func main() {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	publicKey, err := auth.LoadPublicKey(cfg.JWTPublicKeyPath)
	if err != nil {
		log.Fatalf("failed to load public key: %v", err)
	}

	gatewayProxy, err := proxy.NewProxy(cfg.IdentityURL, cfg.HousingURL, cfg.TransportURL)
	if err != nil {
		log.Fatalf("failed to create proxy: %v", err)
	}

	protectedProxy := middleware.JwtAuth(publicKey, gatewayProxy)

	server := &http.Server{
		Addr:    cfg.Port,
		Handler: protectedProxy,
	}

	log.Printf("Starting API Gateway on port %s", cfg.Port)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}