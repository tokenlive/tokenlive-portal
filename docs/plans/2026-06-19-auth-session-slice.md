# Auth And Session Slice Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add session-cookie authentication and passwordless email-code login skeleton for Portal backend.

**Architecture:** Extend the existing Go backend with auth/session tables, domain models, security helpers, repository methods, middleware, and HTTP handlers. Email delivery and OAuth are intentionally deferred; development/test may return plaintext codes for manual testing.

**Tech Stack:** Go 1.24, standard `net/http`, GORM, MySQL, goose SQL migrations, HMAC-SHA256 token hashing, existing API error/request ID primitives.

---

## File Structure

Create:

- `backend/migrations/000003_auth_session.sql`
- `backend/internal/security/session.go`
- `backend/internal/security/session_test.go`
- `backend/internal/repository/auth.go`
- `backend/internal/repository/auth_test.go`
- `backend/internal/api/auth.go`
- `backend/internal/api/auth_test.go`

Modify:

- `backend/internal/domain/models.go`
- `backend/internal/config/config.go`
- `backend/internal/api/error.go`
- `backend/cmd/portal-api/main.go`

---

### Task 1: Migration, Config, Domain Models

**Files:**

- Create: `backend/migrations/000003_auth_session.sql`
- Modify: `backend/internal/config/config.go`
- Modify: `backend/internal/domain/models.go`

- [x] **Step 1: Create migration**

Create `backend/migrations/000003_auth_session.sql`:

```sql
-- +goose Up
CREATE TABLE user_sessions (
    id VARCHAR(32) PRIMARY KEY,
    user_id VARCHAR(32) NOT NULL,
    token_hash VARCHAR(128) NOT NULL,
    status VARCHAR(32) NOT NULL,
    ip VARCHAR(64) NOT NULL DEFAULT '',
    user_agent VARCHAR(512) NOT NULL DEFAULT '',
    expires_at DATETIME(3) NOT NULL,
    last_seen_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL,
    revoked_at DATETIME(3) NULL,
    UNIQUE KEY uk_user_sessions_token_hash (token_hash),
    KEY idx_user_sessions_user_status (user_id, status),
    KEY idx_user_sessions_expires_at (expires_at),
    CONSTRAINT fk_user_sessions_user FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE email_verification_codes (
    id VARCHAR(32) PRIMARY KEY,
    email VARCHAR(320) NOT NULL,
    purpose VARCHAR(32) NOT NULL,
    code_hash VARCHAR(128) NOT NULL,
    status VARCHAR(32) NOT NULL,
    attempt_count INT NOT NULL DEFAULT 0,
    expires_at DATETIME(3) NOT NULL,
    consumed_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL,
    KEY idx_email_codes_lookup (email, purpose, status, created_at),
    CONSTRAINT chk_email_codes_attempt_nonnegative CHECK (attempt_count >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE IF EXISTS email_verification_codes;
DROP TABLE IF EXISTS user_sessions;
```

- [x] **Step 2: Extend config**

Modify `backend/internal/config/config.go`:

```go
type Config struct {
    Env         string
    HTTPAddr    string
    DatabaseDSN string
    AuthPepper  string
}
```

`Load()` should set `AuthPepper` from `PORTAL_AUTH_PEPPER`, defaulting to `"dev-auth-pepper"` only for development convenience.

- [x] **Step 3: Add domain models**

Append to `backend/internal/domain/models.go`:

```go
type SessionStatus string
type EmailCodeStatus string
type EmailCodePurpose string

const (
    SessionStatusActive  SessionStatus = "active"
    SessionStatusRevoked SessionStatus = "revoked"

    EmailCodeStatusPending  EmailCodeStatus = "pending"
    EmailCodeStatusConsumed EmailCodeStatus = "consumed"
    EmailCodeStatusBlocked  EmailCodeStatus = "blocked"

    EmailCodePurposeLogin EmailCodePurpose = "login"
)

type UserSession struct {
    ID         string        `gorm:"primaryKey;size:32"`
    UserID     string        `gorm:"size:32;not null;index:idx_user_sessions_user_status,priority:1"`
    TokenHash  string        `gorm:"size:128;not null;uniqueIndex:uk_user_sessions_token_hash"`
    Status     SessionStatus `gorm:"size:32;not null;index:idx_user_sessions_user_status,priority:2"`
    IP         string        `gorm:"size:64;not null;default:''"`
    UserAgent  string        `gorm:"size:512;not null;default:''"`
    ExpiresAt  time.Time     `gorm:"not null;index"`
    LastSeenAt *time.Time
    CreatedAt  time.Time `gorm:"not null"`
    RevokedAt  *time.Time
}

type EmailVerificationCode struct {
    ID           string           `gorm:"primaryKey;size:32"`
    Email        string           `gorm:"size:320;not null;index:idx_email_codes_lookup,priority:1"`
    Purpose      EmailCodePurpose `gorm:"size:32;not null;index:idx_email_codes_lookup,priority:2"`
    CodeHash     string           `gorm:"size:128;not null"`
    Status       EmailCodeStatus  `gorm:"size:32;not null;index:idx_email_codes_lookup,priority:3"`
    AttemptCount int              `gorm:"not null;default:0"`
    ExpiresAt    time.Time        `gorm:"not null"`
    ConsumedAt   *time.Time
    CreatedAt    time.Time `gorm:"not null;index:idx_email_codes_lookup,priority:4"`
}
```

- [x] **Step 4: Format and test**

Run:

```bash
cd backend
gofmt -w internal/config/config.go internal/domain/models.go
GOCACHE=/private/tmp/go-build-cache go test ./internal/config ./internal/domain
```

Expected: PASS.

---

### Task 2: Session And Code Security Helpers

**Files:**

- Create: `backend/internal/security/session.go`
- Test: `backend/internal/security/session_test.go`

- [x] **Step 1: Add tests**

Tests should cover:

- `GenerateSessionToken` returns `tl_sess_` prefix and enough entropy.
- `GenerateEmailCode` returns six digits.
- `HashSecret` and `VerifySecret` work with HMAC-SHA256.
- Wrong secret fails verification.

- [x] **Step 2: Implement helpers**

Create helpers:

```go
const SessionCookieName = "tl_session"

func GenerateSessionToken() (string, error)
func GenerateEmailCode() (string, error)
func HashSecret(secret string, pepper string) string
func VerifySecret(secret string, pepper string, expectedHash string) bool
```

Use 32 random bytes for session tokens and cryptographic random digits for email code.

- [x] **Step 3: Test**

Run:

```bash
cd backend
GOCACHE=/private/tmp/go-build-cache go test ./internal/security
```

Expected: PASS.

---

### Task 3: Auth Repository

**Files:**

- Create: `backend/internal/repository/auth.go`
- Test: `backend/internal/repository/auth_test.go`

- [x] **Step 1: Implement repository methods**

Add:

- `ErrEmailCodeInvalid`
- `ErrEmailCodeBlocked`
- `ErrSessionNotFound`
- `CreateEmailVerificationCode`
- `VerifyEmailCode`
- `FindUserByPrimaryEmail`
- `MarkUserEmailVerified`
- `CreateSession`
- `FindActiveSessionByTokenHash`
- `RevokeSession`

Rules:

- `VerifyEmailCode` locks latest pending code for email+purpose.
- Wrong code increments `attempt_count`; at 5 attempts, status becomes `blocked`.
- Expired code returns invalid.
- Successful code sets status consumed and consumed_at.
- `FindActiveSessionByTokenHash` rejects revoked/expired sessions.
- `RevokeSession` checks RowsAffected and returns `ErrSessionNotFound` on zero rows.

- [x] **Step 2: Add integration tests**

Use `testDB(t)` and `uniqueSuffix(t)`.

Tests:

- Code verifies once and cannot be reused.
- Wrong code blocks after 5 attempts.
- Expired session is rejected.

Tests skip without `PORTAL_TEST_DATABASE_DSN`.

- [x] **Step 3: Run tests**

```bash
cd backend
GOCACHE=/private/tmp/go-build-cache go test ./internal/repository -count=1 -v
GOCACHE=/private/tmp/go-build-cache go test ./...
```

