package api

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"time"
)

const (
	oauthStateCookieName = "oauth_state"
	oauthStateCookieTTL  = 5 * time.Minute
	oauthStateBytes      = 32
	// oauthBindFlag is set in the state cookie to indicate a bind flow
	oauthBindCookieName = "oauth_bind"
)

type OAuthHandler struct {
	service   AuthService
	env       string
	nowFunc   func() time.Time
	cookieTTL time.Duration
}

func RegisterOAuthRoutes(mux *http.ServeMux, service AuthService, env string) {
	handler := OAuthHandler{
		service:   service,
		env:       env,
		nowFunc:   time.Now,
		cookieTTL: sessionCookieTTL,
	}
	// 登录流程
	mux.HandleFunc("GET /api/auth/google/login", handler.StartGoogleLogin)
	mux.HandleFunc("GET /api/auth/google/callback", handler.GoogleCallback)

	// 绑定流程（已登录用户）
	mux.HandleFunc("GET /api/auth/google/bind", handler.StartGoogleBind)
	mux.HandleFunc("GET /api/auth/google/bind/callback", handler.GoogleBindCallback)

	// Terms 接受
	mux.HandleFunc("POST /api/auth/accept-terms", handler.AcceptTerms)

	// 列出绑定账号
	mux.HandleFunc("GET /api/auth/oauth/accounts", handler.ListOAuthAccounts)
}

// StartGoogleLogin 生成 state 并 302 重定向到 Google OAuth 授权页
func (h OAuthHandler) StartGoogleLogin(w http.ResponseWriter, r *http.Request) {
	state, err := generateOAuthState()
	if err != nil {
		WriteError(w, RequestIDFromContext(r.Context()), ErrInternalError)
		return
	}

	h.setOAuthStateCookie(w, state, false)
	authURL := h.service.GetGoogleAuthURL(state)
	if authURL == "" {
		WriteError(w, RequestIDFromContext(r.Context()), ErrInternalError)
		return
	}

	http.Redirect(w, r, authURL, http.StatusFound)
}

// StartGoogleBind 已登录用户发起绑定 Google 流程
func (h OAuthHandler) StartGoogleBind(w http.ResponseWriter, r *http.Request) {
	// 验证 session
	sessionToken, err := sessionTokenFromRequest(r)
	if err != nil {
		WriteError(w, RequestIDFromContext(r.Context()), mapAuthError(err))
		return
	}
	if _, err := h.service.CurrentUser(r.Context(), sessionToken); err != nil {
		WriteError(w, RequestIDFromContext(r.Context()), mapAuthError(err))
		return
	}

	state, err := generateOAuthState()
	if err != nil {
		WriteError(w, RequestIDFromContext(r.Context()), ErrInternalError)
		return
	}

	h.setOAuthStateCookie(w, state, true)
	authURL := h.service.GetGoogleAuthURL(state)
	if authURL == "" {
		WriteError(w, RequestIDFromContext(r.Context()), ErrInternalError)
		return
	}

	http.Redirect(w, r, authURL, http.StatusFound)
}

// GoogleCallback 处理 Google OAuth 回调（未登录场景）
func (h OAuthHandler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	// 1. 从 query 获取 state 和 code
	queryState := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		// 用户取消授权或 Google 返回错误
		writeOAuthCallbackHTML(w, false, "authorization_denied", "")
		return
	}

	// 2. 校验 state cookie
	cookieState, isBind := h.getOAuthStateCookie(r)
	h.clearOAuthStateCookie(w)
	if cookieState == "" || queryState == "" || cookieState != queryState {
		WriteError(w, RequestIDFromContext(r.Context()), ErrOAuthStateInvalid)
		return
	}
	if isBind {
		// 不应该是绑定流程，错误地进入了登录回调
		WriteError(w, RequestIDFromContext(r.Context()), ErrOAuthStateInvalid)
		return
	}

	// 3. 调用 service 处理回调
	ip := clientIP(r)
	userAgent := r.UserAgent()
	result, err := h.service.HandleGoogleCallback(r.Context(), code, ip, userAgent)
	if err != nil {
		WriteError(w, RequestIDFromContext(r.Context()), mapOAuthError(err))
		return
	}

	// 4. 设置 session cookie
	setSessionCookie(w, result.SessionToken, h.cookieSecure(), h.cookieTTL)

	// 5. 返回 postMessage HTML（前端通过 popup 接收）
	if result.TermsPending {
		writeOAuthCallbackHTML(w, true, "terms_pending", "")
	} else {
		writeOAuthCallbackHTML(w, true, "success", "")
	}
}

// GoogleBindCallback 处理已登录用户绑定 Google 的回调
func (h OAuthHandler) GoogleBindCallback(w http.ResponseWriter, r *http.Request) {
	sessionToken, err := sessionTokenFromRequest(r)
	if err != nil {
		WriteError(w, RequestIDFromContext(r.Context()), mapAuthError(err))
		return
	}

	queryState := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		writeOAuthCallbackHTML(w, false, "authorization_denied", "")
		return
	}

	cookieState, isBind := h.getOAuthStateCookie(r)
	h.clearOAuthStateCookie(w)
	if cookieState == "" || queryState == "" || cookieState != queryState || !isBind {
		WriteError(w, RequestIDFromContext(r.Context()), ErrOAuthStateInvalid)
		return
	}

	identity, err := h.service.HandleGoogleBind(r.Context(), sessionToken, code)
	if err != nil {
		WriteError(w, RequestIDFromContext(r.Context()), mapOAuthError(err))
		return
	}

	writeOAuthCallbackHTML(w, true, "bind_success", identity.Provider)
}

