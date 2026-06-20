package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/tokenlive/tokenlive-portal/backend/internal/config"
	"github.com/tokenlive/tokenlive-portal/backend/internal/domain"
	"github.com/tokenlive/tokenlive-portal/backend/internal/repository"
	"github.com/tokenlive/tokenlive-portal/backend/internal/security"
	"gorm.io/gorm"
)

const emailCodeTTL = 10 * time.Minute

type authServiceStore interface {
	CreateEmailVerificationCode(ctx context.Context, input repository.CreateEmailVerificationCodeInput) (domain.EmailVerificationCode, error)
	CompleteEmailLogin(ctx context.Context, input repository.CompleteEmailLoginInput) (repository.CompleteEmailLoginResult, error)
	FindActiveSessionByTokenHash(ctx context.Context, tokenHash string) (domain.UserSession, error)
	FindUserByID(ctx context.Context, userID string) (domain.User, error)
	RevokeSession(ctx context.Context, sessionID string) error
}

type authServiceRepositoryStore struct {
	repos *repository.Repositories
}

func (s authServiceRepositoryStore) CreateEmailVerificationCode(ctx context.Context, input repository.CreateEmailVerificationCodeInput) (domain.EmailVerificationCode, error) {
	return s.repos.CreateEmailVerificationCode(ctx, input)
}

func (s authServiceRepositoryStore) CompleteEmailLogin(ctx context.Context, input repository.CompleteEmailLoginInput) (repository.CompleteEmailLoginResult, error) {
	return s.repos.CompleteEmailLogin(ctx, input)
}

func (s authServiceRepositoryStore) FindActiveSessionByTokenHash(ctx context.Context, tokenHash string) (domain.UserSession, error) {
	return s.repos.FindActiveSessionByTokenHash(ctx, tokenHash)
}

func (s authServiceRepositoryStore) FindUserByID(ctx context.Context, userID string) (domain.User, error) {
	var user domain.User
	if err := s.repos.DB().WithContext(ctx).Where("id = ?", userID).First(&user).Error; err != nil {
		return domain.User{}, fmt.Errorf("find user by id: %w", err)
	}
	return user, nil
}

func (s authServiceRepositoryStore) RevokeSession(ctx context.Context, sessionID string) error {
	return s.repos.RevokeSession(ctx, sessionID)
}

type authService struct {
	store                authServiceStore
	env                  string
	authPepper           string
	trialCredit          config.TrialCreditConfig
	nowFunc              func() time.Time
	generateEmailCode    func() (string, error)
	generateSessionToken func() (string, error)
	generateSlugSuffix   func() (string, error)
}

func NewAuthService(repos *repository.Repositories, env string, authPepper string, trialCredit config.TrialCreditConfig) (AuthService, error) {
	if repos == nil {
		return nil, errors.New("auth repositories are required")
	}
	return newAuthService(authServiceRepositoryStore{repos: repos}, env, authPepper, trialCredit)
}

func newAuthService(store authServiceStore, env string, authPepper string, trialCredit config.TrialCreditConfig) (*authService, error) {
	if store == nil {
		return nil, errors.New("auth store is required")
	}
	if strings.TrimSpace(authPepper) == "" {
		return nil, errors.New("auth pepper must not be empty")
	}
	if trialCredit.AmountMicroCNY < 0 {
		return nil, errors.New("trial credit amount must be greater than or equal to zero")
	}
	if trialCredit.TTLDays <= 0 {
		return nil, errors.New("trial credit ttl days must be greater than zero")
	}

	return &authService{
		store:                store,
		env:                  normalizeEnv(env),
		authPepper:           authPepper,
		trialCredit:          trialCredit,
		nowFunc:              func() time.Time { return time.Now().UTC() },
		generateEmailCode:    security.GenerateEmailCode,
		generateSessionToken: security.GenerateSessionToken,
		generateSlugSuffix:   defaultWorkspaceSlugSuffix,
	}, nil
}

func (s *authService) StartEmailLogin(ctx context.Context, email string) (StartEmailLoginResult, error) {
	normalizedEmail, err := normalizeEmail(email)
	if err != nil {
		return StartEmailLoginResult{}, err
	}

	code, err := s.generateEmailCode()
	if err != nil {
		return StartEmailLoginResult{}, fmt.Errorf("generate email code: %w", err)
	}

	now := s.nowFunc().UTC()
	if _, err := s.store.CreateEmailVerificationCode(ctx, repository.CreateEmailVerificationCodeInput{
		Email:     normalizedEmail,
		Purpose:   domain.EmailCodePurposeLogin,
		CodeHash:  security.HashSecret(code, s.authPepper),
		ExpiresAt: now.Add(emailCodeTTL),
	}); err != nil {
		return StartEmailLoginResult{}, err
	}

	result := StartEmailLoginResult{Sent: true}
	if shouldReturnDevCode(s.env) {
		result.DevCode = code
	}
	return result, nil
}

