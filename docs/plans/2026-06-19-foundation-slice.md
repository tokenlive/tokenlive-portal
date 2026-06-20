# Foundation Slice Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first TokenLive Portal backend foundation: Go module, MySQL migrations, domain models, repositories, API primitives, and buildable API/worker entrypoints.

**Architecture:** Implement `backend/` as an independent Go module shared by `portal-api` and `portal-worker`. SQL migrations are schema truth; GORM domain models and repositories encode transaction boundaries for API keys, Workspace membership, balance, ledger, and audit. Runtime Gateway/Admin integration is intentionally excluded.

**Tech Stack:** Go 1.24, GORM, MySQL driver, goose SQL migrations, standard `net/http`, `crypto/hmac`, `crypto/sha256`, `testing`.

---

## File Structure

Create:

- `backend/go.mod`: independent Go module.
- `backend/cmd/portal-api/main.go`: HTTP API entrypoint with `/healthz`.
- `backend/cmd/portal-worker/main.go`: buildable worker entrypoint.
- `backend/internal/api/error.go`: stable error codes and JSON error response.
- `backend/internal/api/middleware.go`: request ID middleware.
- `backend/internal/api/health.go`: health handler.
- `backend/internal/config/config.go`: environment configuration.
- `backend/internal/database/database.go`: GORM MySQL connection.
- `backend/internal/domain/models.go`: Foundation Slice GORM models and enums.
- `backend/internal/money/micro_cny.go`: integer money helpers.
- `backend/internal/security/apikey.go`: API key generation, hashing, verification.
- `backend/internal/repository/repository.go`: repository container and shared transaction helper.
- `backend/internal/repository/user.go`: user, identity, and default Workspace creation.
- `backend/internal/repository/workspace.go`: Workspace, member, invitation, and model permission persistence.
- `backend/internal/repository/apikey.go`: API key metadata and whitelist persistence.
- `backend/internal/repository/billing.go`: ledger and balance transactional writes.
- `backend/internal/repository/audit.go`: append-only audit persistence.
- `backend/migrations/000001_foundation.sql`: goose migration for Foundation tables.
- `backend/internal/api/error_test.go`: API error response tests.
- `backend/internal/money/micro_cny_test.go`: money helper tests.
- `backend/internal/security/apikey_test.go`: API key helper tests.
- `backend/internal/repository/billing_test.go`: optional MySQL integration test for ledger idempotency.
- `backend/internal/repository/user_test.go`: optional MySQL integration test for default Workspace creation.

Modify:

- `docs/architecture/domain-model.md`: only if implementation names intentionally differ from current domain doc.
- `docs/architecture/portal-architecture.md`: only if implementation choices intentionally differ from current architecture doc.

---

### Task 1: Create Go Module And Shared Package Skeleton

**Files:**

- Create: `backend/go.mod`
- Create: `backend/internal/config/config.go`
- Create: `backend/internal/money/micro_cny.go`
- Test: `backend/internal/money/micro_cny_test.go`

- [ ] **Step 1: Create `backend/go.mod`**

```go
module github.com/tokenlive/tokenlive-portal/backend

go 1.24

require (
	gorm.io/datatypes v1.2.0
	gorm.io/driver/mysql v1.5.7
	gorm.io/gorm v1.25.12
)
```

- [ ] **Step 2: Add config loader**

Create `backend/internal/config/config.go`:

```go
package config

import "os"

type Config struct {
	Env         string
	HTTPAddr    string
	DatabaseDSN string
}

func Load() Config {
	return Config{
		Env:         envOrDefault("PORTAL_ENV", "development"),
		HTTPAddr:    envOrDefault("PORTAL_HTTP_ADDR", ":8080"),
		DatabaseDSN: os.Getenv("PORTAL_DATABASE_DSN"),
	}
}

func envOrDefault(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
```

- [ ] **Step 3: Write money helper test first**

Create `backend/internal/money/micro_cny_test.go`:

```go
package money

import "testing"

func TestFromCNYString(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    MicroCNY
		wantErr bool
	}{
		{name: "integer yuan", input: "1", want: 1_000_000},
		{name: "six decimals", input: "0.123456", want: 123_456},
		{name: "pads decimals", input: "2.5", want: 2_500_000},
		{name: "rounds too many decimals down by rejecting", input: "1.1234567", wantErr: true},
		{name: "rejects negative", input: "-1", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FromCNYString(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestFormatCNY(t *testing.T) {
	tests := []struct {
		input MicroCNY
		want  string
	}{
		{input: 1_000_000, want: "1.000000"},
		{input: 123_456, want: "0.123456"},
		{input: 2_500_000, want: "2.500000"},
	}

	for _, tt := range tests {
		if got := tt.input.FormatCNY(); got != tt.want {
			t.Fatalf("got %s, want %s", got, tt.want)
		}
	}
}
```

- [ ] **Step 4: Run money test to verify it fails**

Run:

```bash
cd backend
go test ./internal/money
```

Expected: FAIL because `MicroCNY`, `FromCNYString`, and `FormatCNY` are undefined.

- [ ] **Step 5: Implement money helper**

Create `backend/internal/money/micro_cny.go`:

```go
package money

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const Scale int64 = 1_000_000

type MicroCNY int64

func FromCNYString(input string) (MicroCNY, error) {
	if input == "" {
		return 0, errors.New("money string is empty")
	}
	if strings.HasPrefix(input, "-") {
		return 0, errors.New("money cannot be negative")
	}

	parts := strings.Split(input, ".")
	if len(parts) > 2 {
		return 0, errors.New("invalid money format")
	}

	yuan, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid yuan amount: %w", err)
	}

	var fractional int64
	if len(parts) == 2 {
		frac := parts[1]
		if len(frac) > 6 {
			return 0, errors.New("money supports at most 6 decimal places")
		}
		frac = frac + strings.Repeat("0", 6-len(frac))
		fractional, err = strconv.ParseInt(frac, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid fractional amount: %w", err)
		}
	}

	return MicroCNY(yuan*Scale + fractional), nil
}

func (m MicroCNY) FormatCNY() string {
	amount := int64(m)
	yuan := amount / Scale
	fractional := amount % Scale
	return fmt.Sprintf("%d.%06d", yuan, fractional)
}
```

- [ ] **Step 6: Run money test to verify it passes**

Run:

```bash
cd backend
go test ./internal/money
```

Expected: PASS.

---

### Task 2: Add API Key Security Helpers

