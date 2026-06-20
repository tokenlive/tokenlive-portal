package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tokenlive/tokenlive-portal/backend/internal/domain"
	"github.com/tokenlive/tokenlive-portal/backend/internal/security"
)

func TestConsoleCurrentWorkspaceRequiresSession(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	RegisterConsoleRoutes(mux, &fakeConsoleService{}, &fakeAuthService{})
	req := httptest.NewRequest(http.MethodGet, "/api/workspaces/current", nil)
	req.Header.Set("X-Request-ID", "req_console_session")
	rec := httptest.NewRecorder()

	RequestID(mux).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	assertAuthErrorResponse(t, rec, string(CodeAuthSessionRequired), "req_console_session")
}

func TestConsoleCurrentWorkspaceReturnsWorkspace(t *testing.T) {
	t.Parallel()

	trialGrantedAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	console := &fakeConsoleService{
		currentWorkspaceResult: CurrentWorkspaceResponse{
			Workspace: WorkspaceResponse{
				ID:             "wsp_1",
				Name:           "Dev",
				Slug:           "dev",
				Role:           domain.MemberRoleOwner,
				Status:         domain.WorkspaceStatusActive,
				TrialGrantedAt: &trialGrantedAt,
				Balance: WorkspaceBalanceResponse{
					AvailableMicroCNY: 10_000_000,
					AvailableCNY:      "10.000000",
				},
			},
		},
	}
	auth := &fakeAuthService{currentUser: CurrentUser{ID: "usr_1"}}
	mux := http.NewServeMux()
	RegisterConsoleRoutes(mux, console, auth)

	req := httptest.NewRequest(http.MethodGet, "/api/workspaces/current", nil)
	req.AddCookie(&http.Cookie{Name: security.SessionCookieName, Value: "tl_sess_test"})
	rec := httptest.NewRecorder()

	RequestID(mux).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body CurrentWorkspaceResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Workspace.ID != "wsp_1" || body.Workspace.Balance.AvailableCNY != "10.000000" {
		t.Fatalf("body = %+v", body)
	}
	if body.Workspace.TrialGrantedAt == nil || !body.Workspace.TrialGrantedAt.Equal(trialGrantedAt) {
		t.Fatalf("trial_granted_at = %v, want %v", body.Workspace.TrialGrantedAt, trialGrantedAt)
	}
	if auth.currentUserToken != "tl_sess_test" {
		t.Fatalf("current user token = %q, want %q", auth.currentUserToken, "tl_sess_test")
	}
}

func TestConsoleOverviewRequiresSession(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	RegisterConsoleRoutes(mux, &fakeConsoleService{}, &fakeAuthService{})
	req := httptest.NewRequest(http.MethodGet, "/api/console/overview", nil)
	req.Header.Set("X-Request-ID", "req_console_overview_session")
	rec := httptest.NewRecorder()

	RequestID(mux).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	assertAuthErrorResponse(t, rec, string(CodeAuthSessionRequired), "req_console_overview_session")
}

