package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tokenlive/tokenlive-portal/backend/internal/domain"
	"github.com/tokenlive/tokenlive-portal/backend/internal/repository"
)

type UserSearchResult struct {
	ID           string  `json:"id"`
	DisplayName  string  `json:"display_name"`
	PrimaryEmail *string `json:"primary_email"`
}

type SearchUsersResponse struct {
	Users []UserSearchResult `json:"users"`
}

type WorkspaceSearchResult struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Slug       string  `json:"slug"`
	TenantCode *string `json:"tenant_code"`
}

type SearchWorkspacesResponse struct {
	Workspaces []WorkspaceSearchResult `json:"workspaces"`
}

type InternalAPIKeyResult struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	KeyPrefix   string              `json:"key_prefix"`
	SecretLast4 string              `json:"secret_last4"`
	Status      domain.APIKeyStatus `json:"status"`
	ExpiresAt   *time.Time          `json:"expires_at"`
	LastUsedAt  *time.Time          `json:"last_used_at"`
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
}

type WorkspaceAPIKeysResponse struct {
	APIKeys []InternalAPIKeyResult `json:"api_keys"`
}

type BindTenantRequest struct {
	TenantCode string `json:"tenant_code"`
}

type internalRoutesStore interface {
	SearchUsers(ctx context.Context, keyword string, limit int) ([]domain.User, error)
	SearchWorkspaces(ctx context.Context, keyword string, limit int) ([]domain.Workspace, error)
	BindTenantCode(ctx context.Context, id string, tenantCode *string) error
	FindWorkspaceByID(ctx context.Context, id string) (domain.Workspace, error)
	ListAPIKeysByWorkspace(ctx context.Context, workspaceID string) ([]domain.APIKey, error)
}

