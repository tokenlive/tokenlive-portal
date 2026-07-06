package api

import (
	"context"
	"strings"
	"time"

	"github.com/tokenlive/tokenlive-portal/backend/internal/domain"
)

type APIKeyRuntimeRecord struct {
	KeyHash     string
	KeyID       string
	UserID      string
	WorkspaceID string
	ScopeType   string
	ScopeCode   string
	Tenant      string
	UserTenant  string
	Status      int
	Quota       int64
	ExpiresAt   int64
}

type APIKeyRuntimeSyncer interface {
	UpsertAPIKey(ctx context.Context, record APIKeyRuntimeRecord) error
	DeleteAPIKey(ctx context.Context, keyHash string) error
}

type noopAPIKeyRuntimeSyncer struct{}

func NewNoopAPIKeyRuntimeSyncer() APIKeyRuntimeSyncer {
	return noopAPIKeyRuntimeSyncer{}
}

func (noopAPIKeyRuntimeSyncer) UpsertAPIKey(context.Context, APIKeyRuntimeRecord) error {
	return nil
}

func (noopAPIKeyRuntimeSyncer) DeleteAPIKey(context.Context, string) error {
	return nil
}

func runtimeRecordFromAPIKey(workspace domain.Workspace, access domain.WorkspaceRuntimeAccess, key domain.APIKey) (APIKeyRuntimeRecord, bool) {
	keyHash := strings.TrimSpace(key.KeyHash)
	if keyHash == "" {
		return APIKeyRuntimeRecord{}, false
	}
	scopeType := strings.TrimSpace(string(access.ScopeType))
	scopeCode := strings.TrimSpace(access.ScopeCode)
	if access.Status != domain.RuntimeAccessStatusActive || scopeType == "" || scopeCode == "" {
		return APIKeyRuntimeRecord{KeyHash: keyHash}, false
	}
	if key.Status != domain.APIKeyStatusEnabled {
		return APIKeyRuntimeRecord{KeyHash: keyHash}, false
	}

	return APIKeyRuntimeRecord{
		KeyHash:     keyHash,
		KeyID:       key.ID,
		UserID:      key.CreatedByUserID,
		WorkspaceID: workspace.ID,
		ScopeType:   scopeType,
		ScopeCode:   scopeCode,
		Tenant:      scopeCode,
		UserTenant:  scopeCode,
		Status:      1,
		Quota:       -1,
		ExpiresAt:   expiresAtUnix(key.ExpiresAt),
	}, true
}

func expiresAtUnix(expiresAt *time.Time) int64 {
	if expiresAt == nil {
		return 0
	}
	return expiresAt.UTC().Unix()
}