**Files:**

- Create: `backend/internal/security/apikey.go`
- Test: `backend/internal/security/apikey_test.go`

- [ ] **Step 1: Write API key tests first**

Create `backend/internal/security/apikey_test.go`:

```go
package security

import (
	"strings"
	"testing"
)

func TestGenerateAPIKey(t *testing.T) {
	key, err := GenerateAPIKey("live")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(key, "tl_live_") {
		t.Fatalf("key prefix mismatch: %s", key)
	}

	display := DisplayParts(key)
	if display.Prefix == "" {
		t.Fatalf("expected display prefix")
	}
	if len(display.Last4) != 4 {
		t.Fatalf("expected last4 length 4, got %q", display.Last4)
	}
}

func TestHashAndVerifyAPIKey(t *testing.T) {
	key := "tl_live_example_secret"
	pepper := "test-pepper"

	hash := HashAPIKey(key, pepper)
	if hash == "" {
		t.Fatalf("expected hash")
	}
	if strings.Contains(hash, key) {
		t.Fatalf("hash must not contain plaintext key")
	}
	if !VerifyAPIKey(key, pepper, hash) {
		t.Fatalf("expected key to verify")
	}
	if VerifyAPIKey(key+"x", pepper, hash) {
		t.Fatalf("expected modified key to fail verification")
	}
}
```

- [ ] **Step 2: Run security test to verify it fails**

Run:

```bash
cd backend
go test ./internal/security
```

Expected: FAIL because security helpers are undefined.

- [ ] **Step 3: Implement API key helpers**

Create `backend/internal/security/apikey.go`:

```go
package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

type APIKeyDisplay struct {
	Prefix string
	Last4  string
}

func GenerateAPIKey(environment string) (string, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("generate api key random bytes: %w", err)
	}
	secret := base64.RawURLEncoding.EncodeToString(random)
	return fmt.Sprintf("tl_%s_%s", environment, secret), nil
}

func DisplayParts(key string) APIKeyDisplay {
	prefixLen := 12
	if len(key) < prefixLen {
		prefixLen = len(key)
	}
	lastLen := 4
	if len(key) < lastLen {
		lastLen = len(key)
	}
	return APIKeyDisplay{
		Prefix: key[:prefixLen],
		Last4:  key[len(key)-lastLen:],
	}
}

func HashAPIKey(key string, pepper string) string {
	mac := hmac.New(sha256.New, []byte(pepper))
	mac.Write([]byte(key))
	return hex.EncodeToString(mac.Sum(nil))
}

func VerifyAPIKey(key string, pepper string, expectedHash string) bool {
	actualHash := HashAPIKey(key, pepper)
	return subtle.ConstantTimeCompare([]byte(actualHash), []byte(expectedHash)) == 1
}
```

- [ ] **Step 4: Run security test to verify it passes**

Run:

```bash
cd backend
go test ./internal/security
```

Expected: PASS.

---

### Task 3: Create Foundation SQL Migration

**Files:**

- Create: `backend/migrations/000001_foundation.sql`

- [ ] **Step 1: Add goose migration**

Create `backend/migrations/000001_foundation.sql`:

