package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tokenlive/tokenlive-portal/backend/internal/security"
)

func TestAuthStartReturnsInvalidEmailError(t *testing.T) {
	t.Parallel()

	service := &fakeAuthService{
		startErr: ErrAuthInvalidEmail,
	}

	mux := http.NewServeMux()
	RegisterAuthRoutes(mux, service, "development")

	req := httptest.NewRequest(http.MethodPost, "/api/auth/email/start", strings.NewReader(`{"email":"bad"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", "req_invalid_email")
	rec := httptest.NewRecorder()

	RequestID(mux).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var body map[string]map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["error"]["code"] != string(CodeAuthInvalidEmail) {
		t.Fatalf("got code %q", body["error"]["code"])
	}
	if body["error"]["request_id"] != "req_invalid_email" {
		t.Fatalf("got request_id %q", body["error"]["request_id"])
	}
	if service.startEmail != "bad" {
		t.Fatalf("service start email = %q, want %q", service.startEmail, "bad")
	}
}

func TestAuthStartReturnsDevCode(t *testing.T) {
	t.Parallel()

	service := &fakeAuthService{
		startResult: StartEmailLoginResult{
			Sent:    true,
			DevCode: "123456",
		},
	}

	mux := http.NewServeMux()
	RegisterAuthRoutes(mux, service, "development")

	req := httptest.NewRequest(http.MethodPost, "/api/auth/email/start", strings.NewReader(`{"email":"dev@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	RequestID(mux).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusOK)
	}

	var body struct {
		Sent    bool   `json:"sent"`
		DevCode string `json:"dev_code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Sent {
		t.Fatalf("sent = false, want true")
	}
	if body.DevCode != "123456" {
		t.Fatalf("dev_code = %q, want %q", body.DevCode, "123456")
	}
}

func TestAuthVerifySetsSessionCookie(t *testing.T) {
	t.Parallel()

	service := &fakeAuthService{
		verifyResult: VerifyEmailLoginResult{
			SessionToken: "tl_sess_test",
			User: CurrentUser{
				ID:            "usr_123",
				DisplayName:   "Dev",
				PrimaryEmail:  "dev@example.com",
				EmailVerified: true,
			},
		},
	}

	mux := http.NewServeMux()
	RegisterAuthRoutes(mux, service, " Production ")

	req := httptest.NewRequest(http.MethodPost, "/api/auth/email/verify", strings.NewReader(`{"email":"dev@example.com","code":"123456"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	RequestID(mux).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusOK)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("expected session cookie")
	}
	cookie := cookies[0]
	if cookie.Name != security.SessionCookieName {
		t.Fatalf("cookie name = %q, want %q", cookie.Name, security.SessionCookieName)
	}
	if cookie.Value != "tl_sess_test" {
		t.Fatalf("cookie value = %q, want %q", cookie.Value, "tl_sess_test")
	}
	if !cookie.HttpOnly {
		t.Fatalf("cookie HttpOnly = false, want true")
	}
	if !cookie.Secure {
		t.Fatalf("cookie Secure = false, want true")
	}
	if cookie.Path != "/" {
		t.Fatalf("cookie Path = %q, want /", cookie.Path)
	}
}

func TestAuthVerifyReturnsInvalidCodeError(t *testing.T) {
	t.Parallel()

	service := &fakeAuthService{
		verifyErr: ErrAuthInvalidCode,
	}

	mux := http.NewServeMux()
	RegisterAuthRoutes(mux, service, "development")

	req := httptest.NewRequest(http.MethodPost, "/api/auth/email/verify", strings.NewReader(`{"email":"dev@example.com","code":"000000"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", "req_invalid_code")
	rec := httptest.NewRecorder()

	RequestID(mux).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusBadRequest)
	}

	assertAuthErrorResponse(t, rec, string(CodeAuthInvalidCode), "req_invalid_code")
}

func TestMeRequiresSessionCookie(t *testing.T) {
	t.Parallel()

	service := &fakeAuthService{}
	mux := http.NewServeMux()
	RegisterAuthRoutes(mux, service, "development")

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("X-Request-ID", "req_me_cookie")
	rec := httptest.NewRecorder()

	RequestID(mux).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	assertAuthErrorResponse(t, rec, string(CodeAuthSessionRequired), "req_me_cookie")
	if service.currentUserCalls != 0 {
		t.Fatalf("CurrentUser calls = %d, want 0", service.currentUserCalls)
	}
}

func TestLogoutClearsSessionCookie(t *testing.T) {
	t.Parallel()

	service := &fakeAuthService{}
	mux := http.NewServeMux()
	RegisterAuthRoutes(mux, service, "production")

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: security.SessionCookieName, Value: "tl_sess_test"})
	rec := httptest.NewRecorder()

	RequestID(mux).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusOK)
	}
	if service.logoutToken != "tl_sess_test" {
		t.Fatalf("logout token = %q, want %q", service.logoutToken, "tl_sess_test")
	}

	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("expected cleared session cookie")
	}
	cookie := cookies[0]
	if cookie.Name != security.SessionCookieName {
		t.Fatalf("cookie name = %q, want %q", cookie.Name, security.SessionCookieName)
	}
	if cookie.Value != "" {
		t.Fatalf("cookie value = %q, want empty", cookie.Value)
	}
	if cookie.MaxAge != -1 {
		t.Fatalf("cookie MaxAge = %d, want -1", cookie.MaxAge)
	}
	if cookie.Expires.IsZero() {
		t.Fatalf("cookie Expires should be set for deletion")
	}
	if !cookie.Secure {
		t.Fatalf("cookie Secure = false, want true")
	}
}

type fakeAuthService struct {
	startEmail       string
	startResult      StartEmailLoginResult
	startErr         error
	verifyInput      VerifyEmailLoginInput
	verifyResult     VerifyEmailLoginResult
	verifyErr        error
	currentUserToken string
	currentUserCalls int
	currentUser      CurrentUser
	currentUserErr   error
	logoutToken      string
	logoutErr        error
}

func (f *fakeAuthService) StartEmailLogin(_ context.Context, email string) (StartEmailLoginResult, error) {
	f.startEmail = email
	if f.startErr != nil {
		return StartEmailLoginResult{}, f.startErr
	}
	return f.startResult, nil
}

func (f *fakeAuthService) VerifyEmailLogin(_ context.Context, input VerifyEmailLoginInput) (VerifyEmailLoginResult, error) {
	f.verifyInput = input
	if f.verifyErr != nil {
		return VerifyEmailLoginResult{}, f.verifyErr
	}
	return f.verifyResult, nil
}

func (f *fakeAuthService) CurrentUser(_ context.Context, sessionToken string) (CurrentUser, error) {
	f.currentUserToken = sessionToken
	f.currentUserCalls++
	if f.currentUserErr != nil {
		return CurrentUser{}, f.currentUserErr
	}
	return f.currentUser, nil
}

func (f *fakeAuthService) Logout(_ context.Context, sessionToken string) error {
	f.logoutToken = sessionToken
	return f.logoutErr
}

func (f *fakeAuthService) GetGoogleAuthURL(string) string { return "" }
func (f *fakeAuthService) GetGitHubAuthURL(string) string { return "" }
func (f *fakeAuthService) HandleGoogleCallback(context.Context, string, string, string) (OAuthLoginResult, error) {
	return OAuthLoginResult{}, nil
}
func (f *fakeAuthService) HandleGitHubCallback(context.Context, string, string, string) (OAuthLoginResult, error) {
	return OAuthLoginResult{}, nil
}
func (f *fakeAuthService) AcceptTerms(context.Context, string) (AcceptTermsResult, error) {
	return AcceptTermsResult{}, nil
}
func (f *fakeAuthService) HandleGoogleBind(context.Context, string, string) (AccountIdentityDTO, error) {
	return AccountIdentityDTO{}, nil
}
func (f *fakeAuthService) HandleGitHubBind(context.Context, string, string) (AccountIdentityDTO, error) {
	return AccountIdentityDTO{}, nil
}
func (f *fakeAuthService) ListAccountIdentities(context.Context, string) ([]AccountIdentityDTO, error) {
	return nil, nil
}

func TestRequireSessionUsesCurrentUser(t *testing.T) {
	t.Parallel()

	service := &fakeAuthService{
		currentUser: CurrentUser{
			ID:            "usr_123",
			PrimaryEmail:  "dev@example.com",
			EmailVerified: true,
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req = req.WithContext(context.WithValue(req.Context(), requestIDKey, "req_protected"))
	req.AddCookie(&http.Cookie{Name: security.SessionCookieName, Value: "tl_sess_test"})
	rec := httptest.NewRecorder()

	var got CurrentUser
	handler := RequireSession(service, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ok bool
		got, ok = CurrentUserFromContext(r.Context())
		if !ok {
			t.Fatalf("expected current user in context")
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusNoContent)
	}
	if got.ID != "usr_123" {
		t.Fatalf("context user id = %q, want %q", got.ID, "usr_123")
	}
	if service.currentUserToken != "tl_sess_test" {
		t.Fatalf("current user token = %q, want %q", service.currentUserToken, "tl_sess_test")
	}
}

func TestRequireSessionMapsExpiredSession(t *testing.T) {
	t.Parallel()

	service := &fakeAuthService{
		currentUserErr: ErrAuthSessionExpired,
	}

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req = req.WithContext(context.WithValue(req.Context(), requestIDKey, "req_expired"))
	req.AddCookie(&http.Cookie{Name: security.SessionCookieName, Value: "expired"})
	rec := httptest.NewRecorder()

	handler := RequireSession(service, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("next handler should not be called")
	}))

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	assertAuthErrorResponse(t, rec, string(CodeAuthSessionExpired), "req_expired")
}

func TestMapAuthErrorMapsUnauthorizedSentinel(t *testing.T) {
	t.Parallel()

	if got := mapAuthError(ErrAuthUnauthorized); got != ErrUnauthorized {
		t.Fatalf("mapAuthError() = %+v, want %+v", got, ErrUnauthorized)
	}
}

func TestMapAuthErrorFallsBackToInternal(t *testing.T) {
	t.Parallel()

	if got := mapAuthError(errors.New("boom")); got != ErrInternalError {
		t.Fatalf("mapAuthError() = %+v, want %+v", got, ErrInternalError)
	}
}

func assertAuthErrorResponse(t *testing.T, rec *httptest.ResponseRecorder, wantCode string, wantRequestID string) {
	t.Helper()

	var body map[string]map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["error"]["code"] != wantCode {
		t.Fatalf("got code %q, want %q", body["error"]["code"], wantCode)
	}
	if body["error"]["request_id"] != wantRequestID {
		t.Fatalf("got request_id %q, want %q", body["error"]["request_id"], wantRequestID)
	}
}
