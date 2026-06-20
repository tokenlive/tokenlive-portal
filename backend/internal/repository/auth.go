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

const maxEmailCodeAttempts = 5

var (
	ErrEmailCodeInvalid = errors.New("email code invalid")
	ErrEmailCodeBlocked = errors.New("email code blocked")
	ErrSessionNotFound  = errors.New("session not found")
)

type CreateEmailVerificationCodeInput struct {
	Email     string
	Purpose   domain.EmailCodePurpose
	CodeHash  string
	ExpiresAt time.Time
}

type CreateSessionInput struct {
	UserID    string
	TokenHash string
	IP        string
	UserAgent string
	ExpiresAt time.Time
}

type TrialCreditInput struct {
	AmountMicroCNY int64
	TTLDays        int
}

type CompleteEmailLoginInput struct {
	Email            string
	Purpose          domain.EmailCodePurpose
	Code             string
	Pepper           string
	DisplayName      string
	WorkspaceName    string
	WorkspaceSlug    string
	SessionTokenHash string
	SessionIP        string
	SessionUserAgent string
	SessionExpiresAt time.Time
	EmailVerifiedAt  time.Time
	TrialCredit      TrialCreditInput
}

type CompleteEmailLoginResult struct {
	User    domain.User
	Session domain.UserSession
}

type emailUserWorkspaceResult struct {
	User             domain.User
	Workspace        domain.Workspace
	CreatedWorkspace bool
}

func (r *Repositories) CreateEmailVerificationCode(ctx context.Context, input CreateEmailVerificationCodeInput) (domain.EmailVerificationCode, error) {
	id, err := newID("emc_")
	if err != nil {
		return domain.EmailVerificationCode{}, err
	}

	code := domain.EmailVerificationCode{
		ID:           id,
		Email:        input.Email,
		Purpose:      input.Purpose,
		CodeHash:     input.CodeHash,
		Status:       domain.EmailCodeStatusPending,
		AttemptCount: 0,
		ExpiresAt:    input.ExpiresAt.UTC(),
		CreatedAt:    time.Now().UTC(),
	}
	if err := r.db.WithContext(ctx).Create(&code).Error; err != nil {
		return domain.EmailVerificationCode{}, fmt.Errorf("create email verification code: %w", err)
	}

	return code, nil
}

func (r *Repositories) VerifyEmailCode(ctx context.Context, email string, purpose domain.EmailCodePurpose, codeHash string) (domain.EmailVerificationCode, error) {
	var verified domain.EmailVerificationCode

	err := r.withTx(ctx, func(tx *gorm.DB) error {
		code, found, err := findLatestEmailCodeForUpdate(tx, email, purpose)
		if err != nil {
			return err
		}
		if !found {
			return ErrEmailCodeInvalid
		}

		if code.Status == domain.EmailCodeStatusBlocked {
			return ErrEmailCodeBlocked
		}
		if code.Status != domain.EmailCodeStatusPending {
			return ErrEmailCodeInvalid
		}

		now := time.Now().UTC()
		if !code.ExpiresAt.After(now) {
			return ErrEmailCodeInvalid
		}

		if code.CodeHash != codeHash {
			nextAttempts := code.AttemptCount + 1
			updates := map[string]any{
				"attempt_count": nextAttempts,
			}
			if nextAttempts >= maxEmailCodeAttempts {
				updates["status"] = domain.EmailCodeStatusBlocked
			}

			if err := tx.Model(&domain.EmailVerificationCode{}).
				Where("id = ?", code.ID).
				Updates(updates).Error; err != nil {
				return fmt.Errorf("update failed email code attempt: %w", err)
			}
			if nextAttempts >= maxEmailCodeAttempts {
				if err := invalidateOtherPendingEmailCodes(tx, code.ID, email, purpose, domain.EmailCodeStatusBlocked, nil); err != nil {
					return err
				}
				return ErrEmailCodeBlocked
			}
			return ErrEmailCodeInvalid
		}

		code.Status = domain.EmailCodeStatusConsumed
		code.ConsumedAt = &now
		if err := tx.Model(&domain.EmailVerificationCode{}).
			Where("id = ?", code.ID).
			Updates(map[string]any{
				"status":      code.Status,
				"consumed_at": code.ConsumedAt,
			}).Error; err != nil {
			return fmt.Errorf("consume email code: %w", err)
		}
		if err := invalidateOtherPendingEmailCodes(tx, code.ID, email, purpose, domain.EmailCodeStatusConsumed, code.ConsumedAt); err != nil {
			return err
		}

		verified = code
		return nil
	})
	if err != nil {
		return domain.EmailVerificationCode{}, err
	}

	return verified, nil
}

