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
	"gorm.io/datatypes"
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
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
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

type RuntimeAccessRequest struct {
	ScopeType string `json:"scope_type"`
	ScopeCode string `json:"scope_code"`
	Actor     string `json:"actor"`
}

type DisableRuntimeAccessRequest struct {
	Actor string `json:"actor"`
}

type WorkspaceRuntimeAccessResult struct {
	WorkspaceID string     `json:"workspace_id"`
	ScopeType   string     `json:"scope_type"`
	ScopeCode   string     `json:"scope_code"`
	Status      string     `json:"status"`
	ActivatedAt *time.Time `json:"activated_at"`
	ActivatedBy string     `json:"activated_by"`
	DisabledAt  *time.Time `json:"disabled_at"`
	DisabledBy  string     `json:"disabled_by"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type WorkspaceRuntimeAccessResponse struct {
	RuntimeAccess *WorkspaceRuntimeAccessResult `json:"runtime_access"`
}

type PublishModelCatalogInput = repository.PublishModelCatalogInput

type PublishModelCatalogRequest struct {
	Catalog publishModelCatalogCatalogRequest `json:"catalog"`
	I18n    []publishModelCatalogI18nRequest  `json:"i18n"`
	Prices  []publishModelPriceVersionRequest `json:"prices"`
}

type publishModelCatalogCatalogRequest struct {
	ModelID          string                        `json:"model_id"`
	Slug             string                        `json:"slug"`
	Status           domain.ModelCatalogStatus     `json:"status"`
	Visibility       domain.ModelCatalogVisibility `json:"visibility"`
	LogoURL          string                        `json:"logo_url"`
	ContextLength    *int64                        `json:"context_length"`
	KnowledgeCutoff  *time.Time                    `json:"knowledge_cutoff"`
	InputModalities  []string                      `json:"input_modalities"`
	OutputModalities []string                      `json:"output_modalities"`
	Capabilities     []string                      `json:"capabilities"`
	Featured         bool                          `json:"featured"`
	SortWeight       int64                         `json:"sort_weight"`
	PublishedAt      *time.Time                    `json:"published_at"`
}

type publishModelCatalogI18nRequest struct {
	Locale           string   `json:"locale"`
	DisplayName      string   `json:"display_name"`
	ShortDescription string   `json:"short_description"`
	LongDescription  *string  `json:"long_description"`
	SEOTitle         string   `json:"seo_title"`
	SEODescription   string   `json:"seo_description"`
	Tags             []string `json:"tags"`
}

type publishModelPriceVersionRequest struct {
	ID                 string                  `json:"id"`
	Currency           string                  `json:"currency"`
	InputPrice         float64                 `json:"input_price"`
	OutputPrice        float64                 `json:"output_price"`
	CachedPrice        *float64                `json:"cached_price"`
	CacheCreationPrice *float64                `json:"cache_creation_price"`
	EffectiveFrom      time.Time               `json:"effective_from"`
	EffectiveUntil     *time.Time              `json:"effective_until"`
	Status             domain.ModelPriceStatus `json:"status"`
	PublishedAt        time.Time               `json:"published_at"`
}

type internalRoutesStore interface {
	SearchUsers(ctx context.Context, keyword string, limit int) ([]domain.User, error)
	SearchWorkspaces(ctx context.Context, keyword string, limit int) ([]domain.Workspace, error)
	FindWorkspaceByID(ctx context.Context, id string) (domain.Workspace, error)
	FindWorkspaceRuntimeAccess(ctx context.Context, workspaceID string) (domain.WorkspaceRuntimeAccess, error)
	UpsertWorkspaceRuntimeAccess(ctx context.Context, input repository.UpsertWorkspaceRuntimeAccessInput) (domain.WorkspaceRuntimeAccess, error)
	DisableWorkspaceRuntimeAccess(ctx context.Context, workspaceID string, actor string) (domain.WorkspaceRuntimeAccess, error)
	ListAPIKeysByWorkspace(ctx context.Context, workspaceID string) ([]domain.APIKey, error)
	PublishModelCatalog(ctx context.Context, input repository.PublishModelCatalogInput) error
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
				ID:   ws.ID,
				Name: ws.Name,
				Slug: ws.Slug,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(SearchWorkspacesResponse{Workspaces: results})
	})

	mux.HandleFunc("GET /internal/v1/workspaces/{id}/runtime-access", func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		workspaceID := r.PathValue("id")
		if !authorizeInternalRequest(w, r, internalToken) {
			return
		}

		access, err := store.FindWorkspaceRuntimeAccess(r.Context(), workspaceID)
		if err != nil {
			if errors.Is(err, repository.ErrWorkspaceRuntimeAccessNotFound) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(WorkspaceRuntimeAccessResponse{})
				return
			}
			WriteError(w, requestID, ErrInternalError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		result := workspaceRuntimeAccessResultFromDomain(access)
		_ = json.NewEncoder(w).Encode(WorkspaceRuntimeAccessResponse{RuntimeAccess: &result})
	})

	mux.HandleFunc("PUT /internal/v1/workspaces/{id}/runtime-access", func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		workspaceID := r.PathValue("id")
		if !authorizeInternalRequest(w, r, internalToken) {
			return
		}

		var req RuntimeAccessRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, requestID, ErrInvalidRequest)
			return
		}

		scopeType := strings.TrimSpace(req.ScopeType)
		scopeCode := strings.TrimSpace(req.ScopeCode)
		if scopeType == "" {
			scopeType = string(domain.RuntimeAccessScopeTenant)
		}
		if scopeType != string(domain.RuntimeAccessScopeTenant) || scopeCode == "" {
			WriteError(w, requestID, ErrInvalidRequest)
			return
		}

		_, err := store.UpsertWorkspaceRuntimeAccess(r.Context(), repository.UpsertWorkspaceRuntimeAccessInput{
			WorkspaceID: workspaceID,
			ScopeType:   domain.RuntimeAccessScopeType(scopeType),
			ScopeCode:   scopeCode,
			Actor:       strings.TrimSpace(req.Actor),
		})
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

	mux.HandleFunc("POST /internal/v1/workspaces/{id}/runtime-access/disable", func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		workspaceID := r.PathValue("id")
		if !authorizeInternalRequest(w, r, internalToken) {
			return
		}

		var req DisableRuntimeAccessRequest
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&req)
		}

		_, err := store.DisableWorkspaceRuntimeAccess(r.Context(), workspaceID, strings.TrimSpace(req.Actor))
		if err != nil {
			if errors.Is(err, repository.ErrWorkspaceNotFound) || errors.Is(err, repository.ErrWorkspaceRuntimeAccessNotFound) {
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

	mux.HandleFunc("POST /internal/v1/model-catalogs/publish", func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if !authorizeInternalRequest(w, r, internalToken) {
			return
		}

		var req PublishModelCatalogRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, requestID, ErrInvalidRequest)
			return
		}

		input, err := publishModelCatalogInputFromRequest(req)
		if err != nil {
			WriteError(w, requestID, ErrInvalidRequest)
			return
		}

		if err := store.PublishModelCatalog(r.Context(), input); err != nil {
			WriteError(w, requestID, ErrInternalError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}

func publishModelCatalogInputFromRequest(req PublishModelCatalogRequest) (repository.PublishModelCatalogInput, error) {
	now := time.Now().UTC()
	modelID := strings.TrimSpace(req.Catalog.ModelID)
	slug := strings.TrimSpace(req.Catalog.Slug)
	if modelID == "" || slug == "" || req.Catalog.Status == "" || req.Catalog.Visibility == "" {
		return repository.PublishModelCatalogInput{}, errors.New("missing required catalog fields")
	}

	inputModalities, err := jsonStringSlice(req.Catalog.InputModalities)
	if err != nil {
		return repository.PublishModelCatalogInput{}, err
	}
	outputModalities, err := jsonStringSlice(req.Catalog.OutputModalities)
	if err != nil {
		return repository.PublishModelCatalogInput{}, err
	}
	capabilities, err := jsonStringSlice(req.Catalog.Capabilities)
	if err != nil {
		return repository.PublishModelCatalogInput{}, err
	}

	input := repository.PublishModelCatalogInput{
		Catalog: domain.ModelCatalog{
			ModelID:          modelID,
			Slug:             slug,
			Status:           req.Catalog.Status,
			Visibility:       req.Catalog.Visibility,
			LogoURL:          strings.TrimSpace(req.Catalog.LogoURL),
			ContextLength:    req.Catalog.ContextLength,
			KnowledgeCutoff:  req.Catalog.KnowledgeCutoff,
			InputModalities:  inputModalities,
			OutputModalities: outputModalities,
			Capabilities:     capabilities,
			Featured:         req.Catalog.Featured,
			SortWeight:       req.Catalog.SortWeight,
			PublishedAt:      req.Catalog.PublishedAt,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		I18n:   make([]domain.ModelCatalogI18n, 0, len(req.I18n)),
		Prices: make([]domain.ModelPriceVersion, 0, len(req.Prices)),
	}

	for _, row := range req.I18n {
		locale := strings.TrimSpace(row.Locale)
		displayName := strings.TrimSpace(row.DisplayName)
		if locale == "" || displayName == "" {
			return repository.PublishModelCatalogInput{}, errors.New("missing required i18n fields")
		}
		tags, err := jsonStringSlice(row.Tags)
		if err != nil {
			return repository.PublishModelCatalogInput{}, err
		}
		input.I18n = append(input.I18n, domain.ModelCatalogI18n{
			ModelID:          modelID,
			Locale:           locale,
			DisplayName:      displayName,
			ShortDescription: strings.TrimSpace(row.ShortDescription),
			LongDescription:  row.LongDescription,
			SEOTitle:         strings.TrimSpace(row.SEOTitle),
			SEODescription:   strings.TrimSpace(row.SEODescription),
			Tags:             tags,
			UpdatedAt:        now,
		})
	}

	for _, row := range req.Prices {
		id := strings.TrimSpace(row.ID)
		currency := strings.TrimSpace(row.Currency)
		if id == "" || currency == "" || row.Status == "" || row.EffectiveFrom.IsZero() || row.PublishedAt.IsZero() {
			return repository.PublishModelCatalogInput{}, errors.New("missing required price fields")
		}
		input.Prices = append(input.Prices, domain.ModelPriceVersion{
			ID:                 id,
			ModelID:            modelID,
			Currency:           currency,
			InputPrice:         row.InputPrice,
			OutputPrice:        row.OutputPrice,
			CachedPrice:        row.CachedPrice,
			CacheCreationPrice: row.CacheCreationPrice,
			EffectiveFrom:      row.EffectiveFrom,
			EffectiveUntil:     row.EffectiveUntil,
			Status:             row.Status,
			PublishedAt:        row.PublishedAt,
		})
	}

	return input, nil
}

func jsonStringSlice(values []string) (datatypes.JSON, error) {
	if values == nil {
		return nil, nil
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}
	return datatypes.JSON(raw), nil
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
	access, err := store.FindWorkspaceRuntimeAccess(ctx, workspaceID)
	if err != nil {
		if !errors.Is(err, repository.ErrWorkspaceRuntimeAccessNotFound) {
			return err
		}
		access = domain.WorkspaceRuntimeAccess{WorkspaceID: workspaceID, Status: domain.RuntimeAccessStatusDisabled}
	}
	keys, err := store.ListAPIKeysByWorkspace(ctx, workspaceID)
	if err != nil {
		return err
	}
	for _, key := range keys {
		record, shouldUpsert := runtimeRecordFromAPIKey(workspace, access, key)
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

func workspaceRuntimeAccessResultFromDomain(access domain.WorkspaceRuntimeAccess) WorkspaceRuntimeAccessResult {
	return WorkspaceRuntimeAccessResult{
		WorkspaceID: access.WorkspaceID,
		ScopeType:   string(access.ScopeType),
		ScopeCode:   access.ScopeCode,
		Status:      string(access.Status),
		ActivatedAt: access.ActivatedAt,
		ActivatedBy: access.ActivatedBy,
		DisabledAt:  access.DisabledAt,
		DisabledBy:  access.DisabledBy,
		CreatedAt:   access.CreatedAt,
		UpdatedAt:   access.UpdatedAt,
	}
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
