# Trial Credit And Activation Overview Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Do not create git commits unless the user explicitly asks for them.

**Goal:** Grant trial credit to newly registered default Workspaces and expose an authenticated activation overview API.

**Architecture:** Keep the slice inside the existing Go backend layers: config parses trial defaults, repository grants trial credit inside the existing email-login transaction, console service derives activation state, and console handler exposes `GET /api/console/overview`. No new tables are required; this uses `workspaces.trial_granted_at`, `workspace_balances`, and `ledger_entries`.

**Tech Stack:** Go, net/http, GORM, MySQL, goose migrations already present, integer micro-CNY money helpers, repository/service/handler tests.

---

## File Structure

- Modify `backend/internal/config/config.go`: add trial credit config, parse validation, and change `Load()` to return `(Config, error)`.
- Modify `backend/internal/config/config_test.go`: cover trial config defaults, env overrides, invalid amount, and invalid TTL.
- Modify `backend/cmd/portal-api/main.go`: handle `config.Load()` error before route registration.
- Modify `backend/cmd/portal-worker/main.go`: handle `config.Load()` error.
- Modify `backend/cmd/portal-api/main_test.go`: pass trial config into console service seam and register overview route in route tests.
- Modify `backend/internal/repository/auth.go`: add trial config to `CompleteEmailLoginInput`, return created Workspace from the internal find-or-create flow, and grant trial credit in the new-user transaction path.
- Modify `backend/internal/repository/auth_test.go`: verify trial grant, no duplicate existing-user grant, and disabled grant behavior.
- Modify `backend/internal/repository/billing.go`: extract private ledger/balance mutation helper so trial grant can share money semantics with `CreateLedgerEntry`.
- Modify `backend/internal/repository/billing_test.go`: add direct idempotency coverage for the trial helper if needed.
- Modify `backend/internal/api/auth_service.go`: pass configured trial credit from AuthService into repository login completion.
- Modify `backend/internal/api/auth_service_test.go`: update fakes for the new repository input and assert trial config is passed.
- Modify `backend/internal/api/console_service.go`: add overview response types and service method.
- Modify `backend/internal/api/console_service_test.go`: cover overview mapping, billing role access, missing Workspace, and empty user.
- Modify `backend/internal/api/console.go`: register and implement `GET /api/console/overview`.
- Modify `backend/internal/api/console_test.go`: cover overview auth requirement, success response shape, and fake service method.

---

## Task 1: Add Trial Credit Config

**Files:**
- Modify: `backend/internal/config/config.go`
- Modify: `backend/internal/config/config_test.go`
- Modify: `backend/cmd/portal-api/main.go`
- Modify: `backend/cmd/portal-worker/main.go`
- Modify: `backend/cmd/portal-api/main_test.go`

- [ ] **Step 1: Write failing config tests**

Add these tests to `backend/internal/config/config_test.go`:

```go
func TestLoadTrialCreditDefaults(t *testing.T) {
	t.Setenv("PORTAL_ENV", "development")
	t.Setenv("PORTAL_AUTH_PEPPER", "")
	t.Setenv("PORTAL_TRIAL_CREDIT_MICRO_CNY", "")
	t.Setenv("PORTAL_TRIAL_CREDIT_TTL_DAYS", "")

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() err = %v", err)
	}

	if got.TrialCredit.AmountMicroCNY != 10_000_000 {
		t.Fatalf("TrialCredit.AmountMicroCNY = %d, want 10000000", got.TrialCredit.AmountMicroCNY)
	}
	if got.TrialCredit.TTLDays != 7 {
		t.Fatalf("TrialCredit.TTLDays = %d, want 7", got.TrialCredit.TTLDays)
	}
}

func TestLoadTrialCreditEnvOverrides(t *testing.T) {
	t.Setenv("PORTAL_ENV", "development")
	t.Setenv("PORTAL_TRIAL_CREDIT_MICRO_CNY", "2500000")
	t.Setenv("PORTAL_TRIAL_CREDIT_TTL_DAYS", "14")

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() err = %v", err)
	}

	if got.TrialCredit.AmountMicroCNY != 2_500_000 {
		t.Fatalf("TrialCredit.AmountMicroCNY = %d, want 2500000", got.TrialCredit.AmountMicroCNY)
	}
	if got.TrialCredit.TTLDays != 14 {
		t.Fatalf("TrialCredit.TTLDays = %d, want 14", got.TrialCredit.TTLDays)
	}
}

func TestLoadTrialCreditRejectsNegativeAmount(t *testing.T) {
	t.Setenv("PORTAL_ENV", "development")
	t.Setenv("PORTAL_TRIAL_CREDIT_MICRO_CNY", "-1")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "PORTAL_TRIAL_CREDIT_MICRO_CNY") {
		t.Fatalf("Load() err = %v, want trial amount error", err)
	}
}

func TestLoadTrialCreditRejectsInvalidTTL(t *testing.T) {
	t.Setenv("PORTAL_ENV", "development")
	t.Setenv("PORTAL_TRIAL_CREDIT_TTL_DAYS", "0")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "PORTAL_TRIAL_CREDIT_TTL_DAYS") {
		t.Fatalf("Load() err = %v, want trial ttl error", err)
	}
}
```

