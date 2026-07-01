package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tokenlive/tokenlive-portal/backend/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const DefaultSelfCreatedWorkspaceLimit int64 = 3

var (
	ErrWorkspaceLimitExceeded = errors.New("workspace self-created limit exceeded")
	ErrWorkspaceNotFound      = errors.New("workspace not found")
	ErrUserNotFound           = errors.New("user not found")
)

type CurrentWorkspaceResult struct {
	Workspace domain.Workspace
	Member    domain.WorkspaceMember
	Balance   domain.WorkspaceBalance
	Role      domain.MemberRole
}

type CreateWorkspaceInput struct {
	Name            string
	Slug            string
	OwnerUserID     string
	CreatedByUserID string
}

func (r *Repositories) CreateWorkspace(ctx context.Context, input CreateWorkspaceInput) (domain.Workspace, error) {
	var workspace domain.Workspace
	now := time.Now().UTC()

	err := r.withTx(ctx, func(tx *gorm.DB) error {
		var creator domain.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", input.CreatedByUserID).
			First(&creator).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrUserNotFound
			}
			return fmt.Errorf("lock creator user: %w", err)
		}

		var count int64
		if err := tx.Model(&domain.Workspace{}).
			Where("created_by_user_id = ? AND deleted_at IS NULL", input.CreatedByUserID).
			Count(&count).Error; err != nil {
			return fmt.Errorf("count self-created workspaces: %w", err)
		}
		if count >= DefaultSelfCreatedWorkspaceLimit {
			return ErrWorkspaceLimitExceeded
		}

		id, err := newID("wsp_")
		if err != nil {
			return err
		}

		workspace = domain.Workspace{
			ID:              id,
			Name:            input.Name,
			Slug:            input.Slug,
			OwnerUserID:     input.OwnerUserID,
			Status:          domain.WorkspaceStatusActive,
			CreatedByUserID: input.CreatedByUserID,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := tx.Create(&workspace).Error; err != nil {
			return fmt.Errorf("create workspace: %w", err)
		}

		member := domain.WorkspaceMember{
			WorkspaceID: id,
			UserID:      input.OwnerUserID,
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
			WorkspaceID:       id,
			AvailableMicroCNY: 0,
			FrozenMicroCNY:    0,
			Version:           1,
			UpdatedAt:         now,
		}
		if err := tx.Create(&balance).Error; err != nil {
			return fmt.Errorf("create workspace balance: %w", err)
		}

		return nil
	})
	if err != nil {
		return domain.Workspace{}, err
	}

	return workspace, nil
}

func (r *Repositories) ResolveCurrentWorkspace(ctx context.Context, userID string) (CurrentWorkspaceResult, error) {
	var result CurrentWorkspaceResult
	if err := r.db.WithContext(ctx).
		Table("workspaces").
		Select("workspaces.*").
		Joins("JOIN workspace_members ON workspace_members.workspace_id = workspaces.id").
		Where("workspace_members.user_id = ?", userID).
		Where("workspace_members.status = ?", domain.MemberStatusActive).
		Where("workspaces.status = ?", domain.WorkspaceStatusActive).
		Where("workspaces.deleted_at IS NULL").
		Order(clause.Expr{
			SQL:  "CASE WHEN workspaces.owner_user_id = ? THEN 0 ELSE 1 END",
			Vars: []interface{}{userID},
		}).
		Order("COALESCE(workspace_members.joined_at, workspace_members.created_at) ASC").
		Order("workspaces.created_at ASC").
		First(&result.Workspace).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return CurrentWorkspaceResult{}, ErrWorkspaceNotFound
		}
		return CurrentWorkspaceResult{}, fmt.Errorf("resolve current workspace: %w", err)
	}

	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND user_id = ? AND status = ?", result.Workspace.ID, userID, domain.MemberStatusActive).
		First(&result.Member).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return CurrentWorkspaceResult{}, ErrWorkspaceNotFound
		}
		return CurrentWorkspaceResult{}, fmt.Errorf("resolve current workspace member: %w", err)
	}
	result.Role = result.Member.Role

	if err := r.db.WithContext(ctx).
		Where("workspace_id = ?", result.Workspace.ID).
		First(&result.Balance).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return CurrentWorkspaceResult{}, fmt.Errorf("resolve current workspace balance: %w", err)
		}
		result.Balance = domain.WorkspaceBalance{WorkspaceID: result.Workspace.ID}
	}

	return result, nil
}

func (r *Repositories) GrantWorkspaceModel(ctx context.Context, workspaceID string, modelID string, source string, grantedByUserID *string) error {
	permission := domain.WorkspaceModelPermission{
		WorkspaceID:     workspaceID,
		ModelID:         modelID,
		Source:          source,
		GrantedByUserID: grantedByUserID,
		CreatedAt:       time.Now().UTC(),
	}

	if err := r.db.WithContext(ctx).Create(&permission).Error; err != nil {
		return fmt.Errorf("grant workspace model: %w", err)
	}

	return nil
}

// BindTenantCode binds a tenant_code to a workspace. If tenantCode is nil, it unbinds the tenant.
func (r *Repositories) BindTenantCode(ctx context.Context, id string, tenantCode *string) error {
	result := r.db.WithContext(ctx).Model(&domain.Workspace{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("tenant_code", tenantCode)
	if result.Error != nil {
		return fmt.Errorf("bind tenant code: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrWorkspaceNotFound
	}
	return nil
}

func (r *Repositories) FindWorkspaceByID(ctx context.Context, id string) (domain.Workspace, error) {
	var workspace domain.Workspace
	if err := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&workspace).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Workspace{}, ErrWorkspaceNotFound
		}
		return domain.Workspace{}, fmt.Errorf("find workspace by id: %w", err)
	}
	return workspace, nil
}

// SearchWorkspaces searches workspaces by ID, name, or slug.
func (r *Repositories) SearchWorkspaces(ctx context.Context, keyword string, limit int) ([]domain.Workspace, error) {
	var workspaces []domain.Workspace
	db := r.db.WithContext(ctx).Model(&domain.Workspace{}).Where("deleted_at IS NULL")
	if keyword != "" {
		db = db.Where("id = ? OR name LIKE ? OR slug LIKE ?", keyword, "%"+keyword+"%", "%"+keyword+"%")
	}
	if limit <= 0 {
		limit = 20
	}
	err := db.Limit(limit).Find(&workspaces).Error
	return workspaces, err
}