func (r *Repositories) CompleteEmailLogin(ctx context.Context, input CompleteEmailLoginInput) (CompleteEmailLoginResult, error) {
	var result CompleteEmailLoginResult

	err := r.withTx(ctx, func(tx *gorm.DB) error {
		code, found, err := findLatestEmailCodeForUpdate(tx, input.Email, input.Purpose)
		if err != nil {
			return err
		}
		if !found {
			return ErrEmailCodeInvalid
		}

		if code.Status == domain.EmailCodeStatusBlocked {
			return ErrEmailCodeBlocked
		}
		if code.Status != domain.EmailCodeStatusPending {
			return ErrEmailCodeInvalid
		}

		now := time.Now().UTC()
		if !code.ExpiresAt.After(now) {
			return ErrEmailCodeInvalid
		}

		if !security.VerifySecret(input.Code, input.Pepper, code.CodeHash) {
			nextAttempts := code.AttemptCount + 1
			updates := map[string]any{
				"attempt_count": nextAttempts,
			}
			if nextAttempts >= maxEmailCodeAttempts {
				updates["status"] = domain.EmailCodeStatusBlocked
			}

			if err := tx.Model(&domain.EmailVerificationCode{}).
				Where("id = ?", code.ID).
				Updates(updates).Error; err != nil {
				return fmt.Errorf("update failed email code attempt: %w", err)
			}
			if nextAttempts >= maxEmailCodeAttempts {
				if err := invalidateOtherPendingEmailCodes(tx, code.ID, input.Email, input.Purpose, domain.EmailCodeStatusBlocked, nil); err != nil {
					return err
				}
				return ErrEmailCodeBlocked
			}
			return ErrEmailCodeInvalid
		}

		created, err := findOrCreateEmailUserForUpdate(tx, input)
		if err != nil {
			return err
		}
		user := created.User
		if created.CreatedWorkspace {
			if err := grantTrialCreditInTx(tx, created.Workspace.ID, now, input.TrialCredit); err != nil {
				return err
			}
		}

		verifiedAt := input.EmailVerifiedAt.UTC()
		if verifiedAt.IsZero() {
			verifiedAt = now
		}
		user.EmailVerifiedAt = &verifiedAt
		user.UpdatedAt = now
		if err := tx.Model(&domain.User{}).
			Where("id = ?", user.ID).
			Updates(map[string]any{
				"email_verified_at": verifiedAt,
				"updated_at":        now,
			}).Error; err != nil {
			return fmt.Errorf("mark user email verified: %w", err)
		}

		sessionID, err := newID("sess_")
		if err != nil {
			return err
		}
		session := domain.UserSession{
			ID:        sessionID,
			UserID:    user.ID,
			TokenHash: input.SessionTokenHash,
			Status:    domain.SessionStatusActive,
			IP:        input.SessionIP,
			UserAgent: input.SessionUserAgent,
			ExpiresAt: input.SessionExpiresAt.UTC(),
			CreatedAt: now,
		}
		if err := tx.Create(&session).Error; err != nil {
			return fmt.Errorf("create session: %w", err)
		}

		code.Status = domain.EmailCodeStatusConsumed
		code.ConsumedAt = &now
		if err := tx.Model(&domain.EmailVerificationCode{}).
			Where("id = ?", code.ID).
			Updates(map[string]any{
				"status":      code.Status,
				"consumed_at": code.ConsumedAt,
			}).Error; err != nil {
			return fmt.Errorf("consume email code: %w", err)
		}
		if err := invalidateOtherPendingEmailCodes(tx, code.ID, input.Email, input.Purpose, domain.EmailCodeStatusConsumed, code.ConsumedAt); err != nil {
			return err
		}

		result = CompleteEmailLoginResult{
			User:    user,
			Session: session,
		}
		return nil
	})
	if err != nil {
		return CompleteEmailLoginResult{}, err
	}

	return result, nil
}

func (r *Repositories) FindUserByPrimaryEmail(ctx context.Context, email string) (domain.User, error) {
	var user domain.User
	if err := r.db.WithContext(ctx).
		Where("primary_email = ?", email).
		First(&user).Error; err != nil {
		return domain.User{}, fmt.Errorf("find user by primary email: %w", err)
	}

	return user, nil
}

func (r *Repositories) MarkUserEmailVerified(ctx context.Context, userID string) error {
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).
		Model(&domain.User{}).
		Where("id = ?", userID).
		Updates(map[string]any{
			"email_verified_at": now,
			"updated_at":        now,
		})
	if result.Error != nil {
		return fmt.Errorf("mark user email verified: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrUserNotFound
	}

	return nil
}

