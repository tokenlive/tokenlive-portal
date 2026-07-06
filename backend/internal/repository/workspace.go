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
	ErrWorkspaceLimitExceeded         = errors.New("workspace self-created limit exceeded")
	ErrWorkspaceNotFound              = errors.New("workspace not found")
	ErrWorkspaceRuntimeAccessNotFound = errors.New("workspace runtime access not found")
	ErrUserNotFound                   = errors.New("user not found")
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

type UpsertWorkspaceRuntimeAccessInput struct {
	WorkspaceID string
	ScopeType   domain.RuntimeAccessScopeType
	ScopeCode   string
	Actor       string
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

func (r *Repositories) FindWorkspaceRuntimeAccess(ctx context.Context, workspaceID string) (domain.WorkspaceRuntimeAccess, error) {
	var access domain.WorkspaceRuntimeAccess
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		First(&access).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.WorkspaceRuntimeAccess{}, ErrWorkspaceRuntimeAccessNotFound
		}
		return domain.WorkspaceRuntimeAccess{}, fmt.Errorf("find workspace runtime access: %w", err)
	}
	return access, nil
}

func (r *Repositories) UpsertWorkspaceRuntimeAccess(ctx context.Context, input UpsertWorkspaceRuntimeAccessInput) (domain.WorkspaceRuntimeAccess, error) {
	now := time.Now().UTC()
	var access domain.WorkspaceRuntimeAccess
	err := r.withTx(ctx, func(tx *gorm.DB) error {
		var workspace domain.Workspace
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND deleted_at IS NULL", input.WorkspaceID).
			First(&workspace).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrWorkspaceNotFound
			}
			return fmt.Errorf("lock workspace: %w", err)
		}

		access = domain.WorkspaceRuntimeAccess{
			WorkspaceID: input.WorkspaceID,
			ScopeType:   input.ScopeType,
			ScopeCode:   input.ScopeCode,
			Status:      domain.RuntimeAccessStatusActive,
			ActivatedAt: &now,
			ActivatedBy: input.Actor,
			DisabledAt:  nil,
			DisabledBy:  "",
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "workspace_id"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"scope_type":   access.ScopeType,
				"scope_code":   access.ScopeCode,
				"status":       access.Status,
				"activated_at": access.ActivatedAt,
				"activated_by": access.ActivatedBy,
				"disabled_at":  nil,
				"disabled_by":  "",
				"updated_at":   access.UpdatedAt,
			}),
		}).Create(&access).Error
	})
	if err != nil {
		return domain.WorkspaceRuntimeAccess{}, err
	}
	return r.FindWorkspaceRuntimeAccess(ctx, input.WorkspaceID)
}

func (r *Repositories) DisableWorkspaceRuntimeAccess(ctx context.Context, workspaceID string, actor string) (domain.WorkspaceRuntimeAccess, error) {
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).Model(&domain.WorkspaceRuntimeAccess{}).
		Where("workspace_id = ?", workspaceID).
		Updates(map[string]interface{}{
			"status":      domain.RuntimeAccessStatusDisabled,
			"disabled_at": &now,
			"disabled_by": actor,
			"updated_at":  now,
		})
	if result.Error != nil {
		return domain.WorkspaceRuntimeAccess{}, fmt.Errorf("disable workspace runtime access: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.WorkspaceRuntimeAccess{}, ErrWorkspaceRuntimeAccessNotFound
	}
	return r.FindWorkspaceRuntimeAccess(ctx, workspaceID)
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
