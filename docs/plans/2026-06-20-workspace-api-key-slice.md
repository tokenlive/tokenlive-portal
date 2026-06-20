# Workspace API Key Slice Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Add the first authenticated Workspace Console backend APIs for current Workspace lookup and API key management.

**Architecture:** Keep the slice inside the existing Go backend. Repository methods own GORM queries, Workspace scoping, transactions, and audit writes; an API service resolves the current user/Workspace and enforces role checks; HTTP handlers expose JSON routes and reuse the existing session cookie auth contract. Gateway runtime synchronization stays out of scope.

**Tech Stack:** Go 1.24, standard `net/http`, GORM, MySQL-compatible schema, existing `internal/security` API key helpers, existing `internal/money` micro-CNY helper, existing request ID and error primitives.

---

## File Structure

Create:

- `backend/internal/api/console.go`
- `backend/internal/api/console_service.go`
- `backend/internal/api/console_test.go`
- `backend/internal/api/console_service_test.go`

Modify:

- `backend/internal/api/error.go`
- `backend/internal/repository/workspace.go`
- `backend/internal/repository/workspace_test.go`
- `backend/internal/repository/apikey.go`
- `backend/internal/repository/apikey_test.go`
- `backend/cmd/portal-api/main.go`
- `backend/cmd/portal-api/main_test.go`

No migration is required. Foundation tables already include `workspaces`, `workspace_members`, `workspace_balances`, `api_keys`, and `audit_logs`.

Git note: this workspace is not currently a git repository, and the user prefers manual Git actions. Do not add commit steps.

---

### Task 1: API Errors And Money Response Helpers

**Files:**

- Modify: `backend/internal/api/error.go`
- Create: `backend/internal/api/console_service.go`
- Test: `backend/internal/api/console_service_test.go`

- [x] **Step 1: Extend API error codes**

Modify `backend/internal/api/error.go` to add API-key validation errors while keeping existing codes unchanged:

```go
const (
	CodeInternalError            ErrorCode = "internal.error"
	CodeInvalidRequest           ErrorCode = "validation.invalid_request"
	CodeUnauthorized             ErrorCode = "auth.unauthorized"
	CodeAuthInvalidEmail         ErrorCode = "auth.invalid_email"
	CodeAuthInvalidCode          ErrorCode = "auth.invalid_code"
	CodeAuthSessionRequired      ErrorCode = "auth.session_required"
	CodeAuthSessionExpired       ErrorCode = "auth.session_expired"
	CodeWorkspaceNotFound        ErrorCode = "workspace.not_found"
	CodeWorkspaceLimit           ErrorCode = "workspace.limit_exceeded"
	CodePermissionDenied         ErrorCode = "workspace.permission_denied"
	CodeAPIKeyNotFound           ErrorCode = "api_key.not_found"
	CodeAPIKeyInvalidState       ErrorCode = "api_key.invalid_state"
	CodeAPIKeyInvalidName        ErrorCode = "api_key.invalid_name"
	CodeAPIKeyInvalidLimit       ErrorCode = "api_key.invalid_limit"
	CodeAPIKeyInvalidExpiration  ErrorCode = "api_key.invalid_expiration"
	CodeModelNotFound            ErrorCode = "model.not_found"
	CodeModelInvalidQuery        ErrorCode = "model.invalid_query"
	CodeBillingDuplicate         ErrorCode = "billing.duplicate_conflict"
	CodeInsufficientBalance      ErrorCode = "billing.insufficient_balance"
)

var (
	ErrInternalError           = AppError{Code: CodeInternalError, Message: "Internal error", HTTPStatus: http.StatusInternalServerError}
	ErrInvalidRequest          = AppError{Code: CodeInvalidRequest, Message: "Invalid request", HTTPStatus: http.StatusBadRequest}
	ErrUnauthorized            = AppError{Code: CodeUnauthorized, Message: "Unauthorized", HTTPStatus: http.StatusUnauthorized}
	ErrInvalidEmail            = AppError{Code: CodeAuthInvalidEmail, Message: "Invalid email", HTTPStatus: http.StatusBadRequest}
	ErrInvalidCode             = AppError{Code: CodeAuthInvalidCode, Message: "Invalid code", HTTPStatus: http.StatusBadRequest}
	ErrSessionRequired         = AppError{Code: CodeAuthSessionRequired, Message: "Session required", HTTPStatus: http.StatusUnauthorized}
	ErrSessionExpired          = AppError{Code: CodeAuthSessionExpired, Message: "Session expired", HTTPStatus: http.StatusUnauthorized}
	ErrWorkspaceNotFound       = AppError{Code: CodeWorkspaceNotFound, Message: "Workspace not found", HTTPStatus: http.StatusNotFound}
	ErrWorkspaceLimit          = AppError{Code: CodeWorkspaceLimit, Message: "Workspace limit exceeded", HTTPStatus: http.StatusConflict}
	ErrPermissionDenied        = AppError{Code: CodePermissionDenied, Message: "Permission denied", HTTPStatus: http.StatusForbidden}
	ErrAPIKeyNotFound          = AppError{Code: CodeAPIKeyNotFound, Message: "API key not found", HTTPStatus: http.StatusNotFound}
	ErrAPIKeyInvalidState      = AppError{Code: CodeAPIKeyInvalidState, Message: "API key invalid state", HTTPStatus: http.StatusConflict}
	ErrAPIKeyInvalidName       = AppError{Code: CodeAPIKeyInvalidName, Message: "Invalid API key name", HTTPStatus: http.StatusBadRequest}
	ErrAPIKeyInvalidLimit      = AppError{Code: CodeAPIKeyInvalidLimit, Message: "Invalid API key limit", HTTPStatus: http.StatusBadRequest}
	ErrAPIKeyInvalidExpiration = AppError{Code: CodeAPIKeyInvalidExpiration, Message: "Invalid API key expiration", HTTPStatus: http.StatusBadRequest}
	ErrModelNotFound           = AppError{Code: CodeModelNotFound, Message: "Model not found", HTTPStatus: http.StatusNotFound}
	ErrModelInvalidQuery       = AppError{Code: CodeModelInvalidQuery, Message: "Invalid model query", HTTPStatus: http.StatusBadRequest}
	ErrBillingDuplicate        = AppError{Code: CodeBillingDuplicate, Message: "Duplicate billing event conflict", HTTPStatus: http.StatusConflict}
	ErrInsufficientBalance     = AppError{Code: CodeInsufficientBalance, Message: "Insufficient balance", HTTPStatus: http.StatusPaymentRequired}
)
```

- [x] **Step 2: Add response DTO and money helper tests**

Create `backend/internal/api/console_service_test.go` with tests for money formatting and validation helpers:

```go
package api

import (
	"errors"
	"testing"
	"time"

	"github.com/tokenlive/tokenlive-portal/backend/internal/domain"
)

func TestMoneyAmountFromPointerFormatsCNY(t *testing.T) {
	t.Parallel()

	value := int64(1_230_000)
	got := moneyAmountFromPointer(&value)

	if got.MicroCNY == nil || *got.MicroCNY != value {
		t.Fatalf("micro cny = %v, want %d", got.MicroCNY, value)
	}
	if got.CNY == nil || *got.CNY != "1.230000" {
		t.Fatalf("cny = %v, want 1.230000", got.CNY)
	}
}

func TestMoneyAmountFromPointerKeepsNil(t *testing.T) {
	t.Parallel()

	got := moneyAmountFromPointer(nil)

	if got.MicroCNY != nil || got.CNY != nil {
		t.Fatalf("got %+v, want nil amount")
	}
}

func TestValidateCreateAPIKeyInputRejectsBadValues(t *testing.T) {
	t.Parallel()

	service := &consoleService{}
	negative := int64(-1)
	past := time.Now().UTC().Add(-time.Minute)

	tests := []struct {
		name  string
		input CreateAPIKeyRequest
		want  error
	}{
		{name: "blank name", input: CreateAPIKeyRequest{Name: "   "}, want: ErrConsoleAPIKeyInvalidName},
		{name: "long name", input: CreateAPIKeyRequest{Name: string(make([]byte, 161))}, want: ErrConsoleAPIKeyInvalidName},
		{name: "negative daily limit", input: CreateAPIKeyRequest{Name: "dev", DailyLimitMicroCNY: &negative}, want: ErrConsoleAPIKeyInvalidLimit},
		{name: "negative monthly limit", input: CreateAPIKeyRequest{Name: "dev", MonthlyLimitMicroCNY: &negative}, want: ErrConsoleAPIKeyInvalidLimit},
		{name: "past expiration", input: CreateAPIKeyRequest{Name: "dev", ExpiresAt: &past}, want: ErrConsoleAPIKeyInvalidExpiration},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.validateCreateAPIKeyInput(tt.input, time.Now().UTC())
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestCanManageAPIKeys(t *testing.T) {
	t.Parallel()

	if !canManageAPIKeys(domain.MemberRoleOwner) {
		t.Fatalf("owner should manage api keys")
	}
	if !canManageAPIKeys(domain.MemberRoleDeveloper) {
		t.Fatalf("developer should manage api keys")
	}
	if canManageAPIKeys(domain.MemberRoleBilling) {
		t.Fatalf("billing should not manage api keys")
	}
}
```

