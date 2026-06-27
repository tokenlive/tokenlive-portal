package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
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