func TestConsoleOverviewReturnsActivationShape(t *testing.T) {
	t.Parallel()

	trialGrantedAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	trialExpiresAt := trialGrantedAt.AddDate(0, 0, 7)
	console := &fakeConsoleService{
		overviewResult: ConsoleOverviewResponse{
			Workspace: WorkspaceResponse{
				ID:             "wsp_1",
				Name:           "Dev",
				Slug:           "dev",
				Role:           domain.MemberRoleOwner,
				Status:         domain.WorkspaceStatusActive,
				TrialGrantedAt: &trialGrantedAt,
				Balance: WorkspaceBalanceResponse{
					AvailableMicroCNY: 10_000_000,
					AvailableCNY:      "10.000000",
				},
			},
			Activation: ActivationOverviewResponse{
				TrialCreditGranted: true,
				TrialExpiresAt:     &trialExpiresAt,
				APIKeyCreated:      true,
				FirstCallMade:      false,
				Steps: []ActivationStepResponse{
					{Key: "trial_credit", Label: "Trial credit granted", Status: ActivationStepCompleted},
					{Key: "api_key", Label: "Create an API key", Status: ActivationStepCompleted},
					{Key: "first_call", Label: "Make your first API call", Status: ActivationStepPending},
				},
			},
		},
	}
	auth := &fakeAuthService{currentUser: CurrentUser{ID: "usr_1"}}
	mux := http.NewServeMux()
	RegisterConsoleRoutes(mux, console, auth)

	req := httptest.NewRequest(http.MethodGet, "/api/console/overview", nil)
	req.AddCookie(&http.Cookie{Name: security.SessionCookieName, Value: "tl_sess_test"})
	rec := httptest.NewRecorder()

	RequestID(mux).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body ConsoleOverviewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Workspace.ID != "wsp_1" || body.Workspace.Balance.AvailableCNY != "10.000000" {
		t.Fatalf("workspace = %+v", body.Workspace)
	}
	if !body.Activation.TrialCreditGranted || body.Activation.TrialExpiresAt == nil || !body.Activation.TrialExpiresAt.Equal(trialExpiresAt) {
		t.Fatalf("activation trial = %+v", body.Activation)
	}
	if !body.Activation.APIKeyCreated || body.Activation.FirstCallMade {
		t.Fatalf("activation api/first call = %+v", body.Activation)
	}
	if len(body.Activation.Steps) != 3 || body.Activation.Steps[2].Key != "first_call" || body.Activation.Steps[2].Status != ActivationStepPending {
		t.Fatalf("activation steps = %+v", body.Activation.Steps)
	}
	if console.overviewUser.ID != "usr_1" {
		t.Fatalf("overview user id = %q, want %q", console.overviewUser.ID, "usr_1")
	}
}

func TestConsoleListAPIKeysMapsPermissionDenied(t *testing.T) {
	t.Parallel()

	console := &fakeConsoleService{listAPIKeysErr: ErrConsolePermissionDenied}
	auth := &fakeAuthService{currentUser: CurrentUser{ID: "usr_1"}}
	mux := http.NewServeMux()
	RegisterConsoleRoutes(mux, console, auth)

	req := httptest.NewRequest(http.MethodGet, "/api/api-keys", nil)
	req.Header.Set("X-Request-ID", "req_permission")
	req.AddCookie(&http.Cookie{Name: security.SessionCookieName, Value: "tl_sess_test"})
	rec := httptest.NewRecorder()

	RequestID(mux).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	assertAuthErrorResponse(t, rec, string(CodePermissionDenied), "req_permission")
}

func TestConsoleCreateAPIKeyReturnsSecretAndPassesRequest(t *testing.T) {
	t.Parallel()

	dailyLimit := int64(1_000_000)
	console := &fakeConsoleService{
		createAPIKeyResult: CreateAPIKeyResponse{
			APIKey: APIKeyResponse{ID: "ak_1", Name: "local dev", SecretLast4: "wxyz"},
			Secret: "tl_live_secret",
		},
	}
	auth := &fakeAuthService{currentUser: CurrentUser{ID: "usr_1"}}
	mux := http.NewServeMux()
	RegisterConsoleRoutes(mux, console, auth)

	req := httptest.NewRequest(http.MethodPost, "/api/api-keys", strings.NewReader(`{"name":"local dev","daily_limit_micro_cny":1000000}`))
	req.AddCookie(&http.Cookie{Name: security.SessionCookieName, Value: "tl_sess_test"})
	rec := httptest.NewRecorder()

	RequestID(mux).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body CreateAPIKeyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Secret != "tl_live_secret" {
		t.Fatalf("secret = %q, want creation secret", body.Secret)
	}
	if console.createAPIKeyInput.Name != "local dev" {
		t.Fatalf("input name = %q", console.createAPIKeyInput.Name)
	}
	if console.createAPIKeyInput.DailyLimitMicroCNY == nil || *console.createAPIKeyInput.DailyLimitMicroCNY != dailyLimit {
		t.Fatalf("daily limit = %v, want %d", console.createAPIKeyInput.DailyLimitMicroCNY, dailyLimit)
	}
	if console.createAPIKeyUser.ID != "usr_1" {
		t.Fatalf("create user id = %q, want %q", console.createAPIKeyUser.ID, "usr_1")
	}
}