- [x] **Step 3: Add DTO shells and helper implementation**

Create `backend/internal/api/console_service.go` with DTOs and helper methods:

```go
package api

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/tokenlive/tokenlive-portal/backend/internal/domain"
	"github.com/tokenlive/tokenlive-portal/backend/internal/money"
	"github.com/tokenlive/tokenlive-portal/backend/internal/repository"
)

const maxAPIKeyNameLength = 160

var (
	ErrConsoleWorkspaceNotFound       = errors.New("console workspace not found")
	ErrConsolePermissionDenied        = errors.New("console permission denied")
	ErrConsoleAPIKeyNotFound          = errors.New("console api key not found")
	ErrConsoleAPIKeyInvalidState      = errors.New("console api key invalid state")
	ErrConsoleAPIKeyInvalidName       = errors.New("console api key invalid name")
	ErrConsoleAPIKeyInvalidLimit      = errors.New("console api key invalid limit")
	ErrConsoleAPIKeyInvalidExpiration = errors.New("console api key invalid expiration")
)

type ConsoleService interface {
	CurrentWorkspace(ctx context.Context, user CurrentUser) (CurrentWorkspaceResponse, error)
	ListAPIKeys(ctx context.Context, user CurrentUser) (ListAPIKeysResponse, error)
	CreateAPIKey(ctx context.Context, user CurrentUser, input CreateAPIKeyRequest) (CreateAPIKeyResponse, error)
	EnableAPIKey(ctx context.Context, user CurrentUser, apiKeyID string) (APIKeyResponse, error)
	DisableAPIKey(ctx context.Context, user CurrentUser, apiKeyID string) (APIKeyResponse, error)
	RevokeAPIKey(ctx context.Context, user CurrentUser, apiKeyID string) (APIKeyResponse, error)
}

type MoneyAmount struct {
	MicroCNY *int64  `json:"micro_cny,omitempty"`
	CNY      *string `json:"cny,omitempty"`
}

type WorkspaceBalanceResponse struct {
	AvailableMicroCNY int64  `json:"available_micro_cny"`
	FrozenMicroCNY    int64  `json:"frozen_micro_cny"`
	AvailableCNY      string `json:"available_cny"`
	FrozenCNY         string `json:"frozen_cny"`
}

type CurrentWorkspaceResponse struct {
	Workspace WorkspaceResponse `json:"workspace"`
}

type WorkspaceResponse struct {
	ID      string                   `json:"id"`
	Name    string                   `json:"name"`
	Slug    string                   `json:"slug"`
	Role    domain.MemberRole        `json:"role"`
	Status  domain.WorkspaceStatus   `json:"status"`
	Balance WorkspaceBalanceResponse `json:"balance"`
}

type APIKeyResponse struct {
	ID                   string              `json:"id"`
	Name                 string              `json:"name"`
	KeyPrefix            string              `json:"key_prefix"`
	SecretLast4          string              `json:"secret_last4"`
	Status               domain.APIKeyStatus `json:"status"`
	ExpiresAt            *time.Time          `json:"expires_at"`
	DailyLimitMicroCNY   *int64              `json:"daily_limit_micro_cny"`
	DailyLimitCNY        *string             `json:"daily_limit_cny"`
	MonthlyLimitMicroCNY *int64              `json:"monthly_limit_micro_cny"`
	MonthlyLimitCNY      *string             `json:"monthly_limit_cny"`
	LastUsedAt           *time.Time          `json:"last_used_at"`
	TotalSpendMicroCNY   int64               `json:"total_spend_micro_cny"`
	TotalSpendCNY        string              `json:"total_spend_cny"`
	CreatedAt            time.Time           `json:"created_at"`
	UpdatedAt            time.Time           `json:"updated_at"`
}

type ListAPIKeysResponse struct {
	Data []APIKeyResponse `json:"data"`
}

type CreateAPIKeyRequest struct {
	Name                 string     `json:"name"`
	DailyLimitMicroCNY   *int64     `json:"daily_limit_micro_cny"`
	MonthlyLimitMicroCNY *int64     `json:"monthly_limit_micro_cny"`
	ExpiresAt            *time.Time `json:"expires_at"`
}

type CreateAPIKeyResponse struct {
	APIKey APIKeyResponse `json:"api_key"`
	Secret string         `json:"secret"`
}

type consoleStore interface {
	ResolveCurrentWorkspace(ctx context.Context, userID string) (repository.CurrentWorkspaceResult, error)
	ListAPIKeysByWorkspace(ctx context.Context, workspaceID string) ([]domain.APIKey, error)
	CreateAPIKeyWithAudit(ctx context.Context, input repository.CreateAPIKeyWithAuditInput) (repository.CreateAPIKeyResult, error)
	UpdateAPIKeyStatusWithAudit(ctx context.Context, input repository.UpdateAPIKeyStatusWithAuditInput) (domain.APIKey, error)
}

type consoleService struct {
	store      consoleStore
	authPepper string
	nowFunc    func() time.Time
}

func newConsoleService(store consoleStore, authPepper string) (*consoleService, error) {
	if store == nil {
		return nil, errors.New("console store is required")
	}
	if strings.TrimSpace(authPepper) == "" {
		return nil, errors.New("auth pepper must not be empty")
	}
	return &consoleService{
		store:      store,
		authPepper: authPepper,
		nowFunc:    func() time.Time { return time.Now().UTC() },
	}, nil
}

func moneyAmountFromPointer(value *int64) MoneyAmount {
	if value == nil {
		return MoneyAmount{}
	}
	formatted := money.MicroCNY(*value).FormatCNY()
	return MoneyAmount{
		MicroCNY: value,
		CNY:      &formatted,
	}
}

func canManageAPIKeys(role domain.MemberRole) bool {
	return role == domain.MemberRoleOwner || role == domain.MemberRoleDeveloper
}

func (s *consoleService) validateCreateAPIKeyInput(input CreateAPIKeyRequest, now time.Time) (repository.CreateAPIKeyInput, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" || len(name) > maxAPIKeyNameLength {
		return repository.CreateAPIKeyInput{}, ErrConsoleAPIKeyInvalidName
	}
	if input.DailyLimitMicroCNY != nil && *input.DailyLimitMicroCNY < 0 {
		return repository.CreateAPIKeyInput{}, ErrConsoleAPIKeyInvalidLimit
	}
	if input.MonthlyLimitMicroCNY != nil && *input.MonthlyLimitMicroCNY < 0 {
		return repository.CreateAPIKeyInput{}, ErrConsoleAPIKeyInvalidLimit
	}
	if input.ExpiresAt != nil && !input.ExpiresAt.After(now) {
		return repository.CreateAPIKeyInput{}, ErrConsoleAPIKeyInvalidExpiration
	}

	return repository.CreateAPIKeyInput{
		Name:                 name,
		DailyLimitMicroCNY:   input.DailyLimitMicroCNY,
		MonthlyLimitMicroCNY: input.MonthlyLimitMicroCNY,
		ExpiresAt:            input.ExpiresAt,
	}, nil
}
```

- [x] **Step 4: Run focused tests and observe expected failures**

Run:

```bash
cd backend
GOCACHE=/private/tmp/go-build-cache go test ./internal/api -run 'TestMoneyAmountFromPointer|TestValidateCreateAPIKeyInput|TestCanManageAPIKeys'
```

Expected before all implementation is complete: compile may fail because repository input fields do not yet include `ExpiresAt`. Continue to Task 2.

