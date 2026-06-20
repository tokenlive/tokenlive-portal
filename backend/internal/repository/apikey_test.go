package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tokenlive/tokenlive-portal/backend/internal/domain"
)

type testWorkspaceWithUser struct {
	User      domain.User
	Workspace domain.Workspace
}

func createTestWorkspaceWithUser(t *testing.T, repos *Repositories, slug string) testWorkspaceWithUser {
	t.Helper()

	email := slug + "@example.com"
	result, err := repos.CreateUserWithDefaultWorkspace(context.Background(), CreateUserWithWorkspaceInput{
		DisplayName:   slug,
		PrimaryEmail:  &email,
		WorkspaceName: slug,
		WorkspaceSlug: slug,
	})
	if err != nil {
		t.Fatalf("create user with workspace: %v", err)
	}

	return testWorkspaceWithUser{
		User:      result.User,
		Workspace: result.Workspace,
	}
}

func TestUpdateAPIKeyStatusReturnsNotFound(t *testing.T) {
	db := testDB(t)
	repos := New(db)

	err := repos.UpdateAPIKeyStatus(context.Background(), "ak_missing", domain.APIKeyStatusDisabled)
	if !errors.Is(err, ErrAPIKeyNotFound) {
		t.Fatalf("got err %v, want ErrAPIKeyNotFound", err)
	}
}

func TestUpdateAPIKeyStatusBlocksRevokedRestore(t *testing.T) {
	db := testDB(t)
	repos := New(db)
	ctx := context.Background()
	suffix := uniqueSuffix(t)
	workspace := createTestWorkspaceWithUser(t, repos, "api-key-legacy-restore-"+suffix)

	created, err := repos.CreateAPIKey(ctx, CreateAPIKeyInput{
		WorkspaceID:     workspace.Workspace.ID,
		Name:            "legacy restore",
		PlaintextKey:    "tl_live_legacy_restore_" + suffix,
		Pepper:          "pepper",
		CreatedByUserID: workspace.User.ID,
	})
	if err != nil {
		t.Fatalf("create key: %v", err)
	}
	if err := repos.UpdateAPIKeyStatus(ctx, created.APIKey.ID, domain.APIKeyStatusRevoked); err != nil {
		t.Fatalf("revoke key: %v", err)
	}

	err = repos.UpdateAPIKeyStatus(ctx, created.APIKey.ID, domain.APIKeyStatusEnabled)
	if !errors.Is(err, ErrAPIKeyInvalidState) {
		t.Fatalf("restore revoked err = %v, want ErrAPIKeyInvalidState", err)
	}

	var reloaded domain.APIKey
	if err := db.Where("id = ?", created.APIKey.ID).First(&reloaded).Error; err != nil {
		t.Fatalf("reload key: %v", err)
	}
	if reloaded.Status != domain.APIKeyStatusRevoked || reloaded.RevokedAt == nil {
		t.Fatalf("reloaded key = %+v, want still revoked with revoked_at", reloaded)
	}
}

func TestListAPIKeysByWorkspaceScopesResults(t *testing.T) {
	db := testDB(t)
	repos := New(db)
	ctx := context.Background()
	suffix := uniqueSuffix(t)
	workspaceA := createTestWorkspaceWithUser(t, repos, "api-key-list-a-"+suffix)
	workspaceB := createTestWorkspaceWithUser(t, repos, "api-key-list-b-"+suffix)

	if _, err := repos.CreateAPIKey(ctx, CreateAPIKeyInput{
		WorkspaceID:     workspaceA.Workspace.ID,
		Name:            "A",
		PlaintextKey:    "tl_live_a_" + suffix,
		Pepper:          "pepper",
		CreatedByUserID: workspaceA.User.ID,
	}); err != nil {
		t.Fatalf("create key a: %v", err)
	}
	if _, err := repos.CreateAPIKey(ctx, CreateAPIKeyInput{
		WorkspaceID:     workspaceB.Workspace.ID,
		Name:            "B",
		PlaintextKey:    "tl_live_b_" + suffix,
		Pepper:          "pepper",
		CreatedByUserID: workspaceB.User.ID,
	}); err != nil {
		t.Fatalf("create key b: %v", err)
	}

	keys, err := repos.ListAPIKeysByWorkspace(ctx, workspaceA.Workspace.ID)
	if err != nil {
		t.Fatalf("list keys: %v", err)
	}
	if len(keys) != 1 || keys[0].Name != "A" {
		t.Fatalf("keys = %+v, want only workspace A key", keys)
	}
}

