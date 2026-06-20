# Workspace API Key Slice Design

Date: 2026-06-20

## 1. Goal

Build the first authenticated Workspace Console backend slice for TokenLive Portal.

This slice lets a logged-in user inspect their current Workspace and manage API keys. It moves the product activation path from "can log in" to "can create a key and make the first real API call through the existing gateway runtime."

This slice does not implement frontend pages, gateway synchronization, request logs, recharge workflow, member invitation, or multi-Workspace switching.

## 2. Scope

### Included

Backend APIs:

- `GET /api/workspaces/current`
- `GET /api/api-keys`
- `POST /api/api-keys`
- `POST /api/api-keys/{id}/enable`
- `POST /api/api-keys/{id}/disable`
- `POST /api/api-keys/{id}/revoke`

Backend code:

- Authenticated Workspace resolution from the current session user.
- Role checks for API key operations.
- API key list and mutation service.
- API handlers and route wiring.
- Repository methods for current Workspace and API key reads.
- Audit log records for API key creation and state changes.
- Tests for handlers, service behavior, repository behavior, and route wiring.

Security behavior:

- Full API key secret is returned only once during `POST /api/api-keys`.
- Plaintext API key secret is never stored.
- Subsequent list responses show only `key_prefix`, `secret_last4`, status, limits, and timestamps.

### Excluded

Not included:

- Next.js frontend.
- Multiple Workspace switcher.
- Workspace creation API.
- Workspace invitation and member management.
- Google/GitHub OAuth.
- Billing recharge request flow.
- Usage and request logs.
- API key model whitelist editing.
- Gateway runtime key push or synchronization.
- IP allowlist, domain restriction, or fine-grained endpoint permissions.
- CSRF protection and rate limiting.

## 3. Current Workspace Resolution

The Portal console needs one default Workspace before a switcher exists.

Rules:

- All APIs require a valid `tl_session` cookie.
- Resolve the current user from the session using the existing AuthService.
- Select the user's active Workspace membership using this priority:
  1. Active Workspace where `workspaces.owner_user_id = current_user.id`.
  2. Otherwise the oldest active Workspace membership for the user.
- The response includes the user's role in that Workspace.
- If the user has no active Workspace membership, return `workspace.not_found`.

This is intentionally simple. A future multi-Workspace switcher will add explicit Workspace selection and persistence.

## 4. Role Rules

Workspace roles:

- `owner`
- `developer`
- `billing`

Permissions in this slice:

- `owner` can list, create, enable, disable, and revoke API keys.
- `developer` can list, create, enable, disable, and revoke API keys.
- `billing` cannot list or manage API keys.

Rejected role checks return `workspace.permission_denied`.

The first release does not add custom roles.

## 5. API Behavior

### `GET /api/workspaces/current`

Requires valid session.

Response:

```json
{
  "workspace": {
    "id": "wsp_xxx",
    "name": "dev",
    "slug": "personal-xxx",
    "role": "owner",
    "status": "active",
    "balance": {
      "available_micro_cny": 10000000,
      "frozen_micro_cny": 0,
      "available_cny": "10.00",
      "frozen_cny": "0.00"
    }
  }
}
```

Money is stored as integer `micro_cny`, but responses include display strings in CNY yuan because users should see yuan.

### `GET /api/api-keys`

Requires valid session and `owner` or `developer` role.

Returns API keys in the current Workspace, newest first.

Response:

```json
{
  "data": [
    {
      "id": "ak_xxx",
      "name": "local dev",
      "key_prefix": "tl_live_abc123",
      "secret_last4": "wxyz",
      "status": "enabled",
      "expires_at": null,
      "daily_limit_micro_cny": 5000000,
      "daily_limit_cny": "5.00",
      "monthly_limit_micro_cny": null,
      "monthly_limit_cny": null,
      "last_used_at": null,
      "total_spend_micro_cny": 0,
      "total_spend_cny": "0.00",
      "created_at": "2026-06-20T00:00:00Z",
      "updated_at": "2026-06-20T00:00:00Z"
    }
  ]
}
```

The full secret is never returned from this API.

### `POST /api/api-keys`

Requires valid session and `owner` or `developer` role.

Request:

```json
{
  "name": "local dev",
  "daily_limit_micro_cny": 5000000,
  "monthly_limit_micro_cny": null,
  "expires_at": null
}
```

Rules:

- `name` is required after trimming spaces.
- `name` max length is 160 characters.
- `daily_limit_micro_cny` is optional and must be nonnegative when provided.
- `monthly_limit_micro_cny` is optional and must be nonnegative when provided.
- `expires_at` is optional and must be in the future when provided.
- If `daily_limit_micro_cny` is omitted, default to 50% of the Workspace daily budget when that budget exists. Since Workspace daily budget is not implemented yet, omit the per-key daily limit in this slice.
- New API keys are `enabled`.

