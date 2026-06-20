package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tokenlive/tokenlive-portal/backend/internal/domain"
	"github.com/tokenlive/tokenlive-portal/backend/internal/security"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrAPIKeyNotFound     = errors.New("api key not found")
	ErrAPIKeyInvalidState = errors.New("api key invalid state")
)

type CreateAPIKeyInput struct {
	WorkspaceID          string
	Name                 string
	PlaintextKey         string
	Pepper               string
	CreatedByUserID      string
	ExpiresAt            *time.Time
	DailyLimitMicroCNY   *int64
	MonthlyLimitMicroCNY *int64
}

type CreateAPIKeyWithAuditInput struct {
	CreateAPIKeyInput
	ActorUserID string
	IP          string
	UserAgent   string
}

type UpdateAPIKeyStatusWithAuditInput struct {
	WorkspaceID string
	APIKeyID    string
	Status      domain.APIKeyStatus
	ActorUserID string
	IP          string
	UserAgent   string
}

type CreateAPIKeyResult struct {
	APIKey domain.APIKey
	Secret string
}

func (r *Repositories) CreateAPIKey(ctx context.Context, input CreateAPIKeyInput) (CreateAPIKeyResult, error) {
	key := input.PlaintextKey
	if key == "" {
		generated, err := security.GenerateAPIKey("live")
		if err != nil {
			return CreateAPIKeyResult{}, fmt.Errorf("generate api key: %w", err)
		}
		key = generated
	}

	id, err := newID("ak_")
	if err != nil {
		return CreateAPIKeyResult{}, err
	}

	display := security.DisplayParts(key)
	now := time.Now().UTC()
	apiKey := domain.APIKey{
		ID:                   id,
		WorkspaceID:          input.WorkspaceID,
		Name:                 input.Name,
		KeyPrefix:            display.Prefix,
		SecretLast4:          display.Last4,
		KeyHash:              security.HashAPIKey(key, input.Pepper),
		Status:               domain.APIKeyStatusEnabled,
		CreatedByUserID:      input.CreatedByUserID,
		ExpiresAt:            input.ExpiresAt,
		DailyLimitMicroCNY:   input.DailyLimitMicroCNY,
		MonthlyLimitMicroCNY: input.MonthlyLimitMicroCNY,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := r.db.WithContext(ctx).Create(&apiKey).Error; err != nil {
		return CreateAPIKeyResult{}, fmt.Errorf("create api key: %w", err)
	}

	return CreateAPIKeyResult{
		APIKey: apiKey,
		Secret: key,
	}, nil
}

func (r *Repositories) ListAPIKeysByWorkspace(ctx context.Context, workspaceID string) ([]domain.APIKey, error) {
	var keys []domain.APIKey
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Order("created_at DESC").
		Order("id DESC").
		Find(&keys).Error; err != nil {
		return nil, fmt.Errorf("list api keys by workspace: %w", err)
	}

	return keys, nil
}

func (r *Repositories) CreateAPIKeyWithAudit(ctx context.Context, input CreateAPIKeyWithAuditInput) (CreateAPIKeyResult, error) {
	var result CreateAPIKeyResult
	if err := r.withTx(ctx, func(tx *gorm.DB) error {
		txRepos := New(tx)
		created, err := txRepos.CreateAPIKey(ctx, input.CreateAPIKeyInput)
		if err != nil {
			return err
		}
		result = created

		workspaceID := input.WorkspaceID
		actorUserID := input.ActorUserID
		if _, err := txRepos.AppendAuditLog(ctx, AppendAuditInput{
			WorkspaceID:  &workspaceID,
			ActorUserID:  &actorUserID,
			Action:       "api_key.create",
			ResourceType: "api_key",
			ResourceID:   created.APIKey.ID,
			IP:           input.IP,
			UserAgent:    input.UserAgent,
		}); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return CreateAPIKeyResult{}, err
	}

	return result, nil
}

func (r *Repositories) UpdateAPIKeyStatus(ctx context.Context, apiKeyID string, status domain.APIKeyStatus) error {
	return r.withTx(ctx, func(tx *gorm.DB) error {
		var existing domain.APIKey
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", apiKeyID).
			First(&existing).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAPIKeyNotFound
			}
			return fmt.Errorf("lock api key: %w", err)
		}

		if existing.Status == domain.APIKeyStatusRevoked {
			if status == domain.APIKeyStatusRevoked {
				return nil
			}
			return ErrAPIKeyInvalidState
		}

		now := time.Now().UTC()
		updates := map[string]any{
			"status":     status,
			"updated_at": now,
		}
		if status == domain.APIKeyStatusRevoked {
			updates["revoked_at"] = &now
		} else {
			updates["revoked_at"] = nil
		}

		if err := tx.Model(&domain.APIKey{}).
			Where("id = ?", apiKeyID).
			Updates(updates).Error; err != nil {
			return fmt.Errorf("update api key status: %w", err)
		}

		return nil
	})
}

