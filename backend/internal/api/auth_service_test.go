package api

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tokenlive/tokenlive-portal/backend/internal/config"
	"github.com/tokenlive/tokenlive-portal/backend/internal/domain"
	"github.com/tokenlive/tokenlive-portal/backend/internal/repository"
	"github.com/tokenlive/tokenlive-portal/backend/internal/security"
)

func TestNewAuthServiceRejectsEmptyPepper(t *testing.T) {
	t.Parallel()

	_, err := newAuthService(&fakeAuthServiceStore{}, "development", "", validTrialCreditConfig())
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "auth pepper") {
		t.Fatalf("err = %v, want auth pepper message", err)
	}
}

func TestNewAuthServiceRejectsNegativeTrialAmount(t *testing.T) {
	t.Parallel()

	_, err := newAuthService(&fakeAuthServiceStore{}, "development", "pepper", config.TrialCreditConfig{
		AmountMicroCNY: -1,
		TTLDays:        7,
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "trial credit amount") {
		t.Fatalf("err = %v, want trial credit amount message", err)
	}
}

func TestNewAuthServiceRejectsInvalidTrialTTL(t *testing.T) {
	t.Parallel()

	_, err := newAuthService(&fakeAuthServiceStore{}, "development", "pepper", config.TrialCreditConfig{
		AmountMicroCNY: 10_000_000,
		TTLDays:        0,
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "trial credit ttl") {
		t.Fatalf("err = %v, want trial credit ttl message", err)
	}
}

func TestStartEmailLoginNormalizesEmailAndReturnsDevCode(t *testing.T) {
	t.Parallel()

	store := &fakeAuthServiceStore{}
	service, err := newAuthService(store, "development", "pepper", validTrialCreditConfig())
	if err != nil {
		t.Fatalf("new auth service: %v", err)
	}
	service.nowFunc = func() time.Time {
		return time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	}
	service.generateEmailCode = func() (string, error) {
		return "123456", nil
	}

	result, err := service.StartEmailLogin(context.Background(), " Dev@Example.com ")
	if err != nil {
		t.Fatalf("start email login: %v", err)
	}

	if !result.Sent {
		t.Fatalf("sent = false, want true")
	}
	if result.DevCode != "123456" {
		t.Fatalf("dev code = %q, want %q", result.DevCode, "123456")
	}
	if store.createdCodeInput.Email != "dev@example.com" {
		t.Fatalf("stored email = %q, want %q", store.createdCodeInput.Email, "dev@example.com")
	}
	if store.createdCodeInput.Purpose != domain.EmailCodePurposeLogin {
		t.Fatalf("purpose = %q, want %q", store.createdCodeInput.Purpose, domain.EmailCodePurposeLogin)
	}
	if store.createdCodeInput.CodeHash != security.HashSecret("123456", "pepper") {
		t.Fatalf("code hash = %q, want hash of dev code", store.createdCodeInput.CodeHash)
	}
	wantExpiry := service.nowFunc().Add(emailCodeTTL)
	if !store.createdCodeInput.ExpiresAt.Equal(wantExpiry) {
		t.Fatalf("expires_at = %v, want %v", store.createdCodeInput.ExpiresAt, wantExpiry)
	}
}

func TestStartEmailLoginOmitsDevCodeOutsideDevelopmentAndTest(t *testing.T) {
	t.Parallel()

	store := &fakeAuthServiceStore{}
	service, err := newAuthService(store, "production", "pepper", validTrialCreditConfig())
	if err != nil {
		t.Fatalf("new auth service: %v", err)
	}
	service.generateEmailCode = func() (string, error) {
		return "654321", nil
	}

	result, err := service.StartEmailLogin(context.Background(), "prod@example.com")
	if err != nil {
		t.Fatalf("start email login: %v", err)
	}

	if !result.Sent {
		t.Fatalf("sent = false, want true")
	}
	if result.DevCode != "" {
		t.Fatalf("dev code = %q, want empty", result.DevCode)
	}
}

func TestVerifyEmailLoginCreatesUserMarksVerifiedAndCreatesSession(t *testing.T) {
	t.Parallel()

	store := &fakeAuthServiceStore{
		completeEmailLoginResult: repository.CompleteEmailLoginResult{
			User: domain.User{
				ID:              "usr_new",
				DisplayName:     "",
				PrimaryEmail:    stringPtr("dev@example.com"),
				EmailVerifiedAt: timePtr(time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)),
			},
			Session: domain.UserSession{ID: "sess_123"},
		},
	}
	service, err := newAuthService(store, "test", "pepper", config.TrialCreditConfig{
		AmountMicroCNY: 10_000_000,
		TTLDays:        7,
	})
	if err != nil {
		t.Fatalf("new auth service: %v", err)
	}
	service.nowFunc = func() time.Time {
		return time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	}
	service.generateSessionToken = func() (string, error) {
		return "tl_sess_created", nil
	}
	service.generateSlugSuffix = func() (string, error) {
		return "abc123", nil
	}

	result, err := service.VerifyEmailLogin(context.Background(), VerifyEmailLoginInput{
		Email: " Dev@Example.com ",
		Code:  "123456",
	})
	if err != nil {
		t.Fatalf("verify email login: %v", err)
	}

	if store.completeEmailLoginInput.Email != "dev@example.com" {
		t.Fatalf("login email = %q, want %q", store.completeEmailLoginInput.Email, "dev@example.com")
	}
	if store.completeEmailLoginInput.Code != "123456" {
		t.Fatalf("login code = %q, want %q", store.completeEmailLoginInput.Code, "123456")
	}
	if store.completeEmailLoginInput.Pepper != "pepper" {
		t.Fatalf("login pepper = %q, want %q", store.completeEmailLoginInput.Pepper, "pepper")
	}
	if store.completeEmailLoginInput.WorkspaceName != "dev" {
		t.Fatalf("workspace name = %q, want %q", store.completeEmailLoginInput.WorkspaceName, "dev")
	}
	if store.completeEmailLoginInput.WorkspaceSlug != "personal-abc123" {
		t.Fatalf("workspace slug = %q, want %q", store.completeEmailLoginInput.WorkspaceSlug, "personal-abc123")
	}
	if store.completeEmailLoginInput.SessionTokenHash != security.HashSecret("tl_sess_created", "pepper") {
		t.Fatalf("session token hash = %q, want hashed token", store.completeEmailLoginInput.SessionTokenHash)
	}
	if store.completeEmailLoginInput.SessionIP != "" || store.completeEmailLoginInput.SessionUserAgent != "" {
		t.Fatalf("session input should default empty ip/user agent, got ip=%q ua=%q", store.completeEmailLoginInput.SessionIP, store.completeEmailLoginInput.SessionUserAgent)
	}
	wantExpiry := service.nowFunc().Add(sessionCookieTTL)
	if !store.completeEmailLoginInput.SessionExpiresAt.Equal(wantExpiry) {
		t.Fatalf("session expires_at = %v, want %v", store.completeEmailLoginInput.SessionExpiresAt, wantExpiry)
	}
	if !store.completeEmailLoginInput.EmailVerifiedAt.Equal(service.nowFunc()) {
		t.Fatalf("email_verified_at = %v, want %v", store.completeEmailLoginInput.EmailVerifiedAt, service.nowFunc())
	}
	if store.completeEmailLoginInput.TrialCredit.AmountMicroCNY != 10_000_000 {
		t.Fatalf("trial amount = %d, want 10000000", store.completeEmailLoginInput.TrialCredit.AmountMicroCNY)
	}
	if store.completeEmailLoginInput.TrialCredit.TTLDays != 7 {
		t.Fatalf("trial ttl = %d, want 7", store.completeEmailLoginInput.TrialCredit.TTLDays)
	}
	if result.SessionToken != "tl_sess_created" {
		t.Fatalf("session token = %q, want %q", result.SessionToken, "tl_sess_created")
	}
	if result.User.PrimaryEmail != "dev@example.com" {
		t.Fatalf("primary email = %q, want %q", result.User.PrimaryEmail, "dev@example.com")
	}
	if !result.User.EmailVerified {
		t.Fatalf("email verified = false, want true")
	}
}

func TestVerifyEmailLoginUsesExistingUser(t *testing.T) {
	t.Parallel()

	verifiedAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	store := &fakeAuthServiceStore{
		completeEmailLoginResult: repository.CompleteEmailLoginResult{
			User: domain.User{
				ID:              "usr_existing",
				DisplayName:     "Existing",
				PrimaryEmail:    stringPtr("existing@example.com"),
				EmailVerifiedAt: &verifiedAt,
			},
		},
	}
	service, err := newAuthService(store, "development", "pepper", validTrialCreditConfig())
	if err != nil {
		t.Fatalf("new auth service: %v", err)
	}
	service.generateSessionToken = func() (string, error) {
		return "tl_sess_existing", nil
	}

	result, err := service.VerifyEmailLogin(context.Background(), VerifyEmailLoginInput{
		Email: "existing@example.com",
		Code:  "123456",
	})
	if err != nil {
		t.Fatalf("verify email login: %v", err)
	}

	if store.completeEmailLoginInput.WorkspaceName != "existing" {
		t.Fatalf("workspace name = %q, want existing", store.completeEmailLoginInput.WorkspaceName)
	}
	if result.User.ID != "usr_existing" {
		t.Fatalf("user id = %q, want %q", result.User.ID, "usr_existing")
	}
}

func TestCurrentUserMapsMissingSessionToExpired(t *testing.T) {
	t.Parallel()

	store := &fakeAuthServiceStore{
		findActiveSessionErr: repository.ErrSessionNotFound,
	}
	service, err := newAuthService(store, "development", "pepper", validTrialCreditConfig())
	if err != nil {
		t.Fatalf("new auth service: %v", err)
	}

	_, gotErr := service.CurrentUser(context.Background(), "missing")
	if !errors.Is(gotErr, ErrAuthSessionExpired) {
		t.Fatalf("err = %v, want ErrAuthSessionExpired", gotErr)
	}
}

func TestCurrentUserLoadsUserFromSession(t *testing.T) {
	t.Parallel()

	verifiedAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	store := &fakeAuthServiceStore{
		findActiveSessionResult: domain.UserSession{
			ID:     "sess_123",
			UserID: "usr_123",
		},
		findUserByIDResult: domain.User{
			ID:              "usr_123",
			DisplayName:     "Portal User",
			PrimaryEmail:    stringPtr("portal@example.com"),
			EmailVerifiedAt: &verifiedAt,
		},
	}
	service, err := newAuthService(store, "development", "pepper", validTrialCreditConfig())
	if err != nil {
		t.Fatalf("new auth service: %v", err)
	}

	user, err := service.CurrentUser(context.Background(), "tl_sess_live")
	if err != nil {
		t.Fatalf("current user: %v", err)
	}

	if store.findActiveSessionTokenHash != security.HashSecret("tl_sess_live", "pepper") {
		t.Fatalf("token hash = %q, want hashed token", store.findActiveSessionTokenHash)
	}
	if user.ID != "usr_123" {
		t.Fatalf("user id = %q, want %q", user.ID, "usr_123")
	}
	if !user.EmailVerified {
		t.Fatalf("email verified = false, want true")
	}
}

func TestLogoutRevokesResolvedSession(t *testing.T) {
	t.Parallel()

	store := &fakeAuthServiceStore{
		findActiveSessionResult: domain.UserSession{
			ID:     "sess_123",
			UserID: "usr_123",
		},
	}
	service, err := newAuthService(store, "development", "pepper", validTrialCreditConfig())
	if err != nil {
		t.Fatalf("new auth service: %v", err)
	}

	if err := service.Logout(context.Background(), "tl_sess_live"); err != nil {
		t.Fatalf("logout: %v", err)
	}

	if store.revokedSessionID != "sess_123" {
		t.Fatalf("revoked session id = %q, want %q", store.revokedSessionID, "sess_123")
	}
}

type fakeAuthServiceStore struct {
	createdCodeInput repository.CreateEmailVerificationCodeInput
	createdCodeErr   error

	completeEmailLoginInput  repository.CompleteEmailLoginInput
	completeEmailLoginResult repository.CompleteEmailLoginResult
	completeEmailLoginErr    error

	findActiveSessionTokenHash string
	findActiveSessionResult    domain.UserSession
	findActiveSessionErr       error
	findUserByIDUserID         string
	findUserByIDResult         domain.User
	findUserByIDErr            error
	revokedSessionID           string
	revokeSessionErr           error
}

func (f *fakeAuthServiceStore) CreateEmailVerificationCode(_ context.Context, input repository.CreateEmailVerificationCodeInput) (domain.EmailVerificationCode, error) {
	f.createdCodeInput = input
	if f.createdCodeErr != nil {
		return domain.EmailVerificationCode{}, f.createdCodeErr
	}
	return domain.EmailVerificationCode{}, nil
}

func (f *fakeAuthServiceStore) CompleteEmailLogin(_ context.Context, input repository.CompleteEmailLoginInput) (repository.CompleteEmailLoginResult, error) {
	f.completeEmailLoginInput = input
	if f.completeEmailLoginErr != nil {
		return repository.CompleteEmailLoginResult{}, f.completeEmailLoginErr
	}
	return f.completeEmailLoginResult, nil
}

func (f *fakeAuthServiceStore) FindActiveSessionByTokenHash(_ context.Context, tokenHash string) (domain.UserSession, error) {
	f.findActiveSessionTokenHash = tokenHash
	if f.findActiveSessionErr != nil {
		return domain.UserSession{}, f.findActiveSessionErr
	}
	return f.findActiveSessionResult, nil
}

func (f *fakeAuthServiceStore) FindUserByID(_ context.Context, userID string) (domain.User, error) {
	f.findUserByIDUserID = userID
	if f.findUserByIDErr != nil {
		return domain.User{}, f.findUserByIDErr
	}
	return f.findUserByIDResult, nil
}

func (f *fakeAuthServiceStore) RevokeSession(_ context.Context, sessionID string) error {
	f.revokedSessionID = sessionID
	return f.revokeSessionErr
}

func stringPtr(value string) *string {
	return &value
}

func timePtr(value time.Time) *time.Time {
	return &value
}

func validTrialCreditConfig() config.TrialCreditConfig {
	return config.TrialCreditConfig{
		AmountMicroCNY: 10_000_000,
		TTLDays:        7,
	}
}