---

### Task 2: Repository Workspace Resolution And Scoped API Key Mutations

**Files:**

- Modify: `backend/internal/repository/workspace.go`
- Modify: `backend/internal/repository/workspace_test.go`
- Modify: `backend/internal/repository/apikey.go`
- Modify: `backend/internal/repository/apikey_test.go`

- [x] **Step 1: Add repository tests for Workspace resolution**

Append to `backend/internal/repository/workspace_test.go`:

```go
func TestResolveCurrentWorkspacePrefersOwnedWorkspace(t *testing.T) {
	db := testDB(t)
	repos := New(db)
	ctx := context.Background()
	suffix := uniqueSuffix(t)

	email := "workspace-owner-" + suffix + "@example.com"
	userResult, err := repos.CreateUserWithDefaultWorkspace(ctx, CreateUserWithWorkspaceInput{
		DisplayName:   "Owner",
		PrimaryEmail:  &email,
		WorkspaceName: "Owned Workspace",
		WorkspaceSlug: "owned-" + suffix,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	otherEmail := "workspace-other-" + suffix + "@example.com"
	otherResult, err := repos.CreateUserWithDefaultWorkspace(ctx, CreateUserWithWorkspaceInput{
		DisplayName:   "Other",
		PrimaryEmail:  &otherEmail,
		WorkspaceName: "Other Workspace",
		WorkspaceSlug: "other-" + suffix,
	})
	if err != nil {
		t.Fatalf("create other user: %v", err)
	}
	member := domain.WorkspaceMember{
		WorkspaceID: otherResult.Workspace.ID,
		UserID:      userResult.User.ID,
		Role:        domain.MemberRoleDeveloper,
		Status:      domain.MemberStatusActive,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := db.Create(&member).Error; err != nil {
		t.Fatalf("create extra membership: %v", err)
	}

	current, err := repos.ResolveCurrentWorkspace(ctx, userResult.User.ID)
	if err != nil {
		t.Fatalf("resolve current workspace: %v", err)
	}
	if current.Workspace.ID != userResult.Workspace.ID {
		t.Fatalf("workspace id = %s, want owned %s", current.Workspace.ID, userResult.Workspace.ID)
	}
	if current.Role != domain.MemberRoleOwner {
		t.Fatalf("role = %s, want owner", current.Role)
	}
}

func TestResolveCurrentWorkspaceFallsBackToOldestMembership(t *testing.T) {
	db := testDB(t)
	repos := New(db)
	ctx := context.Background()
	suffix := uniqueSuffix(t)

	email := "member-only-" + suffix + "@example.com"
	userResult, err := repos.CreateUserWithDefaultWorkspace(ctx, CreateUserWithWorkspaceInput{
		DisplayName:   "Member",
		PrimaryEmail:  &email,
		WorkspaceName: "Member Owned",
		WorkspaceSlug: "member-owned-" + suffix,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.Model(&domain.Workspace{}).Where("id = ?", userResult.Workspace.ID).Update("status", domain.WorkspaceStatusDeleting).Error; err != nil {
		t.Fatalf("deactivate owned workspace: %v", err)
	}

	hostEmail := "host-" + suffix + "@example.com"
	hostResult, err := repos.CreateUserWithDefaultWorkspace(ctx, CreateUserWithWorkspaceInput{
		DisplayName:   "Host",
		PrimaryEmail:  &hostEmail,
		WorkspaceName: "Host Workspace",
		WorkspaceSlug: "host-" + suffix,
	})
	if err != nil {
		t.Fatalf("create host: %v", err)
	}
	joinedAt := time.Now().UTC().Add(-time.Hour)
	member := domain.WorkspaceMember{
		WorkspaceID: hostResult.Workspace.ID,
		UserID:      userResult.User.ID,
		Role:        domain.MemberRoleBilling,
		Status:      domain.MemberStatusActive,
		JoinedAt:    &joinedAt,
		CreatedAt:   joinedAt,
		UpdatedAt:   joinedAt,
	}
	if err := db.Create(&member).Error; err != nil {
		t.Fatalf("create fallback membership: %v", err)
	}

	current, err := repos.ResolveCurrentWorkspace(ctx, userResult.User.ID)
	if err != nil {
		t.Fatalf("resolve current workspace: %v", err)
	}
	if current.Workspace.ID != hostResult.Workspace.ID {
		t.Fatalf("workspace id = %s, want fallback %s", current.Workspace.ID, hostResult.Workspace.ID)
	}
	if current.Role != domain.MemberRoleBilling {
		t.Fatalf("role = %s, want billing", current.Role)
	}
}
```

- [x] **Step 2: Implement current Workspace repository method**

Add to `backend/internal/repository/workspace.go`:

```go
var ErrWorkspaceNotFound = errors.New("workspace not found")

type CurrentWorkspaceResult struct {
	Workspace domain.Workspace
	Member    domain.WorkspaceMember
	Balance   domain.WorkspaceBalance
	Role      domain.MemberRole
}

func (r *Repositories) ResolveCurrentWorkspace(ctx context.Context, userID string) (CurrentWorkspaceResult, error) {
	var result CurrentWorkspaceResult
	err := r.db.WithContext(ctx).
		Table("workspaces").
		Select("workspaces.*").
		Joins("JOIN workspace_members ON workspace_members.workspace_id = workspaces.id").
		Where("workspace_members.user_id = ?", userID).
		Where("workspace_members.status = ?", domain.MemberStatusActive).
		Where("workspaces.status = ?", domain.WorkspaceStatusActive).
		Where("workspaces.deleted_at IS NULL").
		Order("CASE WHEN workspaces.owner_user_id = ? THEN 0 ELSE 1 END", userID).
		Order("COALESCE(workspace_members.joined_at, workspace_members.created_at) ASC").
		Order("workspaces.created_at ASC").
		First(&result.Workspace).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return CurrentWorkspaceResult{}, ErrWorkspaceNotFound
		}
		return CurrentWorkspaceResult{}, fmt.Errorf("resolve current workspace: %w", err)
	}

	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND user_id = ? AND status = ?", result.Workspace.ID, userID, domain.MemberStatusActive).
		First(&result.Member).Error; err != nil {
		return CurrentWorkspaceResult{}, fmt.Errorf("resolve current workspace member: %w", err)
	}
	result.Role = result.Member.Role

	if err := r.db.WithContext(ctx).
		Where("workspace_id = ?", result.Workspace.ID).
		First(&result.Balance).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			result.Balance = domain.WorkspaceBalance{WorkspaceID: result.Workspace.ID}
		} else {
			return CurrentWorkspaceResult{}, fmt.Errorf("resolve current workspace balance: %w", err)
		}
	}

	return result, nil
}
```

- [x] **Step 3: Extend API key repository types and tests**

Modify `backend/internal/repository/apikey.go` types:

```go
type CreateAPIKeyInput struct {
	WorkspaceID          string
	Name                 string
	PlaintextKey         string
	Pepper               string
	CreatedByUserID      string
	DailyLimitMicroCNY   *int64
	MonthlyLimitMicroCNY *int64
	ExpiresAt            *time.Time
}

type CreateAPIKeyWithAuditInput struct {
	CreateAPIKeyInput
	ActorUserID string
	IP          string
	UserAgent   string
}

type UpdateAPIKeyStatusWithAuditInput struct {
	WorkspaceID string
	APIKeyID    string
	Status      domain.APIKeyStatus
	ActorUserID string
	IP          string
	UserAgent   string
}
```

Append tests to `backend/internal/repository/apikey_test.go`:

```go
func TestListAPIKeysByWorkspaceScopesResults(t *testing.T) {
	db := testDB(t)
	repos := New(db)
	ctx := context.Background()
	suffix := uniqueSuffix(t)
	workspaceA := createTestWorkspaceWithUser(t, repos, "api-key-list-a-"+suffix)
	workspaceB := createTestWorkspaceWithUser(t, repos, "api-key-list-b-"+suffix)

	if _, err := repos.CreateAPIKey(ctx, CreateAPIKeyInput{
		WorkspaceID:     workspaceA.Workspace.ID,
		Name:            "A",
		PlaintextKey:    "tl_live_a_" + suffix,
		Pepper:          "pepper",
		CreatedByUserID: workspaceA.User.ID,
	}); err != nil {
		t.Fatalf("create key a: %v", err)
	}
	if _, err := repos.CreateAPIKey(ctx, CreateAPIKeyInput{
		WorkspaceID:     workspaceB.Workspace.ID,
		Name:            "B",
		PlaintextKey:    "tl_live_b_" + suffix,
		Pepper:          "pepper",
		CreatedByUserID: workspaceB.User.ID,
	}); err != nil {
		t.Fatalf("create key b: %v", err)
	}

	keys, err := repos.ListAPIKeysByWorkspace(ctx, workspaceA.Workspace.ID)
	if err != nil {
		t.Fatalf("list keys: %v", err)
	}
	if len(keys) != 1 || keys[0].Name != "A" {
		t.Fatalf("keys = %+v, want only workspace A key", keys)
	}
}

func TestUpdateAPIKeyStatusWithAuditScopesWorkspaceAndBlocksRevokedRestore(t *testing.T) {
	db := testDB(t)
	repos := New(db)
	ctx := context.Background()
	suffix := uniqueSuffix(t)
	workspaceA := createTestWorkspaceWithUser(t, repos, "api-key-status-a-"+suffix)
	workspaceB := createTestWorkspaceWithUser(t, repos, "api-key-status-b-"+suffix)

	created, err := repos.CreateAPIKey(ctx, CreateAPIKeyInput{
		WorkspaceID:     workspaceA.Workspace.ID,
		Name:            "A",
		PlaintextKey:    "tl_live_status_" + suffix,
		Pepper:          "pepper",
		CreatedByUserID: workspaceA.User.ID,
	})
	if err != nil {
		t.Fatalf("create key: %v", err)
	}

	_, err = repos.UpdateAPIKeyStatusWithAudit(ctx, UpdateAPIKeyStatusWithAuditInput{
		WorkspaceID: workspaceB.Workspace.ID,
		APIKeyID:    created.APIKey.ID,
		Status:      domain.APIKeyStatusDisabled,
		ActorUserID: workspaceB.User.ID,
	})
	if !errors.Is(err, ErrAPIKeyNotFound) {
		t.Fatalf("cross workspace err = %v, want ErrAPIKeyNotFound", err)
	}

	revoked, err := repos.UpdateAPIKeyStatusWithAudit(ctx, UpdateAPIKeyStatusWithAuditInput{
		WorkspaceID: workspaceA.Workspace.ID,
		APIKeyID:    created.APIKey.ID,
		Status:      domain.APIKeyStatusRevoked,
		ActorUserID: workspaceA.User.ID,
	})
	if err != nil {
		t.Fatalf("revoke key: %v", err)
	}
	if revoked.Status != domain.APIKeyStatusRevoked || revoked.RevokedAt == nil {
		t.Fatalf("revoked key = %+v", revoked)
	}

	_, err = repos.UpdateAPIKeyStatusWithAudit(ctx, UpdateAPIKeyStatusWithAuditInput{
		WorkspaceID: workspaceA.Workspace.ID,
		APIKeyID:    created.APIKey.ID,
		Status:      domain.APIKeyStatusEnabled,
		ActorUserID: workspaceA.User.ID,
	})
	if !errors.Is(err, ErrAPIKeyInvalidState) {
		t.Fatalf("restore revoked err = %v, want ErrAPIKeyInvalidState", err)
	}
}

func TestCreateAPIKeyWithAuditWritesAuditLog(t *testing.T) {
	db := testDB(t)
	repos := New(db)
	ctx := context.Background()
	suffix := uniqueSuffix(t)
	workspace := createTestWorkspaceWithUser(t, repos, "api-key-audit-"+suffix)

	created, err := repos.CreateAPIKeyWithAudit(ctx, CreateAPIKeyWithAuditInput{
		CreateAPIKeyInput: CreateAPIKeyInput{
			WorkspaceID:     workspace.Workspace.ID,
			Name:            "Audited",
			PlaintextKey:    "tl_live_audit_" + suffix,
			Pepper:          "pepper",
			CreatedByUserID: workspace.User.ID,
		},
		ActorUserID: workspace.User.ID,
	})
	if err != nil {
		t.Fatalf("create key with audit: %v", err)
	}

	var count int64
	if err := db.Model(&domain.AuditLog{}).
		Where("workspace_id = ? AND actor_user_id = ? AND resource_type = ? AND resource_id = ? AND action = ?",
			workspace.Workspace.ID,
			workspace.User.ID,
			"api_key",
			created.APIKey.ID,
			"api_key.create",
		).
		Count(&count).Error; err != nil {
		t.Fatalf("count audit logs: %v", err)
	}
	if count != 1 {
		t.Fatalf("audit count = %d, want 1", count)
	}
}
```

- [x] **Step 4: Implement scoped API key repository methods**

Modify `backend/internal/repository/apikey.go`:

```go
var (
	ErrAPIKeyNotFound     = errors.New("api key not found")
	ErrAPIKeyInvalidState = errors.New("api key invalid state")
)

func (r *Repositories) CreateAPIKey(ctx context.Context, input CreateAPIKeyInput) (CreateAPIKeyResult, error) {
	// Existing body, plus:
	apiKey.ExpiresAt = input.ExpiresAt
}

func (r *Repositories) ListAPIKeysByWorkspace(ctx context.Context, workspaceID string) ([]domain.APIKey, error) {
	var keys []domain.APIKey
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Order("created_at DESC").
		Order("id DESC").
		Find(&keys).Error; err != nil {
		return nil, fmt.Errorf("list api keys by workspace: %w", err)
	}
	return keys, nil
}

func (r *Repositories) CreateAPIKeyWithAudit(ctx context.Context, input CreateAPIKeyWithAuditInput) (CreateAPIKeyResult, error) {
	var result CreateAPIKeyResult
	err := r.withTx(ctx, func(tx *gorm.DB) error {
		txRepos := New(tx)
		created, err := txRepos.CreateAPIKey(ctx, input.CreateAPIKeyInput)
		if err != nil {
			return err
		}
		result = created
		workspaceID := input.WorkspaceID
		actorUserID := input.ActorUserID
		if _, err := txRepos.AppendAuditLog(ctx, AppendAuditInput{
			WorkspaceID:  &workspaceID,
			ActorUserID:  &actorUserID,
			Action:       "api_key.create",
			ResourceType: "api_key",
			ResourceID:   created.APIKey.ID,
			IP:           input.IP,
			UserAgent:    input.UserAgent,
		}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return CreateAPIKeyResult{}, err
	}
	return result, nil
}

func (r *Repositories) UpdateAPIKeyStatusWithAudit(ctx context.Context, input UpdateAPIKeyStatusWithAuditInput) (domain.APIKey, error) {
	var updated domain.APIKey
	err := r.withTx(ctx, func(tx *gorm.DB) error {
		var existing domain.APIKey
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND workspace_id = ?", input.APIKeyID, input.WorkspaceID).
			First(&existing).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAPIKeyNotFound
			}
			return fmt.Errorf("lock api key: %w", err)
		}
		if existing.Status == domain.APIKeyStatusRevoked && input.Status != domain.APIKeyStatusRevoked {
			return ErrAPIKeyInvalidState
		}

		now := time.Now().UTC()
		updates := map[string]any{
			"status":     input.Status,
			"updated_at": now,
		}
		if input.Status == domain.APIKeyStatusRevoked && existing.RevokedAt == nil {
			updates["revoked_at"] = &now
		}
		if err := tx.Model(&domain.APIKey{}).
			Where("id = ? AND workspace_id = ?", input.APIKeyID, input.WorkspaceID).
			Updates(updates).Error; err != nil {
			return fmt.Errorf("update api key status: %w", err)
		}
		if err := tx.Where("id = ? AND workspace_id = ?", input.APIKeyID, input.WorkspaceID).First(&updated).Error; err != nil {
			return fmt.Errorf("reload api key status: %w", err)
		}

		action := "api_key." + string(input.Status)
		workspaceID := input.WorkspaceID
		actorUserID := input.ActorUserID
		txRepos := New(tx)
		if _, err := txRepos.AppendAuditLog(ctx, AppendAuditInput{
			WorkspaceID:  &workspaceID,
			ActorUserID:  &actorUserID,
			Action:       action,
			ResourceType: "api_key",
			ResourceID:   input.APIKeyID,
			IP:           input.IP,
			UserAgent:    input.UserAgent,
		}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return domain.APIKey{}, err
	}
	return updated, nil
}
```

