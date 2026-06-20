package repository

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tokenlive/tokenlive-portal/backend/internal/domain"
	"github.com/tokenlive/tokenlive-portal/backend/internal/security"
)

func TestVerifyEmailCodeConsumesLatestPendingCodeAndRejectsReuse(t *testing.T) {
	db := testDB(t)
	repos := New(db)
	ctx := context.Background()
	suffix := uniqueSuffix(t)
	email := "auth-code-" + suffix + "@example.com"
	now := time.Now().UTC()

	first, err := repos.CreateEmailVerificationCode(ctx, CreateEmailVerificationCodeInput{
		Email:     email,
		Purpose:   domain.EmailCodePurposeLogin,
		CodeHash:  "hash-first-" + suffix,
		ExpiresAt: now.Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatalf("create first email code: %v", err)
	}

	second, err := repos.CreateEmailVerificationCode(ctx, CreateEmailVerificationCodeInput{
		Email:     email,
		Purpose:   domain.EmailCodePurposeLogin,
		CodeHash:  "hash-second-" + suffix,
		ExpiresAt: now.Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatalf("create second email code: %v", err)
	}

	verified, err := repos.VerifyEmailCode(ctx, email, domain.EmailCodePurposeLogin, second.CodeHash)
	if err != nil {
		t.Fatalf("verify latest pending code: %v", err)
	}

	if verified.ID != second.ID {
		t.Fatalf("verified id = %s, want %s", verified.ID, second.ID)
	}
	if verified.Status != domain.EmailCodeStatusConsumed {
		t.Fatalf("verified status = %s, want %s", verified.Status, domain.EmailCodeStatusConsumed)
	}
	if verified.ConsumedAt == nil {
		t.Fatalf("expected consumed_at to be set")
	}

	var firstRow domain.EmailVerificationCode
	if err := db.Where("id = ?", first.ID).First(&firstRow).Error; err != nil {
		t.Fatalf("find first code: %v", err)
	}
	if firstRow.Status == domain.EmailCodeStatusPending {
		t.Fatalf("older code must not remain pending after newer code is consumed")
	}

	_, err = repos.VerifyEmailCode(ctx, email, domain.EmailCodePurposeLogin, second.CodeHash)
	if !errors.Is(err, ErrEmailCodeInvalid) {
		t.Fatalf("reuse verify err = %v, want ErrEmailCodeInvalid", err)
	}

	_, err = repos.VerifyEmailCode(ctx, email, domain.EmailCodePurposeLogin, first.CodeHash)
	if !errors.Is(err, ErrEmailCodeInvalid) {
		t.Fatalf("older code err = %v, want ErrEmailCodeInvalid", err)
	}
}

func TestVerifyEmailCodeBlocksAfterFiveWrongAttempts(t *testing.T) {
	db := testDB(t)
	repos := New(db)
	ctx := context.Background()
	suffix := uniqueSuffix(t)
	email := "auth-block-" + suffix + "@example.com"
	now := time.Now().UTC()

	older, err := repos.CreateEmailVerificationCode(ctx, CreateEmailVerificationCodeInput{
		Email:     email,
		Purpose:   domain.EmailCodePurposeLogin,
		CodeHash:  "hash-older-" + suffix,
		ExpiresAt: now.Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatalf("create older email code: %v", err)
	}

	created, err := repos.CreateEmailVerificationCode(ctx, CreateEmailVerificationCodeInput{
		Email:     email,
		Purpose:   domain.EmailCodePurposeLogin,
		CodeHash:  "hash-correct-" + suffix,
		ExpiresAt: now.Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatalf("create email code: %v", err)
	}

	for attempt := 1; attempt <= 4; attempt++ {
		_, err := repos.VerifyEmailCode(ctx, email, domain.EmailCodePurposeLogin, "hash-wrong-"+suffix)
		if !errors.Is(err, ErrEmailCodeInvalid) {
			t.Fatalf("attempt %d err = %v, want ErrEmailCodeInvalid", attempt, err)
		}
	}

	_, err = repos.VerifyEmailCode(ctx, email, domain.EmailCodePurposeLogin, "hash-wrong-"+suffix)
	if !errors.Is(err, ErrEmailCodeBlocked) {
		t.Fatalf("attempt 5 err = %v, want ErrEmailCodeBlocked", err)
	}

	var blocked domain.EmailVerificationCode
	if err := db.Where("id = ?", created.ID).First(&blocked).Error; err != nil {
		t.Fatalf("find blocked code: %v", err)
	}
	if blocked.AttemptCount != 5 {
		t.Fatalf("attempt_count = %d, want 5", blocked.AttemptCount)
	}
	if blocked.Status != domain.EmailCodeStatusBlocked {
		t.Fatalf("status = %s, want %s", blocked.Status, domain.EmailCodeStatusBlocked)
	}

	var olderRow domain.EmailVerificationCode
	if err := db.Where("id = ?", older.ID).First(&olderRow).Error; err != nil {
		t.Fatalf("find older code: %v", err)
	}
	if olderRow.Status == domain.EmailCodeStatusPending {
		t.Fatalf("older code must not remain pending after latest code is blocked")
	}

	_, err = repos.VerifyEmailCode(ctx, email, domain.EmailCodePurposeLogin, created.CodeHash)
	if !errors.Is(err, ErrEmailCodeBlocked) {
		t.Fatalf("verify blocked code err = %v, want ErrEmailCodeBlocked", err)
	}

	_, err = repos.VerifyEmailCode(ctx, email, domain.EmailCodePurposeLogin, older.CodeHash)
	if !errors.Is(err, ErrEmailCodeBlocked) {
		t.Fatalf("older code after block err = %v, want ErrEmailCodeBlocked", err)
	}
}

func TestFindActiveSessionByTokenHashRejectsExpiredSessions(t *testing.T) {
	db := testDB(t)
	repos := New(db)
	ctx := context.Background()
	suffix := uniqueSuffix(t)

	email := "auth-session-" + suffix + "@example.com"
	userResult, err := repos.CreateUserWithDefaultWorkspace(ctx, CreateUserWithWorkspaceInput{
		DisplayName:   "Session User",
		PrimaryEmail:  &email,
		WorkspaceName: "Session Workspace",
		WorkspaceSlug: "session-workspace-" + suffix,
	})
	if err != nil {
		t.Fatalf("create user with workspace: %v", err)
	}

	activeSession, err := repos.CreateSession(ctx, CreateSessionInput{
		UserID:    userResult.User.ID,
		TokenHash: "token-hash-active-" + suffix,
		IP:        "127.0.0.1",
		UserAgent: "repository-test",
		ExpiresAt: time.Now().UTC().Add(30 * time.Minute),
	})
	if err != nil {
		t.Fatalf("create active session: %v", err)
	}

	found, err := repos.FindActiveSessionByTokenHash(ctx, activeSession.TokenHash)
	if err != nil {
		t.Fatalf("find active session: %v", err)
	}
	if found.ID != activeSession.ID {
		t.Fatalf("found session id = %s, want %s", found.ID, activeSession.ID)
	}

	expiredSession, err := repos.CreateSession(ctx, CreateSessionInput{
		UserID:    userResult.User.ID,
		TokenHash: "token-hash-expired-" + suffix,
		IP:        "127.0.0.1",
		UserAgent: "repository-test",
		ExpiresAt: time.Now().UTC().Add(-1 * time.Minute),
	})
	if err != nil {
		t.Fatalf("create expired session: %v", err)
	}

	_, err = repos.FindActiveSessionByTokenHash(ctx, expiredSession.TokenHash)
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("find expired session err = %v, want ErrSessionNotFound", err)
	}
}

func TestFindActiveSessionByTokenHashRejectsRevokedSessions(t *testing.T) {
	db := testDB(t)
	repos := New(db)
	ctx := context.Background()
	suffix := uniqueSuffix(t)

	email := "auth-session-revoked-" + suffix + "@example.com"
	userResult, err := repos.CreateUserWithDefaultWorkspace(ctx, CreateUserWithWorkspaceInput{
		DisplayName:   "Revoked Session User",
		PrimaryEmail:  &email,
		WorkspaceName: "Revoked Session Workspace",
		WorkspaceSlug: "revoked-session-workspace-" + suffix,
	})
	if err != nil {
		t.Fatalf("create user with workspace: %v", err)
	}

	session, err := repos.CreateSession(ctx, CreateSessionInput{
		UserID:    userResult.User.ID,
		TokenHash: "token-hash-revoked-" + suffix,
		IP:        "127.0.0.1",
		UserAgent: "repository-test",
		ExpiresAt: time.Now().UTC().Add(30 * time.Minute),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := repos.RevokeSession(ctx, session.ID); err != nil {
		t.Fatalf("revoke session: %v", err)
	}

	_, err = repos.FindActiveSessionByTokenHash(ctx, session.TokenHash)
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("find revoked session err = %v, want ErrSessionNotFound", err)
	}
}

func TestRevokeSessionReturnsNotFoundWhenSessionMissing(t *testing.T) {
	db := testDB(t)
	repos := New(db)

	err := repos.RevokeSession(context.Background(), "sess_missing")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("got err %v, want ErrSessionNotFound", err)
	}
}

func TestRevokeSessionReturnsNotFoundWhenAlreadyRevoked(t *testing.T) {
	db := testDB(t)
	repos := New(db)
	ctx := context.Background()
	suffix := uniqueSuffix(t)

	email := "auth-session-revoke-twice-" + suffix + "@example.com"
	userResult, err := repos.CreateUserWithDefaultWorkspace(ctx, CreateUserWithWorkspaceInput{
		DisplayName:   "Revoke Twice User",
		PrimaryEmail:  &email,
		WorkspaceName: "Revoke Twice Workspace",
		WorkspaceSlug: "revoke-twice-workspace-" + suffix,
	})
	if err != nil {
		t.Fatalf("create user with workspace: %v", err)
	}

	session, err := repos.CreateSession(ctx, CreateSessionInput{
		UserID:    userResult.User.ID,
		TokenHash: "token-hash-revoke-twice-" + suffix,
		IP:        "127.0.0.1",
		UserAgent: "repository-test",
		ExpiresAt: time.Now().UTC().Add(30 * time.Minute),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := repos.RevokeSession(ctx, session.ID); err != nil {
		t.Fatalf("first revoke session: %v", err)
	}

	err = repos.RevokeSession(ctx, session.ID)
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("second revoke err = %v, want ErrSessionNotFound", err)
	}
}

func TestCompleteEmailLoginCreatesUserSessionAndConsumesCodeAtomically(t *testing.T) {
	db := testDB(t)
	repos := New(db)
	ctx := context.Background()
	suffix := uniqueSuffix(t)
	email := "complete-login-" + suffix + "@example.com"
	now := time.Now().UTC()

	older, err := repos.CreateEmailVerificationCode(ctx, CreateEmailVerificationCodeInput{
		Email:     email,
		Purpose:   domain.EmailCodePurposeLogin,
		CodeHash:  security.HashSecret("111111", "pepper"),
		ExpiresAt: now.Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatalf("create older code: %v", err)
	}
	latest, err := repos.CreateEmailVerificationCode(ctx, CreateEmailVerificationCodeInput{
		Email:     email,
		Purpose:   domain.EmailCodePurposeLogin,
		CodeHash:  security.HashSecret("222222", "pepper"),
		ExpiresAt: now.Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatalf("create latest code: %v", err)
	}

	completed, err := repos.CompleteEmailLogin(ctx, CompleteEmailLoginInput{
		Email:            email,
		Purpose:          domain.EmailCodePurposeLogin,
		Code:             "222222",
		Pepper:           "pepper",
		WorkspaceName:    "Complete Login",
		WorkspaceSlug:    "complete-login-" + suffix,
		SessionTokenHash: "session-hash-" + suffix,
		SessionExpiresAt: now.Add(30 * 24 * time.Hour),
		EmailVerifiedAt:  now,
	})
	if err != nil {
		t.Fatalf("complete email login: %v", err)
	}

	if completed.User.ID == "" {
		t.Fatalf("expected created user")
	}
	if completed.User.PrimaryEmail == nil || *completed.User.PrimaryEmail != email {
		t.Fatalf("primary email = %v, want %s", completed.User.PrimaryEmail, email)
	}
	if completed.User.EmailVerifiedAt == nil {
		t.Fatalf("expected email_verified_at")
	}
	if completed.Session.UserID != completed.User.ID {
		t.Fatalf("session user id = %s, want %s", completed.Session.UserID, completed.User.ID)
	}

	var latestRow domain.EmailVerificationCode
	if err := db.Where("id = ?", latest.ID).First(&latestRow).Error; err != nil {
		t.Fatalf("find latest code: %v", err)
	}
	if latestRow.Status != domain.EmailCodeStatusConsumed {
		t.Fatalf("latest status = %s, want %s", latestRow.Status, domain.EmailCodeStatusConsumed)
	}

	var olderRow domain.EmailVerificationCode
	if err := db.Where("id = ?", older.ID).First(&olderRow).Error; err != nil {
		t.Fatalf("find older code: %v", err)
	}
	if olderRow.Status == domain.EmailCodeStatusPending {
		t.Fatalf("older code must not remain pending")
	}

	foundSession, err := repos.FindActiveSessionByTokenHash(ctx, "session-hash-"+suffix)
	if err != nil {
		t.Fatalf("find active session: %v", err)
	}
	if foundSession.ID != completed.Session.ID {
		t.Fatalf("found session id = %s, want %s", foundSession.ID, completed.Session.ID)
	}
}

func TestCompleteEmailLoginGrantsTrialCreditForNewDefaultWorkspace(t *testing.T) {
	db := testDB(t)
	repos := New(db)
	ctx := context.Background()
	suffix := uniqueSuffix(t)
	email := "trial-login-" + suffix + "@example.com"
	now := time.Now().UTC().Truncate(time.Second)

	_, err := repos.CreateEmailVerificationCode(ctx, CreateEmailVerificationCodeInput{
		Email:     email,
		Purpose:   domain.EmailCodePurposeLogin,
		CodeHash:  security.HashSecret("222222", "pepper"),
		ExpiresAt: now.Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatalf("create email code: %v", err)
	}

	completed, err := repos.CompleteEmailLogin(ctx, CompleteEmailLoginInput{
		Email:            email,
		Purpose:          domain.EmailCodePurposeLogin,
		Code:             "222222",
		Pepper:           "pepper",
		WorkspaceName:    "Trial Login",
		WorkspaceSlug:    "trial-login-" + suffix,
		SessionTokenHash: "session-hash-" + suffix,
		SessionExpiresAt: now.Add(30 * 24 * time.Hour),
		EmailVerifiedAt:  now,
		TrialCredit: TrialCreditInput{
			AmountMicroCNY: 10_000_000,
			TTLDays:        7,
		},
	})
	if err != nil {
		t.Fatalf("complete email login: %v", err)
	}

	var workspace domain.Workspace
	if err := db.Where("owner_user_id = ?", completed.User.ID).First(&workspace).Error; err != nil {
		t.Fatalf("find workspace: %v", err)
	}
	if workspace.TrialGrantedAt == nil {
		t.Fatalf("expected trial_granted_at")
	}

	var balance domain.WorkspaceBalance
	if err := db.Where("workspace_id = ?", workspace.ID).First(&balance).Error; err != nil {
		t.Fatalf("find balance: %v", err)
	}
	if balance.AvailableMicroCNY != 10_000_000 {
		t.Fatalf("available_micro_cny = %d, want 10000000", balance.AvailableMicroCNY)
	}

	var ledger domain.LedgerEntry
	if err := db.Where("workspace_id = ? AND type = ?", workspace.ID, domain.LedgerTypeTrialGrant).First(&ledger).Error; err != nil {
		t.Fatalf("find trial ledger: %v", err)
	}
	if ledger.Direction != domain.LedgerDirectionCredit || ledger.AmountMicroCNY != 10_000_000 {
		t.Fatalf("ledger = %+v, want 10000000 credit", ledger)
	}
	if ledger.IdempotencyKey != "trial-grant:"+workspace.ID {
		t.Fatalf("idempotency key = %q, want trial-grant:<workspace>", ledger.IdempotencyKey)
	}
	if !strings.Contains(string(ledger.Metadata), `"trial_ttl_days":7`) {
		t.Fatalf("metadata = %s, want trial ttl", string(ledger.Metadata))
	}
}

func TestCompleteEmailLoginDoesNotGrantTrialCreditForExistingUser(t *testing.T) {
	db := testDB(t)
	repos := New(db)
	ctx := context.Background()
	suffix := uniqueSuffix(t)
	email := "trial-existing-" + suffix + "@example.com"
	now := time.Now().UTC()

	_, err := repos.CreateEmailVerificationCode(ctx, CreateEmailVerificationCodeInput{
		Email:     email,
		Purpose:   domain.EmailCodePurposeLogin,
		CodeHash:  security.HashSecret("111111", "pepper"),
		ExpiresAt: now.Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatalf("create first code: %v", err)
	}
	first, err := repos.CompleteEmailLogin(ctx, CompleteEmailLoginInput{
		Email:            email,
		Purpose:          domain.EmailCodePurposeLogin,
		Code:             "111111",
		Pepper:           "pepper",
		WorkspaceName:    "Existing Trial",
		WorkspaceSlug:    "existing-trial-" + suffix,
		SessionTokenHash: "session-hash-first-" + suffix,
		SessionExpiresAt: now.Add(30 * 24 * time.Hour),
		EmailVerifiedAt:  now,
		TrialCredit:      TrialCreditInput{AmountMicroCNY: 10_000_000, TTLDays: 7},
	})
	if err != nil {
		t.Fatalf("first login: %v", err)
	}

	_, err = repos.CreateEmailVerificationCode(ctx, CreateEmailVerificationCodeInput{
		Email:     email,
		Purpose:   domain.EmailCodePurposeLogin,
		CodeHash:  security.HashSecret("222222", "pepper"),
		ExpiresAt: now.Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatalf("create second code: %v", err)
	}
	_, err = repos.CompleteEmailLogin(ctx, CompleteEmailLoginInput{
		Email:            email,
		Purpose:          domain.EmailCodePurposeLogin,
		Code:             "222222",
		Pepper:           "pepper",
		WorkspaceName:    "Should Not Create",
		WorkspaceSlug:    "should-not-create-" + suffix,
		SessionTokenHash: "session-hash-second-" + suffix,
		SessionExpiresAt: now.Add(30 * 24 * time.Hour),
		EmailVerifiedAt:  now,
		TrialCredit:      TrialCreditInput{AmountMicroCNY: 10_000_000, TTLDays: 7},
	})
	if err != nil {
		t.Fatalf("second login: %v", err)
	}

	var workspace domain.Workspace
	if err := db.Where("owner_user_id = ?", first.User.ID).First(&workspace).Error; err != nil {
		t.Fatalf("find workspace: %v", err)
	}

	var count int64
	if err := db.Model(&domain.LedgerEntry{}).
		Where("workspace_id = ? AND type = ?", workspace.ID, domain.LedgerTypeTrialGrant).
		Count(&count).Error; err != nil {
		t.Fatalf("count trial ledgers: %v", err)
	}
	if count != 1 {
		t.Fatalf("trial ledger count = %d, want 1", count)
	}
}

func TestCompleteEmailLoginSkipsTrialCreditWhenAmountIsZero(t *testing.T) {
	db := testDB(t)
	repos := New(db)
	ctx := context.Background()
	suffix := uniqueSuffix(t)
	email := "trial-disabled-" + suffix + "@example.com"
	now := time.Now().UTC()

	_, err := repos.CreateEmailVerificationCode(ctx, CreateEmailVerificationCodeInput{
		Email:     email,
		Purpose:   domain.EmailCodePurposeLogin,
		CodeHash:  security.HashSecret("222222", "pepper"),
		ExpiresAt: now.Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatalf("create code: %v", err)
	}

	completed, err := repos.CompleteEmailLogin(ctx, CompleteEmailLoginInput{
		Email:            email,
		Purpose:          domain.EmailCodePurposeLogin,
		Code:             "222222",
		Pepper:           "pepper",
		WorkspaceName:    "Trial Disabled",
		WorkspaceSlug:    "trial-disabled-" + suffix,
		SessionTokenHash: "session-hash-" + suffix,
		SessionExpiresAt: now.Add(30 * 24 * time.Hour),
		EmailVerifiedAt:  now,
		TrialCredit:      TrialCreditInput{AmountMicroCNY: 0, TTLDays: 7},
	})
	if err != nil {
		t.Fatalf("complete email login: %v", err)
	}

	var workspace domain.Workspace
	if err := db.Where("owner_user_id = ?", completed.User.ID).First(&workspace).Error; err != nil {
		t.Fatalf("find workspace: %v", err)
	}
	if workspace.TrialGrantedAt != nil {
		t.Fatalf("trial_granted_at = %v, want nil", workspace.TrialGrantedAt)
	}

	var balance domain.WorkspaceBalance
	if err := db.Where("workspace_id = ?", workspace.ID).First(&balance).Error; err != nil {
		t.Fatalf("find balance: %v", err)
	}
	if balance.AvailableMicroCNY != 0 {
		t.Fatalf("available_micro_cny = %d, want 0", balance.AvailableMicroCNY)
	}
}

func TestCompleteEmailLoginRollsBackWhenSessionCannotBeCreated(t *testing.T) {
	db := testDB(t)
	repos := New(db)
	ctx := context.Background()
	suffix := uniqueSuffix(t)
	email := "complete-rollback-" + suffix + "@example.com"
	now := time.Now().UTC()

	code, err := repos.CreateEmailVerificationCode(ctx, CreateEmailVerificationCodeInput{
		Email:     email,
		Purpose:   domain.EmailCodePurposeLogin,
		CodeHash:  security.HashSecret("333333", "pepper"),
		ExpiresAt: now.Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatalf("create code: %v", err)
	}

	occupiedEmail := "occupied-session-" + suffix + "@example.com"
	occupiedUser, err := repos.CreateUserWithDefaultWorkspace(ctx, CreateUserWithWorkspaceInput{
		DisplayName:   "Occupied Session",
		PrimaryEmail:  &occupiedEmail,
		WorkspaceName: "Occupied Session",
		WorkspaceSlug: "occupied-session-" + suffix,
	})
	if err != nil {
		t.Fatalf("create occupied user: %v", err)
	}
	const occupiedTokenHash = "occupied-session-token-hash"
	if _, err := repos.CreateSession(ctx, CreateSessionInput{
		UserID:    occupiedUser.User.ID,
		TokenHash: occupiedTokenHash,
		ExpiresAt: now.Add(30 * 24 * time.Hour),
	}); err != nil {
		t.Fatalf("create occupied session: %v", err)
	}

	_, err = repos.CompleteEmailLogin(ctx, CompleteEmailLoginInput{
		Email:            email,
		Purpose:          domain.EmailCodePurposeLogin,
		Code:             "333333",
		Pepper:           "pepper",
		WorkspaceName:    "Rollback Login",
		WorkspaceSlug:    "complete-rollback-" + suffix,
		SessionTokenHash: occupiedTokenHash,
		SessionExpiresAt: now.Add(30 * 24 * time.Hour),
		EmailVerifiedAt:  now,
	})
	if err == nil {
		t.Fatalf("expected session creation error")
	}

	var codeRow domain.EmailVerificationCode
	if err := db.Where("id = ?", code.ID).First(&codeRow).Error; err != nil {
		t.Fatalf("find code: %v", err)
	}
	if codeRow.Status != domain.EmailCodeStatusPending {
		t.Fatalf("code status = %s, want pending after rollback", codeRow.Status)
	}

	var userCount int64
	if err := db.Model(&domain.User{}).Where("primary_email = ?", email).Count(&userCount).Error; err != nil {
		t.Fatalf("count users: %v", err)
	}
	if userCount != 0 {
		t.Fatalf("user count = %d, want 0 after rollback", userCount)
	}
}