Update the existing tests in the same file to call `got, err := Load()` and assert `err == nil`.

- [ ] **Step 2: Run config tests and verify they fail**

Run:

```bash
GOCACHE=/private/tmp/go-build-cache go test ./internal/config -count=1
```

Expected: compile fails because `Load()` still returns one value and `Config.TrialCredit` does not exist.

- [ ] **Step 3: Implement config parsing**

Replace `backend/internal/config/config.go` with this shape, keeping existing environment behavior:

```go
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	defaultTrialCreditMicroCNY = int64(10_000_000)
	defaultTrialCreditTTLDays  = 7
)

type Config struct {
	Env         string
	HTTPAddr    string
	DatabaseDSN string
	AuthPepper  string
	TrialCredit TrialCreditConfig
}

type TrialCreditConfig struct {
	AmountMicroCNY int64
	TTLDays        int
}

func Load() (Config, error) {
	env := normalizeEnv(envOrDefault("PORTAL_ENV", "development"))
	authPepper := os.Getenv("PORTAL_AUTH_PEPPER")
	if authPepper == "" && env != "production" {
		authPepper = "dev-auth-pepper"
	}

	trialCredit, err := loadTrialCreditConfig()
	if err != nil {
		return Config{}, err
	}

	return Config{
		Env:         env,
		HTTPAddr:    envOrDefault("PORTAL_HTTP_ADDR", ":8080"),
		DatabaseDSN: os.Getenv("PORTAL_DATABASE_DSN"),
		AuthPepper:  authPepper,
		TrialCredit: trialCredit,
	}, nil
}

func loadTrialCreditConfig() (TrialCreditConfig, error) {
	amount, err := int64EnvOrDefault("PORTAL_TRIAL_CREDIT_MICRO_CNY", defaultTrialCreditMicroCNY)
	if err != nil {
		return TrialCreditConfig{}, err
	}
	if amount < 0 {
		return TrialCreditConfig{}, fmt.Errorf("PORTAL_TRIAL_CREDIT_MICRO_CNY must be greater than or equal to zero")
	}

	ttlDays, err := intEnvOrDefault("PORTAL_TRIAL_CREDIT_TTL_DAYS", defaultTrialCreditTTLDays)
	if err != nil {
		return TrialCreditConfig{}, err
	}
	if ttlDays <= 0 {
		return TrialCreditConfig{}, fmt.Errorf("PORTAL_TRIAL_CREDIT_TTL_DAYS must be greater than zero")
	}

	return TrialCreditConfig{
		AmountMicroCNY: amount,
		TTLDays:        ttlDays,
	}, nil
}

func int64EnvOrDefault(key string, fallback int64) (int64, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}
	return parsed, nil
}

func intEnvOrDefault(key string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}
	return parsed, nil
}

func envOrDefault(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func normalizeEnv(env string) string {
	return strings.ToLower(strings.TrimSpace(env))
}
```

- [ ] **Step 4: Update command startup callers**

In `backend/cmd/portal-api/main.go`, replace:

```go
cfg := config.Load()
```

with:

```go
cfg, err := config.Load()
if err != nil {
	log.Fatalf("load config: %v", err)
}
```

In `backend/cmd/portal-worker/main.go`, make the same change and reuse its existing logger/imports.

In `backend/cmd/portal-api/main.go`, change the service seam:

```go
newPortalAuthService = api.NewAuthService
newPortalConsoleService = api.NewConsoleService
```

to:

```go
newPortalAuthService = api.NewAuthService
newPortalConsoleService = api.NewConsoleService
```

Keep names unchanged now; later tasks will update signatures.

- [ ] **Step 5: Update existing config tests**

In existing config tests, change calls from:

```go
got := Load()
```

to:

```go
got, err := Load()
if err != nil {
	t.Fatalf("Load() err = %v", err)
}
```

Add `strings` to the test imports for the invalid-env assertions.

- [ ] **Step 6: Run config tests**

Run:

```bash
GOCACHE=/private/tmp/go-build-cache go test ./internal/config -count=1
```

Expected: PASS.

---

## Task 2: Grant Trial Credit In The Login Transaction

**Files:**
- Modify: `backend/internal/repository/auth.go`
- Modify: `backend/internal/repository/auth_test.go`
- Modify: `backend/internal/repository/billing.go`

- [ ] **Step 1: Write failing repository test for new-user trial grant**

Add this test to `backend/internal/repository/auth_test.go`:

```go
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
```

Add `strings` to imports.

- [ ] **Step 2: Write failing repository test for existing-user login**

Add:

```go
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
```

- [ ] **Step 3: Write failing repository test for disabled trial amount**

Add:

```go
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
```

- [ ] **Step 4: Run repository tests and verify they fail**

