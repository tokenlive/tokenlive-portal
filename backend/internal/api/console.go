package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

type ConsoleHandler struct {
	service ConsoleService
	auth    AuthService
}

func RegisterConsoleRoutes(mux *http.ServeMux, service ConsoleService, auth AuthService) {
	if service == nil {
		panic("console service is required")
	}
	if auth == nil {
		panic("auth service is required")
	}

	handler := ConsoleHandler{service: service, auth: auth}
	mux.HandleFunc("GET /api/console/overview", handler.Overview)
	mux.HandleFunc("GET /api/workspaces/current", handler.CurrentWorkspace)
	mux.HandleFunc("GET /api/api-keys", handler.ListAPIKeys)
	mux.HandleFunc("POST /api/api-keys", handler.CreateAPIKey)
	mux.HandleFunc("POST /api/api-keys/{id}/enable", handler.EnableAPIKey)
	mux.HandleFunc("POST /api/api-keys/{id}/disable", handler.DisableAPIKey)
	mux.HandleFunc("POST /api/api-keys/{id}/revoke", handler.RevokeAPIKey)
}

func (h ConsoleHandler) Overview(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	result, err := h.service.Overview(r.Context(), user)
	if err != nil {
		WriteError(w, RequestIDFromContext(r.Context()), mapConsoleError(err))
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (h ConsoleHandler) CurrentWorkspace(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	result, err := h.service.CurrentWorkspace(r.Context(), user)
	if err != nil {
		WriteError(w, RequestIDFromContext(r.Context()), mapConsoleError(err))
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (h ConsoleHandler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	result, err := h.service.ListAPIKeys(r.Context(), user)
	if err != nil {
		WriteError(w, RequestIDFromContext(r.Context()), mapConsoleError(err))
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (h ConsoleHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	var req CreateAPIKeyRequest
	if err := decodeCreateAPIKeyRequest(r, &req); err != nil {
		WriteError(w, RequestIDFromContext(r.Context()), ErrInvalidRequest)
		return
	}

	result, err := h.service.CreateAPIKey(r.Context(), user, req)
	if err != nil {
		WriteError(w, RequestIDFromContext(r.Context()), mapConsoleError(err))
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func decodeCreateAPIKeyRequest(r *http.Request, req *CreateAPIKeyRequest) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(req); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return errors.New("unexpected extra JSON value")
		}
		return err
	}
	return nil
}

func (h ConsoleHandler) EnableAPIKey(w http.ResponseWriter, r *http.Request) {
	h.updateAPIKey(w, r, h.service.EnableAPIKey)
}

func (h ConsoleHandler) DisableAPIKey(w http.ResponseWriter, r *http.Request) {
	h.updateAPIKey(w, r, h.service.DisableAPIKey)
}

func (h ConsoleHandler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	h.updateAPIKey(w, r, h.service.RevokeAPIKey)
}

func (h ConsoleHandler) updateAPIKey(w http.ResponseWriter, r *http.Request, update func(context.Context, CurrentUser, string) (APIKeyResponse, error)) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	apiKeyID := strings.TrimSpace(r.PathValue("id"))
	result, err := update(r.Context(), user, apiKeyID)
	if err != nil {
		WriteError(w, RequestIDFromContext(r.Context()), mapConsoleError(err))
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (h ConsoleHandler) currentUser(w http.ResponseWriter, r *http.Request) (CurrentUser, bool) {
	sessionToken, err := sessionTokenFromRequest(r)
	if err != nil {
		WriteError(w, RequestIDFromContext(r.Context()), mapAuthError(err))
		return CurrentUser{}, false
	}

	user, err := h.auth.CurrentUser(r.Context(), sessionToken)
	if err != nil {
		WriteError(w, RequestIDFromContext(r.Context()), mapAuthError(err))
		return CurrentUser{}, false
	}

	return user, true
}
