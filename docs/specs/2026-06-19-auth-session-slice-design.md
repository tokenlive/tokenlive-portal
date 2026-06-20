# Auth And Session Slice Design

Date: 2026-06-19

## 1. Goal

Add the first authenticated-user foundation for TokenLive Portal.

This slice introduces session persistence, session cookies, current-user APIs, and a passwordless email-code login skeleton. It prepares the backend for Workspace Console and API Key management APIs.

This slice does not implement real email delivery, Google OAuth, GitHub OAuth, frontend pages, device fingerprinting, or risk scoring.

## 2. Scope

### Included

Database tables:

- `user_sessions`
- `email_verification_codes`

Backend code:

- Domain models for sessions and email verification codes.
- Repository methods for session lifecycle.
- Repository methods for email verification code lifecycle.
- Session token generation and hashing.
- Auth middleware for session cookie authentication.
- APIs:
  - `GET /api/me`
  - `POST /api/auth/logout`
  - `POST /api/auth/email/start`
  - `POST /api/auth/email/verify`

Development behavior:

- Email code generation is stored hashed in DB.
- In development/test environment, `POST /api/auth/email/start` may return the plaintext code in the JSON response to avoid requiring email delivery.
- In production, plaintext code is never returned.

### Excluded

Not included:

- Real email sending.
- Google OAuth callback.
- GitHub OAuth callback.
- Account binding UI.
- Workspace permission APIs.
- CSRF protection.
- Rate limiting.
- Device fingerprinting.
- Risk scoring.
- Admin user management.

## 3. Session Model

### `user_sessions`

Key columns:

- `id VARCHAR(32) PRIMARY KEY`
- `user_id VARCHAR(32) NOT NULL`
- `token_hash VARCHAR(128) NOT NULL UNIQUE`
- `status VARCHAR(32) NOT NULL`
- `ip VARCHAR(64) NOT NULL DEFAULT ''`
- `user_agent VARCHAR(512) NOT NULL DEFAULT ''`
- `expires_at DATETIME(3) NOT NULL`
- `last_seen_at DATETIME(3) NULL`
- `created_at DATETIME(3) NOT NULL`
- `revoked_at DATETIME(3) NULL`

Rules:

- Plain session token is never stored.
- Cookie stores the plaintext session token.
- Database stores HMAC-SHA256 hash of the token.
- Session default TTL is 30 days.
- Logout revokes current session.
- Expired or revoked sessions are not accepted.

Cookie:

- Name: `tl_session`
- `HttpOnly`
- `SameSite=Lax`
- `Path=/`
- `Secure` when `PORTAL_ENV=production`
- Max-Age matches session TTL

## 4. Email Verification Code Model

### `email_verification_codes`

Key columns:

- `id VARCHAR(32) PRIMARY KEY`
- `email VARCHAR(320) NOT NULL`
- `purpose VARCHAR(32) NOT NULL`
- `code_hash VARCHAR(128) NOT NULL`
- `status VARCHAR(32) NOT NULL`
- `attempt_count INT NOT NULL DEFAULT 0`
- `expires_at DATETIME(3) NOT NULL`
- `consumed_at DATETIME(3) NULL`
- `created_at DATETIME(3) NOT NULL`

Rules:

- Plain code is never stored.
- Default code length is 6 digits.
- Code TTL is 10 minutes.
- Maximum verify attempts is 5.
- Successful verification consumes the code.
- Reuse after consumption fails.
- Purpose for this slice: `login`.

## 5. API Behavior

### `POST /api/auth/email/start`

Request:

```json
{
  "email": "dev@example.com"
}
```

Behavior:

- Normalize email by trimming spaces and lowercasing.
- Reject invalid email format.
- Create a `login` verification code.
- In development/test, return `dev_code`.
- In production, return only `sent: true`.

Response:

```json
{
  "sent": true,
  "dev_code": "123456"
}
```

`dev_code` is omitted outside development/test.

### `POST /api/auth/email/verify`

Request:

```json
{
  "email": "dev@example.com",
  "code": "123456"
}
```

Behavior:

- Normalize email.
- Verify latest pending login code for that email.
- If no User exists with that email, create a User and default Workspace.
- Mark email verified.
- Create a session.
- Set `tl_session` cookie.
- Return current user.

### `GET /api/me`

Requires valid session cookie.

Returns:

```json
{
  "user": {
    "id": "usr_xxx",
    "display_name": "",
    "primary_email": "dev@example.com",
    "email_verified": true
  }
}
```

### `POST /api/auth/logout`

Requires valid session cookie.

Behavior:

- Revoke current session.
- Clear `tl_session` cookie.
- Return `{ "ok": true }`.

## 6. Error Codes

Add:

- `auth.invalid_email`
- `auth.invalid_code`
- `auth.session_required`
- `auth.session_expired`

Existing `auth.unauthorized` remains available for generic auth failures.

## 7. Repository Design

Add to existing `repository.Repositories`:

- `CreateEmailVerificationCode`
- `VerifyEmailCode`
- `CompleteEmailLogin`
- `FindUserByPrimaryEmail`
- `MarkUserEmailVerified`
- `CreateSession`
- `FindActiveSessionByTokenHash`
- `RevokeSession`

Verification must be transactional:

- Lock the latest pending code row.
- Increment attempts on failed verification.
- Consume code on success.

`POST /api/auth/email/verify` uses `CompleteEmailLogin` so code verification, user/default Workspace creation, email verification, session creation, and code consumption commit or roll back together. A later failure in user/session creation must not burn the one-time code.

## 8. Testing Strategy

Unit tests:

- Email normalization and validation.
- Session token hash/verify helper.
- Cookie creation flags.
- Handler tests with fake auth service.

Repository integration tests:

- Email code verification succeeds once.
- Complete email login creates user/session and consumes the code atomically.
- Complete email login rolls back code consumption when session creation fails.
- Wrong code increments attempts and eventually blocks.
- Consumed code cannot be reused.
- Expired session is rejected.

Integration tests skip cleanly if `PORTAL_TEST_DATABASE_DSN` is unset.

## 9. Acceptance Criteria

This slice is complete when:

- Migration `000003_auth_session.sql` exists.
- Domain models exist for sessions and email codes.
- Session/token helper code exists and is tested.
- Auth repository methods exist and are tested.
- Auth middleware exists.
- Email start/verify, me, and logout handlers exist.
- `portal-api` registers auth routes.
- `go test ./...` passes.
- `portal-api` and `portal-worker` build.