func TestListAPIKeysByWorkspaceReturnsNewestFirst(t *testing.T) {
	db := testDB(t)
	repos := New(db)
	ctx := context.Background()
	suffix := uniqueSuffix(t)
	workspace := createTestWorkspaceWithUser(t, repos, "api-key-list-order-"+suffix)

	oldKey, err := repos.CreateAPIKey(ctx, CreateAPIKeyInput{
		WorkspaceID:     workspace.Workspace.ID,
		Name:            "old",
		PlaintextKey:    "tl_live_old_" + suffix,
		Pepper:          "pepper",
		CreatedByUserID: workspace.User.ID,
	})
	if err != nil {
		t.Fatalf("create old key: %v", err)
	}
	newKey, err := repos.CreateAPIKey(ctx, CreateAPIKeyInput{
		WorkspaceID:     workspace.Workspace.ID,
		Name:            "new",
		PlaintextKey:    "tl_live_new_" + suffix,
		Pepper:          "pepper",
		CreatedByUserID: workspace.User.ID,
	})
	if err != nil {
		t.Fatalf("create new key: %v", err)
	}

	oldCreatedAt := time.Now().UTC().Add(-2 * time.Hour)
	newCreatedAt := time.Now().UTC().Add(-time.Hour)
	if err := db.Model(&domain.APIKey{}).Where("id = ?", oldKey.APIKey.ID).
		Updates(map[string]any{"created_at": oldCreatedAt, "updated_at": oldCreatedAt}).Error; err != nil {
		t.Fatalf("age old key: %v", err)
	}
	if err := db.Model(&domain.APIKey{}).Where("id = ?", newKey.APIKey.ID).
		Updates(map[string]any{"created_at": newCreatedAt, "updated_at": newCreatedAt}).Error; err != nil {
		t.Fatalf("age new key: %v", err)
	}

	keys, err := repos.ListAPIKeysByWorkspace(ctx, workspace.Workspace.ID)
	if err != nil {
		t.Fatalf("list keys: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("key count = %d, want 2", len(keys))
	}
	if keys[0].ID != newKey.APIKey.ID || keys[1].ID != oldKey.APIKey.ID {
		t.Fatalf("key order = [%s, %s], want [%s, %s]", keys[0].ID, keys[1].ID, newKey.APIKey.ID, oldKey.APIKey.ID)
	}
}

func TestCreateAPIKeyPersistsExpiresAt(t *testing.T) {
	db := testDB(t)
	repos := New(db)
	ctx := context.Background()
	suffix := uniqueSuffix(t)
	workspace := createTestWorkspaceWithUser(t, repos, "api-key-expires-"+suffix)
	expiresAt := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)

	created, err := repos.CreateAPIKey(ctx, CreateAPIKeyInput{
		WorkspaceID:     workspace.Workspace.ID,
		Name:            "expires",
		PlaintextKey:    "tl_live_expires_" + suffix,
		Pepper:          "pepper",
		CreatedByUserID: workspace.User.ID,
		ExpiresAt:       &expiresAt,
	})
	if err != nil {
		t.Fatalf("create key: %v", err)
	}

	var reloaded domain.APIKey
	if err := db.Where("id = ?", created.APIKey.ID).First(&reloaded).Error; err != nil {
		t.Fatalf("reload key: %v", err)
	}
	if reloaded.ExpiresAt == nil || !reloaded.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("expires_at = %v, want %v", reloaded.ExpiresAt, expiresAt)
	}
}