func RegisterInternalRoutes(mux *http.ServeMux, store internalRoutesStore, internalToken string, runtimeSyncers ...APIKeyRuntimeSyncer) {
	runtimeSyncer := NewNoopAPIKeyRuntimeSyncer()
	if len(runtimeSyncers) > 0 && runtimeSyncers[0] != nil {
		runtimeSyncer = runtimeSyncers[0]
	}

	// 1. 用户搜索接口
	mux.HandleFunc("GET /internal/v1/users/search", func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")

		// 验证 Token
		if !authorizeInternalRequest(w, r, internalToken) {
			return
		}

		// 解析参数
		keyword := r.URL.Query().Get("keyword")
		limitStr := r.URL.Query().Get("limit")
		limit := 20
		if limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
				limit = l
			}
		}

		users, err := store.SearchUsers(r.Context(), keyword, limit)
		if err != nil {
			WriteError(w, requestID, ErrInternalError)
			return
		}

		results := make([]UserSearchResult, 0, len(users))
		for _, u := range users {
			results = append(results, UserSearchResult{
				ID:           u.ID,
				DisplayName:  u.DisplayName,
				PrimaryEmail: u.PrimaryEmail,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(SearchUsersResponse{Users: results})
	})

	// 2. 工作空间搜索接口
	mux.HandleFunc("GET /internal/v1/workspaces/search", func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")

		// 验证 Token
		if !authorizeInternalRequest(w, r, internalToken) {
			return
		}

		// 解析参数
		keyword := r.URL.Query().Get("keyword")
		limitStr := r.URL.Query().Get("limit")
		limit := 20
		if limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
				limit = l
			}
		}

		workspaces, err := store.SearchWorkspaces(r.Context(), keyword, limit)
		if err != nil {
			WriteError(w, requestID, ErrInternalError)
			return
		}

		results := make([]WorkspaceSearchResult, 0, len(workspaces))
		for _, ws := range workspaces {
			results = append(results, WorkspaceSearchResult{
				ID:         ws.ID,
				Name:       ws.Name,
				Slug:       ws.Slug,
				TenantCode: ws.TenantCode,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(SearchWorkspacesResponse{Workspaces: results})
	})

	// 3. 工作空间绑定租户接口
	mux.HandleFunc("POST /internal/v1/workspaces/{id}/bind-tenant", func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		workspaceID := r.PathValue("id")

		// 验证 Token
		if !authorizeInternalRequest(w, r, internalToken) {
			return
		}

		var req BindTenantRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, requestID, ErrInvalidRequest)
			return
		}

		tenantCode := strings.TrimSpace(req.TenantCode)
		if tenantCode == "" {
			WriteError(w, requestID, ErrInvalidRequest)
			return
		}

		err := store.BindTenantCode(r.Context(), workspaceID, &tenantCode)
		if err != nil {
			if errors.Is(err, repository.ErrWorkspaceNotFound) {
				WriteError(w, requestID, ErrWorkspaceNotFound)
				return
			}
			WriteError(w, requestID, ErrInternalError)
			return
		}
		syncWorkspaceAPIKeysBestEffort(r.Context(), store, runtimeSyncer, workspaceID)

		w.WriteHeader(http.StatusNoContent)
	})

	// 4. 工作空间解绑租户接口
	mux.HandleFunc("POST /internal/v1/workspaces/{id}/unbind-tenant", func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		workspaceID := r.PathValue("id")

		// 验证 Token
		if !authorizeInternalRequest(w, r, internalToken) {
			return
		}

		err := store.BindTenantCode(r.Context(), workspaceID, nil)
		if err != nil {
			if errors.Is(err, repository.ErrWorkspaceNotFound) {
				WriteError(w, requestID, ErrWorkspaceNotFound)
				return
			}
			WriteError(w, requestID, ErrInternalError)
			return
		}
		syncWorkspaceAPIKeysBestEffort(r.Context(), store, runtimeSyncer, workspaceID)

		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("GET /internal/v1/workspaces/{id}/api-keys", func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		workspaceID := r.PathValue("id")
		if !authorizeInternalRequest(w, r, internalToken) {
			return
		}

		keys, err := store.ListAPIKeysByWorkspace(r.Context(), workspaceID)
		if err != nil {
			WriteError(w, requestID, ErrInternalError)
			return
		}

		resp := WorkspaceAPIKeysResponse{APIKeys: make([]InternalAPIKeyResult, 0, len(keys))}
		for _, key := range keys {
			resp.APIKeys = append(resp.APIKeys, internalAPIKeyResultFromDomain(key))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("POST /internal/v1/workspaces/{id}/runtime-sync", func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		workspaceID := r.PathValue("id")
		if !authorizeInternalRequest(w, r, internalToken) {
			return
		}

		if err := syncWorkspaceAPIKeys(r.Context(), store, runtimeSyncer, workspaceID); err != nil {
			if errors.Is(err, repository.ErrWorkspaceNotFound) {
				WriteError(w, requestID, ErrWorkspaceNotFound)
				return
			}
			WriteError(w, requestID, ErrInternalError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}

func authorizeInternalRequest(w http.ResponseWriter, r *http.Request, internalToken string) bool {
	if internalToken == "" {
		return true
	}
	authHeader := r.Header.Get("Authorization")
	token := ""
	if strings.HasPrefix(authHeader, "Bearer ") {
		token = strings.TrimPrefix(authHeader, "Bearer ")
	}
	if token != internalToken {
		WriteError(w, r.Header.Get("X-Request-ID"), ErrUnauthorized)
		return false
	}
	return true
}

func syncWorkspaceAPIKeys(ctx context.Context, store internalRoutesStore, runtimeSyncer APIKeyRuntimeSyncer, workspaceID string) error {
	workspace, err := store.FindWorkspaceByID(ctx, workspaceID)
	if err != nil {
		return err
	}
	keys, err := store.ListAPIKeysByWorkspace(ctx, workspaceID)
	if err != nil {
		return err
	}
	for _, key := range keys {
		record, shouldUpsert := runtimeRecordFromAPIKey(workspace, key)
		if shouldUpsert {
			if err := runtimeSyncer.UpsertAPIKey(ctx, record); err != nil {
				return err
			}
			continue
		}
		if record.KeyHash == "" {
			continue
		}
		if err := runtimeSyncer.DeleteAPIKey(ctx, record.KeyHash); err != nil {
			return err
		}
	}
	return nil
}

func syncWorkspaceAPIKeysBestEffort(ctx context.Context, store internalRoutesStore, runtimeSyncer APIKeyRuntimeSyncer, workspaceID string) {
	if err := syncWorkspaceAPIKeys(ctx, store, runtimeSyncer, workspaceID); err != nil {
		log.Printf("portal workspace runtime sync failed: workspace_id=%s err=%v", workspaceID, err)
	}
}

func internalAPIKeyResultFromDomain(key domain.APIKey) InternalAPIKeyResult {
	return InternalAPIKeyResult{
		ID:          key.ID,
		Name:        key.Name,
		KeyPrefix:   key.KeyPrefix,
		SecretLast4: key.SecretLast4,
		Status:      key.Status,
		ExpiresAt:   key.ExpiresAt,
		LastUsedAt:  key.LastUsedAt,
		CreatedAt:   key.CreatedAt,
		UpdatedAt:   key.UpdatedAt,
	}
}