```sql
-- +goose Up
CREATE TABLE users (
    id VARCHAR(32) PRIMARY KEY,
    display_name VARCHAR(120) NOT NULL DEFAULT '',
    primary_email VARCHAR(320) NOT NULL DEFAULT '',
    email_verified_at DATETIME(3) NULL,
    avatar_url VARCHAR(1024) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL,
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL,
    deleted_at DATETIME(3) NULL,
    UNIQUE KEY uk_users_primary_email (primary_email),
    KEY idx_users_status (status),
    KEY idx_users_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE account_identities (
    id VARCHAR(32) PRIMARY KEY,
    user_id VARCHAR(32) NOT NULL,
    provider VARCHAR(32) NOT NULL,
    provider_subject VARCHAR(191) NOT NULL,
    email VARCHAR(320) NOT NULL DEFAULT '',
    email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL,
    UNIQUE KEY uk_account_identities_provider_subject (provider, provider_subject),
    KEY idx_account_identities_user_id (user_id),
    CONSTRAINT fk_account_identities_user FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE workspaces (
    id VARCHAR(32) PRIMARY KEY,
    name VARCHAR(160) NOT NULL,
    slug VARCHAR(160) NOT NULL,
    owner_user_id VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL,
    trial_granted_at DATETIME(3) NULL,
    created_by_user_id VARCHAR(32) NOT NULL,
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL,
    deleted_at DATETIME(3) NULL,
    UNIQUE KEY uk_workspaces_slug (slug),
    KEY idx_workspaces_owner_user_id (owner_user_id),
    KEY idx_workspaces_created_by_user_id (created_by_user_id),
    KEY idx_workspaces_deleted_at (deleted_at),
    CONSTRAINT fk_workspaces_owner_user FOREIGN KEY (owner_user_id) REFERENCES users(id),
    CONSTRAINT fk_workspaces_created_by_user FOREIGN KEY (created_by_user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE workspace_members (
    workspace_id VARCHAR(32) NOT NULL,
    user_id VARCHAR(32) NOT NULL,
    role VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL,
    joined_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL,
    PRIMARY KEY (workspace_id, user_id),
    KEY idx_workspace_members_user_id (user_id),
    CONSTRAINT fk_workspace_members_workspace FOREIGN KEY (workspace_id) REFERENCES workspaces(id),
    CONSTRAINT fk_workspace_members_user FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE workspace_invitations (
    id VARCHAR(32) PRIMARY KEY,
    workspace_id VARCHAR(32) NOT NULL,
    email VARCHAR(320) NOT NULL,
    role VARCHAR(32) NOT NULL,
    token_hash VARCHAR(128) NOT NULL,
    status VARCHAR(32) NOT NULL,
    invited_by_user_id VARCHAR(32) NOT NULL,
    accepted_by_user_id VARCHAR(32) NULL,
    expires_at DATETIME(3) NOT NULL,
    accepted_at DATETIME(3) NULL,
    revoked_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL,
    UNIQUE KEY uk_workspace_invitations_token_hash (token_hash),
    KEY idx_workspace_invitations_workspace_email (workspace_id, email),
    KEY idx_workspace_invitations_status (status),
    CONSTRAINT fk_workspace_invitations_workspace FOREIGN KEY (workspace_id) REFERENCES workspaces(id),
    CONSTRAINT fk_workspace_invitations_invited_by FOREIGN KEY (invited_by_user_id) REFERENCES users(id),
    CONSTRAINT fk_workspace_invitations_accepted_by FOREIGN KEY (accepted_by_user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE api_keys (
    id VARCHAR(32) PRIMARY KEY,
    workspace_id VARCHAR(32) NOT NULL,
    name VARCHAR(160) NOT NULL,
    key_prefix VARCHAR(32) NOT NULL,
    secret_last4 VARCHAR(8) NOT NULL,
    key_hash VARCHAR(128) NOT NULL,
    status VARCHAR(32) NOT NULL,
    created_by_user_id VARCHAR(32) NOT NULL,
    expires_at DATETIME(3) NULL,
    daily_limit_micro_cny BIGINT NULL,
    monthly_limit_micro_cny BIGINT NULL,
    last_used_at DATETIME(3) NULL,
    total_spend_micro_cny BIGINT NOT NULL DEFAULT 0,
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL,
    revoked_at DATETIME(3) NULL,
    UNIQUE KEY uk_api_keys_key_hash (key_hash),
    KEY idx_api_keys_workspace_id (workspace_id),
    KEY idx_api_keys_created_by_user_id (created_by_user_id),
    KEY idx_api_keys_status (status),
    CONSTRAINT fk_api_keys_workspace FOREIGN KEY (workspace_id) REFERENCES workspaces(id),
    CONSTRAINT fk_api_keys_created_by_user FOREIGN KEY (created_by_user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE workspace_model_permissions (
    workspace_id VARCHAR(32) NOT NULL,
    model_id VARCHAR(191) NOT NULL,
    source VARCHAR(32) NOT NULL,
    granted_by_user_id VARCHAR(32) NULL,
    created_at DATETIME(3) NOT NULL,
    PRIMARY KEY (workspace_id, model_id),
    KEY idx_workspace_model_permissions_model_id (model_id),
    CONSTRAINT fk_workspace_model_permissions_workspace FOREIGN KEY (workspace_id) REFERENCES workspaces(id),
    CONSTRAINT fk_workspace_model_permissions_granted_by FOREIGN KEY (granted_by_user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE api_key_model_whitelists (
    api_key_id VARCHAR(32) NOT NULL,
    model_id VARCHAR(191) NOT NULL,
    created_at DATETIME(3) NOT NULL,
    PRIMARY KEY (api_key_id, model_id),
    KEY idx_api_key_model_whitelists_model_id (model_id),
    CONSTRAINT fk_api_key_model_whitelists_api_key FOREIGN KEY (api_key_id) REFERENCES api_keys(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE workspace_balances (
    workspace_id VARCHAR(32) PRIMARY KEY,
    available_micro_cny BIGINT NOT NULL DEFAULT 0,
    frozen_micro_cny BIGINT NOT NULL DEFAULT 0,
    version BIGINT NOT NULL DEFAULT 1,
    updated_at DATETIME(3) NOT NULL,
    CONSTRAINT fk_workspace_balances_workspace FOREIGN KEY (workspace_id) REFERENCES workspaces(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE ledger_entries (
    id VARCHAR(32) PRIMARY KEY,
    workspace_id VARCHAR(32) NOT NULL,
    type VARCHAR(32) NOT NULL,
    direction VARCHAR(16) NOT NULL,
    amount_micro_cny BIGINT NOT NULL,
    balance_after_micro_cny BIGINT NOT NULL,
    currency VARCHAR(8) NOT NULL,
    idempotency_key VARCHAR(191) NOT NULL,
    request_id VARCHAR(191) NULL,
    api_key_id VARCHAR(32) NULL,
    api_key_name_snapshot VARCHAR(160) NOT NULL DEFAULT '',
    model_id VARCHAR(191) NOT NULL DEFAULT '',
    model_display_name_snapshot VARCHAR(255) NOT NULL DEFAULT '',
    price_version_id VARCHAR(32) NOT NULL DEFAULT '',
    unit_price_snapshot JSON NULL,
    metadata JSON NULL,
    created_at DATETIME(3) NOT NULL,
    UNIQUE KEY uk_ledger_entries_workspace_idempotency (workspace_id, idempotency_key),
    UNIQUE KEY uk_ledger_entries_request_id (request_id),
    KEY idx_ledger_entries_workspace_created (workspace_id, created_at),
    KEY idx_ledger_entries_api_key_id (api_key_id),
    CONSTRAINT fk_ledger_entries_workspace FOREIGN KEY (workspace_id) REFERENCES workspaces(id),
    CONSTRAINT fk_ledger_entries_api_key FOREIGN KEY (api_key_id) REFERENCES api_keys(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE audit_logs (
    id VARCHAR(32) PRIMARY KEY,
    workspace_id VARCHAR(32) NULL,
    actor_user_id VARCHAR(32) NULL,
    action VARCHAR(96) NOT NULL,
    resource_type VARCHAR(64) NOT NULL,
    resource_id VARCHAR(64) NOT NULL,
    before_data JSON NULL,
    after_data JSON NULL,
    ip VARCHAR(64) NOT NULL DEFAULT '',
    user_agent VARCHAR(512) NOT NULL DEFAULT '',
    created_at DATETIME(3) NOT NULL,
    KEY idx_audit_logs_workspace_created (workspace_id, created_at),
    KEY idx_audit_logs_actor_created (actor_user_id, created_at),
    KEY idx_audit_logs_resource (resource_type, resource_id),
    CONSTRAINT fk_audit_logs_workspace FOREIGN KEY (workspace_id) REFERENCES workspaces(id),
    CONSTRAINT fk_audit_logs_actor FOREIGN KEY (actor_user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS ledger_entries;
DROP TABLE IF EXISTS workspace_balances;
DROP TABLE IF EXISTS api_key_model_whitelists;
DROP TABLE IF EXISTS workspace_model_permissions;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS workspace_invitations;
DROP TABLE IF EXISTS workspace_members;
DROP TABLE IF EXISTS workspaces;
DROP TABLE IF EXISTS account_identities;
DROP TABLE IF EXISTS users;
```

- [ ] **Step 2: Validate migration syntax with goose if available**

Run:

```bash
cd backend
go install github.com/pressly/goose/v3/cmd/goose@v3.24.1
```

Expected: goose installs if network is available. If network is blocked, skip install and rely on SQL review until dependencies are available.

Run only when a local MySQL DSN exists:

```bash
cd backend
goose -dir migrations mysql "$PORTAL_TEST_DATABASE_DSN" up
```

Expected: migration applies successfully.

---

### Task 4: Add Domain Models And Database Connection

**Files:**

- Create: `backend/internal/domain/models.go`
- Create: `backend/internal/database/database.go`

- [ ] **Step 1: Add domain models**

Create `backend/internal/domain/models.go`:

```go
package domain

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type UserStatus string
type WorkspaceStatus string
type MemberRole string
type MemberStatus string
type InvitationStatus string
type APIKeyStatus string
type LedgerType string
type LedgerDirection string

const (
	UserStatusActive   UserStatus = "active"
	UserStatusBlocked  UserStatus = "blocked"
	UserStatusDeleting UserStatus = "deleting"

	WorkspaceStatusActive   WorkspaceStatus = "active"
	WorkspaceStatusBlocked  WorkspaceStatus = "blocked"
	WorkspaceStatusDeleting WorkspaceStatus = "deleting"

	MemberRoleOwner     MemberRole = "owner"
	MemberRoleDeveloper MemberRole = "developer"
	MemberRoleBilling   MemberRole = "billing"

	MemberStatusActive  MemberStatus = "active"
	MemberStatusRemoved MemberStatus = "removed"

	InvitationStatusPending  InvitationStatus = "pending"
	InvitationStatusAccepted InvitationStatus = "accepted"
	InvitationStatusRevoked  InvitationStatus = "revoked"
	InvitationStatusExpired  InvitationStatus = "expired"

	APIKeyStatusEnabled  APIKeyStatus = "enabled"
	APIKeyStatusDisabled APIKeyStatus = "disabled"
	APIKeyStatusRevoked  APIKeyStatus = "revoked"

	LedgerTypeRecharge   LedgerType = "recharge"
	LedgerTypeTrialGrant LedgerType = "trial_grant"
	LedgerTypeConsumption LedgerType = "consumption"
	LedgerTypeRefund     LedgerType = "refund"
	LedgerTypeAdjustment LedgerType = "adjustment"

	LedgerDirectionCredit LedgerDirection = "credit"
	LedgerDirectionDebit  LedgerDirection = "debit"
)

type User struct {
	ID              string         `gorm:"primaryKey;size:32"`
	DisplayName     string         `gorm:"size:120;not null;default:''"`
	PrimaryEmail    string         `gorm:"size:320;not null;default:'';uniqueIndex:uk_users_primary_email"`
	EmailVerifiedAt *time.Time
	AvatarURL       string         `gorm:"size:1024;not null;default:''"`
	Status          UserStatus     `gorm:"size:32;not null;index"`
	CreatedAt       time.Time      `gorm:"not null"`
	UpdatedAt       time.Time      `gorm:"not null"`
	DeletedAt       gorm.DeletedAt `gorm:"index"`
}

type AccountIdentity struct {
	ID              string    `gorm:"primaryKey;size:32"`
	UserID          string    `gorm:"size:32;not null;index"`
	Provider        string    `gorm:"size:32;not null;uniqueIndex:uk_account_identities_provider_subject,priority:1"`
	ProviderSubject string    `gorm:"size:191;not null;uniqueIndex:uk_account_identities_provider_subject,priority:2"`
	Email           string    `gorm:"size:320;not null;default:''"`
	EmailVerified   bool      `gorm:"not null;default:false"`
	CreatedAt       time.Time `gorm:"not null"`
	UpdatedAt       time.Time `gorm:"not null"`
}

type Workspace struct {
	ID             string          `gorm:"primaryKey;size:32"`
	Name           string          `gorm:"size:160;not null"`
	Slug           string          `gorm:"size:160;not null;uniqueIndex"`
	OwnerUserID    string          `gorm:"size:32;not null;index"`
	Status         WorkspaceStatus `gorm:"size:32;not null"`
	TrialGrantedAt *time.Time
	CreatedByUserID string        `gorm:"size:32;not null;index"`
	CreatedAt      time.Time      `gorm:"not null"`
	UpdatedAt      time.Time      `gorm:"not null"`
	DeletedAt      gorm.DeletedAt `gorm:"index"`
}

type WorkspaceMember struct {
	WorkspaceID string       `gorm:"primaryKey;size:32"`
	UserID      string       `gorm:"primaryKey;size:32;index"`
	Role        MemberRole   `gorm:"size:32;not null"`
	Status      MemberStatus `gorm:"size:32;not null"`
	JoinedAt    *time.Time
	CreatedAt   time.Time `gorm:"not null"`
	UpdatedAt   time.Time `gorm:"not null"`
}

type WorkspaceInvitation struct {
	ID               string           `gorm:"primaryKey;size:32"`
	WorkspaceID      string           `gorm:"size:32;not null;index"`
	Email            string           `gorm:"size:320;not null"`
	Role             MemberRole       `gorm:"size:32;not null"`
	TokenHash        string           `gorm:"size:128;not null;uniqueIndex"`
	Status           InvitationStatus `gorm:"size:32;not null;index"`
	InvitedByUserID  string           `gorm:"size:32;not null"`
	AcceptedByUserID *string          `gorm:"size:32"`
	ExpiresAt        time.Time        `gorm:"not null"`
	AcceptedAt       *time.Time
	RevokedAt        *time.Time
	CreatedAt        time.Time `gorm:"not null"`
	UpdatedAt        time.Time `gorm:"not null"`
}

type APIKey struct {
	ID                  string       `gorm:"primaryKey;size:32"`
	WorkspaceID         string       `gorm:"size:32;not null;index"`
	Name                string       `gorm:"size:160;not null"`
	KeyPrefix           string       `gorm:"size:32;not null"`
	SecretLast4         string       `gorm:"size:8;not null"`
	KeyHash             string       `gorm:"size:128;not null;uniqueIndex"`
	Status              APIKeyStatus `gorm:"size:32;not null;index"`
	CreatedByUserID     string       `gorm:"size:32;not null;index"`
	ExpiresAt           *time.Time
	DailyLimitMicroCNY   *int64
	MonthlyLimitMicroCNY *int64
	LastUsedAt           *time.Time
	TotalSpendMicroCNY   int64     `gorm:"not null;default:0"`
	CreatedAt            time.Time `gorm:"not null"`
	UpdatedAt            time.Time `gorm:"not null"`
	RevokedAt            *time.Time
}

type WorkspaceModelPermission struct {
	WorkspaceID     string    `gorm:"primaryKey;size:32"`
	ModelID         string    `gorm:"primaryKey;size:191;index"`
	Source          string    `gorm:"size:32;not null"`
	GrantedByUserID *string   `gorm:"size:32"`
	CreatedAt       time.Time `gorm:"not null"`
}

type APIKeyModelWhitelist struct {
	APIKeyID  string    `gorm:"primaryKey;size:32"`
	ModelID   string    `gorm:"primaryKey;size:191;index"`
	CreatedAt time.Time `gorm:"not null"`
}

type WorkspaceBalance struct {
	WorkspaceID         string    `gorm:"primaryKey;size:32"`
	AvailableMicroCNY  int64     `gorm:"not null;default:0"`
	FrozenMicroCNY      int64     `gorm:"not null;default:0"`
	Version             int64     `gorm:"not null;default:1"`
	UpdatedAt           time.Time `gorm:"not null"`
}

type LedgerEntry struct {
	ID                       string          `gorm:"primaryKey;size:32"`
	WorkspaceID              string          `gorm:"size:32;not null;index:idx_ledger_entries_workspace_created,priority:1;uniqueIndex:uk_ledger_entries_workspace_idempotency,priority:1"`
	Type                     LedgerType      `gorm:"size:32;not null"`
	Direction                LedgerDirection `gorm:"size:16;not null"`
	AmountMicroCNY           int64           `gorm:"not null"`
	BalanceAfterMicroCNY     int64           `gorm:"not null"`
	Currency                 string          `gorm:"size:8;not null"`
	IdempotencyKey           string          `gorm:"size:191;not null;uniqueIndex:uk_ledger_entries_workspace_idempotency,priority:2"`
	RequestID                *string         `gorm:"size:191;uniqueIndex"`
	APIKeyID                 *string         `gorm:"size:32;index"`
	APIKeyNameSnapshot       string          `gorm:"size:160;not null;default:''"`
	ModelID                  string          `gorm:"size:191;not null;default:''"`
	ModelDisplayNameSnapshot string          `gorm:"size:255;not null;default:''"`
	PriceVersionID           string          `gorm:"size:32;not null;default:''"`
	UnitPriceSnapshot        datatypes.JSON
	Metadata                 datatypes.JSON
	CreatedAt                time.Time `gorm:"not null;index:idx_ledger_entries_workspace_created,priority:2"`
}

type AuditLog struct {
	ID           string         `gorm:"primaryKey;size:32"`
	WorkspaceID  *string        `gorm:"size:32;index:idx_audit_logs_workspace_created,priority:1"`
	ActorUserID  *string        `gorm:"size:32;index:idx_audit_logs_actor_created,priority:1"`
	Action       string         `gorm:"size:96;not null"`
	ResourceType string         `gorm:"size:64;not null;index:idx_audit_logs_resource,priority:1"`
	ResourceID   string         `gorm:"size:64;not null;index:idx_audit_logs_resource,priority:2"`
	BeforeData   datatypes.JSON
	AfterData    datatypes.JSON
	IP           string    `gorm:"size:64;not null;default:''"`
	UserAgent    string    `gorm:"size:512;not null;default:''"`
	CreatedAt    time.Time `gorm:"not null;index:idx_audit_logs_workspace_created,priority:2;index:idx_audit_logs_actor_created,priority:2"`
}
```

