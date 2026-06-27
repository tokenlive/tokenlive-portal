package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/tokenlive/tokenlive-portal/backend/internal/domain"
	"gorm.io/gorm"
)

type CreateUserWithWorkspaceInput struct {
	DisplayName   string
	PrimaryEmail  *string
	WorkspaceName string
	WorkspaceSlug string
}

type CreateUserWithWorkspaceResult struct {
	User      domain.User
	Workspace domain.Workspace
}

func (r *Repositories) CreateUserWithDefaultWorkspace(ctx context.Context, input CreateUserWithWorkspaceInput) (CreateUserWithWorkspaceResult, error) {
	var result CreateUserWithWorkspaceResult
	now := time.Now().UTC()

	err := r.withTx(ctx, func(tx *gorm.DB) error {
		userID, err := newID("usr_")
		if err != nil {
			return err
		}

		workspaceID, err := newID("wsp_")
		if err != nil {
			return err
		}

		user := domain.User{
			ID:           userID,
			DisplayName:  input.DisplayName,
			PrimaryEmail: input.PrimaryEmail,
			Status:       domain.UserStatusActive,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := tx.Create(&user).Error; err != nil {
			return fmt.Errorf("create user: %w", err)
		}

		workspace := domain.Workspace{
			ID:              workspaceID,
			Name:            input.WorkspaceName,
			Slug:            input.WorkspaceSlug,
			OwnerUserID:     userID,
			Status:          domain.WorkspaceStatusActive,
			CreatedByUserID: userID,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := tx.Create(&workspace).Error; err != nil {
			return fmt.Errorf("create workspace: %w", err)
		}

		member := domain.WorkspaceMember{
			WorkspaceID: workspaceID,
			UserID:      userID,
			Role:        domain.MemberRoleOwner,
			Status:      domain.MemberStatusActive,
			JoinedAt:    &now,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := tx.Create(&member).Error; err != nil {
			return fmt.Errorf("create owner membership: %w", err)
		}

		balance := domain.WorkspaceBalance{
			WorkspaceID:       workspaceID,
			AvailableMicroCNY: 0,
			FrozenMicroCNY:    0,
			Version:           1,
			UpdatedAt:         now,
		}
		if err := tx.Create(&balance).Error; err != nil {
			return fmt.Errorf("create workspace balance: %w", err)
		}

		result = CreateUserWithWorkspaceResult{
			User:      user,
			Workspace: workspace,
		}
		return nil
	})
	if err != nil {
		return CreateUserWithWorkspaceResult{}, err
	}

	return result, nil
}

func (r *Repositories) SearchUsers(ctx context.Context, keyword string, limit int) ([]domain.User, error) {
	var users []domain.User
	db := r.db.WithContext(ctx).Model(&domain.User{}).Where("deleted_at IS NULL")
	if keyword != "" {
		db = db.Where("id = ? OR display_name LIKE ? OR primary_email LIKE ?", keyword, "%"+keyword+"%", "%"+keyword+"%")
	}
	if limit <= 0 {
		limit = 20
	}
	err := db.Limit(limit).Find(&users).Error
	return users, err
}
