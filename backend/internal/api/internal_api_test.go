package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tokenlive/tokenlive-portal/backend/internal/database"
	"github.com/tokenlive/tokenlive-portal/backend/internal/domain"
	"github.com/tokenlive/tokenlive-portal/backend/internal/repository"
)

func testRepos(t *testing.T) *repository.Repositories {
	t.Helper()

	dsn := os.Getenv("PORTAL_TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("PORTAL_TEST_DATABASE_DSN is not set")
	}

	db, err := database.Open(dsn)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}

	return repository.New(db)
}

func TestInternalAPIListAPIKeysReturnsSafeMetadata(t *testing.T) {
	t.Parallel()

	store := &fakeInternalStore{
		apiKeys: []domain.APIKey{{
			ID:          "ak_1",
			WorkspaceID: "wsp_1",
			Name:        "prod",
			KeyPrefix:   "tl_live_abc",
			SecretLast4: "wxyz",
			KeyHash:     "hash-must-not-leak",
			Status:      domain.APIKeyStatusEnabled,
			CreatedAt:   time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC),
			UpdatedAt:   time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC),
		}},
	}
	mux := http.NewServeMux()
	RegisterInternalRoutes(mux, store, "token")

	req := httptest.NewRequest(http.MethodGet, "/internal/v1/workspaces/wsp_1/api-keys", nil)
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "hash-must-not-leak") || strings.Contains(body, "key_hash") || strings.Contains(body, "tl_live_secret") {
		t.Fatalf("response leaked secret material: %s", body)
	}
	var resp WorkspaceAPIKeysResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.APIKeys) != 1 || resp.APIKeys[0].ID != "ak_1" || resp.APIKeys[0].SecretLast4 != "wxyz" {
		t.Fatalf("api keys = %+v", resp.APIKeys)
	}
	if store.listAPIKeysWorkspaceID != "wsp_1" {
		t.Fatalf("list workspace = %q, want wsp_1", store.listAPIKeysWorkspaceID)
	}
}

func TestInternalAPIRuntimeSyncReplaysWorkspaceAPIKeys(t *testing.T) {
	t.Parallel()

	tenantCode := "tenant_a"
	store := &fakeInternalStore{
		workspace: domain.Workspace{ID: "wsp_1", TenantCode: &tenantCode, Status: domain.WorkspaceStatusActive},
		apiKeys: []domain.APIKey{
			{ID: "ak_enabled", WorkspaceID: "wsp_1", KeyHash: "hash_enabled", Status: domain.APIKeyStatusEnabled, CreatedByUserID: "usr_1"},
			{ID: "ak_disabled", WorkspaceID: "wsp_1", KeyHash: "hash_disabled", Status: domain.APIKeyStatusDisabled, CreatedByUserID: "usr_1"},
		},
	}
	syncer := &fakeInternalRuntimeSyncer{}
	mux := http.NewServeMux()
	RegisterInternalRoutes(mux, store, "token", syncer)

	req := httptest.NewRequest(http.MethodPost, "/internal/v1/workspaces/wsp_1/runtime-sync", nil)
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if len(syncer.upserts) != 1 || syncer.upserts[0].KeyHash != "hash_enabled" || syncer.upserts[0].Tenant != "tenant_a" {
		t.Fatalf("upserts = %+v", syncer.upserts)
	}
	if len(syncer.deletes) != 1 || syncer.deletes[0] != "hash_disabled" {
		t.Fatalf("deletes = %+v", syncer.deletes)
	}
}