- [ ] **Step 2: Add database connection package**

Create `backend/internal/database/database.go`:

```go
package database

import (
	"errors"
	"fmt"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func Open(dsn string) (*gorm.DB, error) {
	if dsn == "" {
		return nil, errors.New("database dsn is required")
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	return db, nil
}
```

- [ ] **Step 3: Run compile check**

Run:

```bash
cd backend
go test ./internal/domain ./internal/database
```

Expected: PASS after dependencies are available.

---

### Task 5: Add API Primitives

**Files:**

- Create: `backend/internal/api/error.go`
- Create: `backend/internal/api/middleware.go`
- Create: `backend/internal/api/health.go`
- Test: `backend/internal/api/error_test.go`

- [ ] **Step 1: Write API error test first**

Create `backend/internal/api/error_test.go`:

```go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, "req_test", ErrWorkspaceNotFound)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusNotFound)
	}

	var body map[string]map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	got := body["error"]
	if got["code"] != "workspace.not_found" {
		t.Fatalf("got code %q", got["code"])
	}
	if got["request_id"] != "req_test" {
		t.Fatalf("got request_id %q", got["request_id"])
	}
}
```

- [ ] **Step 2: Run API test to verify it fails**

Run:

```bash
cd backend
go test ./internal/api
```

Expected: FAIL because API primitives are undefined.

- [ ] **Step 3: Implement API error primitives**

Create `backend/internal/api/error.go`:

```go
package api

import (
	"encoding/json"
	"net/http"
)

type ErrorCode string

const (
	CodeInternalError        ErrorCode = "internal.error"
	CodeInvalidRequest       ErrorCode = "validation.invalid_request"
	CodeUnauthorized         ErrorCode = "auth.unauthorized"
	CodeWorkspaceNotFound    ErrorCode = "workspace.not_found"
	CodeWorkspaceLimit       ErrorCode = "workspace.limit_exceeded"
	CodePermissionDenied     ErrorCode = "workspace.permission_denied"
	CodeAPIKeyNotFound       ErrorCode = "api_key.not_found"
	CodeAPIKeyInvalidState   ErrorCode = "api_key.invalid_state"
	CodeBillingDuplicate     ErrorCode = "billing.duplicate_conflict"
	CodeInsufficientBalance  ErrorCode = "billing.insufficient_balance"
)

type AppError struct {
	Code       ErrorCode
	Message    string
	HTTPStatus int
}

var (
	ErrInternalError       = AppError{Code: CodeInternalError, Message: "Internal error", HTTPStatus: http.StatusInternalServerError}
	ErrInvalidRequest      = AppError{Code: CodeInvalidRequest, Message: "Invalid request", HTTPStatus: http.StatusBadRequest}
	ErrUnauthorized        = AppError{Code: CodeUnauthorized, Message: "Unauthorized", HTTPStatus: http.StatusUnauthorized}
	ErrWorkspaceNotFound   = AppError{Code: CodeWorkspaceNotFound, Message: "Workspace not found", HTTPStatus: http.StatusNotFound}
	ErrWorkspaceLimit      = AppError{Code: CodeWorkspaceLimit, Message: "Workspace limit exceeded", HTTPStatus: http.StatusConflict}
	ErrPermissionDenied    = AppError{Code: CodePermissionDenied, Message: "Permission denied", HTTPStatus: http.StatusForbidden}
	ErrAPIKeyNotFound      = AppError{Code: CodeAPIKeyNotFound, Message: "API key not found", HTTPStatus: http.StatusNotFound}
	ErrAPIKeyInvalidState  = AppError{Code: CodeAPIKeyInvalidState, Message: "API key invalid state", HTTPStatus: http.StatusConflict}
	ErrBillingDuplicate    = AppError{Code: CodeBillingDuplicate, Message: "Duplicate billing event conflict", HTTPStatus: http.StatusConflict}
	ErrInsufficientBalance = AppError{Code: CodeInsufficientBalance, Message: "Insufficient balance", HTTPStatus: http.StatusPaymentRequired}
)

type errorResponse struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code      ErrorCode `json:"code"`
	Message   string    `json:"message"`
	RequestID string    `json:"request_id"`
}

func WriteError(w http.ResponseWriter, requestID string, appErr AppError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(appErr.HTTPStatus)
	_ = json.NewEncoder(w).Encode(errorResponse{
		Error: errorBody{
			Code:      appErr.Code,
			Message:   appErr.Message,
			RequestID: requestID,
		},
	})
}
```

