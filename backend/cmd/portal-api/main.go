package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/tokenlive/tokenlive-portal/backend/internal/api"
	"github.com/tokenlive/tokenlive-portal/backend/internal/config"
	"github.com/tokenlive/tokenlive-portal/backend/internal/database"
	"github.com/tokenlive/tokenlive-portal/backend/internal/repository"
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
	newPortalConsoleService         = api.NewConsoleService
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

	handler := api.RequestID(mux)
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

	authService, err := newPortalAuthService(modelRepository, cfg.Env, cfg.AuthPepper, cfg.TrialCredit, cfg.GoogleOAuth)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("create auth service: %w", err)
	}

	consoleService, err := newPortalConsoleService(modelRepository, cfg.AuthPepper, cfg.TrialCredit)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("create console service: %w", err)
	}

	registerPortalPublicModelRoutes(mux, modelRepository)
	registerPortalAuthRoutes(mux, authService, cfg.Env)
	registerPortalOAuthRoutes(mux, authService, cfg.Env)
	registerPortalConsoleRoutes(mux, consoleService, authService)
	api.RegisterInternalRoutes(mux, modelRepository, cfg.InternalAPIToken)

	return cleanup, nil
}
