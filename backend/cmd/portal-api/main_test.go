package main

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tokenlive/tokenlive-portal/backend/internal/api"
	"github.com/tokenlive/tokenlive-portal/backend/internal/config"
	"github.com/tokenlive/tokenlive-portal/backend/internal/domain"
	"github.com/tokenlive/tokenlive-portal/backend/internal/repository"
	"gorm.io/gorm"
)

func TestRegisterDatabaseBackedRoutesDisablesAuthPublicAndConsoleRoutesWithoutDSN(t *testing.T) {
	mux := http.NewServeMux()
	var logs bytes.Buffer
	logger := log.New(&logs, "", 0)

	cleanup, err := registerDatabaseBackedRoutes(mux, config.Config{
		Env:         "development",
		DatabaseDSN: "",
	}, logger)
	if err != nil {
		t.Fatalf("register database backed routes: %v", err)
	}
	cleanup()

	for _, path := range []string{"/api/auth/email/start", "/api/public/models", "/api/workspaces/current", "/api/console/overview", "/api/api-keys"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want %d", path, rec.Code, http.StatusNotFound)
		}
	}

	logOutput := logs.String()
	if !strings.Contains(logOutput, "public model routes disabled") {
		t.Fatalf("logs = %q, want public model disabled message", logOutput)
	}
	if !strings.Contains(logOutput, "auth routes disabled") {
		t.Fatalf("logs = %q, want auth disabled message", logOutput)
	}
	if !strings.Contains(logOutput, "console routes disabled") {
		t.Fatalf("logs = %q, want console disabled message", logOutput)
	}
}

func TestRegisterDatabaseBackedRoutesRegistersPublicAuthAndConsoleRoutes(t *testing.T) {
	restore := stubPortalRouteSeams(t)
	defer restore()

	mux := http.NewServeMux()
	var logs bytes.Buffer
	logger := log.New(&logs, "", 0)

	cleanup, err := registerDatabaseBackedRoutes(mux, config.Config{
		Env:         "development",
		DatabaseDSN: "test-dsn",
		AuthPepper:  "pepper",
		TrialCredit: config.TrialCreditConfig{
			AmountMicroCNY: 10_000_000,
			TTLDays:        7,
		},
	}, logger)
	if err != nil {
		t.Fatalf("register database backed routes: %v", err)
	}
	cleanup()

	tests := []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/public/models"},
		{method: http.MethodPost, path: "/api/auth/email/start"},
		{method: http.MethodGet, path: "/api/console/overview"},
		{method: http.MethodGet, path: "/api/workspaces/current"},
		{method: http.MethodGet, path: "/api/api-keys"},
	}
	for _, tt := range tests {
		req := httptest.NewRequest(tt.method, tt.path, nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code == http.StatusNotFound {
			t.Fatalf("%s %s status = %d, want registered route", tt.method, tt.path, rec.Code)
		}
	}
	if capturedPortalAuthTrialCredit.AmountMicroCNY != 10_000_000 {
		t.Fatalf("trial amount = %d, want 10000000", capturedPortalAuthTrialCredit.AmountMicroCNY)
	}
	if capturedPortalAuthTrialCredit.TTLDays != 7 {
		t.Fatalf("trial ttl = %d, want 7", capturedPortalAuthTrialCredit.TTLDays)
	}
	if capturedPortalConsoleTrialCredit.AmountMicroCNY != 10_000_000 {
		t.Fatalf("console trial amount = %d, want 10000000", capturedPortalConsoleTrialCredit.AmountMicroCNY)
	}
	if capturedPortalConsoleTrialCredit.TTLDays != 7 {
		t.Fatalf("console trial ttl = %d, want 7", capturedPortalConsoleTrialCredit.TTLDays)
	}
}

func TestRegisterDatabaseBackedRoutesDoesNotMutateMuxWhenConsoleServiceFails(t *testing.T) {
	restore := stubPortalRouteSeams(t)
	defer restore()

	cleanupCalled := false
	openPortalDatabase = func(_ string) (*gorm.DB, func() error, error) {
		return &gorm.DB{}, func() error {
			cleanupCalled = true
			return nil
		}, nil
	}
	newPortalConsoleService = func(_ *repository.Repositories, _ string, _ config.TrialCreditConfig) (api.ConsoleService, error) {
		return nil, errors.New("console constructor failed")
	}

	mux := http.NewServeMux()
	cleanup, err := registerDatabaseBackedRoutes(mux, config.Config{
		Env:         "development",
		DatabaseDSN: "test-dsn",
		AuthPepper:  "pepper",
	}, log.New(&bytes.Buffer{}, "", 0))
	if err == nil {
		if cleanup != nil {
			cleanup()
		}
		t.Fatalf("expected console service error")
	}
	if !strings.Contains(err.Error(), "create console service") {
		t.Fatalf("err = %v, want console service context", err)
	}
	if !cleanupCalled {
		t.Fatalf("database cleanup was not called")
	}

	for _, path := range []string{"/api/public/models", "/api/auth/email/start", "/api/workspaces/current", "/api/console/overview", "/api/api-keys"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want %d", path, rec.Code, http.StatusNotFound)
		}
	}
}