Run:

```bash
GOCACHE=/private/tmp/go-build-cache go test ./internal/repository -run 'TestCompleteEmailLogin.*Trial' -count=1
```

Expected: compile fails because `TrialCreditInput` and `CompleteEmailLoginInput.TrialCredit` do not exist.

- [ ] **Step 5: Add repository trial input types**

In `backend/internal/repository/auth.go`, add:

```go
type TrialCreditInput struct {
	AmountMicroCNY int64
	TTLDays        int
}
```

Add this field to `CompleteEmailLoginInput`:

```go
TrialCredit TrialCreditInput
```

Change the internal result for find-or-create:

```go
type emailUserWorkspaceResult struct {
	User             domain.User
	Workspace        domain.Workspace
	CreatedWorkspace bool
}
```

- [ ] **Step 6: Refactor find-or-create to return Workspace creation state**

Change `findOrCreateEmailUserForUpdate` to return `emailUserWorkspaceResult`.

Existing-user path:

```go
if err == nil {
	return emailUserWorkspaceResult{User: user}, nil
}
```

New-user path after creating the balance:

```go
return emailUserWorkspaceResult{
	User:             user,
	Workspace:        workspace,
	CreatedWorkspace: true,
}, nil
```

Update the call site in `CompleteEmailLogin`:

```go
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
```

- [ ] **Step 7: Implement transaction-local trial grant**

In `backend/internal/repository/billing.go`, add a private helper:

```go
func grantTrialCreditInTx(tx *gorm.DB, workspaceID string, now time.Time, input TrialCreditInput) error {
	if input.AmountMicroCNY == 0 {
		return nil
	}
	if input.AmountMicroCNY < 0 {
		return fmt.Errorf("trial credit amount must be greater than or equal to zero")
	}
	if input.TTLDays <= 0 {
		return fmt.Errorf("trial credit ttl days must be greater than zero")
	}

	var workspace domain.Workspace
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", workspaceID).
		First(&workspace).Error; err != nil {
		return fmt.Errorf("lock trial workspace: %w", err)
	}
	if workspace.TrialGrantedAt != nil {
		return nil
	}

	expiresAt := now.AddDate(0, 0, input.TTLDays)
	metadata, err := datatypes.NewJSONType(map[string]any{
		"source":           "email_registration",
		"trial_expires_at": expiresAt.Format(time.RFC3339Nano),
		"trial_ttl_days":   input.TTLDays,
	}).MarshalJSON()
	if err != nil {
		return fmt.Errorf("marshal trial metadata: %w", err)
	}

	entryInput := CreateLedgerInput{
		WorkspaceID:    workspaceID,
		Type:           domain.LedgerTypeTrialGrant,
		Direction:      domain.LedgerDirectionCredit,
		AmountMicroCNY: input.AmountMicroCNY,
		Currency:       "CNY",
		IdempotencyKey: "trial-grant:" + workspaceID,
		Metadata:       datatypes.JSON(metadata),
	}
	if _, err := createLedgerEntryInTx(tx, entryInput, now); err != nil {
		return err
	}

	if err := tx.Model(&domain.Workspace{}).
		Where("id = ? AND trial_granted_at IS NULL", workspaceID).
		Updates(map[string]any{
			"trial_granted_at": now,
			"updated_at":       now,
		}).Error; err != nil {
		return fmt.Errorf("mark trial granted: %w", err)
	}

	return nil
}
```

If `datatypes.NewJSONType(...).MarshalJSON()` is awkward with the installed GORM version, use `encoding/json.Marshal` and cast to `datatypes.JSON`.

- [ ] **Step 8: Extract shared ledger mutation helper**

In `backend/internal/repository/billing.go`, move the body of `CreateLedgerEntry`'s transaction into:

```go
func createLedgerEntryInTx(tx *gorm.DB, input CreateLedgerInput, now time.Time) (domain.LedgerEntry, error)
```

Then `CreateLedgerEntry` becomes:

```go
func (r *Repositories) CreateLedgerEntry(ctx context.Context, input CreateLedgerInput) (domain.LedgerEntry, error) {
	var output domain.LedgerEntry
	now := time.Now().UTC()

	err := r.withTx(ctx, func(tx *gorm.DB) error {
		entry, err := createLedgerEntryInTx(tx, input, now)
		if err != nil {
			return err
		}
		output = entry
		return nil
	})
	if err != nil {
		return domain.LedgerEntry{}, err
	}

	return output, nil
}
```

The helper must preserve the existing duplicate-idempotency behavior and balance update semantics exactly.

- [ ] **Step 9: Run repository trial tests**

Run:

```bash
GOCACHE=/private/tmp/go-build-cache go test ./internal/repository -run 'TestCompleteEmailLogin.*Trial|TestCreateLedgerEntryIdempotent' -count=1
```

Expected: PASS.

---

## Task 3: Pass Trial Config Through Auth Service