func (r *Repositories) CreateSession(ctx context.Context, input CreateSessionInput) (domain.UserSession, error) {
	id, err := newID("sess_")
	if err != nil {
		return domain.UserSession{}, err
	}

	session := domain.UserSession{
		ID:        id,
		UserID:    input.UserID,
		TokenHash: input.TokenHash,
		Status:    domain.SessionStatusActive,
		IP:        input.IP,
		UserAgent: input.UserAgent,
		ExpiresAt: input.ExpiresAt.UTC(),
		CreatedAt: time.Now().UTC(),
	}
	if err := r.db.WithContext(ctx).Create(&session).Error; err != nil {
		return domain.UserSession{}, fmt.Errorf("create session: %w", err)
	}

	return session, nil
}

func (r *Repositories) FindActiveSessionByTokenHash(ctx context.Context, tokenHash string) (domain.UserSession, error) {
	var session domain.UserSession
	err := r.db.WithContext(ctx).
		Where("token_hash = ? AND status = ? AND revoked_at IS NULL AND expires_at > ?",
			tokenHash,
			domain.SessionStatusActive,
			time.Now().UTC(),
		).
		First(&session).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.UserSession{}, ErrSessionNotFound
		}
		return domain.UserSession{}, fmt.Errorf("find active session by token hash: %w", err)
	}

	return session, nil
}

func (r *Repositories) RevokeSession(ctx context.Context, sessionID string) error {
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).
		Model(&domain.UserSession{}).
		Where("id = ? AND status = ? AND revoked_at IS NULL", sessionID, domain.SessionStatusActive).
		Updates(map[string]any{
			"status":     domain.SessionStatusRevoked,
			"revoked_at": now,
		})
	if result.Error != nil {
		return fmt.Errorf("revoke session: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrSessionNotFound
	}

	return nil
}

func findOrCreateEmailUserForUpdate(tx *gorm.DB, input CompleteEmailLoginInput) (emailUserWorkspaceResult, error) {
	var user domain.User
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("primary_email = ?", input.Email).
		First(&user).Error
	if err == nil {
		return emailUserWorkspaceResult{User: user}, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return emailUserWorkspaceResult{}, fmt.Errorf("find user by primary email: %w", err)
	}

	now := time.Now().UTC()
	userID, err := newID("usr_")
	if err != nil {
		return emailUserWorkspaceResult{}, err
	}
	workspaceID, err := newID("wsp_")
	if err != nil {
		return emailUserWorkspaceResult{}, err
	}

	user = domain.User{
		ID:           userID,
		DisplayName:  input.DisplayName,
		PrimaryEmail: &input.Email,
		Status:       domain.UserStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := tx.Create(&user).Error; err != nil {
		return emailUserWorkspaceResult{}, fmt.Errorf("create user: %w", err)
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
		return emailUserWorkspaceResult{}, fmt.Errorf("create workspace: %w", err)
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
		return emailUserWorkspaceResult{}, fmt.Errorf("create owner membership: %w", err)
	}

	balance := domain.WorkspaceBalance{
		WorkspaceID:       workspaceID,
		AvailableMicroCNY: 0,
		FrozenMicroCNY:    0,
		Version:           1,
		UpdatedAt:         now,
	}
	if err := tx.Create(&balance).Error; err != nil {
		return emailUserWorkspaceResult{}, fmt.Errorf("create workspace balance: %w", err)
	}

	return emailUserWorkspaceResult{
		User:             user,
		Workspace:        workspace,
		CreatedWorkspace: true,
	}, nil
}

func findLatestEmailCodeForUpdate(tx *gorm.DB, email string, purpose domain.EmailCodePurpose) (domain.EmailVerificationCode, bool, error) {
	var code domain.EmailVerificationCode
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("email = ? AND purpose = ?", email, purpose).
		Order("created_at DESC").
		Order("id DESC").
		First(&code).Error
	if err == nil {
		return code, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.EmailVerificationCode{}, false, nil
	}
	return domain.EmailVerificationCode{}, false, fmt.Errorf("find latest email code for update: %w", err)
}

func invalidateOtherPendingEmailCodes(tx *gorm.DB, currentID string, email string, purpose domain.EmailCodePurpose, status domain.EmailCodeStatus, consumedAt *time.Time) error {
	updates := map[string]any{
		"status": status,
	}
	if consumedAt != nil {
		updates["consumed_at"] = consumedAt
	}

	if err := tx.Model(&domain.EmailVerificationCode{}).
		Where("id <> ? AND email = ? AND purpose = ? AND status = ?",
			currentID,
			email,
			purpose,
			domain.EmailCodeStatusPending,
		).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("invalidate older pending email codes: %w", err)
	}

	return nil
}