func (r *Repositories) UpdateAPIKeyStatusWithAudit(ctx context.Context, input UpdateAPIKeyStatusWithAuditInput) (domain.APIKey, error) {
	var updated domain.APIKey
	if err := r.withTx(ctx, func(tx *gorm.DB) error {
		var existing domain.APIKey
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND workspace_id = ?", input.APIKeyID, input.WorkspaceID).
			First(&existing).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAPIKeyNotFound
			}
			return fmt.Errorf("lock api key: %w", err)
		}

		if existing.Status == domain.APIKeyStatusRevoked && input.Status != domain.APIKeyStatusRevoked {
			return ErrAPIKeyInvalidState
		}
		if existing.Status == input.Status {
			updated = existing
			return nil
		}

		now := time.Now().UTC()
		updates := map[string]any{
			"status":     input.Status,
			"updated_at": now,
		}
		if input.Status == domain.APIKeyStatusRevoked && existing.RevokedAt == nil {
			updates["revoked_at"] = &now
		}

		if err := tx.Model(&domain.APIKey{}).
			Where("id = ? AND workspace_id = ?", input.APIKeyID, input.WorkspaceID).
			Updates(updates).Error; err != nil {
			return fmt.Errorf("update api key status: %w", err)
		}
		if err := tx.Where("id = ? AND workspace_id = ?", input.APIKeyID, input.WorkspaceID).
			First(&updated).Error; err != nil {
			return fmt.Errorf("reload api key status: %w", err)
		}

		workspaceID := input.WorkspaceID
		actorUserID := input.ActorUserID
		txRepos := New(tx)
		if _, err := txRepos.AppendAuditLog(ctx, AppendAuditInput{
			WorkspaceID:  &workspaceID,
			ActorUserID:  &actorUserID,
			Action:       "api_key." + string(input.Status),
			ResourceType: "api_key",
			ResourceID:   input.APIKeyID,
			IP:           input.IP,
			UserAgent:    input.UserAgent,
		}); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return domain.APIKey{}, err
	}

	return updated, nil
}

func (r *Repositories) ReplaceAPIKeyWhitelist(ctx context.Context, apiKeyID string, modelIDs []string) error {
	return r.withTx(ctx, func(tx *gorm.DB) error {
		if err := tx.Where("api_key_id = ?", apiKeyID).Delete(&domain.APIKeyModelWhitelist{}).Error; err != nil {
			return fmt.Errorf("delete api key whitelist: %w", err)
		}

		now := time.Now().UTC()
		seen := make(map[string]struct{}, len(modelIDs))
		for _, modelID := range modelIDs {
			if _, ok := seen[modelID]; ok {
				continue
			}
			seen[modelID] = struct{}{}

			row := domain.APIKeyModelWhitelist{
				APIKeyID:  apiKeyID,
				ModelID:   modelID,
				CreatedAt: now,
			}
			if err := tx.Create(&row).Error; err != nil {
				return fmt.Errorf("create api key whitelist row: %w", err)
			}
		}

		return nil
	})
}
