package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

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

type BindTenantRequest struct {
	TenantCode string `json:"tenant_code"`
}

func RegisterInternalRoutes(mux *http.ServeMux, repo *repository.Repositories, internalToken string) {
	// 1. 用户搜索接口
	mux.HandleFunc("GET /internal/v1/users/search", func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")

		// 验证 Token
		if internalToken != "" {
			authHeader := r.Header.Get("Authorization")
			token := ""
			if strings.HasPrefix(authHeader, "Bearer ") {
				token = strings.TrimPrefix(authHeader, "Bearer ")
			}
			if token != internalToken {
				WriteError(w, requestID, ErrUnauthorized)
				return
			}
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

		users, err := repo.SearchUsers(r.Context(), keyword, limit)
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
		if internalToken != "" {
			authHeader := r.Header.Get("Authorization")
			token := ""
			if strings.HasPrefix(authHeader, "Bearer ") {
				token = strings.TrimPrefix(authHeader, "Bearer ")
			}
			if token != internalToken {
				WriteError(w, requestID, ErrUnauthorized)
				return
			}
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

		workspaces, err := repo.SearchWorkspaces(r.Context(), keyword, limit)
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
		if internalToken != "" {
			authHeader := r.Header.Get("Authorization")
			token := ""
			if strings.HasPrefix(authHeader, "Bearer ") {
				token = strings.TrimPrefix(authHeader, "Bearer ")
			}
			if token != internalToken {
				WriteError(w, requestID, ErrUnauthorized)
				return
			}
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

		err := repo.BindTenantCode(r.Context(), workspaceID, &tenantCode)
		if err != nil {
			if errors.Is(err, repository.ErrWorkspaceNotFound) {
				WriteError(w, requestID, ErrWorkspaceNotFound)
				return
			}
			WriteError(w, requestID, ErrInternalError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})

	// 4. 工作空间解绑租户接口
	mux.HandleFunc("POST /internal/v1/workspaces/{id}/unbind-tenant", func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		workspaceID := r.PathValue("id")

		// 验证 Token
		if internalToken != "" {
			authHeader := r.Header.Get("Authorization")
			token := ""
			if strings.HasPrefix(authHeader, "Bearer ") {
				token = strings.TrimPrefix(authHeader, "Bearer ")
			}
			if token != internalToken {
				WriteError(w, requestID, ErrUnauthorized)
				return
			}
		}

		err := repo.BindTenantCode(r.Context(), workspaceID, nil)
		if err != nil {
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
