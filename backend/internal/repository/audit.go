package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/tokenlive/tokenlive-portal/backend/internal/domain"
	"gorm.io/datatypes"
)

type AppendAuditInput struct {
	WorkspaceID  *string
	ActorUserID  *string
	Action       string
	ResourceType string
	ResourceID   string
	BeforeData   datatypes.JSON
	AfterData    datatypes.JSON
	IP           string
	UserAgent    string
}

func (r *Repositories) AppendAuditLog(ctx context.Context, input AppendAuditInput) (domain.AuditLog, error) {
	id, err := newID("aud_")
	if err != nil {
		return domain.AuditLog{}, err
	}

	row := domain.AuditLog{
		ID:           id,
		WorkspaceID:  input.WorkspaceID,
		ActorUserID:  input.ActorUserID,
		Action:       input.Action,
		ResourceType: input.ResourceType,
		ResourceID:   input.ResourceID,
		BeforeData:   input.BeforeData,
		AfterData:    input.AfterData,
		IP:           input.IP,
		UserAgent:    input.UserAgent,
		CreatedAt:    time.Now().UTC(),
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.AuditLog{}, fmt.Errorf("append audit log: %w", err)
	}

	return row, nil
}