func TestConsoleCreateAPIKeyInvalidJSON(t *testing.T) {
	t.Parallel()

	console := &fakeConsoleService{}
	auth := &fakeAuthService{currentUser: CurrentUser{ID: "usr_1"}}
	mux := http.NewServeMux()
	RegisterConsoleRoutes(mux, console, auth)

	req := httptest.NewRequest(http.MethodPost, "/api/api-keys", strings.NewReader(`{"name":`))
	req.Header.Set("X-Request-ID", "req_bad_json")
	req.AddCookie(&http.Cookie{Name: security.SessionCookieName, Value: "tl_sess_test"})
	rec := httptest.NewRecorder()

	RequestID(mux).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	assertAuthErrorResponse(t, rec, string(CodeInvalidRequest), "req_bad_json")
}

func TestConsoleCreateAPIKeyRejectsTrailingJSON(t *testing.T) {
	t.Parallel()

	console := &fakeConsoleService{}
	auth := &fakeAuthService{currentUser: CurrentUser{ID: "usr_1"}}
	mux := http.NewServeMux()
	RegisterConsoleRoutes(mux, console, auth)

	req := httptest.NewRequest(http.MethodPost, "/api/api-keys", strings.NewReader(`{"name":"local dev"} {"name":"extra"}`))
	req.Header.Set("X-Request-ID", "req_trailing_json")
	req.AddCookie(&http.Cookie{Name: security.SessionCookieName, Value: "tl_sess_test"})
	rec := httptest.NewRecorder()

	RequestID(mux).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	assertAuthErrorResponse(t, rec, string(CodeInvalidRequest), "req_trailing_json")
	if console.createAPIKeyInput.Name != "" {
		t.Fatalf("service should not receive invalid request, got input %+v", console.createAPIKeyInput)
	}
}

func TestConsoleCreateAPIKeyRejectsUnknownFields(t *testing.T) {
	t.Parallel()

	console := &fakeConsoleService{}
	auth := &fakeAuthService{currentUser: CurrentUser{ID: "usr_1"}}
	mux := http.NewServeMux()
	RegisterConsoleRoutes(mux, console, auth)

	req := httptest.NewRequest(http.MethodPost, "/api/api-keys", strings.NewReader(`{"name":"local dev","unknown":true}`))
	req.Header.Set("X-Request-ID", "req_unknown_field")
	req.AddCookie(&http.Cookie{Name: security.SessionCookieName, Value: "tl_sess_test"})
	rec := httptest.NewRecorder()

	RequestID(mux).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	assertAuthErrorResponse(t, rec, string(CodeInvalidRequest), "req_unknown_field")
	if console.createAPIKeyInput.Name != "" {
		t.Fatalf("service should not receive invalid request, got input %+v", console.createAPIKeyInput)
	}
}

func TestConsoleRegisterRoutesRequiresService(t *testing.T) {
	t.Parallel()

	assertPanicMessage(t, "console service is required", func() {
		RegisterConsoleRoutes(http.NewServeMux(), nil, &fakeAuthService{})
	})
}

func TestConsoleRegisterRoutesRequiresAuthService(t *testing.T) {
	t.Parallel()

	assertPanicMessage(t, "auth service is required", func() {
		RegisterConsoleRoutes(http.NewServeMux(), &fakeConsoleService{}, nil)
	})
}