func TestInternalAPIBindTenantTriggersRuntimeSync(t *testing.T) {
	t.Parallel()

	store := &fakeInternalStore{
		workspace: domain.Workspace{ID: "wsp_1", Status: domain.WorkspaceStatusActive},
		apiKeys: []domain.APIKey{{
			ID:              "ak_1",
			WorkspaceID:     "wsp_1",
			KeyHash:         "hash_1",
			Status:          domain.APIKeyStatusEnabled,
			CreatedByUserID: "usr_1",
		}},
	}
	syncer := &fakeInternalRuntimeSyncer{}
	mux := http.NewServeMux()
	RegisterInternalRoutes(mux, store, "token", syncer)

	req := httptest.NewRequest(http.MethodPost, "/internal/v1/workspaces/wsp_1/bind-tenant", strings.NewReader(`{"tenant_code":"tenant_a"}`))
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if store.boundWorkspaceID != "wsp_1" || store.boundTenantCode == nil || *store.boundTenantCode != "tenant_a" {
		t.Fatalf("bind call = workspace:%q tenant:%v", store.boundWorkspaceID, store.boundTenantCode)
	}
	if len(syncer.upserts) != 1 || syncer.upserts[0].KeyHash != "hash_1" || syncer.upserts[0].Tenant != "tenant_a" {
		t.Fatalf("upserts = %+v", syncer.upserts)
	}
}

func TestInternalAPIBindTenantIgnoresRuntimeSyncFailure(t *testing.T) {
	t.Parallel()

	store := &fakeInternalStore{
		workspace: domain.Workspace{ID: "wsp_1", Status: domain.WorkspaceStatusActive},
		apiKeys: []domain.APIKey{{
			ID:              "ak_1",
			WorkspaceID:     "wsp_1",
			KeyHash:         "hash_1",
			Status:          domain.APIKeyStatusEnabled,
			CreatedByUserID: "usr_1",
		}},
	}
	syncer := &fakeInternalRuntimeSyncer{upsertErr: errors.New("redis down")}
	mux := http.NewServeMux()
	RegisterInternalRoutes(mux, store, "token", syncer)

	req := httptest.NewRequest(http.MethodPost, "/internal/v1/workspaces/wsp_1/bind-tenant", strings.NewReader(`{"tenant_code":"tenant_a"}`))
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d body = %s, want 204 despite sync failure", rec.Code, rec.Body.String())
	}
	if store.boundTenantCode == nil || *store.boundTenantCode != "tenant_a" {
		t.Fatalf("tenant was not bound: %v", store.boundTenantCode)
	}
	if len(syncer.upserts) != 1 || syncer.upserts[0].KeyHash != "hash_1" {
		t.Fatalf("upserts = %+v, want attempted sync", syncer.upserts)
	}
}

func TestInternalAPIRuntimeSyncReturnsErrorOnRuntimeSyncFailure(t *testing.T) {
	t.Parallel()

	tenantCode := "tenant_a"
	store := &fakeInternalStore{
		workspace: domain.Workspace{ID: "wsp_1", TenantCode: &tenantCode, Status: domain.WorkspaceStatusActive},
		apiKeys: []domain.APIKey{{
			ID:              "ak_1",
			WorkspaceID:     "wsp_1",
			KeyHash:         "hash_1",
			Status:          domain.APIKeyStatusEnabled,
			CreatedByUserID: "usr_1",
		}},
	}
	syncer := &fakeInternalRuntimeSyncer{upsertErr: errors.New("redis down")}
	mux := http.NewServeMux()
	RegisterInternalRoutes(mux, store, "token", syncer)

	req := httptest.NewRequest(http.MethodPost, "/internal/v1/workspaces/wsp_1/runtime-sync", nil)
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d body = %s, want 500 for explicit runtime sync", rec.Code, rec.Body.String())
	}
	if len(syncer.upserts) != 1 || syncer.upserts[0].KeyHash != "hash_1" {
		t.Fatalf("upserts = %+v, want attempted sync", syncer.upserts)
	}
}