- [ ] **Step 4: Implement request ID middleware and health handler**

Create `backend/internal/api/middleware.go`:

```go
package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

type contextKey string

const requestIDKey contextKey = "request_id"

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = newRequestID()
		}
		w.Header().Set("X-Request-ID", requestID)
		ctx := context.WithValue(r.Context(), requestIDKey, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequestIDFromContext(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey).(string)
	return value
}

func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "req_unavailable"
	}
	return "req_" + hex.EncodeToString(b[:])
}
```

Create `backend/internal/api/health.go`:

```go
package api

import (
	"encoding/json"
	"net/http"
)

func HealthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
```

- [ ] **Step 5: Run API tests**

Run:

```bash
cd backend
go test ./internal/api
```

Expected: PASS.

---

### Task 6: Add Repository Layer

**Files:**

- Create: `backend/internal/repository/repository.go`
- Create: `backend/internal/repository/user.go`
- Create: `backend/internal/repository/workspace.go`
- Create: `backend/internal/repository/apikey.go`
- Create: `backend/internal/repository/billing.go`
- Create: `backend/internal/repository/audit.go`

- [ ] **Step 1: Add repository root and ID helper**

Create `backend/internal/repository/repository.go`:

```go
package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"gorm.io/gorm"
)

type Repositories struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repositories {
	return &Repositories{db: db}
}

func (r *Repositories) DB() *gorm.DB {
	return r.db
}

func (r *Repositories) withTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return r.db.WithContext(ctx).Transaction(fn)
}

func newID(prefix string) (string, error) {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return prefix + hex.EncodeToString(b[:]), nil
}
```

- [ ] **Step 2: Add user repository**

Create `backend/internal/repository/user.go`:

```go
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/tokenlive/tokenlive-portal/backend/internal/domain"
	"gorm.io/gorm"
)

type CreateUserWithWorkspaceInput struct {
	DisplayName  string
	PrimaryEmail string
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
			WorkspaceID:        workspaceID,
			AvailableMicroCNY: 0,
			FrozenMicroCNY:     0,
			Version:            1,
			UpdatedAt:          now,
		}
		if err := tx.Create(&balance).Error; err != nil {
			return fmt.Errorf("create workspace balance: %w", err)
		}

		result = CreateUserWithWorkspaceResult{User: user, Workspace: workspace}
		return nil
	})

	return result, err
}
```

- [ ] **Step 3: Add workspace repository**

Create `backend/internal/repository/workspace.go`:

```go
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/tokenlive/tokenlive-portal/backend/internal/domain"
	"gorm.io/gorm"
)

const DefaultSelfCreatedWorkspaceLimit int64 = 3

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

		balance := domain.WorkspaceBalance{WorkspaceID: id, Version: 1, UpdatedAt: now}
		if err := tx.Create(&balance).Error; err != nil {
			return fmt.Errorf("create workspace balance: %w", err)
		}

		return nil
	})

	return workspace, err
}

func (r *Repositories) GrantWorkspaceModel(ctx context.Context, workspaceID string, modelID string, source string, grantedByUserID *string) error {
	permission := domain.WorkspaceModelPermission{
		WorkspaceID:     workspaceID,
		ModelID:         modelID,
		Source:          source,
		GrantedByUserID: grantedByUserID,
		CreatedAt:       time.Now().UTC(),
	}
	return r.db.WithContext(ctx).Create(&permission).Error
}

var ErrWorkspaceLimitExceeded = fmt.Errorf("workspace self-created limit exceeded")
```

- [ ] **Step 4: Add API key repository**

Create `backend/internal/repository/apikey.go`:

```go
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/tokenlive/tokenlive-portal/backend/internal/domain"
	"github.com/tokenlive/tokenlive-portal/backend/internal/security"
	"gorm.io/gorm"
)

type CreateAPIKeyInput struct {
	WorkspaceID         string
	Name                string
	PlaintextKey        string
	Pepper              string
	CreatedByUserID     string
	DailyLimitMicroCNY   *int64
	MonthlyLimitMicroCNY *int64
}

type CreateAPIKeyResult struct {
	APIKey domain.APIKey
	Secret string
}

func (r *Repositories) CreateAPIKey(ctx context.Context, input CreateAPIKeyInput) (CreateAPIKeyResult, error) {
	key := input.PlaintextKey
	var err error
	if key == "" {
		key, err = security.GenerateAPIKey("live")
		if err != nil {
			return CreateAPIKeyResult{}, err
		}
	}

	id, err := newID("ak_")
	if err != nil {
		return CreateAPIKeyResult{}, err
	}
	display := security.DisplayParts(key)
	now := time.Now().UTC()
	apiKey := domain.APIKey{
		ID:                  id,
		WorkspaceID:         input.WorkspaceID,
		Name:                input.Name,
		KeyPrefix:           display.Prefix,
		SecretLast4:         display.Last4,
		KeyHash:             security.HashAPIKey(key, input.Pepper),
		Status:              domain.APIKeyStatusEnabled,
		CreatedByUserID:     input.CreatedByUserID,
		DailyLimitMicroCNY:   input.DailyLimitMicroCNY,
		MonthlyLimitMicroCNY: input.MonthlyLimitMicroCNY,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := r.db.WithContext(ctx).Create(&apiKey).Error; err != nil {
		return CreateAPIKeyResult{}, fmt.Errorf("create api key: %w", err)
	}
	return CreateAPIKeyResult{APIKey: apiKey, Secret: key}, nil
}

func (r *Repositories) UpdateAPIKeyStatus(ctx context.Context, apiKeyID string, status domain.APIKeyStatus) error {
	updates := map[string]any{
		"status":     status,
		"updated_at": time.Now().UTC(),
	}
	if status == domain.APIKeyStatusRevoked {
		now := time.Now().UTC()
		updates["revoked_at"] = &now
	}
	return r.db.WithContext(ctx).Model(&domain.APIKey{}).Where("id = ?", apiKeyID).Updates(updates).Error
}

func (r *Repositories) ReplaceAPIKeyWhitelist(ctx context.Context, apiKeyID string, modelIDs []string) error {
	return r.withTx(ctx, func(tx *gorm.DB) error {
		if err := tx.Where("api_key_id = ?", apiKeyID).Delete(&domain.APIKeyModelWhitelist{}).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		for _, modelID := range modelIDs {
			row := domain.APIKeyModelWhitelist{APIKeyID: apiKeyID, ModelID: modelID, CreatedAt: now}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
```