func TestUpdateAPIKeyStatusWithAuditScopesWorkspaceAndBlocksRevokedRestore(t *testing.T) {
	db := testDB(t)
	repos := New(db)
	ctx := context.Background()
	suffix := uniqueSuffix(t)
	workspaceA := createTestWorkspaceWithUser(t, repos, "api-key-status-a-"+suffix)
	workspaceB := createTestWorkspaceWithUser(t, repos, "api-key-status-b-"+suffix)

	created, err := repos.CreateAPIKey(ctx, CreateAPIKeyInput{
		WorkspaceID:     workspaceA.Workspace.ID,
		Name:            "A",
		PlaintextKey:    "tl_live_status_" + suffix,
		Pepper:          "pepper",
		CreatedByUserID: workspaceA.User.ID,
	})
	if err != nil {
		t.Fatalf("create key: %v", err)
	}

	_, err = repos.UpdateAPIKeyStatusWithAudit(ctx, UpdateAPIKeyStatusWithAuditInput{
		WorkspaceID: workspaceB.Workspace.ID,
		APIKeyID:    created.APIKey.ID,
		Status:      domain.APIKeyStatusDisabled,
		ActorUserID: workspaceB.User.ID,
	})
	if !errors.Is(err, ErrAPIKeyNotFound) {
		t.Fatalf("cross workspace err = %v, want ErrAPIKeyNotFound", err)
	}

	revoked, err := repos.UpdateAPIKeyStatusWithAudit(ctx, UpdateAPIKeyStatusWithAuditInput{
		WorkspaceID: workspaceA.Workspace.ID,
		APIKeyID:    created.APIKey.ID,
		Status:      domain.APIKeyStatusRevoked,
		ActorUserID: workspaceA.User.ID,
	})
	if err != nil {
		t.Fatalf("revoke key: %v", err)
	}
	if revoked.Status != domain.APIKeyStatusRevoked || revoked.RevokedAt == nil {
		t.Fatalf("revoked key = %+v", revoked)
	}
	preservedUpdatedAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	if err := db.Model(&domain.APIKey{}).
		Where("id = ?", created.APIKey.ID).
		Update("updated_at", preservedUpdatedAt).Error; err != nil {
		t.Fatalf("set preserved updated_at: %v", err)
	}
	var revokeAuditCountBefore int64
	if err := db.Model(&domain.AuditLog{}).
		Where("resource_type = ? AND resource_id = ? AND action = ?", "api_key", created.APIKey.ID, "api_key.revoked").
		Count(&revokeAuditCountBefore).Error; err != nil {
		t.Fatalf("count revoke audit logs before repeat: %v", err)
	}

	revokedAgain, err := repos.UpdateAPIKeyStatusWithAudit(ctx, UpdateAPIKeyStatusWithAuditInput{
		WorkspaceID: workspaceA.Workspace.ID,
		APIKeyID:    created.APIKey.ID,
		Status:      domain.APIKeyStatusRevoked,
		ActorUserID: workspaceA.User.ID,
	})
	if err != nil {
		t.Fatalf("revoke key again: %v", err)
	}
	if !revokedAgain.RevokedAt.Equal(*revoked.RevokedAt) {
		t.Fatalf("second revoke changed revoked_at from %v to %v", revoked.RevokedAt, revokedAgain.RevokedAt)
	}
	if !revokedAgain.UpdatedAt.Equal(preservedUpdatedAt) {
		t.Fatalf("second revoke changed updated_at from %v to %v", preservedUpdatedAt, revokedAgain.UpdatedAt)
	}
	var revokeAuditCountAfter int64
	if err := db.Model(&domain.AuditLog{}).
		Where("resource_type = ? AND resource_id = ? AND action = ?", "api_key", created.APIKey.ID, "api_key.revoked").
		Count(&revokeAuditCountAfter).Error; err != nil {
		t.Fatalf("count revoke audit logs after repeat: %v", err)
	}
	if revokeAuditCountAfter != revokeAuditCountBefore {
		t.Fatalf("revoke audit count changed from %d to %d", revokeAuditCountBefore, revokeAuditCountAfter)
	}

	_, err = repos.UpdateAPIKeyStatusWithAudit(ctx, UpdateAPIKeyStatusWithAuditInput{
		WorkspaceID: workspaceA.Workspace.ID,
		APIKeyID:    created.APIKey.ID,
		Status:      domain.APIKeyStatusEnabled,
		ActorUserID: workspaceA.User.ID,
	})
	if !errors.Is(err, ErrAPIKeyInvalidState) {
		t.Fatalf("restore revoked err = %v, want ErrAPIKeyInvalidState", err)
	}
}

func TestCreateAPIKeyWithAuditWritesAuditLog(t *testing.T) {
	db := testDB(t)
	repos := New(db)
	ctx := context.Background()
	suffix := uniqueSuffix(t)
	workspace := createTestWorkspaceWithUser(t, repos, "api-key-audit-"+suffix)

	created, err := repos.CreateAPIKeyWithAudit(ctx, CreateAPIKeyWithAuditInput{
		CreateAPIKeyInput: CreateAPIKeyInput{
			WorkspaceID:     workspace.Workspace.ID,
			Name:            "Audited",
			PlaintextKey:    "tl_live_audit_" + suffix,
			Pepper:          "pepper",
			CreatedByUserID: workspace.User.ID,
		},
		ActorUserID: workspace.User.ID,
		IP:          "127.0.0.1",
		UserAgent:   "repository-test",
	})
	if err != nil {
		t.Fatalf("create key with audit: %v", err)
	}

	var count int64
	if err := db.Model(&domain.AuditLog{}).
		Where("workspace_id = ? AND actor_user_id = ? AND resource_type = ? AND resource_id = ? AND action = ?",
			workspace.Workspace.ID,
			workspace.User.ID,
			"api_key",
			created.APIKey.ID,
			"api_key.create",
		).
		Count(&count).Error; err != nil {
		t.Fatalf("count audit logs: %v", err)
	}
	if count != 1 {
		t.Fatalf("audit count = %d, want 1", count)
	}
}