func TestInternalAPIPublishModelCatalog(t *testing.T) {
	t.Parallel()

	store := &fakeInternalStore{}
	mux := http.NewServeMux()
	RegisterInternalRoutes(mux, store, "token")

	body := `{
		"catalog": {
			"model_id": "gpt-4.1-mini",
			"slug": "gpt-4-1-mini",
			"status": "available",
			"visibility": "public",
			"logo_url": "https://example.com/logo.png",
			"context_length": 128000,
			"input_modalities": ["text"],
			"output_modalities": ["text"],
			"capabilities": ["chat"],
			"featured": true,
			"sort_weight": 50,
			"published_at": "2026-07-05T01:02:03Z"
		},
		"i18n": [{
			"locale": "en",
			"display_name": "GPT 4.1 Mini",
			"short_description": "Fast small model",
			"long_description": "Fast small model for production.",
			"seo_title": "GPT 4.1 Mini",
			"seo_description": "Fast small model",
			"tags": ["fast", "cheap"]
		}],
		"prices": [{
			"id": "prc_4_1_mini",
			"currency": "CNY",
			"input_price": 0.001,
			"output_price": 0.002,
			"cached_price": 0.0005,
			"effective_from": "2026-07-05T01:00:00Z",
			"status": "active",
			"published_at": "2026-07-05T01:02:03Z"
		}]
	}`

	req := httptest.NewRequest(http.MethodPost, "/internal/v1/model-catalogs/publish", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if store.publishedModel.Catalog.ModelID != "gpt-4.1-mini" || store.publishedModel.Catalog.Slug != "gpt-4-1-mini" {
		t.Fatalf("catalog = %+v", store.publishedModel.Catalog)
	}
	if len(store.publishedModel.I18n) != 1 || store.publishedModel.I18n[0].ModelID != "gpt-4.1-mini" || store.publishedModel.I18n[0].Locale != "en" {
		t.Fatalf("i18n = %+v", store.publishedModel.I18n)
	}
	if len(store.publishedModel.Prices) != 1 || store.publishedModel.Prices[0].ModelID != "gpt-4.1-mini" || store.publishedModel.Prices[0].OutputPrice != 0.002 {
		t.Fatalf("prices = %+v", store.publishedModel.Prices)
	}
}

func TestInternalAPIPublishModelCatalogRejectsInvalidRequest(t *testing.T) {
	t.Parallel()

	store := &fakeInternalStore{}
	mux := http.NewServeMux()
	RegisterInternalRoutes(mux, store, "token")

	req := httptest.NewRequest(http.MethodPost, "/internal/v1/model-catalogs/publish", strings.NewReader(`{"catalog":{"model_id":"missing-slug"}}`))
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if store.publishCalled {
		t.Fatal("publish should not be called for invalid request")
	}
}

func TestInternalAPIRoutes(t *testing.T) {
	repos := testRepos(t)
	ctx := context.Background()
	nowNano := time.Now().UnixNano()
	suffix := fmt.Sprintf("%d", nowNano)

	// 创建测试用户与工作空间
	email := "internal-test-" + suffix + "@example.com"
	userResult, err := repos.CreateUserWithDefaultWorkspace(ctx, repository.CreateUserWithWorkspaceInput{
		DisplayName:   "Internal Test User",
		PrimaryEmail:  &email,
		WorkspaceName: "Internal Test WS " + suffix,
		WorkspaceSlug: "internal-test-ws-" + suffix,
	})
	if err != nil {
		t.Fatalf("create user and workspace: %v", err)
	}

	wsID := userResult.Workspace.ID
	userID := userResult.User.ID

	internalToken := "test-internal-token-123"
	mux := http.NewServeMux()
	RegisterInternalRoutes(mux, repos, internalToken)

	// Helper function to send requests
	sendReq := func(method, path string, body []byte, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, bytes.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}

	t.Run("Unauthorized check", func(t *testing.T) {
		rec := sendReq(http.MethodGet, "/internal/v1/users/search?keyword=Internal", nil, "wrong-token")
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}

		rec = sendReq(http.MethodGet, "/internal/v1/workspaces/search", nil, "")
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("Search users", func(t *testing.T) {
		rec := sendReq(http.MethodGet, "/internal/v1/users/search?keyword="+userID, nil, internalToken)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp SearchUsersResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}

		if len(resp.Users) != 1 || resp.Users[0].ID != userID {
			t.Errorf("expected user ID %s, got %+v", userID, resp.Users)
		}
	})

	t.Run("Search workspaces", func(t *testing.T) {
		rec := sendReq(http.MethodGet, "/internal/v1/workspaces/search?keyword="+wsID, nil, internalToken)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var resp SearchWorkspacesResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}

		if len(resp.Workspaces) != 1 || resp.Workspaces[0].ID != wsID {
			t.Errorf("expected workspace ID %s, got %+v", wsID, resp.Workspaces)
		}
	})

	t.Run("Bind and unbind tenant", func(t *testing.T) {
		// 1. 验证初始状态无 TenantCode
		db := repos.DB()
		var ws domain.Workspace
		if err := db.First(&ws, "id = ?", wsID).Error; err != nil {
			t.Fatalf("get workspace: %v", err)
		}
		if ws.TenantCode != nil {
			t.Errorf("expected nil tenant_code, got %v", ws.TenantCode)
		}

		// 2. 绑定租户
		reqBody, _ := json.Marshal(BindTenantRequest{TenantCode: "test-tenant-code"})
		rec := sendReq(http.MethodPost, "/internal/v1/workspaces/"+wsID+"/bind-tenant", reqBody, internalToken)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
		}

		if err := db.First(&ws, "id = ?", wsID).Error; err != nil {
			t.Fatalf("get workspace: %v", err)
		}
		if ws.TenantCode == nil || *ws.TenantCode != "test-tenant-code" {
			t.Errorf("expected tenant_code 'test-tenant-code', got %v", ws.TenantCode)
		}

		// 3. 解绑租户
		rec = sendReq(http.MethodPost, "/internal/v1/workspaces/"+wsID+"/unbind-tenant", nil, internalToken)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", rec.Code)
		}

		if err := db.First(&ws, "id = ?", wsID).Error; err != nil {
			t.Fatalf("get workspace: %v", err)
		}
		if ws.TenantCode != nil {
			t.Errorf("expected nil tenant_code, got %s", *ws.TenantCode)
		}
	})
}