func stubPortalRouteSeams(t *testing.T) func() {
	t.Helper()

	originalOpenDatabase := openPortalDatabase
	originalNewRepositories := newPortalRepositories
	originalNewAuthService := newPortalAuthService
	originalNewConsoleService := newPortalConsoleService
	originalRegisterPublicModelRoutes := registerPortalPublicModelRoutes
	originalRegisterAuthRoutes := registerPortalAuthRoutes
	originalRegisterConsoleRoutes := registerPortalConsoleRoutes

	openPortalDatabase = func(_ string) (*gorm.DB, func() error, error) {
		return &gorm.DB{}, func() error { return nil }, nil
	}
	newPortalRepositories = func(_ *gorm.DB) *repository.Repositories {
		return &repository.Repositories{}
	}
	capturedPortalAuthTrialCredit = config.TrialCreditConfig{}
	capturedPortalConsoleTrialCredit = config.TrialCreditConfig{}
	newPortalAuthService = func(_ *repository.Repositories, _ string, _ string, trialCredit config.TrialCreditConfig) (api.AuthService, error) {
		capturedPortalAuthTrialCredit = trialCredit
		return fakePortalAuthService{}, nil
	}
	newPortalConsoleService = func(_ *repository.Repositories, _ string, trialCredit config.TrialCreditConfig) (api.ConsoleService, error) {
		capturedPortalConsoleTrialCredit = trialCredit
		return fakePortalConsoleService{}, nil
	}
	registerPortalPublicModelRoutes = func(mux *http.ServeMux, _ api.PublicModelReader) {
		mux.HandleFunc("GET /api/public/models", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})
	}
	registerPortalAuthRoutes = func(mux *http.ServeMux, _ api.AuthService, _ string) {
		mux.HandleFunc("POST /api/auth/email/start", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})
	}
	registerPortalConsoleRoutes = func(mux *http.ServeMux, _ api.ConsoleService, _ api.AuthService) {
		mux.HandleFunc("GET /api/console/overview", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})
		mux.HandleFunc("GET /api/workspaces/current", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})
		mux.HandleFunc("GET /api/api-keys", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})
	}

	return func() {
		openPortalDatabase = originalOpenDatabase
		newPortalRepositories = originalNewRepositories
		newPortalAuthService = originalNewAuthService
		newPortalConsoleService = originalNewConsoleService
		registerPortalPublicModelRoutes = originalRegisterPublicModelRoutes
		registerPortalAuthRoutes = originalRegisterAuthRoutes
		registerPortalConsoleRoutes = originalRegisterConsoleRoutes
	}
}

var capturedPortalAuthTrialCredit config.TrialCreditConfig
var capturedPortalConsoleTrialCredit config.TrialCreditConfig

type fakePortalAuthService struct{}

func (fakePortalAuthService) StartEmailLogin(context.Context, string) (api.StartEmailLoginResult, error) {
	return api.StartEmailLoginResult{}, nil
}

func (fakePortalAuthService) VerifyEmailLogin(context.Context, api.VerifyEmailLoginInput) (api.VerifyEmailLoginResult, error) {
	return api.VerifyEmailLoginResult{}, nil
}

func (fakePortalAuthService) CurrentUser(context.Context, string) (api.CurrentUser, error) {
	return api.CurrentUser{}, nil
}

func (fakePortalAuthService) Logout(context.Context, string) error {
	return nil
}

type fakePortalConsoleService struct{}

func (fakePortalConsoleService) Overview(context.Context, api.CurrentUser) (api.ConsoleOverviewResponse, error) {
	return api.ConsoleOverviewResponse{}, nil
}

func (fakePortalConsoleService) CurrentWorkspace(context.Context, api.CurrentUser) (api.CurrentWorkspaceResponse, error) {
	return api.CurrentWorkspaceResponse{}, nil
}

func (fakePortalConsoleService) ListAPIKeys(context.Context, api.CurrentUser) (api.ListAPIKeysResponse, error) {
	return api.ListAPIKeysResponse{}, nil
}

func (fakePortalConsoleService) CreateAPIKey(context.Context, api.CurrentUser, api.CreateAPIKeyRequest) (api.CreateAPIKeyResponse, error) {
	return api.CreateAPIKeyResponse{}, nil
}

func (fakePortalConsoleService) EnableAPIKey(context.Context, api.CurrentUser, string) (api.APIKeyResponse, error) {
	return api.APIKeyResponse{Status: domain.APIKeyStatusEnabled}, nil
}

func (fakePortalConsoleService) DisableAPIKey(context.Context, api.CurrentUser, string) (api.APIKeyResponse, error) {
	return api.APIKeyResponse{Status: domain.APIKeyStatusDisabled}, nil
}

func (fakePortalConsoleService) RevokeAPIKey(context.Context, api.CurrentUser, string) (api.APIKeyResponse, error) {
	return api.APIKeyResponse{Status: domain.APIKeyStatusRevoked}, nil
}