// AcceptTerms 用户接受条款（POST 接口）
func (h OAuthHandler) AcceptTerms(w http.ResponseWriter, r *http.Request) {
	sessionToken, err := sessionTokenFromRequest(r)
	if err != nil {
		WriteError(w, RequestIDFromContext(r.Context()), mapAuthError(err))
		return
	}

	result, err := h.service.AcceptTerms(r.Context(), sessionToken)
	if err != nil {
		WriteError(w, RequestIDFromContext(r.Context()), mapOAuthError(err))
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ListOAuthAccounts 列出当前用户的第三方绑定账号
func (h OAuthHandler) ListOAuthAccounts(w http.ResponseWriter, r *http.Request) {
	sessionToken, err := sessionTokenFromRequest(r)
	if err != nil {
		WriteError(w, RequestIDFromContext(r.Context()), mapAuthError(err))
		return
	}

	identities, err := h.service.ListAccountIdentities(r.Context(), sessionToken)
	if err != nil {
		WriteError(w, RequestIDFromContext(r.Context()), mapOAuthError(err))
		return
	}

	writeJSON(w, http.StatusOK, struct {
		Accounts []AccountIdentityDTO `json:"accounts"`
	}{
		Accounts: identities,
	})
}

func (h OAuthHandler) cookieSecure() bool {
	return normalizeEnv(h.env) == "production"
}

// setOAuthStateCookie 设置 OAuth state cookie，bind=true 表示绑定流程
func (h OAuthHandler) setOAuthStateCookie(w http.ResponseWriter, state string, isBind bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    state,
		Path:     "/api/auth/google/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.cookieSecure(),
		MaxAge:   int(oauthStateCookieTTL.Seconds()),
	})
	if isBind {
		http.SetCookie(w, &http.Cookie{
			Name:     oauthBindCookieName,
			Value:    "1",
			Path:     "/api/auth/google/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   h.cookieSecure(),
			MaxAge:   int(oauthStateCookieTTL.Seconds()),
		})
	}
}

func (h OAuthHandler) getOAuthStateCookie(r *http.Request) (state string, isBind bool) {
	cookie, err := r.Cookie(oauthStateCookieName)
	if err != nil || cookie.Value == "" {
		return "", false
	}
	state = cookie.Value
	if bindCookie, err := r.Cookie(oauthBindCookieName); err == nil && bindCookie.Value == "1" {
		isBind = true
	}
	return state, isBind
}

func (h OAuthHandler) clearOAuthStateCookie(w http.ResponseWriter) {
	clear := func(name string) {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/api/auth/google/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   h.cookieSecure(),
			MaxAge:   -1,
			Expires:  h.nowFunc().Add(-time.Hour),
		})
	}
	clear(oauthStateCookieName)
	clear(oauthBindCookieName)
}

func generateOAuthState() (string, error) {
	var b [oauthStateBytes]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// 取第一个 IP
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}
	return r.RemoteAddr
}

// writeOAuthCallbackHTML 返回 HTML 页面，通过 postMessage 通知 opener window 认证结果。
// 前端在 popup 中接收此消息后刷新状态并关闭 popup。
func writeOAuthCallbackHTML(w http.ResponseWriter, success bool, code string, provider string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`<!DOCTYPE html>
<html><head><title>OAuth Callback</title></head><body>
<script>
(function() {
  var msg = {
    type: "oauth-callback",
    success: ` + boolStr(success) + `,
    code: "` + code + `",
    provider: "` + provider + `"
  };
  if (window.opener && !window.opener.closed) {
    window.opener.postMessage(msg, window.location.origin);
    window.close();
  } else {
    // 没有 opener（非 popup 模式），向 parent 发送消息
    if (window.parent && window.parent !== window) {
      window.parent.postMessage(msg, window.location.origin);
    }
    document.body.innerHTML = "<p>Authentication " + (msg.success ? "succeeded" : "failed") + ". You can close this window.</p>";
  }
})();
</script>
</body></html>`))
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func mapOAuthError(err error) AppError {
	switch {
	case errors.Is(err, ErrAuthInvalidRequest):
		return ErrInvalidRequest
	case errors.Is(err, ErrAuthOAuthEmailConflict):
		return ErrOAuthEmailTaken
	case errors.Is(err, ErrAuthOAuthIdentityBound):
		return ErrOAuthIdentityBound
	case errors.Is(err, ErrAuthTermsAlreadyAccepted):
		return ErrTermsRequired
	case errors.Is(err, ErrAuthOAuthNotConfigured):
		return ErrInternalError
	case errors.Is(err, ErrAuthSessionRequired):
		return ErrSessionRequired
	case errors.Is(err, ErrAuthSessionExpired):
		return ErrSessionExpired
	default:
		return mapAuthError(err)
	}
}