type fakeInternalStore struct {
	users []domain.User

	workspaces []domain.Workspace
	workspace  domain.Workspace

	apiKeys                []domain.APIKey
	listAPIKeysWorkspaceID string

	boundWorkspaceID string
	boundTenantCode  *string

	publishedModel PublishModelCatalogInput
	publishCalled  bool
}

func (f *fakeInternalStore) SearchUsers(context.Context, string, int) ([]domain.User, error) {
	return f.users, nil
}

func (f *fakeInternalStore) SearchWorkspaces(context.Context, string, int) ([]domain.Workspace, error) {
	if len(f.workspaces) > 0 {
		return f.workspaces, nil
	}
	return []domain.Workspace{f.workspace}, nil
}

func (f *fakeInternalStore) BindTenantCode(_ context.Context, id string, tenantCode *string) error {
	f.boundWorkspaceID = id
	f.boundTenantCode = tenantCode
	f.workspace.ID = id
	f.workspace.TenantCode = tenantCode
	return nil
}

func (f *fakeInternalStore) FindWorkspaceByID(context.Context, string) (domain.Workspace, error) {
	return f.workspace, nil
}

func (f *fakeInternalStore) ListAPIKeysByWorkspace(_ context.Context, workspaceID string) ([]domain.APIKey, error) {
	f.listAPIKeysWorkspaceID = workspaceID
	return f.apiKeys, nil
}

func (f *fakeInternalStore) PublishModelCatalog(_ context.Context, input PublishModelCatalogInput) error {
	f.publishCalled = true
	f.publishedModel = input
	return nil
}

type fakeInternalRuntimeSyncer struct {
	upserts   []APIKeyRuntimeRecord
	deletes   []string
	upsertErr error
	deleteErr error
}

func (f *fakeInternalRuntimeSyncer) UpsertAPIKey(_ context.Context, record APIKeyRuntimeRecord) error {
	f.upserts = append(f.upserts, record)
	return f.upsertErr
}

func (f *fakeInternalRuntimeSyncer) DeleteAPIKey(_ context.Context, keyHash string) error {
	f.deletes = append(f.deletes, keyHash)
	return f.deleteErr
}