Also import `gorm.io/gorm/clause`.

- [x] **Step 5: Add test helper if missing**

If `backend/internal/repository/apikey_test.go` does not already have a compact workspace fixture, add:

```go
type testWorkspaceWithUser struct {
	User      domain.User
	Workspace domain.Workspace
}

func createTestWorkspaceWithUser(t *testing.T, repos *Repositories, slug string) testWorkspaceWithUser {
	t.Helper()
	email := slug + "@example.com"
	result, err := repos.CreateUserWithDefaultWorkspace(context.Background(), CreateUserWithWorkspaceInput{
		DisplayName:   slug,
		PrimaryEmail:  &email,
		WorkspaceName: slug,
		WorkspaceSlug: slug,
	})
	if err != nil {
		t.Fatalf("create user with workspace: %v", err)
	}
	return testWorkspaceWithUser{
		User:      result.User,
		Workspace: result.Workspace,
	}
}
```

- [x] **Step 6: Run repository tests**

Run:

```bash
cd backend
GOCACHE=/private/tmp/go-build-cache go test ./internal/repository -count=1
```

Expected: PASS when `PORTAL_TEST_DATABASE_DSN` is set; otherwise DB integration tests skip cleanly and package still passes.

---

### Task 3: Console Service

**Files:**

- Modify: `backend/internal/api/console_service.go`
- Modify: `backend/internal/api/console_service_test.go`

- [x] **Step 1: Add service tests**

Append to `backend/internal/api/console_service_test.go`:

```go
func TestConsoleServiceCurrentWorkspaceMapsBalance(t *testing.T) {
	t.Parallel()

	store := &fakeConsoleStore{
		currentWorkspace: repository.CurrentWorkspaceResult{
			Workspace: domain.Workspace{ID: "wsp_1", Name: "Dev", Slug: "dev", Status: domain.WorkspaceStatusActive},
			Role:      domain.MemberRoleOwner,
			Balance: domain.WorkspaceBalance{
				AvailableMicroCNY: 12_340_000,
				FrozenMicroCNY:    500_000,
			},
		},
	}
	service, err := newConsoleService(store, "pepper")
	if err != nil {
		t.Fatalf("new console service: %v", err)
	}

	got, err := service.CurrentWorkspace(context.Background(), CurrentUser{ID: "usr_1"})
	if err != nil {
		t.Fatalf("current workspace: %v", err)
	}

	if got.Workspace.ID != "wsp_1" || got.Workspace.Role != domain.MemberRoleOwner {
		t.Fatalf("workspace = %+v", got.Workspace)
	}
	if got.Workspace.Balance.AvailableCNY != "12.340000" {
		t.Fatalf("available cny = %s", got.Workspace.Balance.AvailableCNY)
	}
}

func TestConsoleServiceBillingCannotListAPIKeys(t *testing.T) {
	t.Parallel()

	store := &fakeConsoleStore{
		currentWorkspace: repository.CurrentWorkspaceResult{
			Workspace: domain.Workspace{ID: "wsp_1", Status: domain.WorkspaceStatusActive},
			Role:      domain.MemberRoleBilling,
		},
	}
	service, err := newConsoleService(store, "pepper")
	if err != nil {
		t.Fatalf("new console service: %v", err)
	}

	_, err = service.ListAPIKeys(context.Background(), CurrentUser{ID: "usr_1"})
	if !errors.Is(err, ErrConsolePermissionDenied) {
		t.Fatalf("err = %v, want ErrConsolePermissionDenied", err)
	}
}

func TestConsoleServiceCreateAPIKeyReturnsSecretOnce(t *testing.T) {
	t.Parallel()

	store := &fakeConsoleStore{
		currentWorkspace: repository.CurrentWorkspaceResult{
			Workspace: domain.Workspace{ID: "wsp_1", Status: domain.WorkspaceStatusActive},
			Role:      domain.MemberRoleDeveloper,
		},
		createAPIKeyResult: repository.CreateAPIKeyResult{
			APIKey: domain.APIKey{
				ID:              "ak_1",
				WorkspaceID:     "wsp_1",
				Name:            "local dev",
				KeyPrefix:       "tl_live_abc",
				SecretLast4:     "wxyz",
				Status:          domain.APIKeyStatusEnabled,
				CreatedByUserID: "usr_1",
				CreatedAt:       time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC),
				UpdatedAt:       time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC),
			},
			Secret: "tl_live_secret",
		},
	}
	service, err := newConsoleService(store, "pepper")
	if err != nil {
		t.Fatalf("new console service: %v", err)
	}

	got, err := service.CreateAPIKey(context.Background(), CurrentUser{ID: "usr_1"}, CreateAPIKeyRequest{Name: " local dev "})
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}

	if got.Secret != "tl_live_secret" {
		t.Fatalf("secret = %q, want creation secret", got.Secret)
	}
	if store.createAPIKeyInput.WorkspaceID != "wsp_1" {
		t.Fatalf("workspace id = %s, want wsp_1", store.createAPIKeyInput.WorkspaceID)
	}
	if store.createAPIKeyInput.Name != "local dev" {
		t.Fatalf("name = %q, want trimmed name", store.createAPIKeyInput.Name)
	}
	if store.createAPIKeyInput.Pepper != "pepper" {
		t.Fatalf("pepper = %q, want pepper", store.createAPIKeyInput.Pepper)
	}
}

func TestConsoleServiceListAPIKeysDoesNotExposeSecret(t *testing.T) {
	t.Parallel()

	store := &fakeConsoleStore{
		currentWorkspace: repository.CurrentWorkspaceResult{
			Workspace: domain.Workspace{ID: "wsp_1", Status: domain.WorkspaceStatusActive},
			Role:      domain.MemberRoleOwner,
		},
		listAPIKeysResult: []domain.APIKey{{
			ID:              "ak_1",
			Name:            "local dev",
			KeyPrefix:       "tl_live_abc",
			SecretLast4:     "wxyz",
			KeyHash:         "hash-should-not-appear",
			Status:          domain.APIKeyStatusEnabled,
			CreatedByUserID: "usr_1",
			CreatedAt:       time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC),
			UpdatedAt:       time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC),
		}},
	}
	service, err := newConsoleService(store, "pepper")
	if err != nil {
		t.Fatalf("new console service: %v", err)
	}

	got, err := service.ListAPIKeys(context.Background(), CurrentUser{ID: "usr_1"})
	if err != nil {
		t.Fatalf("list api keys: %v", err)
	}

	if len(got.Data) != 1 {
		t.Fatalf("data len = %d, want 1", len(got.Data))
	}
	if got.Data[0].SecretLast4 != "wxyz" || got.Data[0].KeyPrefix != "tl_live_abc" {
		t.Fatalf("api key response = %+v", got.Data[0])
	}
}
```

Add the fake store at the end of the file:

```go
type fakeConsoleStore struct {
	currentWorkspace    repository.CurrentWorkspaceResult
	currentWorkspaceErr error

	listAPIKeysResult []domain.APIKey
	listAPIKeysErr    error

	createAPIKeyInput  repository.CreateAPIKeyWithAuditInput
	createAPIKeyResult repository.CreateAPIKeyResult
	createAPIKeyErr    error

	updateAPIKeyInput  repository.UpdateAPIKeyStatusWithAuditInput
	updateAPIKeyResult domain.APIKey
	updateAPIKeyErr    error
}

func (f *fakeConsoleStore) ResolveCurrentWorkspace(_ context.Context, userID string) (repository.CurrentWorkspaceResult, error) {
	if f.currentWorkspaceErr != nil {
		return repository.CurrentWorkspaceResult{}, f.currentWorkspaceErr
	}
	return f.currentWorkspace, nil
}

func (f *fakeConsoleStore) ListAPIKeysByWorkspace(_ context.Context, workspaceID string) ([]domain.APIKey, error) {
	if f.listAPIKeysErr != nil {
		return nil, f.listAPIKeysErr
	}
	return f.listAPIKeysResult, nil
}

func (f *fakeConsoleStore) CreateAPIKeyWithAudit(_ context.Context, input repository.CreateAPIKeyWithAuditInput) (repository.CreateAPIKeyResult, error) {
	f.createAPIKeyInput = input
	if f.createAPIKeyErr != nil {
		return repository.CreateAPIKeyResult{}, f.createAPIKeyErr
	}
	return f.createAPIKeyResult, nil
}

func (f *fakeConsoleStore) UpdateAPIKeyStatusWithAudit(_ context.Context, input repository.UpdateAPIKeyStatusWithAuditInput) (domain.APIKey, error) {
	f.updateAPIKeyInput = input
	if f.updateAPIKeyErr != nil {
		return domain.APIKey{}, f.updateAPIKeyErr
	}
	return f.updateAPIKeyResult, nil
}
```