- [ ] **Step 5: Add billing repository**

Create `backend/internal/repository/billing.go`:

```go
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

var ErrDuplicateLedgerConflict = errors.New("duplicate ledger idempotency key with conflicting payload")

type CreateLedgerInput struct {
	WorkspaceID            string
	Type                   domain.LedgerType
	Direction              domain.LedgerDirection
	AmountMicroCNY         int64
	Currency               string
	IdempotencyKey         string
	RequestID              *string
	APIKeyID               *string
	APIKeyNameSnapshot     string
	ModelID                string
	ModelDisplayNameSnapshot string
	PriceVersionID         string
}

func (r *Repositories) CreateLedgerEntry(ctx context.Context, input CreateLedgerInput) (domain.LedgerEntry, error) {
	var output domain.LedgerEntry
	now := time.Now().UTC()

	err := r.withTx(ctx, func(tx *gorm.DB) error {
		var existing domain.LedgerEntry
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("workspace_id = ? AND idempotency_key = ?", input.WorkspaceID, input.IdempotencyKey).
			First(&existing).Error
		if err == nil {
			if existing.AmountMicroCNY != input.AmountMicroCNY || existing.Type != input.Type || existing.Direction != input.Direction {
				return ErrDuplicateLedgerConflict
			}
			output = existing
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("lookup existing ledger: %w", err)
		}

		var balance domain.WorkspaceBalance
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("workspace_id = ?", input.WorkspaceID).
			First(&balance).Error; err != nil {
			return fmt.Errorf("lock workspace balance: %w", err)
		}

		nextAvailable := balance.AvailableMicroCNY
		switch input.Direction {
		case domain.LedgerDirectionCredit:
			nextAvailable += input.AmountMicroCNY
		case domain.LedgerDirectionDebit:
			if nextAvailable < input.AmountMicroCNY {
				return ErrInsufficientBalance
			}
			nextAvailable -= input.AmountMicroCNY
		default:
			return fmt.Errorf("unsupported ledger direction: %s", input.Direction)
		}

		id, err := newID("led_")
		if err != nil {
			return err
		}
		entry := domain.LedgerEntry{
			ID:                       id,
			WorkspaceID:              input.WorkspaceID,
			Type:                     input.Type,
			Direction:                input.Direction,
			AmountMicroCNY:           input.AmountMicroCNY,
			BalanceAfterMicroCNY:     nextAvailable,
			Currency:                 input.Currency,
			IdempotencyKey:           input.IdempotencyKey,
			RequestID:                input.RequestID,
			APIKeyID:                 input.APIKeyID,
			APIKeyNameSnapshot:       input.APIKeyNameSnapshot,
			ModelID:                  input.ModelID,
			ModelDisplayNameSnapshot: input.ModelDisplayNameSnapshot,
			PriceVersionID:           input.PriceVersionID,
			CreatedAt:                now,
		}
		if err := tx.Create(&entry).Error; err != nil {
			return fmt.Errorf("create ledger entry: %w", err)
		}

		if err := tx.Model(&domain.WorkspaceBalance{}).
			Where("workspace_id = ?", input.WorkspaceID).
			Updates(map[string]any{
				"available_micro_cny": nextAvailable,
				"version":             balance.Version + 1,
				"updated_at":          now,
			}).Error; err != nil {
			return fmt.Errorf("update workspace balance: %w", err)
		}

		output = entry
		return nil
	})

	return output, err
}

var ErrInsufficientBalance = errors.New("insufficient balance")
```

- [ ] **Step 6: Add audit repository**

Create `backend/internal/repository/audit.go`:

```go
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/tokenlive/tokenlive-portal/backend/internal/domain"
	"gorm.io/datatypes"
)

type AppendAuditInput struct {
	WorkspaceID  *string
	ActorUserID  *string
	Action       string
	ResourceType string
	ResourceID   string
	BeforeData   datatypes.JSON
	AfterData    datatypes.JSON
	IP           string
	UserAgent    string
}

func (r *Repositories) AppendAuditLog(ctx context.Context, input AppendAuditInput) (domain.AuditLog, error) {
	id, err := newID("aud_")
	if err != nil {
		return domain.AuditLog{}, err
	}
	row := domain.AuditLog{
		ID:           id,
		WorkspaceID:  input.WorkspaceID,
		ActorUserID:  input.ActorUserID,
		Action:       input.Action,
		ResourceType: input.ResourceType,
		ResourceID:   input.ResourceID,
		BeforeData:   input.BeforeData,
		AfterData:    input.AfterData,
		IP:           input.IP,
		UserAgent:    input.UserAgent,
		CreatedAt:    time.Now().UTC(),
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.AuditLog{}, fmt.Errorf("append audit log: %w", err)
	}
	return row, nil
}
```

- [ ] **Step 7: Fix imports and run package compile**

Run:

```bash
cd backend
gofmt -w internal/repository internal/domain internal/database internal/security internal/money internal/api
go test ./internal/...
```

Expected: PASS.

---

### Task 7: Add Optional Repository Integration Tests