Response:

```json
{
  "api_key": {
    "id": "ak_xxx",
    "name": "local dev",
    "key_prefix": "tl_live_abc123",
    "secret_last4": "wxyz",
    "status": "enabled",
    "expires_at": null,
    "daily_limit_micro_cny": 5000000,
    "daily_limit_cny": "5.00",
    "monthly_limit_micro_cny": null,
    "monthly_limit_cny": null,
    "last_used_at": null,
    "total_spend_micro_cny": 0,
    "total_spend_cny": "0.00",
    "created_at": "2026-06-20T00:00:00Z",
    "updated_at": "2026-06-20T00:00:00Z"
  },
  "secret": "tl_live_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
}
```

The `secret` field appears only in this creation response.

### `POST /api/api-keys/{id}/enable`

Requires valid session and `owner` or `developer` role.

Rules:

- Only keys in the current Workspace can be modified.
- Revoked keys cannot be enabled.
- Already enabled keys return success with the current representation.

### `POST /api/api-keys/{id}/disable`

Requires valid session and `owner` or `developer` role.

Rules:

- Only keys in the current Workspace can be modified.
- Revoked keys cannot be disabled.
- Already disabled keys return success with the current representation.

### `POST /api/api-keys/{id}/revoke`

Requires valid session and `owner` or `developer` role.

Rules:

- Only keys in the current Workspace can be revoked.
- Revocation sets status `revoked` and `revoked_at`.
- Revoked keys remain visible in the list for auditability.
- Revoking an already revoked key returns success with the current representation.

## 6. API Key Format

Generated plaintext API key format:

```text
tl_live_<random>
```

Storage:

- `key_prefix`: first visible prefix segment for UI identification.
- `secret_last4`: last four characters of the plaintext key.
- `key_hash`: HMAC-SHA256 hash using `PORTAL_AUTH_PEPPER`.

The existing `internal/security` API key helpers should be reused when possible.

## 7. Error Codes

Add:

- `workspace.not_found`
- `api_key.not_found`
- `api_key.invalid_name`
- `api_key.invalid_limit`
- `api_key.invalid_expiration`

Use existing:

- `auth.session_required`
- `auth.session_expired`
- `auth.unauthorized`
- `workspace.permission_denied`
- `validation.invalid_request`
- `internal.error`

## 8. Repository Design

Add repository behavior to `repository.Repositories`:

- Resolve current active Workspace for a user.
- Get Workspace balance.
- List API keys by Workspace.
- Create API key metadata.
- Update API key status within a Workspace.
- Write audit logs for create, enable, disable, and revoke.

Rules:

- Repository methods must scope API key updates by `workspace_id`.
- Status transitions should not accidentally restore revoked keys.
- API key creation and audit log insertion should happen in one transaction.
- API key state changes and audit log insertion should happen in one transaction.

## 9. Service Design

Add a console/API key service in `internal/api` or a small service package following the existing AuthService style.

Responsibilities:

- Resolve current user and Workspace.
- Enforce role checks.
- Validate create request.
- Generate API key secret.
- Hash secret and persist metadata.
- Convert money fields to both micro-CNY and display CNY strings.
- Map repository errors to stable API errors.

Handlers should not call GORM directly.

## 10. Testing Strategy

Unit tests:

- Handler requires a session cookie.
- Billing role receives `workspace.permission_denied` for API key list and mutation.
- API key create validates name, limits, and expiration.
- API key create returns full secret once.
- API key list never returns full secret.
- Enable, disable, and revoke use current Workspace scoping.
- `GET /api/workspaces/current` includes balance in micro-CNY and yuan display strings.

Repository integration tests:

- Current Workspace resolution selects owned Workspace first.
- Current Workspace falls back to oldest active membership.
- API key list is scoped to Workspace.
- API key status update cannot modify another Workspace's key.
- Revoked key cannot be re-enabled.
- API key create writes audit log in the same transaction.

Integration tests skip cleanly if `PORTAL_TEST_DATABASE_DSN` is unset.

## 11. Acceptance Criteria

This slice is complete when:

- Authenticated current Workspace API exists.
- API key list/create/enable/disable/revoke APIs exist.
- `owner` and `developer` can manage API keys.
- `billing` cannot view or manage API keys.
- Full API key secret is returned only at creation.
- API key plaintext is never stored.
- State changes are scoped by Workspace.
- Audit logs are written for API key mutations.
- `portal-api` registers the new routes only when DB-backed auth is enabled.
- `go test ./...` passes.
- `portal-api` and `portal-worker` build.