func TestUpdateAPIKeyStatusWithAuditWritesStatusActionAuditLogs(t *testing.T) {
	db := testDB(t)
	repos := New(db)
	ctx := context.Background()
	suffix := uniqueSuffix(t)
	workspace := createTestWorkspaceWithUser(t, repos, "api-key-status-audit-"+suffix)

	created, err := repos.CreateAPIKey(ctx, CreateAPIKeyInput{
		WorkspaceID:     workspace.Workspace.ID,
		Name:            "Status Audited",
		PlaintextKey:    "tl_live_status_audit_" + suffix,
		Pepper:          "pepper",
		CreatedByUserID: workspace.User.ID,
	})
	if err != nil {
		t.Fatalf("create key: %v", err)
	}

	for _, status := range []domain.APIKeyStatus{domain.APIKeyStatusDisabled, domain.APIKeyStatusEnabled} {
		if _, err := repos.UpdateAPIKeyStatusWithAudit(ctx, UpdateAPIKeyStatusWithAuditInput{
			WorkspaceID: workspace.Workspace.ID,
			APIKeyID:    created.APIKey.ID,
			Status:      status,
			ActorUserID: workspace.User.ID,
			IP:          "127.0.0.1",
			UserAgent:   "repository-test",
		}); err != nil {
			t.Fatalf("update key to %s: %v", status, err)
		}
	}

	for _, action := range []string{"api_key.disabled", "api_key.enabled"} {
		var count int64
		if err := db.Model(&domain.AuditLog{}).
			Where("workspace_id = ? AND actor_user_id = ? AND resource_type = ? AND resource_id = ? AND action = ?",
				workspace.Workspace.ID,
				workspace.User.ID,
				"api_key",
				created.APIKey.ID,
				action,
			).
			Count(&count).Error; err != nil {
			t.Fatalf("count %s audit logs: %v", action, err)
		}
		if count != 1 {
			t.Fatalf("%s audit count = %d, want 1", action, count)
		}
	}
}

func TestUpdateAPIKeyStatusWithAuditNoopsWhenStatusAlreadyMatches(t *testing.T) {
	db := testDB(t)
	repos := New(db)
	ctx := context.Background()
	suffix := uniqueSuffix(t)
	workspace := createTestWorkspaceWithUser(t, repos, "api-key-status-noop-"+suffix)

	created, err := repos.CreateAPIKey(ctx, CreateAPIKeyInput{
		WorkspaceID:     workspace.Workspace.ID,
		Name:            "Status Noop",
		PlaintextKey:    "tl_live_status_noop_" + suffix,
		Pepper:          "pepper",
		CreatedByUserID: workspace.User.ID,
	})
	if err != nil {
		t.Fatalf("create key: %v", err)
	}

	for _, status := range []domain.APIKeyStatus{domain.APIKeyStatusDisabled, domain.APIKeyStatusEnabled} {
		updated, err := repos.UpdateAPIKeyStatusWithAudit(ctx, UpdateAPIKeyStatusWithAuditInput{
			WorkspaceID: workspace.Workspace.ID,
			APIKeyID:    created.APIKey.ID,
			Status:      status,
			ActorUserID: workspace.User.ID,
		})
		if err != nil {
			t.Fatalf("update key to %s: %v", status, err)
		}

		preservedUpdatedAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
		if err := db.Model(&domain.APIKey{}).
			Where("id = ?", updated.ID).
			Update("updated_at", preservedUpdatedAt).Error; err != nil {
			t.Fatalf("set preserved updated_at for %s: %v", status, err)
		}

		action := "api_key." + string(status)
		var auditCountBefore int64
		if err := db.Model(&domain.AuditLog{}).
			Where("resource_type = ? AND resource_id = ? AND action = ?", "api_key", updated.ID, action).
			Count(&auditCountBefore).Error; err != nil {
			t.Fatalf("count %s audit logs before repeat: %v", action, err)
		}

		repeated, err := repos.UpdateAPIKeyStatusWithAudit(ctx, UpdateAPIKeyStatusWithAuditInput{
			WorkspaceID: workspace.Workspace.ID,
			APIKeyID:    created.APIKey.ID,
			Status:      status,
			ActorUserID: workspace.User.ID,
		})
		if err != nil {
			t.Fatalf("repeat update key to %s: %v", status, err)
		}
		if repeated.Status != status {
			t.Fatalf("status = %s, want %s", repeated.Status, status)
		}
		if !repeated.UpdatedAt.Equal(preservedUpdatedAt) {
			t.Fatalf("repeat %s changed updated_at from %v to %v", status, preservedUpdatedAt, repeated.UpdatedAt)
		}

		var auditCountAfter int64
		if err := db.Model(&domain.AuditLog{}).
			Where("resource_type = ? AND resource_id = ? AND action = ?", "api_key", updated.ID, action).
			Count(&auditCountAfter).Error; err != nil {
			t.Fatalf("count %s audit logs after repeat: %v", action, err)
		}
		if auditCountAfter != auditCountBefore {
			t.Fatalf("%s audit count changed from %d to %d", action, auditCountBefore, auditCountAfter)
		}
	}
}