**Files:**
- Modify: `backend/internal/api/auth_service.go`
- Modify: `backend/internal/api/auth_service_test.go`
- Modify: `backend/cmd/portal-api/main.go`
- Modify: `backend/cmd/portal-api/main_test.go`

- [ ] **Step 1: Write failing auth service test**

In `backend/internal/api/auth_service_test.go`, find the fake store that records `CompleteEmailLoginInput`. Add this assertion to the successful verify-email-login test:

```go
if store.completeEmailLoginInput.TrialCredit.AmountMicroCNY != 10_000_000 {
	t.Fatalf("trial amount = %d, want 10000000", store.completeEmailLoginInput.TrialCredit.AmountMicroCNY)
}
if store.completeEmailLoginInput.TrialCredit.TTLDays != 7 {
	t.Fatalf("trial ttl = %d, want 7", store.completeEmailLoginInput.TrialCredit.TTLDays)
}
```

Construct the service in that test with:

```go
service, err := newAuthService(store, "test", "pepper", config.TrialCreditConfig{
	AmountMicroCNY: 10_000_000,
	TTLDays:        7,
})
```

Add the import:

```go
"github.com/tokenlive/tokenlive-portal/backend/internal/config"
```

- [ ] **Step 2: Run auth service tests and verify they fail**

Run:

```bash
GOCACHE=/private/tmp/go-build-cache go test ./internal/api -run 'TestAuth.*|TestVerify.*|TestStart.*' -count=1
```

Expected: compile fails because `newAuthService` does not accept trial config.

- [ ] **Step 3: Update AuthService constructor signatures**

In `backend/internal/api/auth_service.go`, add to `authService`:

```go
trialCredit config.TrialCreditConfig
```

Update imports to include config:

```go
"github.com/tokenlive/tokenlive-portal/backend/internal/config"
```

Change constructors:

```go
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
```

- [ ] **Step 4: Pass trial config into repository login completion**

In `VerifyEmailLogin`, add:

```go
TrialCredit: repository.TrialCreditInput{
	AmountMicroCNY: s.trialCredit.AmountMicroCNY,
	TTLDays:        s.trialCredit.TTLDays,
},
```

to `repository.CompleteEmailLoginInput`.

- [ ] **Step 5: Update command wiring**

In `backend/cmd/portal-api/main.go`, change seam type inference by using the new constructor signature:

```go
authService, err := newPortalAuthService(modelRepository, cfg.Env, cfg.AuthPepper, cfg.TrialCredit)
```

Update `backend/cmd/portal-api/main_test.go` fake seam:

```go
newPortalAuthService = func(_ *repository.Repositories, _ string, _ string, _ config.TrialCreditConfig) (api.AuthService, error) {
	return fakePortalAuthService{}, nil
}
```

- [ ] **Step 6: Update auth service tests to use default trial config helper**

Where tests call `newAuthService(store, env, pepper)`, update to:

```go
newAuthService(store, env, pepper, config.TrialCreditConfig{
	AmountMicroCNY: 10_000_000,
	TTLDays:        7,
})
```

- [ ] **Step 7: Run auth and main tests**

Run:

```bash
GOCACHE=/private/tmp/go-build-cache go test ./internal/api ./cmd/portal-api -run 'Test.*Auth|Test.*Email|TestRegisterDatabaseBackedRoutes' -count=1
```

Expected: PASS.

---

## Task 4: Add Console Overview Service

**Files:**
- Modify: `backend/internal/api/console_service.go`
- Modify: `backend/internal/api/console_service_test.go`

- [ ] **Step 1: Write failing overview service tests**

Add this test to `backend/internal/api/console_service_test.go`:

