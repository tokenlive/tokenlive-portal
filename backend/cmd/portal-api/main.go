package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/tokenlive/tokenlive-portal/backend/internal/api"
	"github.com/tokenlive/tokenlive-portal/backend/internal/config"
	"github.com/tokenlive/tokenlive-portal/backend/internal/database"
	"github.com/tokenlive/tokenlive-portal/backend/internal/repository"
	"github.com/tokenlive/tokenlive-portal/backend/internal/usage"
	"gorm.io/gorm"
)

var (
	openPortalDatabase = func(dsn string) (*gorm.DB, func() error, error) {
		db, err := database.Open(dsn)
		if err != nil {
			return nil, nil, err
		}

		sqlDB, err := db.DB()
		if err != nil {
			return nil, nil, err
		}

		return db, sqlDB.Close, nil
	}
	newPortalRepositories           = repository.New
	newPortalAuthService            = api.NewAuthService
	newPortalConsoleService         = api.NewConsoleServiceWithRuntimeSyncerAndUsage
	newPortalUsageReader            = usage.NewClickHouseReader
	newPortalAPIKeyRuntimeSyncer    = newAPIKeyRuntimeSyncer
	registerPortalPublicModelRoutes = api.RegisterPublicModelRoutes
	registerPortalAuthRoutes        = api.RegisterAuthRoutes
	registerPortalOAuthRoutes       = api.RegisterOAuthRoutes
	registerPortalConsoleRoutes     = api.RegisterConsoleRoutes
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", api.HealthHandler)

	cleanup, err := registerDatabaseBackedRoutes(mux, cfg, log.Default())
	if err != nil {
		log.Fatalf("register database-backed routes: %v", err)
	}
	defer cleanup()

	handler := api.CORS(cfg.CORSAllowedOrigins)(api.RequestID(mux))
	server := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: handler,
	}

	log.Printf("portal-api listening on %s env=%s", cfg.HTTPAddr, cfg.Env)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	select {
	case err := <-serverErr:
		if err != nil {
			log.Fatalf("portal-api stopped: %v", err)
		}
	case <-ctx.Done():
		log.Printf("portal-api shutdown requested")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Fatalf("portal-api shutdown failed: %v", err)
		}
		log.Printf("portal-api stopped")
	}
}

func registerDatabaseBackedRoutes(mux *http.ServeMux, cfg config.Config, logger *log.Logger) (func(), error) {
	if logger == nil {
		logger = log.Default()
	}
	if cfg.DatabaseDSN == "" {
		logger.Printf("portal-api public model routes disabled: PORTAL_DATABASE_DSN is empty")
		logger.Printf("portal-api auth routes disabled: PORTAL_DATABASE_DSN is empty")
		logger.Printf("portal-api console routes disabled: PORTAL_DATABASE_DSN is empty")
		return func() {}, nil
	}

	db, closeDB, err := openPortalDatabase(cfg.DatabaseDSN)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	cleanup := func() {
		if closeDB == nil {
			return
		}
		if err := closeDB(); err != nil {
			logger.Printf("portal-api sql db close failed: %v", err)
		}
	}

	modelRepository := newPortalRepositories(db)
	apiKeyRuntimeSyncer, closeRuntimeSyncer, err := newPortalAPIKeyRuntimeSyncer(cfg.GatewayRedis)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("create api key runtime syncer: %w", err)
	}

	cleanupWithRuntime := func() {
		if closeRuntimeSyncer != nil {
			if err := closeRuntimeSyncer(); err != nil {
				logger.Printf("portal-api api key runtime syncer close failed: %v", err)
			}
		}
		cleanup()
	}

	usageReader, closeUsageReader, err := newPortalUsageReader(cfg.ClickHouse)
	if err != nil {
		cleanupWithRuntime()
		return nil, fmt.Errorf("create usage reader: %w", err)
	}

	cleanupWithUsage := func() {
		if closeUsageReader != nil {
			if err := closeUsageReader(); err != nil {
				logger.Printf("portal-api usage reader close failed: %v", err)
			}
		}
		cleanupWithRuntime()
	}

	authService, err := newPortalAuthService(modelRepository, cfg.Env, cfg.AuthPepper, cfg.TrialCredit, cfg.GoogleOAuth, cfg.GitHubOAuth)
	if err != nil {
		cleanupWithUsage()
		return nil, fmt.Errorf("create auth service: %w", err)
	}

	usageService := usage.NewService(usageReader, func() time.Time { return time.Now().UTC() })
	consoleService, err := newPortalConsoleService(modelRepository, cfg.AuthPepper, cfg.TrialCredit, apiKeyRuntimeSyncer, usageService)
	if err != nil {
		cleanupWithUsage()
		return nil, fmt.Errorf("create console service: %w", err)
	}

	registerPortalPublicModelRoutes(mux, modelRepository)
	registerPortalAuthRoutes(mux, authService, cfg.Env)
	registerPortalOAuthRoutes(mux, authService, cfg.Env)
	registerPortalConsoleRoutes(mux, consoleService, authService)
	api.RegisterInternalRoutes(mux, modelRepository, cfg.InternalAPIToken, apiKeyRuntimeSyncer)

	return cleanupWithUsage, nil
}

func newAPIKeyRuntimeSyncer(cfg config.GatewayRedisConfig) (api.APIKeyRuntimeSyncer, func() error, error) {
	if !cfg.Enabled() {
		return api.NewNoopAPIKeyRuntimeSyncer(), nil, nil
	}
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	return api.NewRedisAPIKeyRuntimeSyncer(client), client.Close, nil
}
