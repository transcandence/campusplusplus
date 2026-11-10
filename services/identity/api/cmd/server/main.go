package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/transcandence/campusplusplus/platform/go/auth"
	"github.com/transcandence/campusplusplus/services/identity/api/internal/config"
	"github.com/transcandence/campusplusplus/services/identity/api/internal/handler"
	"github.com/transcandence/campusplusplus/services/identity/api/internal/repository"
	"github.com/transcandence/campusplusplus/services/identity/api/internal/service"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	// Database connection pool
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("parsing database url: %w", err)
	}
	poolConfig.MaxConns = int32(cfg.DBMaxConns)
	dbPool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer dbPool.Close()
	if err := dbPool.Ping(ctx); err != nil {
		return fmt.Errorf("pinging database: %w", err)
	}
	// RSA Keypair for JWT issuance and verification
	privKeyPath := cfg.JWTPrivateKeyPath
	pubKeyPath := cfg.JWTPublicKeyPath

	if _, err := os.Stat(privKeyPath); os.IsNotExist(err) {
		if cfg.Env == "prod" {
			return fmt.Errorf("private key not found at %s in production: provision keys before starting", privKeyPath)
		}
		// In dev, auto-generate so local onboarding works out of the box
		fmt.Fprintf(os.Stderr, "WARNING: RSA keys not found at %s. Generating local development keys...\n", privKeyPath)
		if err := auth.GenerateKeys(); err != nil {
			return fmt.Errorf("generating dev rsa keys: %w", err)
		}
	}

	privateKey, err := auth.LoadPrivateKey(privKeyPath)
	if err != nil {
		return fmt.Errorf("loading private key from %s: %w", privKeyPath, err)
	}

	publicKey, err := auth.LoadPublicKey(pubKeyPath)
	if err != nil {
		return fmt.Errorf("loading public key from %s: %w", pubKeyPath, err)
	}
	userRepo := repository.NewUserRepository(dbPool)
	sessionRepo := repository.NewSessionRepository(dbPool)
	authService := service.NewAuthService(
		userRepo,
		sessionRepo,
		privateKey,
		cfg.JWTAccessTTL,
		cfg.JWTRefreshTTL,
	)
	authHandler := handler.NewAuthHandler(authService, publicKey)
	if cfg.Env == "prod" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gin.Logger())
	router.Use(handler.RequestIDMiddleware())
	router.Use(handler.CORSMiddleware())

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	handler.RegisterHandlersWithOptions(router, authHandler, handler.GinServerOptions{
		BaseURL: "",
		Middlewares: []handler.MiddlewareFunc{
			handler.BearerAuthMiddleware(publicKey),
		},
		ErrorHandler: handler.CustomErrorHandler,
	})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("identity-service listening on %s (env=%s)", cfg.HTTPAddr, cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case <-ctx.Done():
		log.Println("shutting down identity-service gracefully...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("server shutdown: %w", err)
		}
		log.Println("identity-service stopped.")
		return nil
	case err := <-serverErr:
		return fmt.Errorf("server listen: %w", err)
	}
}