```go
func TestConsoleServiceOverviewMapsActivationState(t *testing.T) {
	t.Parallel()

	trialGrantedAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	store := &fakeConsoleStore{
		currentWorkspace: repository.CurrentWorkspaceResult{
			Workspace: domain.Workspace{
				ID:             "wsp_1",
				Name:           "Dev",
				Slug:           "dev",
				Status:         domain.WorkspaceStatusActive,
				TrialGrantedAt: &trialGrantedAt,
			},
			Role: domain.MemberRoleBilling,
			Balance: domain.WorkspaceBalance{
				AvailableMicroCNY: 10_000_000,
			},
		},
		listAPIKeysResult: []domain.APIKey{{
			ID:     "ak_1",
			Status: domain.APIKeyStatusRevoked,
		}},
	}
	service, err := newConsoleService(store, "pepper", config.TrialCreditConfig{
		AmountMicroCNY: 10_000_000,
		TTLDays:        7,
	})
	if err != nil {
		t.Fatalf("new console service: %v", err)
	}

	got, err := service.Overview(context.Background(), CurrentUser{ID: "usr_1"})
	if err != nil {
		t.Fatalf("overview: %v", err)
	}

	if got.Workspace.TrialGrantedAt == nil || !got.Workspace.TrialGrantedAt.Equal(trialGrantedAt) {
		t.Fatalf("trial_granted_at = %v, want %v", got.Workspace.TrialGrantedAt, trialGrantedAt)
	}
	if got.Workspace.Balance.AvailableCNY != "10.000000" {
		t.Fatalf("available cny = %s, want 10.000000", got.Workspace.Balance.AvailableCNY)
	}
	if !got.Activation.TrialCreditGranted {
		t.Fatalf("expected trial credit granted")
	}
	wantExpiresAt := trialGrantedAt.AddDate(0, 0, 7)
	if got.Activation.TrialExpiresAt == nil || !got.Activation.TrialExpiresAt.Equal(wantExpiresAt) {
		t.Fatalf("trial expires at = %v, want %v", got.Activation.TrialExpiresAt, wantExpiresAt)
	}
	if !got.Activation.APIKeyCreated {
		t.Fatalf("expected api key created")
	}
	if got.Activation.FirstCallMade {
		t.Fatalf("first_call_made should remain false until usage slice")
	}
	if len(got.Activation.Steps) != 3 {
		t.Fatalf("steps len = %d, want 3", len(got.Activation.Steps))
	}
	if got.Activation.Steps[0].Key != "trial_credit" || got.Activation.Steps[0].Status != ActivationStepCompleted {
		t.Fatalf("trial step = %+v", got.Activation.Steps[0])
	}
	if got.Activation.Steps[1].Key != "api_key" || got.Activation.Steps[1].Status != ActivationStepCompleted {
		t.Fatalf("api key step = %+v", got.Activation.Steps[1])
	}
	if got.Activation.Steps[2].Key != "first_call" || got.Activation.Steps[2].Status != ActivationStepPending {
		t.Fatalf("first call step = %+v", got.Activation.Steps[2])
	}
}
```

Add `config` import.

- [ ] **Step 2: Add missing-user and missing-Workspace service tests**

Add:

```go
func TestConsoleServiceOverviewRequiresUser(t *testing.T) {
	t.Parallel()

	service, err := newConsoleService(&fakeConsoleStore{}, "pepper", config.TrialCreditConfig{AmountMicroCNY: 10_000_000, TTLDays: 7})
	if err != nil {
		t.Fatalf("new console service: %v", err)
	}

	_, err = service.Overview(context.Background(), CurrentUser{})
	if !errors.Is(err, ErrAuthUnauthorized) {
		t.Fatalf("err = %v, want ErrAuthUnauthorized", err)
	}
}

func TestConsoleServiceOverviewMapsWorkspaceNotFound(t *testing.T) {
	t.Parallel()

	store := &fakeConsoleStore{currentWorkspaceErr: repository.ErrWorkspaceNotFound}
	service, err := newConsoleService(store, "pepper", config.TrialCreditConfig{AmountMicroCNY: 10_000_000, TTLDays: 7})
	if err != nil {
		t.Fatalf("new console service: %v", err)
	}

	_, err = service.Overview(context.Background(), CurrentUser{ID: "usr_1"})
	if !errors.Is(err, ErrConsoleWorkspaceNotFound) {
		t.Fatalf("err = %v, want ErrConsoleWorkspaceNotFound", err)
	}
}
```

- [ ] **Step 3: Run service tests and verify they fail**

Run:

```bash
GOCACHE=/private/tmp/go-build-cache go test ./internal/api -run 'TestConsoleServiceOverview|TestConsoleServiceCurrentWorkspace' -count=1
```

Expected: compile fails because `Overview`, response types, statuses, and the new console constructor signature do not exist.

- [ ] **Step 4: Add overview types and service method**

In `backend/internal/api/console_service.go`, import config:

```go
"github.com/tokenlive/tokenlive-portal/backend/internal/config"
```

Add:

```go
type ActivationStepStatus string

const (
	ActivationStepCompleted ActivationStepStatus = "completed"
	ActivationStepPending   ActivationStepStatus = "pending"
)

type ConsoleOverviewResponse struct {
	Workspace  WorkspaceResponse          `json:"workspace"`
	Activation ActivationOverviewResponse `json:"activation"`
}

type ActivationOverviewResponse struct {
	TrialCreditGranted bool                     `json:"trial_credit_granted"`
	TrialExpiresAt     *time.Time               `json:"trial_expires_at"`
	APIKeyCreated      bool                     `json:"api_key_created"`
	FirstCallMade      bool                     `json:"first_call_made"`
	Steps              []ActivationStepResponse `json:"steps"`
}

type ActivationStepResponse struct {
	Key    string               `json:"key"`
	Label  string               `json:"label"`
	Status ActivationStepStatus `json:"status"`
}
```

Add `TrialGrantedAt` to `WorkspaceResponse`:

```go
TrialGrantedAt *time.Time `json:"trial_granted_at"`
```

Add `Overview` to `ConsoleService`:

```go
Overview(ctx context.Context, user CurrentUser) (ConsoleOverviewResponse, error)
```

Add `trialCredit config.TrialCreditConfig` to `consoleService` and change constructors:

```go
func newConsoleService(store consoleStore, authPepper string, trialCredit config.TrialCreditConfig) (*consoleService, error)
func NewConsoleService(repos *repository.Repositories, authPepper string, trialCredit config.TrialCreditConfig) (ConsoleService, error)
```

Validate amount and TTL in the constructor like AuthService.

- [ ] **Step 5: Implement Overview**

Add:

```go
func (s *consoleService) Overview(ctx context.Context, user CurrentUser) (ConsoleOverviewResponse, error) {
	current, err := s.resolveWorkspace(ctx, user)
	if err != nil {
		return ConsoleOverviewResponse{}, err
	}

	keys, err := s.store.ListAPIKeysByWorkspace(ctx, current.Workspace.ID)
	if err != nil {
		return ConsoleOverviewResponse{}, mapConsoleRepositoryError(err)
	}

	trialGranted := current.Workspace.TrialGrantedAt != nil
	var trialExpiresAt *time.Time
	if current.Workspace.TrialGrantedAt != nil {
		expires := current.Workspace.TrialGrantedAt.AddDate(0, 0, s.trialCredit.TTLDays)
		trialExpiresAt = &expires
	}
	apiKeyCreated := len(keys) > 0
	firstCallMade := false

	return ConsoleOverviewResponse{
		Workspace: workspaceResponseFromRepository(current),
		Activation: ActivationOverviewResponse{
			TrialCreditGranted: trialGranted,
			TrialExpiresAt:     trialExpiresAt,
			APIKeyCreated:      apiKeyCreated,
			FirstCallMade:      firstCallMade,
			Steps:              activationSteps(trialGranted, apiKeyCreated, firstCallMade),
		},
	}, nil
}

func activationSteps(trialGranted bool, apiKeyCreated bool, firstCallMade bool) []ActivationStepResponse {
	return []ActivationStepResponse{
		{Key: "trial_credit", Label: "Receive trial credit", Status: activationStatus(trialGranted)},
		{Key: "api_key", Label: "Create API key", Status: activationStatus(apiKeyCreated)},
		{Key: "first_call", Label: "Make first API call", Status: activationStatus(firstCallMade)},
	}
}

func activationStatus(done bool) ActivationStepStatus {
	if done {
		return ActivationStepCompleted
	}
	return ActivationStepPending
}
```

Update `workspaceResponseFromRepository`:

```go
TrialGrantedAt: current.Workspace.TrialGrantedAt,
```

- [ ] **Step 6: Update console service tests for constructor signature**

Replace every:

```go
newConsoleService(store, "pepper")
```

with:

```go
newConsoleService(store, "pepper", config.TrialCreditConfig{
	AmountMicroCNY: 10_000_000,
	TTLDays:        7,
})
```

- [ ] **Step 7: Run console service tests**

Run:

```bash
GOCACHE=/private/tmp/go-build-cache go test ./internal/api -run 'TestConsoleService|TestMoneyAmount|TestCanManageAPIKeys|TestMapConsoleError' -count=1
```

Expected: PASS.

---

## Task 5: Add Overview Handler And Route

**Files:**
- Modify: `backend/internal/api/console.go`
- Modify: `backend/internal/api/console_test.go`
- Modify: `backend/cmd/portal-api/main.go`
- Modify: `backend/cmd/portal-api/main_test.go`

- [ ] **Step 1: Write failing handler tests**

Add to `backend/internal/api/console_test.go`:

```go
func TestConsoleOverviewRequiresSession(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	RegisterConsoleRoutes(mux, &fakeConsoleService{}, &fakeAuthService{})
	req := httptest.NewRequest(http.MethodGet, "/api/console/overview", nil)
	req.Header.Set("X-Request-ID", "req_console_overview_session")
	rec := httptest.NewRecorder()

	RequestID(mux).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	assertAuthErrorResponse(t, rec, string(CodeAuthSessionRequired), "req_console_overview_session")
}

func TestConsoleOverviewReturnsActivationState(t *testing.T) {
	t.Parallel()

	trialGrantedAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	trialExpiresAt := trialGrantedAt.AddDate(0, 0, 7)
	console := &fakeConsoleService{
		overviewResult: ConsoleOverviewResponse{
			Workspace: WorkspaceResponse{
				ID:             "wsp_1",
				Name:           "Dev",
				Slug:           "dev",
				Role:           domain.MemberRoleOwner,
				Status:         domain.WorkspaceStatusActive,
				TrialGrantedAt: &trialGrantedAt,
				Balance: WorkspaceBalanceResponse{
					AvailableMicroCNY: 10_000_000,
					AvailableCNY:      "10.000000",
				},
			},
			Activation: ActivationOverviewResponse{
				TrialCreditGranted: true,
				TrialExpiresAt:     &trialExpiresAt,
				APIKeyCreated:      true,
				FirstCallMade:      false,
				Steps: []ActivationStepResponse{
					{Key: "trial_credit", Label: "Receive trial credit", Status: ActivationStepCompleted},
					{Key: "api_key", Label: "Create API key", Status: ActivationStepCompleted},
					{Key: "first_call", Label: "Make first API call", Status: ActivationStepPending},
				},
			},
		},
	}
	auth := &fakeAuthService{currentUser: CurrentUser{ID: "usr_1"}}
	mux := http.NewServeMux()
	RegisterConsoleRoutes(mux, console, auth)

	req := httptest.NewRequest(http.MethodGet, "/api/console/overview", nil)
	req.AddCookie(&http.Cookie{Name: security.SessionCookieName, Value: "tl_sess_test"})
	rec := httptest.NewRecorder()

	RequestID(mux).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body ConsoleOverviewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Workspace.ID != "wsp_1" || body.Workspace.Balance.AvailableCNY != "10.000000" {
		t.Fatalf("workspace = %+v", body.Workspace)
	}
	if !body.Activation.TrialCreditGranted || !body.Activation.APIKeyCreated || body.Activation.FirstCallMade {
		t.Fatalf("activation = %+v", body.Activation)
	}
	if len(body.Activation.Steps) != 3 {
		t.Fatalf("steps len = %d, want 3", len(body.Activation.Steps))
	}
	if console.overviewUser.ID != "usr_1" {
		t.Fatalf("overview user id = %q, want usr_1", console.overviewUser.ID)
	}
}
```