func (s *authService) VerifyEmailLogin(ctx context.Context, input VerifyEmailLoginInput) (VerifyEmailLoginResult, error) {
	normalizedEmail, err := normalizeEmail(input.Email)
	if err != nil {
		return VerifyEmailLoginResult{}, err
	}

	code := strings.TrimSpace(input.Code)
	if code == "" {
		return VerifyEmailLoginResult{}, ErrAuthInvalidCode
	}

	sessionToken, err := s.generateSessionToken()
	if err != nil {
		return VerifyEmailLoginResult{}, fmt.Errorf("generate session token: %w", err)
	}

	now := s.nowFunc().UTC()
	slugSuffix, err := s.generateSlugSuffix()
	if err != nil {
		return VerifyEmailLoginResult{}, fmt.Errorf("generate workspace slug suffix: %w", err)
	}

	completed, err := s.store.CompleteEmailLogin(ctx, repository.CompleteEmailLoginInput{
		Email:            normalizedEmail,
		Purpose:          domain.EmailCodePurposeLogin,
		Code:             code,
		Pepper:           s.authPepper,
		DisplayName:      "",
		WorkspaceName:    workspaceNameFromEmail(normalizedEmail),
		WorkspaceSlug:    "personal-" + slugSuffix,
		SessionTokenHash: security.HashSecret(sessionToken, s.authPepper),
		SessionIP:        "",
		SessionUserAgent: "",
		SessionExpiresAt: now.Add(sessionCookieTTL),
		EmailVerifiedAt:  now,
		TrialCredit: repository.TrialCreditInput{
			AmountMicroCNY: s.trialCredit.AmountMicroCNY,
			TTLDays:        s.trialCredit.TTLDays,
		},
	})
	if err != nil {
		if errors.Is(err, repository.ErrEmailCodeInvalid) || errors.Is(err, repository.ErrEmailCodeBlocked) {
			return VerifyEmailLoginResult{}, ErrAuthInvalidCode
		}
		return VerifyEmailLoginResult{}, err
	}

	return VerifyEmailLoginResult{
		SessionToken: sessionToken,
		User:         currentUserFromDomain(completed.User),
	}, nil
}

func (s *authService) CurrentUser(ctx context.Context, sessionToken string) (CurrentUser, error) {
	session, err := s.resolveActiveSession(ctx, sessionToken)
	if err != nil {
		return CurrentUser{}, err
	}

	user, err := s.store.FindUserByID(ctx, session.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, repository.ErrUserNotFound) {
			return CurrentUser{}, ErrAuthUnauthorized
		}
		return CurrentUser{}, err
	}

	return currentUserFromDomain(user), nil
}

func (s *authService) Logout(ctx context.Context, sessionToken string) error {
	session, err := s.resolveActiveSession(ctx, sessionToken)
	if err != nil {
		return err
	}

	if err := s.store.RevokeSession(ctx, session.ID); err != nil {
		if errors.Is(err, repository.ErrSessionNotFound) {
			return ErrAuthSessionExpired
		}
		return err
	}
	return nil
}

func (s *authService) resolveActiveSession(ctx context.Context, sessionToken string) (domain.UserSession, error) {
	token := strings.TrimSpace(sessionToken)
	if token == "" {
		return domain.UserSession{}, ErrAuthSessionRequired
	}

	session, err := s.store.FindActiveSessionByTokenHash(ctx, security.HashSecret(token, s.authPepper))
	if err != nil {
		if errors.Is(err, repository.ErrSessionNotFound) {
			return domain.UserSession{}, ErrAuthSessionExpired
		}
		return domain.UserSession{}, err
	}
	return session, nil
}

func normalizeEmail(email string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(email))
	if normalized == "" {
		return "", ErrAuthInvalidEmail
	}

	parsed, err := mail.ParseAddress(normalized)
	if err != nil || parsed.Address != normalized {
		return "", ErrAuthInvalidEmail
	}

	return normalized, nil
}

func shouldReturnDevCode(env string) bool {
	switch normalizeEnv(env) {
	case "development", "test":
		return true
	default:
		return false
	}
}

func normalizeEnv(env string) string {
	return strings.ToLower(strings.TrimSpace(env))
}

func workspaceNameFromEmail(email string) string {
	localPart, _, found := strings.Cut(email, "@")
	if !found || localPart == "" {
		return "personal"
	}
	return localPart
}

func defaultWorkspaceSlugSuffix() (string, error) {
	var randomBytes [4]byte
	if _, err := rand.Read(randomBytes[:]); err != nil {
		return "", fmt.Errorf("read workspace slug random bytes: %w", err)
	}
	return fmt.Sprintf("%d%s", time.Now().UTC().Unix(), hex.EncodeToString(randomBytes[:])), nil
}

func currentUserFromDomain(user domain.User) CurrentUser {
	return CurrentUser{
		ID:            user.ID,
		DisplayName:   user.DisplayName,
		PrimaryEmail:  derefString(user.PrimaryEmail),
		EmailVerified: user.EmailVerifiedAt != nil,
	}
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