**Files:**

- Create: `backend/internal/repository/testutil_test.go`
- Test: `backend/internal/repository/user_test.go`
- Test: `backend/internal/repository/billing_test.go`

- [ ] **Step 1: Add repository test utility**

Create `backend/internal/repository/testutil_test.go`:

```go
package repository

import (
	"os"
	"testing"

	"github.com/tokenlive/tokenlive-portal/backend/internal/database"
	"gorm.io/gorm"
)

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("PORTAL_TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("PORTAL_TEST_DATABASE_DSN is not set")
	}
	db, err := database.Open(dsn)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	return db
}
```

- [ ] **Step 2: Add user repository integration test**

Create `backend/internal/repository/user_test.go`:

```go
package repository

import (
	"context"
	"testing"

	"github.com/tokenlive/tokenlive-portal/backend/internal/domain"
)

func TestCreateUserWithDefaultWorkspace(t *testing.T) {
	db := testDB(t)
	repos := New(db)

	result, err := repos.CreateUserWithDefaultWorkspace(context.Background(), CreateUserWithWorkspaceInput{
		DisplayName:   "Test User",
		PrimaryEmail:  "test-user@example.com",
		WorkspaceName: "Test Workspace",
		WorkspaceSlug: "test-workspace",
	})
	if err != nil {
		t.Fatalf("create user with workspace: %v", err)
	}
	if result.User.ID == "" || result.Workspace.ID == "" {
		t.Fatalf("expected user and workspace ids")
	}

	var member domain.WorkspaceMember
	if err := db.Where("workspace_id = ? AND user_id = ?", result.Workspace.ID, result.User.ID).First(&member).Error; err != nil {
		t.Fatalf("find owner member: %v", err)
	}
	if member.Role != domain.MemberRoleOwner {
		t.Fatalf("got role %s", member.Role)
	}
}
```

- [ ] **Step 3: Add billing repository integration test**

Create `backend/internal/repository/billing_test.go`:

```go
package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/tokenlive/tokenlive-portal/backend/internal/domain"
)

func TestCreateLedgerEntryIdempotent(t *testing.T) {
	db := testDB(t)
	repos := New(db)
	ctx := context.Background()

	result, err := repos.CreateUserWithDefaultWorkspace(ctx, CreateUserWithWorkspaceInput{
		DisplayName:   "Billing User",
		PrimaryEmail:  "billing-user@example.com",
		WorkspaceName: "Billing Workspace",
		WorkspaceSlug: "billing-workspace",
	})
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	input := CreateLedgerInput{
		WorkspaceID:    result.Workspace.ID,
		Type:           domain.LedgerTypeTrialGrant,
		Direction:      domain.LedgerDirectionCredit,
		AmountMicroCNY: 1_000_000,
		Currency:       "CNY",
		IdempotencyKey: "trial-grant:test",
	}

	first, err := repos.CreateLedgerEntry(ctx, input)
	if err != nil {
		t.Fatalf("create first ledger: %v", err)
	}
	second, err := repos.CreateLedgerEntry(ctx, input)
	if err != nil {
		t.Fatalf("replay ledger: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("expected idempotent replay to return same ledger")
	}

	input.AmountMicroCNY = 2_000_000
	_, err = repos.CreateLedgerEntry(ctx, input)
	if !errors.Is(err, ErrDuplicateLedgerConflict) {
		t.Fatalf("got err %v, want ErrDuplicateLedgerConflict", err)
	}
}
```

- [ ] **Step 4: Run repository tests without DSN**

Run:

```bash
cd backend
go test ./internal/repository
```

Expected without `PORTAL_TEST_DATABASE_DSN`: PASS with integration tests skipped.

- [ ] **Step 5: Run repository tests with DSN when MySQL exists**

Run:

```bash
cd backend
go test ./internal/repository -count=1
```

Expected with migrated test DB: PASS.

---

### Task 8: Add API And Worker Entrypoints

**Files:**

- Create: `backend/cmd/portal-api/main.go`
- Create: `backend/cmd/portal-worker/main.go`

- [ ] **Step 1: Add `portal-api` main**

Create `backend/cmd/portal-api/main.go`:

```go
package main

import (
	"log"
	"net/http"

	"github.com/tokenlive/tokenlive-portal/backend/internal/api"
	"github.com/tokenlive/tokenlive-portal/backend/internal/config"
)

func main() {
	cfg := config.Load()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", api.HealthHandler)

	handler := api.RequestID(mux)

	log.Printf("portal-api listening on %s env=%s", cfg.HTTPAddr, cfg.Env)
	if err := http.ListenAndServe(cfg.HTTPAddr, handler); err != nil {
		log.Fatalf("portal-api stopped: %v", err)
	}
}
```

- [ ] **Step 2: Add `portal-worker` main**

Create `backend/cmd/portal-worker/main.go`:

```go
package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/tokenlive/tokenlive-portal/backend/internal/config"
)

func main() {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Printf("portal-worker started env=%s", cfg.Env)
	<-ctx.Done()
	log.Printf("portal-worker stopped")
}
```

- [ ] **Step 3: Run build checks**

Run:

```bash
cd backend
go test ./...
go build ./cmd/portal-api
go build ./cmd/portal-worker
```

Expected: all commands PASS.

- [ ] **Step 4: Run API manually**

Run:

```bash
cd backend
PORTAL_HTTP_ADDR=:18080 go run ./cmd/portal-api
```

Expected: server logs `portal-api listening on :18080`.

In another shell:

```bash
curl -s http://localhost:18080/healthz
```

Expected:

```json
{"status":"ok"}
```

Stop the server with Ctrl-C.

---

## Final Verification

- [ ] Run Go formatting:

```bash
cd backend
gofmt -w ./cmd ./internal
```

- [ ] Run all tests:

```bash
cd backend
go test ./...
```

Expected: PASS. Repository integration tests skip if `PORTAL_TEST_DATABASE_DSN` is not set.

- [ ] Run build:

```bash
cd backend
go build ./cmd/portal-api
go build ./cmd/portal-worker
```

Expected: PASS.

- [ ] Check docs and SQL for unresolved marker strings:

```bash
rg -n 'TB[D]|TO[D]O|place''holder|fill[ ]in|implement[ ]later' backend docs
```

Expected: no matches.

- [ ] Check git diff:

```bash
git diff -- docs backend
```

Expected: only Foundation Slice files and docs are changed.