func TestConsoleAPIKeyStateRoutes(t *testing.T) {
	t.Parallel()

	routes := []struct {
		name       string
		path       string
		wantStatus domain.APIKeyStatus
	}{
		{name: "enable", path: "/api/api-keys/%20ak_1%20/enable", wantStatus: domain.APIKeyStatusEnabled},
		{name: "disable", path: "/api/api-keys/%20ak_1%20/disable", wantStatus: domain.APIKeyStatusDisabled},
		{name: "revoke", path: "/api/api-keys/%20ak_1%20/revoke", wantStatus: domain.APIKeyStatusRevoked},
	}

	for _, tt := range routes {
		t.Run(tt.name, func(t *testing.T) {
			console := &fakeConsoleService{stateResult: APIKeyResponse{ID: "ak_1", Status: tt.wantStatus}}
			auth := &fakeAuthService{currentUser: CurrentUser{ID: "usr_1"}}
			mux := http.NewServeMux()
			RegisterConsoleRoutes(mux, console, auth)

			req := httptest.NewRequest(http.MethodPost, tt.path, nil)
			req.AddCookie(&http.Cookie{Name: security.SessionCookieName, Value: "tl_sess_test"})
			rec := httptest.NewRecorder()

			RequestID(mux).ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			if console.stateAPIKeyID != "ak_1" || console.stateStatus != tt.wantStatus {
				t.Fatalf("state api key id=%s status=%s", console.stateAPIKeyID, console.stateStatus)
			}
		})
	}
}

func assertPanicMessage(t *testing.T, want string, fn func()) {
	t.Helper()

	defer func() {
		got := recover()
		if got == nil {
			t.Fatalf("expected panic %q", want)
		}
		if got != want {
			t.Fatalf("panic = %q, want %q", got, want)
		}
	}()
	fn()
}

type fakeConsoleService struct {
	overviewResult         ConsoleOverviewResponse
	overviewErr            error
	overviewUser           CurrentUser
	currentWorkspaceResult CurrentWorkspaceResponse
	currentWorkspaceErr    error
	listAPIKeysResult      ListAPIKeysResponse
	listAPIKeysErr         error
	createAPIKeyInput      CreateAPIKeyRequest
	createAPIKeyUser       CurrentUser
	createAPIKeyResult     CreateAPIKeyResponse
	createAPIKeyErr        error
	stateAPIKeyID          string
	stateStatus            domain.APIKeyStatus
	stateResult            APIKeyResponse
	stateErr               error
}

func (f *fakeConsoleService) Overview(_ context.Context, user CurrentUser) (ConsoleOverviewResponse, error) {
	f.overviewUser = user
	return f.overviewResult, f.overviewErr
}

func (f *fakeConsoleService) CurrentWorkspace(_ context.Context, _ CurrentUser) (CurrentWorkspaceResponse, error) {
	return f.currentWorkspaceResult, f.currentWorkspaceErr
}

func (f *fakeConsoleService) ListAPIKeys(_ context.Context, _ CurrentUser) (ListAPIKeysResponse, error) {
	return f.listAPIKeysResult, f.listAPIKeysErr
}

func (f *fakeConsoleService) CreateAPIKey(_ context.Context, user CurrentUser, input CreateAPIKeyRequest) (CreateAPIKeyResponse, error) {
	f.createAPIKeyUser = user
	f.createAPIKeyInput = input
	return f.createAPIKeyResult, f.createAPIKeyErr
}

func (f *fakeConsoleService) EnableAPIKey(_ context.Context, _ CurrentUser, apiKeyID string) (APIKeyResponse, error) {
	f.stateAPIKeyID = apiKeyID
	f.stateStatus = domain.APIKeyStatusEnabled
	return f.stateResult, f.stateErr
}

func (f *fakeConsoleService) DisableAPIKey(_ context.Context, _ CurrentUser, apiKeyID string) (APIKeyResponse, error) {
	f.stateAPIKeyID = apiKeyID
	f.stateStatus = domain.APIKeyStatusDisabled
	return f.stateResult, f.stateErr
}

func (f *fakeConsoleService) RevokeAPIKey(_ context.Context, _ CurrentUser, apiKeyID string) (APIKeyResponse, error) {
	f.stateAPIKeyID = apiKeyID
	f.stateStatus = domain.APIKeyStatusRevoked
	return f.stateResult, f.stateErr
}
