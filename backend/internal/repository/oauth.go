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

var (
	ErrAccountIdentityNotFound = errors.New("account identity not found")
	ErrUserEmailExists         = errors.New("user with this email already exists")
	ErrIdentityAlreadyBound    = errors.New("identity already bound to another user")
	ErrUserAlreadyHasProvider  = errors.New("user already has this provider bound")
)

type CreateOAuthUserInput struct {
	Provider        string
	ProviderSubject string
	Email           string
	EmailVerified   bool
	DisplayName     string
	AvatarURL       string
}

type CreateOAuthUserResult struct {
	User     domain.User
	Identity domain.AccountIdentity
}

type LinkOAuthIdentityInput struct {
	UserID          string
	Provider        string
	ProviderSubject string
	Email           string
	EmailVerified   bool
	DisplayName     string
	AvatarURL       string
}

type CompleteUserOnboardingInput struct {
	UserID             string
	WorkspaceName      string
	WorkspaceSlug      string
	TrialCredit        TrialCreditInput
}

type CompleteUserOnboardingResult struct {
	User      domain.User
	Workspace domain.Workspace
}

// FindAccountIdentityByProviderSubject 根据 (provider, subject) 查找第三方身份。
func (r *Repositories) FindAccountIdentityByProviderSubject(ctx context.Context, provider, subject string) (domain.AccountIdentity, error) {
	var identity domain.AccountIdentity
	err := r.db.WithContext(ctx).
		Where("provider = ? AND provider_subject = ?", provider, subject).
		First(&identity).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.AccountIdentity{}, ErrAccountIdentityNotFound
		}
		return domain.AccountIdentity{}, fmt.Errorf("find account identity by provider subject: %w", err)
	}
	return identity, nil
}

// FindAccountIdentityByUserProvider 查找某用户绑定的某 provider 身份。
func (r *Repositories) FindAccountIdentityByUserProvider(ctx context.Context, userID, provider string) (domain.AccountIdentity, error) {
	var identity domain.AccountIdentity
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND provider = ?", userID, provider).
		First(&identity).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.AccountIdentity{}, ErrAccountIdentityNotFound
		}
		return domain.AccountIdentity{}, fmt.Errorf("find account identity by user provider: %w", err)
	}
	return identity, nil
}

// ListAccountIdentitiesByUserID 列出用户已绑定的所有第三方账号。
func (r *Repositories) ListAccountIdentitiesByUserID(ctx context.Context, userID string) ([]domain.AccountIdentity, error) {
	var identities []domain.AccountIdentity
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("linked_at ASC, created_at ASC").
		Find(&identities).Error; err != nil {
		return nil, fmt.Errorf("list account identities: %w", err)
	}
	return identities, nil
}