- [x] **Step 2: Implement service methods**

Add to `backend/internal/api/console_service.go`:

```go
func NewConsoleService(repos *repository.Repositories, authPepper string) (ConsoleService, error) {
	if repos == nil {
		return nil, errors.New("console repositories are required")
	}
	return newConsoleService(repos, authPepper)
}

func (s *consoleService) CurrentWorkspace(ctx context.Context, user CurrentUser) (CurrentWorkspaceResponse, error) {
	current, err := s.resolveWorkspace(ctx, user)
	if err != nil {
		return CurrentWorkspaceResponse{}, err
	}
	return CurrentWorkspaceResponse{Workspace: workspaceResponseFromRepository(current)}, nil
}

func (s *consoleService) ListAPIKeys(ctx context.Context, user CurrentUser) (ListAPIKeysResponse, error) {
	current, err := s.resolveManageableWorkspace(ctx, user)
	if err != nil {
		return ListAPIKeysResponse{}, err
	}
	keys, err := s.store.ListAPIKeysByWorkspace(ctx, current.Workspace.ID)
	if err != nil {
		return ListAPIKeysResponse{}, err
	}
	resp := ListAPIKeysResponse{Data: make([]APIKeyResponse, 0, len(keys))}
	for _, key := range keys {
		resp.Data = append(resp.Data, apiKeyResponseFromDomain(key))
	}
	return resp, nil
}

func (s *consoleService) CreateAPIKey(ctx context.Context, user CurrentUser, input CreateAPIKeyRequest) (CreateAPIKeyResponse, error) {
	current, err := s.resolveManageableWorkspace(ctx, user)
	if err != nil {
		return CreateAPIKeyResponse{}, err
	}
	createInput, err := s.validateCreateAPIKeyInput(input, s.nowFunc())
	if err != nil {
		return CreateAPIKeyResponse{}, err
	}
	createInput.WorkspaceID = current.Workspace.ID
	createInput.CreatedByUserID = user.ID
	createInput.Pepper = s.authPepper

	created, err := s.store.CreateAPIKeyWithAudit(ctx, repository.CreateAPIKeyWithAuditInput{
		CreateAPIKeyInput: createInput,
		ActorUserID:       user.ID,
	})
	if err != nil {
		return CreateAPIKeyResponse{}, err
	}
	return CreateAPIKeyResponse{
		APIKey: apiKeyResponseFromDomain(created.APIKey),
		Secret: created.Secret,
	}, nil
}

func (s *consoleService) EnableAPIKey(ctx context.Context, user CurrentUser, apiKeyID string) (APIKeyResponse, error) {
	return s.updateAPIKeyStatus(ctx, user, apiKeyID, domain.APIKeyStatusEnabled)
}

func (s *consoleService) DisableAPIKey(ctx context.Context, user CurrentUser, apiKeyID string) (APIKeyResponse, error) {
	return s.updateAPIKeyStatus(ctx, user, apiKeyID, domain.APIKeyStatusDisabled)
}

func (s *consoleService) RevokeAPIKey(ctx context.Context, user CurrentUser, apiKeyID string) (APIKeyResponse, error) {
	return s.updateAPIKeyStatus(ctx, user, apiKeyID, domain.APIKeyStatusRevoked)
}

func (s *consoleService) updateAPIKeyStatus(ctx context.Context, user CurrentUser, apiKeyID string, status domain.APIKeyStatus) (APIKeyResponse, error) {
	current, err := s.resolveManageableWorkspace(ctx, user)
	if err != nil {
		return APIKeyResponse{}, err
	}
	apiKeyID = strings.TrimSpace(apiKeyID)
	if apiKeyID == "" {
		return APIKeyResponse{}, ErrConsoleAPIKeyNotFound
	}
	updated, err := s.store.UpdateAPIKeyStatusWithAudit(ctx, repository.UpdateAPIKeyStatusWithAuditInput{
		WorkspaceID: current.Workspace.ID,
		APIKeyID:    apiKeyID,
		Status:      status,
		ActorUserID: user.ID,
	})
	if err != nil {
		return APIKeyResponse{}, mapConsoleRepositoryError(err)
	}
	return apiKeyResponseFromDomain(updated), nil
}

func (s *consoleService) resolveWorkspace(ctx context.Context, user CurrentUser) (repository.CurrentWorkspaceResult, error) {
	if strings.TrimSpace(user.ID) == "" {
		return repository.CurrentWorkspaceResult{}, ErrAuthUnauthorized
	}
	current, err := s.store.ResolveCurrentWorkspace(ctx, user.ID)
	if err != nil {
		return repository.CurrentWorkspaceResult{}, mapConsoleRepositoryError(err)
	}
	return current, nil
}

func (s *consoleService) resolveManageableWorkspace(ctx context.Context, user CurrentUser) (repository.CurrentWorkspaceResult, error) {
	current, err := s.resolveWorkspace(ctx, user)
	if err != nil {
		return repository.CurrentWorkspaceResult{}, err
	}
	if !canManageAPIKeys(current.Role) {
		return repository.CurrentWorkspaceResult{}, ErrConsolePermissionDenied
	}
	return current, nil
}

func mapConsoleRepositoryError(err error) error {
	switch {
	case errors.Is(err, repository.ErrWorkspaceNotFound):
		return ErrConsoleWorkspaceNotFound
	case errors.Is(err, repository.ErrAPIKeyNotFound):
		return ErrConsoleAPIKeyNotFound
	case errors.Is(err, repository.ErrAPIKeyInvalidState):
		return ErrConsoleAPIKeyInvalidState
	default:
		return err
	}
}

func workspaceResponseFromRepository(current repository.CurrentWorkspaceResult) WorkspaceResponse {
	return WorkspaceResponse{
		ID:     current.Workspace.ID,
		Name:   current.Workspace.Name,
		Slug:   current.Workspace.Slug,
		Role:   current.Role,
		Status: current.Workspace.Status,
		Balance: WorkspaceBalanceResponse{
			AvailableMicroCNY: current.Balance.AvailableMicroCNY,
			FrozenMicroCNY:    current.Balance.FrozenMicroCNY,
			AvailableCNY:      money.MicroCNY(current.Balance.AvailableMicroCNY).FormatCNY(),
			FrozenCNY:         money.MicroCNY(current.Balance.FrozenMicroCNY).FormatCNY(),
		},
	}
}

func apiKeyResponseFromDomain(key domain.APIKey) APIKeyResponse {
	daily := moneyAmountFromPointer(key.DailyLimitMicroCNY)
	monthly := moneyAmountFromPointer(key.MonthlyLimitMicroCNY)
	return APIKeyResponse{
		ID:                   key.ID,
		Name:                 key.Name,
		KeyPrefix:            key.KeyPrefix,
		SecretLast4:          key.SecretLast4,
		Status:               key.Status,
		ExpiresAt:            key.ExpiresAt,
		DailyLimitMicroCNY:   daily.MicroCNY,
		DailyLimitCNY:        daily.CNY,
		MonthlyLimitMicroCNY: monthly.MicroCNY,
		MonthlyLimitCNY:      monthly.CNY,
		LastUsedAt:           key.LastUsedAt,
		TotalSpendMicroCNY:   key.TotalSpendMicroCNY,
		TotalSpendCNY:        money.MicroCNY(key.TotalSpendMicroCNY).FormatCNY(),
		CreatedAt:            key.CreatedAt,
		UpdatedAt:            key.UpdatedAt,
	}
}
```

- [x] **Step 3: Run service tests**

Run:

```bash
cd backend
GOCACHE=/private/tmp/go-build-cache go test ./internal/api -run 'TestConsoleService|TestMoneyAmountFromPointer|TestValidateCreateAPIKeyInput|TestCanManageAPIKeys'
```

Expected: PASS.

---

### Task 4: HTTP Handlers And Route Registration

**Files:**

- Create: `backend/internal/api/console.go`
- Create: `backend/internal/api/console_test.go`

