package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tokenlive/tokenlive-portal/backend/internal/security"
)

func TestGitHubLoginRedirectsToProvider(t *testing.T) {
	t.Parallel()

	service := &fakeOAuthRouteService{
		githubAuthURL: "https://github.com/login/oauth/authorize?state=state123",
	}
	mux := http.NewServeMux()
	RegisterOAuthRoutes(mux, service, "development")

	req := httptest.NewRequest(http.MethodGet, "/api/auth/github/login", nil)
	rec := httptest.NewRecorder()

	RequestID(mux).ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusFound, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); got != service.githubAuthURL {
		t.Fatalf("Location = %q, want %q", got, service.githubAuthURL)
	}
	if service.githubState == "" {
		t.Fatalf("expected generated github state")
	}
	assertOAuthCookiePath(t, rec, "/api/auth/github/")
}

func TestGitHubCallbackCreatesSession(t *testing.T) {
	t.Parallel()

	service := &fakeOAuthRouteService{
		githubCallbackResult: OAuthLoginResult{
			SessionToken: "tl_sess_github",
			User:         CurrentUser{ID: "usr_gh", TermsAccepted: true},
		},
	}
	mux := http.NewServeMux()
	RegisterOAuthRoutes(mux, service, "production")

	req := httptest.NewRequest(http.MethodGet, "/api/auth/github/callback?state=state123&code=code123", nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: "state123", Path: "/api/auth/github/"})
	rec := httptest.NewRecorder()

	RequestID(mux).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if service.githubCallbackCode != "code123" {
		t.Fatalf("callback code = %q, want code123", service.githubCallbackCode)
	}
	if !strings.Contains(rec.Body.String(), `code: "success"`) {
		t.Fatalf("callback html = %s", rec.Body.String())
	}

	var sessionCookie *http.Cookie
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == security.SessionCookieName {
			sessionCookie = cookie
		}
	}
	if sessionCookie == nil {
		t.Fatalf("expected session cookie")
	}
	if sessionCookie.Value != "tl_sess_github" {
		t.Fatalf("session cookie = %q, want tl_sess_github", sessionCookie.Value)
	}
	if !sessionCookie.Secure {
		t.Fatalf("session cookie should be secure in production")
	}
}

func TestGitHubBindCallbackReturnsProvider(t *testing.T) {
	t.Parallel()

	service := &fakeOAuthRouteService{
		currentUser: CurrentUser{ID: "usr_1"},
		githubBindResult: AccountIdentityDTO{
			Provider: "github",
			Email:    "dev@example.com",
		},
	}
	mux := http.NewServeMux()
	RegisterOAuthRoutes(mux, service, "development")

	req := httptest.NewRequest(http.MethodGet, "/api/auth/github/bind/callback?state=state123&code=bind123", nil)
	req.AddCookie(&http.Cookie{Name: security.SessionCookieName, Value: "tl_sess_live"})
	req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: "state123", Path: "/api/auth/github/"})
	req.AddCookie(&http.Cookie{Name: oauthBindCookieName, Value: "1", Path: "/api/auth/github/"})
	rec := httptest.NewRecorder()

	RequestID(mux).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if service.githubBindToken != "tl_sess_live" {
		t.Fatalf("bind token = %q, want tl_sess_live", service.githubBindToken)
	}
	if service.githubBindCode != "bind123" {
		t.Fatalf("bind code = %q, want bind123", service.githubBindCode)
	}
	if !strings.Contains(rec.Body.String(), `code: "bind_success"`) || !strings.Contains(rec.Body.String(), `provider: "github"`) {
		t.Fatalf("callback html = %s", rec.Body.String())
	}
}

func assertOAuthCookiePath(t *testing.T, rec *httptest.ResponseRecorder, wantPath string) {
	t.Helper()
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == oauthStateCookieName {
			if cookie.Path != wantPath {
				t.Fatalf("oauth cookie path = %q, want %q", cookie.Path, wantPath)
			}
			return
		}
	}
	t.Fatalf("expected oauth state cookie")
}

type fakeOAuthRouteService struct {
	githubAuthURL        string
	githubState          string
	githubCallbackCode   string
	githubCallbackResult OAuthLoginResult
	githubCallbackErr    error
	githubBindToken      string
	githubBindCode       string
	githubBindResult     AccountIdentityDTO
	githubBindErr        error
	currentUser          CurrentUser
	currentUserErr       error
}

func (f *fakeOAuthRouteService) StartEmailLogin(context.Context, string) (StartEmailLoginResult, error) {
	return StartEmailLoginResult{}, nil
}

func (f *fakeOAuthRouteService) VerifyEmailLogin(context.Context, VerifyEmailLoginInput) (VerifyEmailLoginResult, error) {
	return VerifyEmailLoginResult{}, nil
}

func (f *fakeOAuthRouteService) CurrentUser(_ context.Context, _ string) (CurrentUser, error) {
	if f.currentUserErr != nil {
		return CurrentUser{}, f.currentUserErr
	}
	return f.currentUser, nil
}

func (f *fakeOAuthRouteService) Logout(context.Context, string) error {
	return nil
}

func (f *fakeOAuthRouteService) GetGoogleAuthURL(string) string {
	return ""
}

func (f *fakeOAuthRouteService) GetGitHubAuthURL(state string) string {
	f.githubState = state
	return f.githubAuthURL
}

func (f *fakeOAuthRouteService) HandleGoogleCallback(context.Context, string, string, string) (OAuthLoginResult, error) {
	return OAuthLoginResult{}, nil
}

func (f *fakeOAuthRouteService) HandleGitHubCallback(_ context.Context, code string, _, _ string) (OAuthLoginResult, error) {
	f.githubCallbackCode = code
	if f.githubCallbackErr != nil {
		return OAuthLoginResult{}, f.githubCallbackErr
	}
	return f.githubCallbackResult, nil
}

func (f *fakeOAuthRouteService) AcceptTerms(context.Context, string) (AcceptTermsResult, error) {
	return AcceptTermsResult{}, nil
}

func (f *fakeOAuthRouteService) HandleGoogleBind(context.Context, string, string) (AccountIdentityDTO, error) {
	return AccountIdentityDTO{}, nil
}

func (f *fakeOAuthRouteService) HandleGitHubBind(_ context.Context, sessionToken string, code string) (AccountIdentityDTO, error) {
	f.githubBindToken = sessionToken
	f.githubBindCode = code
	if f.githubBindErr != nil {
		return AccountIdentityDTO{}, f.githubBindErr
	}
	return f.githubBindResult, nil
}

func (f *fakeOAuthRouteService) ListAccountIdentities(context.Context, string) ([]AccountIdentityDTO, error) {
	return nil, nil
}