Add `time` import if missing.

- [ ] **Step 2: Update fake console service**

In `fakeConsoleService`, add:

```go
overviewUser   CurrentUser
overviewResult ConsoleOverviewResponse
overviewErr    error
```

Add method:

```go
func (f *fakeConsoleService) Overview(_ context.Context, user CurrentUser) (ConsoleOverviewResponse, error) {
	f.overviewUser = user
	return f.overviewResult, f.overviewErr
}
```

- [ ] **Step 3: Run handler tests and verify they fail**

Run:

```bash
GOCACHE=/private/tmp/go-build-cache go test ./internal/api -run 'TestConsoleOverview|TestConsoleRegisterRoutes' -count=1
```

Expected: `/api/console/overview` returns 404 or compile fails because `ConsoleService` has no handler method wired.

- [ ] **Step 4: Register and implement overview handler**

In `RegisterConsoleRoutes`, add:

```go
mux.HandleFunc("GET /api/console/overview", handler.Overview)
```

Add:

```go
func (h ConsoleHandler) Overview(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	result, err := h.service.Overview(r.Context(), user)
	if err != nil {
		WriteError(w, RequestIDFromContext(r.Context()), mapConsoleError(err))
		return
	}

	writeJSON(w, http.StatusOK, result)
}
```

- [ ] **Step 5: Update command route tests**

In `backend/cmd/portal-api/main_test.go`, add `/api/console/overview` to the path lists in:

- `TestRegisterDatabaseBackedRoutesDisablesAuthPublicAndConsoleRoutesWithoutDSN`
- `TestRegisterDatabaseBackedRoutesRegistersPublicAuthAndConsoleRoutes`
- `TestRegisterDatabaseBackedRoutesDoesNotMutateMuxWhenConsoleServiceFails`

Update `stubPortalRouteSeams` to register:

```go
mux.HandleFunc("GET /api/console/overview", func(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
})
```

Add `Overview` to `fakePortalConsoleService`:

```go
func (fakePortalConsoleService) Overview(context.Context, api.CurrentUser) (api.ConsoleOverviewResponse, error) {
	return api.ConsoleOverviewResponse{}, nil
}
```

- [ ] **Step 6: Update console service command constructor wiring**

In `backend/cmd/portal-api/main.go`, change:

```go
consoleService, err := newPortalConsoleService(modelRepository, cfg.AuthPepper)
```

to:

```go
consoleService, err := newPortalConsoleService(modelRepository, cfg.AuthPepper, cfg.TrialCredit)
```

Update the fake seam in `main_test.go`:

```go
newPortalConsoleService = func(_ *repository.Repositories, _ string, _ config.TrialCreditConfig) (api.ConsoleService, error) {
	return fakePortalConsoleService{}, nil
}
```

- [ ] **Step 7: Run API and command tests**

Run:

```bash
GOCACHE=/private/tmp/go-build-cache go test ./internal/api ./cmd/portal-api -run 'TestConsoleOverview|TestConsoleRegisterRoutes|TestRegisterDatabaseBackedRoutes' -count=1
```

Expected: PASS.

---

## Task 6: Keep Existing Workspace And API Key Responses Compatible

**Files:**
- Modify: `backend/internal/api/console_service_test.go`
- Modify: `backend/internal/api/console_test.go`

- [ ] **Step 1: Add compatibility assertions**

In `TestConsoleServiceCurrentWorkspaceMapsBalance`, set:

```go
trialGrantedAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
```

and include in fake Workspace:

```go
TrialGrantedAt: &trialGrantedAt,
```

Assert:

```go
if got.Workspace.TrialGrantedAt == nil || !got.Workspace.TrialGrantedAt.Equal(trialGrantedAt) {
	t.Fatalf("trial_granted_at = %v, want %v", got.Workspace.TrialGrantedAt, trialGrantedAt)
}
```

In `TestConsoleCurrentWorkspaceReturnsWorkspace`, add `TrialGrantedAt` to the fake response and assert it survives JSON round-trip:

```go
if body.Workspace.TrialGrantedAt == nil {
	t.Fatalf("expected trial_granted_at in current workspace response")
}
```

- [ ] **Step 2: Run current Workspace tests**

Run:

```bash
GOCACHE=/private/tmp/go-build-cache go test ./internal/api -run 'TestConsoleCurrentWorkspace|TestConsoleServiceCurrentWorkspace' -count=1
```

Expected: PASS.

---

## Task 7: Update All Constructor Call Sites And Compile

**Files:**
- Modify any Go test or production file still using old constructor signatures.

- [ ] **Step 1: Find stale constructor calls**

Run:

```bash
rg -n "newAuthService\\(|NewAuthService\\(|newConsoleService\\(|NewConsoleService\\(|config\\.Load\\(" backend
```

Expected:

- `config.Load()` call sites use two-value assignment.
- `NewAuthService` and `newAuthService` pass `config.TrialCreditConfig`.
- `NewConsoleService` and `newConsoleService` pass `config.TrialCreditConfig`.

- [ ] **Step 2: Run full Go test suite**

Run:

```bash
GOCACHE=/private/tmp/go-build-cache go test ./...
```

Expected: PASS.

- [ ] **Step 3: Fix compile issues from signature churn**

If tests fail due to old signatures, update the exact call site to pass:

```go
config.TrialCreditConfig{
	AmountMicroCNY: 10_000_000,
	TTLDays:        7,
}
```

If production startup fails due to `config.Load()`, update the command to:

```go
cfg, err := config.Load()
if err != nil {
	log.Fatalf("load config: %v", err)
}
```

- [ ] **Step 4: Re-run full Go test suite**

Run:

```bash
GOCACHE=/private/tmp/go-build-cache go test ./...
```

Expected: PASS.

---

## Task 8: Final Build And Documentation Check

**Files:**
- Verify only; no source changes expected unless checks reveal a real issue.

- [ ] **Step 1: Format Go code**

Run:

```bash
gofmt -w backend/cmd/portal-api backend/cmd/portal-worker backend/internal/api backend/internal/config backend/internal/repository
```

Expected: command exits successfully with no output.

- [ ] **Step 2: Run full tests**

Run:

```bash
GOCACHE=/private/tmp/go-build-cache go test ./...
```

Expected: PASS.

- [ ] **Step 3: Build portal-api**

Run:

```bash
GOCACHE=/private/tmp/go-build-cache go build -o /private/tmp/tokenlive-portal-api ./cmd/portal-api
```

Expected: command exits successfully.

- [ ] **Step 4: Build portal-worker**

Run:

```bash
GOCACHE=/private/tmp/go-build-cache go build -o /private/tmp/tokenlive-portal-worker ./cmd/portal-worker
```

Expected: command exits successfully.

- [ ] **Step 5: Scan docs and plan for placeholders**

Run:

```bash
rg -n "T(O)DO|T(B)D|F(I)XME|\\?\\?" docs/specs/2026-06-20-trial-credit-activation-overview-slice-design.md docs/plans/2026-06-20-trial-credit-activation-overview-slice.md docs/product/implementation-status.md
```

Expected: no output.

- [ ] **Step 6: Check worktree diff**

Run:

```bash
git diff -- docs/product/implementation-status.md docs/product/portal-prd.md docs/architecture/portal-architecture.md docs/specs/2026-06-20-trial-credit-activation-overview-slice-design.md docs/plans/2026-06-20-trial-credit-activation-overview-slice.md backend
```

Expected: diff contains only documentation updates and trial-credit/overview implementation changes.

---

## Self-Review

Spec coverage:

- Registration trial grant is covered by Tasks 2 and 3.
- Configurable 10 yuan / 7 day defaults are covered by Task 1.
- Ledger and balance mutation in one transaction is covered by Task 2.
- `trial_granted_at` is covered by Task 2.
- `GET /api/console/overview` is covered by Tasks 4 and 5.
- Activation steps are covered by Tasks 4 and 5.
- Billing role overview access is covered by Task 4.
- Gateway/runtime enforcement, model whitelist enforcement, usage ingestion, automatic expiration, recharge, and frontend work remain outside scope.

Placeholder scan:

- This plan intentionally avoids placeholder directives and names exact files, tests, and commands.

Type consistency:

- `config.TrialCreditConfig` is the public config type.
- `repository.TrialCreditInput` is the repository transaction input type.
- `ConsoleOverviewResponse`, `ActivationOverviewResponse`, and `ActivationStepResponse` are API response types.
- `ActivationStepCompleted` and `ActivationStepPending` are the only step statuses in this slice.