- [x] **Step 1: Add handler tests**

Create `backend/internal/api/console_test.go`:

```go
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tokenlive/tokenlive-portal/backend/internal/domain"
	"github.com/tokenlive/tokenlive-portal/backend/internal/security"
)

func TestConsoleCurrentWorkspaceRequiresSession(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	RegisterConsoleRoutes(mux, &fakeConsoleService{}, &fakeAuthService{})
	req := httptest.NewRequest(http.MethodGet, "/api/workspaces/current", nil)
	req.Header.Set("X-Request-ID", "req_console_session")
	rec := httptest.NewRecorder()

	RequestID(mux).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	assertAuthErrorResponse(t, rec, string(CodeAuthSessionRequired), "req_console_session")
}

func TestConsoleCurrentWorkspaceReturnsWorkspace(t *testing.T) {
	t.Parallel()

	console := &fakeConsoleService{
		currentWorkspaceResult: CurrentWorkspaceResponse{
			Workspace: WorkspaceResponse{
				ID:     "wsp_1",
				Name:   "Dev",
				Slug:   "dev",
				Role:   domain.MemberRoleOwner,
				Status: domain.WorkspaceStatusActive,
				Balance: WorkspaceBalanceResponse{
					AvailableMicroCNY: 10_000_000,
					AvailableCNY:      "10.000000",
				},
			},
		},
	}
	auth := &fakeAuthService{currentUser: CurrentUser{ID: "usr_1"}}
	mux := http.NewServeMux()
	RegisterConsoleRoutes(mux, console, auth)

	req := httptest.NewRequest(http.MethodGet, "/api/workspaces/current", nil)
	req.AddCookie(&http.Cookie{Name: security.SessionCookieName, Value: "tl_sess_test"})
	rec := httptest.NewRecorder()

	RequestID(mux).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body CurrentWorkspaceResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Workspace.ID != "wsp_1" || body.Workspace.Balance.AvailableCNY != "10.000000" {
		t.Fatalf("body = %+v", body)
	}
}

func TestConsoleListAPIKeysMapsPermissionDenied(t *testing.T) {
	t.Parallel()

	console := &fakeConsoleService{listAPIKeysErr: ErrConsolePermissionDenied}
	auth := &fakeAuthService{currentUser: CurrentUser{ID: "usr_1"}}
	mux := http.NewServeMux()
	RegisterConsoleRoutes(mux, console, auth)

	req := httptest.NewRequest(http.MethodGet, "/api/api-keys", nil)
	req.Header.Set("X-Request-ID", "req_permission")
	req.AddCookie(&http.Cookie{Name: security.SessionCookieName, Value: "tl_sess_test"})
	rec := httptest.NewRecorder()

	RequestID(mux).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	assertAuthErrorResponse(t, rec, string(CodePermissionDenied), "req_permission")
}

func TestConsoleCreateAPIKeyReturnsSecret(t *testing.T) {
	t.Parallel()

	console := &fakeConsoleService{
		createAPIKeyResult: CreateAPIKeyResponse{
			APIKey: APIKeyResponse{ID: "ak_1", Name: "local dev", SecretLast4: "wxyz"},
			Secret: "tl_live_secret",
		},
	}
	auth := &fakeAuthService{currentUser: CurrentUser{ID: "usr_1"}}
	mux := http.NewServeMux()
	RegisterConsoleRoutes(mux, console, auth)

	req := httptest.NewRequest(http.MethodPost, "/api/api-keys", strings.NewReader(`{"name":"local dev"}`))
	req.AddCookie(&http.Cookie{Name: security.SessionCookieName, Value: "tl_sess_test"})
	rec := httptest.NewRecorder()

	RequestID(mux).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body CreateAPIKeyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Secret != "tl_live_secret" {
		t.Fatalf("secret = %q, want creation secret", body.Secret)
	}
	if console.createAPIKeyInput.Name != "local dev" {
		t.Fatalf("input name = %q", console.createAPIKeyInput.Name)
	}
}

func TestConsoleAPIKeyStateRoutes(t *testing.T) {
	t.Parallel()

	routes := []struct {
		name       string
		path       string
		wantStatus domain.APIKeyStatus
	}{
		{name: "enable", path: "/api/api-keys/ak_1/enable", wantStatus: domain.APIKeyStatusEnabled},
		{name: "disable", path: "/api/api-keys/ak_1/disable", wantStatus: domain.APIKeyStatusDisabled},
		{name: "revoke", path: "/api/api-keys/ak_1/revoke", wantStatus: domain.APIKeyStatusRevoked},
	}

	for _, tt := range routes {
		t.Run(tt.name, func(t *testing.T) {
			console := &fakeConsoleService{stateResult: APIKeyResponse{ID: "ak_1", Status: tt.wantStatus}}
			auth := &fakeAuthService{currentUser: CurrentUser{ID: "usr_1"}}
			mux := http.NewServeMux()
			RegisterConsoleRoutes(mux, console, auth)

			req := httptest.NewRequest(http.MethodPost, tt.path, nil)
			req.AddCookie(&http.Cookie{Name: security.SessionCookieName, Value: "tl_sess_test"})
			rec := httptest.NewRecorder()

			RequestID(mux).ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			if console.stateAPIKeyID != "ak_1" || console.stateStatus != tt.wantStatus {
				t.Fatalf("state api key id=%s status=%s", console.stateAPIKeyID, console.stateStatus)
			}
		})
	}
}
```

Add fake console service at the bottom:

```go
type fakeConsoleService struct {
	currentWorkspaceResult CurrentWorkspaceResponse
	currentWorkspaceErr    error
	listAPIKeysResult      ListAPIKeysResponse
	listAPIKeysErr         error
	createAPIKeyInput      CreateAPIKeyRequest
	createAPIKeyResult     CreateAPIKeyResponse
	createAPIKeyErr        error
	stateAPIKeyID          string
	stateStatus            domain.APIKeyStatus
	stateResult            APIKeyResponse
	stateErr               error
}

func (f *fakeConsoleService) CurrentWorkspace(_ context.Context, _ CurrentUser) (CurrentWorkspaceResponse, error) {
	return f.currentWorkspaceResult, f.currentWorkspaceErr
}

func (f *fakeConsoleService) ListAPIKeys(_ context.Context, _ CurrentUser) (ListAPIKeysResponse, error) {
	return f.listAPIKeysResult, f.listAPIKeysErr
}

func (f *fakeConsoleService) CreateAPIKey(_ context.Context, _ CurrentUser, input CreateAPIKeyRequest) (CreateAPIKeyResponse, error) {
	f.createAPIKeyInput = input
	return f.createAPIKeyResult, f.createAPIKeyErr
}

func (f *fakeConsoleService) EnableAPIKey(_ context.Context, _ CurrentUser, apiKeyID string) (APIKeyResponse, error) {
	f.stateAPIKeyID = apiKeyID
	f.stateStatus = domain.APIKeyStatusEnabled
	return f.stateResult, f.stateErr
}

func (f *fakeConsoleService) DisableAPIKey(_ context.Context, _ CurrentUser, apiKeyID string) (APIKeyResponse, error) {
	f.stateAPIKeyID = apiKeyID
	f.stateStatus = domain.APIKeyStatusDisabled
	return f.stateResult, f.stateErr
}

func (f *fakeConsoleService) RevokeAPIKey(_ context.Context, _ CurrentUser, apiKeyID string) (APIKeyResponse, error) {
	f.stateAPIKeyID = apiKeyID
	f.stateStatus = domain.APIKeyStatusRevoked
	return f.stateResult, f.stateErr
}
```

- [x] **Step 2: Implement console handlers**

Create `backend/internal/api/console.go`:

```go
package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

type ConsoleHandler struct {
	service ConsoleService
	auth    AuthService
}

func RegisterConsoleRoutes(mux *http.ServeMux, service ConsoleService, auth AuthService) {
	handler := ConsoleHandler{service: service, auth: auth}
	mux.HandleFunc("GET /api/workspaces/current", handler.CurrentWorkspace)
	mux.HandleFunc("GET /api/api-keys", handler.ListAPIKeys)
	mux.HandleFunc("POST /api/api-keys", handler.CreateAPIKey)
	mux.HandleFunc("POST /api/api-keys/{id}/enable", handler.EnableAPIKey)
	mux.HandleFunc("POST /api/api-keys/{id}/disable", handler.DisableAPIKey)
	mux.HandleFunc("POST /api/api-keys/{id}/revoke", handler.RevokeAPIKey)
}

func (h ConsoleHandler) CurrentWorkspace(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	result, err := h.service.CurrentWorkspace(r.Context(), user)
	if err != nil {
		WriteError(w, RequestIDFromContext(r.Context()), mapConsoleError(err))
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h ConsoleHandler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	result, err := h.service.ListAPIKeys(r.Context(), user)
	if err != nil {
		WriteError(w, RequestIDFromContext(r.Context()), mapConsoleError(err))
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h ConsoleHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	var req CreateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, RequestIDFromContext(r.Context()), ErrInvalidRequest)
		return
	}
	result, err := h.service.CreateAPIKey(r.Context(), user, req)
	if err != nil {
		WriteError(w, RequestIDFromContext(r.Context()), mapConsoleError(err))
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h ConsoleHandler) EnableAPIKey(w http.ResponseWriter, r *http.Request) {
	h.updateAPIKey(w, r, h.service.EnableAPIKey)
}

func (h ConsoleHandler) DisableAPIKey(w http.ResponseWriter, r *http.Request) {
	h.updateAPIKey(w, r, h.service.DisableAPIKey)
}

func (h ConsoleHandler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	h.updateAPIKey(w, r, h.service.RevokeAPIKey)
}

func (h ConsoleHandler) updateAPIKey(w http.ResponseWriter, r *http.Request, fn func(context.Context, CurrentUser, string) (APIKeyResponse, error)) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	apiKeyID := strings.TrimSpace(r.PathValue("id"))
	result, err := fn(r.Context(), user, apiKeyID)
	if err != nil {
		WriteError(w, RequestIDFromContext(r.Context()), mapConsoleError(err))
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h ConsoleHandler) currentUser(w http.ResponseWriter, r *http.Request) (CurrentUser, bool) {
	sessionToken, err := sessionTokenFromRequest(r)
	if err != nil {
		WriteError(w, RequestIDFromContext(r.Context()), mapAuthError(err))
		return CurrentUser{}, false
	}
	user, err := h.auth.CurrentUser(r.Context(), sessionToken)
	if err != nil {
		WriteError(w, RequestIDFromContext(r.Context()), mapAuthError(err))
		return CurrentUser{}, false
	}
	return user, true
}

func mapConsoleError(err error) AppError {
	switch {
	case errors.Is(err, ErrConsoleWorkspaceNotFound):
		return ErrWorkspaceNotFound
	case errors.Is(err, ErrConsolePermissionDenied):
		return ErrPermissionDenied
	case errors.Is(err, ErrConsoleAPIKeyNotFound):
		return ErrAPIKeyNotFound
	case errors.Is(err, ErrConsoleAPIKeyInvalidState):
		return ErrAPIKeyInvalidState
	case errors.Is(err, ErrConsoleAPIKeyInvalidName):
		return ErrAPIKeyInvalidName
	case errors.Is(err, ErrConsoleAPIKeyInvalidLimit):
		return ErrAPIKeyInvalidLimit
	case errors.Is(err, ErrConsoleAPIKeyInvalidExpiration):
		return ErrAPIKeyInvalidExpiration
	case errors.Is(err, ErrAuthUnauthorized):
		return ErrUnauthorized
	default:
		return ErrInternalError
	}
}
```

Add `context` to imports in this file.

- [x] **Step 3: Run handler tests**

Run:

```bash
cd backend
GOCACHE=/private/tmp/go-build-cache go test ./internal/api -run 'TestConsole'
```

Expected: PASS.

---

### Task 5: Route Wiring In portal-api

**Files:**

- Modify: `backend/cmd/portal-api/main.go`
- Modify: `backend/cmd/portal-api/main_test.go`

- [x] **Step 1: Add route wiring test**

Append to `backend/cmd/portal-api/main_test.go`:

```go
func TestRegisterDatabaseBackedRoutesRejectsEmptyAuthPepperWhenDatabaseEnabled(t *testing.T) {
	mux := http.NewServeMux()
	cfg := config.Config{
		Env:         "production",
		DatabaseDSN: "bad-dsn-for-pepper-test",
		AuthPepper:  "",
	}

	cleanup, err := registerDatabaseBackedRoutes(mux, cfg, log.New(io.Discard, "", 0))
	if err == nil {
		if cleanup != nil {
			cleanup()
		}
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "auth pepper") {
		t.Fatalf("err = %v, want auth pepper error", err)
	}
}
```

If imports are missing, add:

```go
import (
	"io"
	"log"
	"net/http"
	"strings"
	"testing"

	"github.com/tokenlive/tokenlive-portal/backend/internal/config"
)
```

- [x] **Step 2: Wire console service and routes**

Modify `backend/cmd/portal-api/main.go` inside `registerDatabaseBackedRoutes` after auth route registration:

```go
	authService, err := api.NewAuthService(modelRepository, cfg.Env, cfg.AuthPepper)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("create auth service: %w", err)
	}
	api.RegisterAuthRoutes(mux, authService, cfg.Env)

	consoleService, err := api.NewConsoleService(modelRepository, cfg.AuthPepper)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("create console service: %w", err)
	}
	api.RegisterConsoleRoutes(mux, consoleService, authService)
```

Also update empty DSN logs:

```go
if cfg.DatabaseDSN == "" {
	logger.Printf("portal-api public model routes disabled: PORTAL_DATABASE_DSN is empty")
	logger.Printf("portal-api auth routes disabled: PORTAL_DATABASE_DSN is empty")
	logger.Printf("portal-api console routes disabled: PORTAL_DATABASE_DSN is empty")
	return func() {}, nil
}
```

- [x] **Step 3: Run portal-api tests**

Run:

```bash
cd backend
GOCACHE=/private/tmp/go-build-cache go test ./cmd/portal-api
```

Expected: PASS.

---

### Task 6: Final Verification And Plan Status

**Files:**

- Modify: `docs/plans/2026-06-20-workspace-api-key-slice.md`

- [x] **Step 1: Format touched Go files**

Run:

```bash
cd backend
gofmt -w cmd/portal-api/main.go cmd/portal-api/main_test.go internal/api/error.go internal/api/console.go internal/api/console_service.go internal/api/console_test.go internal/api/console_service_test.go internal/repository/workspace.go internal/repository/workspace_test.go internal/repository/apikey.go internal/repository/apikey_test.go
```

Expected: command exits 0.

- [x] **Step 2: Run full backend tests**

Run:

```bash
cd backend
GOCACHE=/private/tmp/go-build-cache go test ./...
```

Expected: PASS. Repository integration tests may skip when `PORTAL_TEST_DATABASE_DSN` is unset.

- [x] **Step 3: Build both backend entrypoints**

Run:

```bash
cd backend
GOCACHE=/private/tmp/go-build-cache go build -o /private/tmp/tokenlive-portal-api ./cmd/portal-api
GOCACHE=/private/tmp/go-build-cache go build -o /private/tmp/tokenlive-portal-worker ./cmd/portal-worker
```

Expected: both commands exit 0.

- [x] **Step 4: Check no backend binary was left in the repo**

Run:

```bash
find backend -maxdepth 2 -type f -perm -111
```

Expected: no output.

- [x] **Step 5: Scan for accidental unfinished-work markers**

Run:

```bash
rg -n 'TB[D]|TO[D]O|place''holder|implement[ ]later|fill[ ]in' backend docs
```

Expected: no new runtime unfinished-work markers. Existing intentional product-doc markers, if any, must be called out in the final response.

- [x] **Step 6: Mark completed steps**

After all verification passes, update this plan's checkbox steps from `[ ]` to `[x]` for completed work.

---

## Self-Review Notes

- Spec coverage: current Workspace API, API key list/create/enable/disable/revoke, role checks, one-time secret behavior, Workspace scoping, audit logs, route wiring, tests, and final builds are covered.
- Scope control: no frontend, OAuth, gateway sync, usage logs, recharge, invitation, or Workspace switcher work is included.
- Type consistency: plan uses existing `domain.MemberRole`, `domain.APIKeyStatus`, `repository.Repositories`, `CurrentUser`, `AuthService`, and existing security/money packages.
- Error consistency: role denial maps to existing `workspace.permission_denied`; invalid request maps to existing `validation.invalid_request`.