// CreateOAuthUser 事务中创建通过 OAuth 注册的新用户及其第三方身份。
// 此时用户尚未接受条款、尚未创建 Workspace、尚未发试用金。
func (r *Repositories) CreateOAuthUser(ctx context.Context, input CreateOAuthUserInput) (CreateOAuthUserResult, error) {
	var result CreateOAuthUserResult

	err := r.withTx(ctx, func(tx *gorm.DB) error {
		// 1. 检查 (provider, subject) 是否已存在
		var existing domain.AccountIdentity
		err := tx.Where("provider = ? AND provider_subject = ?", input.Provider, input.ProviderSubject).
			First(&existing).Error
		if err == nil {
			return ErrIdentityAlreadyBound
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("check existing identity: %w", err)
		}

		// 2. 如果有 email，检查是否已被其他用户使用（拒绝策略）
		if input.Email != "" {
			var existingUser domain.User
			err := tx.Where("primary_email = ?", input.Email).First(&existingUser).Error
			if err == nil {
				return ErrUserEmailExists
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("check existing email user: %w", err)
			}
		}

		now := time.Now().UTC()

		// 3. 创建用户
		userID, err := newID("usr_")
		if err != nil {
			return err
		}

		var primaryEmail *string
		if input.Email != "" {
			e := input.Email
			primaryEmail = &e
		}

		user := domain.User{
			ID:           userID,
			DisplayName:  input.DisplayName,
			PrimaryEmail: primaryEmail,
			AvatarURL:    input.AvatarURL,
			Status:       domain.UserStatusActive,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if input.EmailVerified {
			user.EmailVerifiedAt = &now
		}
		// terms_accepted_at 保持 NULL，等待用户接受条款

		if err := tx.Create(&user).Error; err != nil {
			return fmt.Errorf("create oauth user: %w", err)
		}

		// 4. 创建第三方身份
		identityID, err := newID("aid_")
		if err != nil {
			return err
		}

		identity := domain.AccountIdentity{
			ID:              identityID,
			UserID:          userID,
			Provider:        input.Provider,
			ProviderSubject: input.ProviderSubject,
			Email:           input.Email,
			EmailVerified:   input.EmailVerified,
			DisplayName:     input.DisplayName,
			AvatarURL:       input.AvatarURL,
			LinkedAt:        &now,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := tx.Create(&identity).Error; err != nil {
			if isMySQLDuplicateError(err) {
				return ErrIdentityAlreadyBound
			}
			return fmt.Errorf("create account identity: %w", err)
		}

		result = CreateOAuthUserResult{
			User:     user,
			Identity: identity,
		}
		return nil
	})
	if err != nil {
		return CreateOAuthUserResult{}, err
	}
	return result, nil
}

// LinkOAuthIdentity 为已登录用户绑定一个第三方账号。
// 前置条件：(provider, subject) 未被绑定；该 user 尚未绑定同一 provider。
func (r *Repositories) LinkOAuthIdentity(ctx context.Context, input LinkOAuthIdentityInput) (domain.AccountIdentity, error) {
	var identity domain.AccountIdentity

	err := r.withTx(ctx, func(tx *gorm.DB) error {
		// 1. 该 identity 是否已被其他用户绑定
		var existing domain.AccountIdentity
		err := tx.Where("provider = ? AND provider_subject = ?", input.Provider, input.ProviderSubject).
			First(&existing).Error
		if err == nil {
			if existing.UserID != input.UserID {
				return ErrIdentityAlreadyBound
			}
			identity = existing
			return nil // 已绑定到当前用户，幂等返回
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("check existing identity: %w", err)
		}

		// 2. 当前用户是否已绑定该 provider
		var userExisting domain.AccountIdentity
		err = tx.Where("user_id = ? AND provider = ?", input.UserID, input.Provider).
			First(&userExisting).Error
		if err == nil {
			return ErrUserAlreadyHasProvider
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("check user existing provider: %w", err)
		}

		now := time.Now().UTC()

		identityID, err := newID("aid_")
		if err != nil {
			return err
		}

		identity = domain.AccountIdentity{
			ID:              identityID,
			UserID:          input.UserID,
			Provider:        input.Provider,
			ProviderSubject: input.ProviderSubject,
			Email:           input.Email,
			EmailVerified:   input.EmailVerified,
			DisplayName:     input.DisplayName,
			AvatarURL:       input.AvatarURL,
			LinkedAt:        &now,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := tx.Create(&identity).Error; err != nil {
			if isMySQLDuplicateError(err) {
				return ErrIdentityAlreadyBound
			}
			return fmt.Errorf("create linked identity: %w", err)
		}

		// 3. 如果用户之前没有头像或昵称，用 OAuth 资料填充
		updates := map[string]any{"updated_at": now}
		var user domain.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", input.UserID).First(&user).Error; err != nil {
			return fmt.Errorf("lock user for profile update: %w", err)
		}
		if user.DisplayName == "" && input.DisplayName != "" {
			updates["display_name"] = input.DisplayName
		}
		if user.AvatarURL == "" && input.AvatarURL != "" {
			updates["avatar_url"] = input.AvatarURL
		}
		if user.PrimaryEmail == nil && input.Email != "" {
			updates["primary_email"] = input.Email
			if input.EmailVerified {
				updates["email_verified_at"] = now
			}
		}
		if len(updates) > 1 {
			if err := tx.Model(&domain.User{}).Where("id = ?", input.UserID).Updates(updates).Error; err != nil {
				return fmt.Errorf("update user profile from oauth: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return domain.AccountIdentity{}, err
	}
	return identity, nil
}

// createDefaultWorkspaceInTx 在事务中为用户创建默认 Workspace + Owner 成员关系 + 余额。
// 不发放试用金（调用方负责），不修改用户 terms_accepted_at。
func createDefaultWorkspaceInTx(tx *gorm.DB, userID, workspaceName, workspaceSlug string, now time.Time) (domain.Workspace, error) {
	workspaceID, err := newID("wsp_")
	if err != nil {
		return domain.Workspace{}, err
	}

	workspace := domain.Workspace{
		ID:              workspaceID,
		Name:            workspaceName,
		Slug:            workspaceSlug,
		OwnerUserID:     userID,
		Status:          domain.WorkspaceStatusActive,
		CreatedByUserID: userID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := tx.Create(&workspace).Error; err != nil {
		return domain.Workspace{}, fmt.Errorf("create workspace: %w", err)
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
		return domain.Workspace{}, fmt.Errorf("create owner membership: %w", err)
	}

	balance := domain.WorkspaceBalance{
		WorkspaceID:       workspaceID,
		AvailableMicroCNY: 0,
		FrozenMicroCNY:    0,
		Version:           1,
		UpdatedAt:         now,
	}
	if err := tx.Create(&balance).Error; err != nil {
		return domain.Workspace{}, fmt.Errorf("create workspace balance: %w", err)
	}

	return workspace, nil
}

// CreateSessionForOAuthUser 为 OAuth 流程创建 session（独立方法，复用 CreateSession）。
func (r *Repositories) CreateSessionForOAuthUser(ctx context.Context, userID, tokenHash, ip, userAgent string, expiresAt time.Time) (domain.UserSession, error) {
	return r.CreateSession(ctx, CreateSessionInput{
		UserID:    userID,
		TokenHash: tokenHash,
		IP:        ip,
		UserAgent: userAgent,
		ExpiresAt: expiresAt,
	})
}

// CompleteUserOnboarding 用户接受条款后，创建默认 Workspace + 发放试用金 + 标记 terms_accepted_at。
func (r *Repositories) CompleteUserOnboarding(ctx context.Context, input CompleteUserOnboardingInput) (CompleteUserOnboardingResult, error) {
	var result CompleteUserOnboardingResult

	err := r.withTx(ctx, func(tx *gorm.DB) error {
		now := time.Now().UTC()

		// 1. 锁定用户行
		var user domain.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", input.UserID).
			First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrUserNotFound
			}
			return fmt.Errorf("lock user for onboarding: %w", err)
		}

		// 2. 如果 terms 已接受，幂等查询已有 workspace 并返回
		if user.TermsAcceptedAt != nil {
			// 查找用户的第一个 workspace（owner 优先）
			var workspace domain.Workspace
			err := tx.Joins("JOIN workspace_members ON workspace_members.workspace_id = workspaces.id").
				Where("workspace_members.user_id = ? AND workspace_members.role = ? AND workspaces.status = ? AND workspace_members.status = ?",
					input.UserID, domain.MemberRoleOwner, domain.WorkspaceStatusActive, domain.MemberStatusActive).
				Order("workspaces.created_at ASC").
				First(&workspace).Error
			if err != nil {
				return fmt.Errorf("find existing workspace on onboarding: %w", err)
			}
			result = CompleteUserOnboardingResult{User: user, Workspace: workspace}
			return nil
		}

		// 3. 标记 terms accepted
		if err := tx.Model(&domain.User{}).
			Where("id = ?", input.UserID).
			Updates(map[string]any{
				"terms_accepted_at": now,
				"updated_at":        now,
			}).Error; err != nil {
			return fmt.Errorf("mark terms accepted: %w", err)
		}
		user.TermsAcceptedAt = &now
		user.UpdatedAt = now

		// 4. 创建默认 Workspace + Owner Member + Balance
		workspace, err := createDefaultWorkspaceInTx(tx, user.ID, input.WorkspaceName, input.WorkspaceSlug, now)
		if err != nil {
			return err
		}

		// 5. 发放试用金
		if err := grantTrialCreditInTx(tx, workspace.ID, now, input.TrialCredit); err != nil {
			return fmt.Errorf("grant trial credit on onboarding: %w", err)
		}

		result = CompleteUserOnboardingResult{
			User:      user,
			Workspace: workspace,
		}
		return nil
	})
	if err != nil {
		return CompleteUserOnboardingResult{}, err
	}
	return result, nil
}
