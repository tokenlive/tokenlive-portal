package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/tokenlive/tokenlive-portal/backend/internal/security"
)

const sessionCookieTTL = 30 * 24 * time.Hour

var (
	ErrAuthInvalidEmail    = errors.New("auth invalid email")
	ErrAuthInvalidCode     = errors.New("auth invalid code")
	ErrAuthUnauthorized    = errors.New("auth unauthorized")
	ErrAuthSessionRequired = errors.New("auth session required")
	ErrAuthSessionExpired  = errors.New("auth session expired")
)

type authContextKey string

const currentUserKey authContextKey = "current_user"

type AuthService interface {
	StartEmailLogin(ctx context.Context, email string) (StartEmailLoginResult, error)
	VerifyEmailLogin(ctx context.Context, input VerifyEmailLoginInput) (VerifyEmailLoginResult, error)
	CurrentUser(ctx context.Context, sessionToken string) (CurrentUser, error)
	Logout(ctx context.Context, sessionToken string) error
}

type StartEmailLoginResult struct {
	Sent    bool
	DevCode string
}

type VerifyEmailLoginInput struct {
	Email string
	Code  string
}

type VerifyEmailLoginResult struct {
	SessionToken string
	User         CurrentUser
}

type CurrentUser struct {
	ID            string `json:"id"`
	DisplayName   string `json:"display_name"`
	PrimaryEmail  string `json:"primary_email"`
	EmailVerified bool   `json:"email_verified"`
}

type AuthHandler struct {
	service   AuthService
	env       string
	nowFunc   func() time.Time
	cookieTTL time.Duration
}

func RegisterAuthRoutes(mux *http.ServeMux, service AuthService, env string) {
	handler := AuthHandler{
		service:   service,
		env:       env,
		nowFunc:   time.Now,
		cookieTTL: sessionCookieTTL,
	}
	mux.HandleFunc("POST /api/auth/email/start", handler.StartEmailLogin)
	mux.HandleFunc("POST /api/auth/email/verify", handler.VerifyEmailLogin)
	mux.HandleFunc("GET /api/me", handler.CurrentUser)
	mux.HandleFunc("POST /api/auth/logout", handler.Logout)
}

func (h AuthHandler) StartEmailLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, RequestIDFromContext(r.Context()), ErrInvalidRequest)
		return
	}

	result, err := h.service.StartEmailLogin(r.Context(), req.Email)
	if err != nil {
		WriteError(w, RequestIDFromContext(r.Context()), mapAuthError(err))
		return
	}

	resp := struct {
		Sent    bool   `json:"sent"`
		DevCode string `json:"dev_code,omitempty"`
	}{
		Sent:    result.Sent,
		DevCode: result.DevCode,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h AuthHandler) VerifyEmailLogin(w http.ResponseWriter, r *http.Request) {
	var req VerifyEmailLoginInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, RequestIDFromContext(r.Context()), ErrInvalidRequest)
		return
	}

	result, err := h.service.VerifyEmailLogin(r.Context(), req)
	if err != nil {
		WriteError(w, RequestIDFromContext(r.Context()), mapAuthError(err))
		return
	}

	setSessionCookie(w, result.SessionToken, h.cookieSecure(), h.cookieTTL)
	writeJSON(w, http.StatusOK, struct {
		User CurrentUser `json:"user"`
	}{
		User: result.User,
	})
}

func (h AuthHandler) CurrentUser(w http.ResponseWriter, r *http.Request) {
	sessionToken, err := sessionTokenFromRequest(r)
	if err != nil {
		WriteError(w, RequestIDFromContext(r.Context()), mapAuthError(err))
		return
	}

	user, err := h.service.CurrentUser(r.Context(), sessionToken)
	if err != nil {
		WriteError(w, RequestIDFromContext(r.Context()), mapAuthError(err))
		return
	}

	writeJSON(w, http.StatusOK, struct {
		User CurrentUser `json:"user"`
	}{
		User: user,
	})
}

func (h AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	sessionToken, err := sessionTokenFromRequest(r)
	if err != nil {
		WriteError(w, RequestIDFromContext(r.Context()), mapAuthError(err))
		return
	}

	if err := h.service.Logout(r.Context(), sessionToken); err != nil {
		WriteError(w, RequestIDFromContext(r.Context()), mapAuthError(err))
		return
	}

	clearSessionCookie(w, h.cookieSecure(), h.nowFunc())
	writeJSON(w, http.StatusOK, struct {
		OK bool `json:"ok"`
	}{
		OK: true,
	})
}

func (h AuthHandler) cookieSecure() bool {
	return normalizeEnv(h.env) == "production"
}

func RequireSession(service AuthService, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessionToken, err := sessionTokenFromRequest(r)
		if err != nil {
			WriteError(w, RequestIDFromContext(r.Context()), mapAuthError(err))
			return
		}

		user, err := service.CurrentUser(r.Context(), sessionToken)
		if err != nil {
			WriteError(w, RequestIDFromContext(r.Context()), mapAuthError(err))
			return
		}

		ctx := context.WithValue(r.Context(), currentUserKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func CurrentUserFromContext(ctx context.Context) (CurrentUser, bool) {
	user, ok := ctx.Value(currentUserKey).(CurrentUser)
	return user, ok
}

func sessionTokenFromRequest(r *http.Request) (string, error) {
	cookie, err := r.Cookie(security.SessionCookieName)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			return "", ErrAuthSessionRequired
		}
		return "", ErrAuthSessionRequired
	}
	if cookie.Value == "" {
		return "", ErrAuthSessionRequired
	}
	return cookie.Value, nil
}

func setSessionCookie(w http.ResponseWriter, sessionToken string, secure bool, ttl time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     security.SessionCookieName,
		Value:    sessionToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   int(ttl.Seconds()),
	})
}

func clearSessionCookie(w http.ResponseWriter, secure bool, now time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     security.SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   -1,
		Expires:  now.Add(-time.Hour),
	})
}

func mapAuthError(err error) AppError {
	switch {
	case errors.Is(err, ErrAuthInvalidEmail):
		return ErrInvalidEmail
	case errors.Is(err, ErrAuthInvalidCode):
		return ErrInvalidCode
	case errors.Is(err, ErrAuthUnauthorized):
		return ErrUnauthorized
	case errors.Is(err, ErrAuthSessionRequired):
		return ErrSessionRequired
	case errors.Is(err, ErrAuthSessionExpired):
		return ErrSessionExpired
	default:
		return ErrInternalError
	}
}