Expected: PASS; DB tests skip without DSN.

---

### Task 4: Auth API And Middleware

**Files:**

- Modify: `backend/internal/api/error.go`
- Create: `backend/internal/api/auth.go`
- Test: `backend/internal/api/auth_test.go`

- [x] **Step 1: Add auth errors**

Add:

- `auth.invalid_email`
- `auth.invalid_code`
- `auth.session_required`
- `auth.session_expired`

- [x] **Step 2: Implement AuthService interface and handlers**

Use handler-facing interface so tests can fake it:

```go
type AuthService interface {
    StartEmailLogin(ctx context.Context, email string) (StartEmailLoginResult, error)
    VerifyEmailLogin(ctx context.Context, input VerifyEmailLoginInput) (VerifyEmailLoginResult, error)
    CurrentUser(ctx context.Context, sessionToken string) (CurrentUser, error)
    Logout(ctx context.Context, sessionToken string) error
}
```

Implement:

- `POST /api/auth/email/start`
- `POST /api/auth/email/verify`
- `GET /api/me`
- `POST /api/auth/logout`

Cookie helper:

- set `tl_session` on verify.
- clear `tl_session` on logout.
- secure flag when env is production.

Middleware:

- `RequireSession(service AuthService, next http.Handler) http.Handler`

For this slice, route handlers may call service directly; middleware exists for future routes and `/api/me`.

- [x] **Step 3: Add handler unit tests**

Tests:

- start rejects invalid email with `auth.invalid_email`.
- start returns dev_code when fake returns it.
- verify sets `tl_session` cookie.
- me requires cookie.
- logout clears cookie.

- [x] **Step 4: Run tests**

```bash
cd backend
GOCACHE=/private/tmp/go-build-cache go test ./internal/api
GOCACHE=/private/tmp/go-build-cache go test ./...
```

Expected: PASS.

---

### Task 5: Auth Service And Route Wiring

**Files:**

- Create: `backend/internal/api/auth_service.go`
- Modify: `backend/cmd/portal-api/main.go`

- [x] **Step 1: Implement service**

`auth_service.go` should adapt repositories/security helpers to `AuthService`.

Responsibilities:

- Normalize and validate email.
- Create email verification code.
- Return dev code only when env is `development` or `test`.
- Verify code.
- Find existing user by email or create user with default Workspace.
- Mark email verified.
- Create session.
- Resolve current user from session token.
- Logout by session token.

Default Workspace naming:

- Workspace name: email local part before `@`
- Slug: `personal-` + generated suffix from repository ID helper or a deterministic safe fallback.

Keep this simple; refinement can happen in Workspace Console slice.

- [x] **Step 2: Wire routes**

In `portal-api`:

- If DB DSN exists, create auth service and register auth routes.
- If DB DSN is empty, auth routes may be disabled with a log.
- Public model route behavior remains unchanged.

- [x] **Step 3: Verify**

```bash
cd backend
gofmt -w cmd/portal-api/main.go internal/api internal/repository internal/security internal/domain internal/config
GOCACHE=/private/tmp/go-build-cache go test ./...
GOCACHE=/private/tmp/go-build-cache go build -o /private/tmp/tokenlive-portal-api ./cmd/portal-api
GOCACHE=/private/tmp/go-build-cache go build -o /private/tmp/tokenlive-portal-worker ./cmd/portal-worker
```

Expected: PASS.

---

## Final Verification

Run:

```bash
cd backend
GOCACHE=/private/tmp/go-build-cache go test ./...
GOCACHE=/private/tmp/go-build-cache go build -o /private/tmp/tokenlive-portal-api ./cmd/portal-api
GOCACHE=/private/tmp/go-build-cache go build -o /private/tmp/tokenlive-portal-worker ./cmd/portal-worker
```

Then from repo root:

```bash
find backend -maxdepth 2 -type f -perm +111 -print
rg -n 'TB[D]|TO[D]O|place''holder|fill[ ]in|implement[ ]later' backend docs
```

Expected:

- Tests pass.
- Builds pass.
- No build binaries under `backend`.
- Marker scan only reports intentional ICP placeholder lines in product docs and plan verification text.
