# Foundation Slice Design

Date: 2026-06-19

## 1. Goal

Build the first backend foundation for TokenLive Portal without touching the frontend yet.

This slice creates the backend project shape and implements the domain foundation that later Portal features depend on:

- Identity
- Workspace
- Membership and invitations
- API key records and model permissions
- Workspace balance
- Immutable ledger
- Audit log
- Shared API response and error shape
- API and worker entrypoints

The goal is not to ship a usable Portal UI. The goal is to make the backend ground solid enough that model catalog, usage, recharge, tickets, and dashboard work can be added without reworking identity or money storage.

## 2. Scope

### Included

Project structure:

- `backend/` as an independent Go module.
- `backend/cmd/portal-api` HTTP API entrypoint.
- `backend/cmd/portal-worker` worker entrypoint.
- `backend/migrations/` goose SQL migrations.
- `backend/internal/domain/` GORM models.
- `backend/internal/repository/` repository layer.
- `backend/internal/api/` shared HTTP API primitives.
- `backend/internal/config/` configuration loading.
- `backend/internal/database/` MySQL connection setup.

Foundation entities:

- `users`
- `account_identities`
- `workspaces`
- `workspace_members`
- `workspace_invitations`
- `api_keys`
- `workspace_model_permissions`
- `api_key_model_whitelists`
- `workspace_balances`
- `ledger_entries`
- `audit_logs`

Core repository behavior:

- Create user with default Workspace.
- Create additional Workspace with self-created limit check.
- Add and query Workspace members.
- Create Workspace invitation.
- Create API key metadata using hash-only storage.
- Update API key status.
- Add Workspace model permission.
- Set API key whitelist.
- Insert ledger entry and update balance in one transaction.
- Enforce idempotent ledger writes.
- Write audit log entries for sensitive actions.

API primitives:

- Request ID middleware.
- Standard JSON error response.
- Stable business error code type.
- `/healthz` endpoint.

Worker:

- Buildable `portal-worker` entrypoint.
- No business jobs in this slice.

Testing:

- Unit tests for money helpers, API key secret hashing, and error response.
- Repository tests for ledger idempotency and balance transaction if local MySQL is available.
- If MySQL is not available, repository integration tests are structured but skipped behind environment configuration.

### Excluded

Not included in this slice:

- Next.js frontend.
- OAuth login flow.
- Email verification delivery.
- Model catalog tables.
- Price version tables.
- Request logs.
- Usage aggregates.
- Recharge request flow.
- Support tickets.
- Status incidents.
- Notification events.
- OpenAPI generation.
- Full authentication middleware.
- Gateway synchronization.
- Admin integration APIs.
- Docker images.

## 3. Recommended Approach

Use the **Backend Foundation Only** approach.

Why:

- The Portal repo is currently empty.
- The riskiest early decisions are data ownership, money storage, API key storage, and transaction boundaries.
- Building frontend and backend at the same time would add package-manager and UI decisions before the backend contracts are stable.
- Writing only migrations would miss repository-level invariants and transaction behavior.

This slice should leave the project in a state where:

- Migrations can create the first database schema.
- Go code can compile.
- Critical money and key-storage rules are represented in code.
- Later slices can add API endpoints without reshaping core storage.

## 4. Architecture

```text
backend/
  cmd/
    portal-api/
      main.go
    portal-worker/
      main.go
  internal/
    api/
    config/
    database/
    domain/
    repository/
    security/
  migrations/
  go.mod
```

### API Process

`portal-api` starts HTTP server, installs request ID middleware, exposes `/healthz`, and wires configuration plus database dependencies.

It does not expose full product APIs in this slice.

### Worker Process

`portal-worker` starts from the same Go module and configuration package. It should be buildable and ready for future jobs such as email, reconciliation, cleanup, and aggregation.

It can log startup and exit cleanly or wait on context with no jobs enabled.

### Domain Layer

`internal/domain` contains persistence models and shared enum constants.

GORM tags may be used for mapping, but migration SQL is the source of schema truth.

### Repository Layer

`internal/repository` owns database access and transaction boundaries.

Business services and API handlers should not call GORM directly. Even in this first slice, repository methods should make the intended invariants visible:

- Ledger and balance update happen in one transaction.
- Idempotent writes detect duplicate keys.
- API keys never expose plaintext after creation.
- Workspace membership is checked through explicit queries.

## 5. Schema Design

### IDs And Time

Use string IDs for externally visible records. Recommended prefix style:

- `usr_`
- `aid_`
- `wsp_`
- `winv_`
- `ak_`
- `led_`
- `aud_`

Use `datetime(3)` for timestamps in MySQL.

Every table should have:

- `created_at`
- `updated_at` where records are mutable

Soft-deletable tables use:

- `deleted_at`

### Money

All amounts use integer `micro_cny`.

```text
1 CNY = 1,000,000 micro_cny
```

Frontend and API display money as CNY yuan later, but storage and calculations stay integer.

### API Key Storage

The database stores:

- `key_prefix`
- `secret_last4`
- `key_hash`

Plaintext key is never stored.

Recommended generated key format:

```text
tl_live_<random_secret>
```

The full key is returned only at creation time.

### Ledger And Balance

`ledger_entries` is immutable.

`workspace_balances` is the fast read model.

Every balance mutation:

1. Inserts one `ledger_entries` row.
2. Updates `workspace_balances`.
3. Commits both in the same transaction.

`ledger_entries.idempotency_key` is unique per workspace for general entries.

Consumption entries later also use unique `request_id`.

## 6. Repository Contracts

### UserRepository

Responsibilities:

- Create User.
- Find User by ID.
- Find or attach AccountIdentity.
- Create initial User plus default Workspace inside one transaction.

### WorkspaceRepository

Responsibilities:

- Create Workspace.
- Enforce self-created Workspace limit.
- Add owner member.
- Add, update, and remove members.
- Create and revoke invitations.

### ApiKeyRepository

Responsibilities:

- Create API key metadata with hashed secret.
- Update status: enabled, disabled, revoked.
- Update spend limits.
- Store model whitelist.
- Query effective key metadata for Portal display.

Runtime synchronization to Gateway is excluded from this slice.

### BillingRepository

Responsibilities:

- Get Workspace balance.
- Insert ledger and update balance in one transaction.
- Return existing ledger on idempotent replay.
- Flag conflicting idempotency payloads.

### AuditRepository

Responsibilities:

- Append audit events.
- Audit events are not updated after insertion.

## 7. Error Handling

Use stable business error codes.

API error shape:

```json
{
  "error": {
    "code": "workspace.insufficient_balance",
    "message": "Insufficient balance",
    "request_id": "req_xxx"
  }
}
```

Rules:

- `code` is stable and documented.
- `message` is English developer-facing text.
- `request_id` is always present in API responses.
- Internal stack traces are never returned to clients.

Initial error codes:

- `internal.error`
- `validation.invalid_request`
- `auth.unauthorized`
- `workspace.not_found`
- `workspace.limit_exceeded`
- `workspace.permission_denied`
- `api_key.not_found`
- `api_key.invalid_state`
- `billing.duplicate_conflict`
- `billing.insufficient_balance`

## 8. Configuration

Use environment variables for first slice:

- `PORTAL_HTTP_ADDR`
- `PORTAL_DATABASE_DSN`
- `PORTAL_ENV`

No configuration file is required yet.

## 9. Testing Strategy

### Unit Tests

Cover:

- `micro_cny` formatting/parsing helpers if implemented.
- API key generation and hash verification.
- Error response serialization.
- Request ID propagation.

### Repository Integration Tests

If `PORTAL_TEST_DATABASE_DSN` is set:

- Run migrations.
- Create user and default Workspace.
- Create additional Workspaces and verify self-created limit.
- Insert recharge/trial ledger and verify balance.
- Replay same idempotency key and verify no duplicate ledger.
- Replay same idempotency key with conflicting payload and verify conflict error.

If the test DSN is not set, integration tests skip cleanly.

## 10. Implementation Order

1. Create Go module and directory structure.
2. Add core dependencies:
   - Gin or standard `net/http` for `/healthz`
   - GORM MySQL driver
   - goose CLI dependency or migration docs
   - UUID/ID helper if needed
3. Write first migration file for Foundation tables.
4. Add domain models and enums.
5. Add database connection package.
6. Add security helpers for API key generation and hashing.
7. Add repository interfaces and GORM implementations.
8. Add API request ID and error primitives.
9. Add `portal-api` and `portal-worker` entrypoints.
10. Add unit tests and optional repository integration tests.

## 11. Risks And Mitigations

Risk: Schema becomes too broad in the first migration.

Mitigation: Limit this slice to identity, workspace, API key, balance, ledger, and audit only.

Risk: Ledger and balance drift.

Mitigation: Implement ledger insertion and balance update in one repository transaction from the first slice.

Risk: API key plaintext leaks into logs or DB.

Mitigation: Generate secret in security package, return plaintext only from creation method, store only hash, prefix, and last four characters.

Risk: MySQL is not available locally.

Mitigation: Keep migrations and Go code buildable; repository integration tests skip unless `PORTAL_TEST_DATABASE_DSN` is provided.

Risk: Admin and Gateway integration assumptions leak into Foundation code too early.

Mitigation: Model the authoritative Portal state only; leave synchronization interfaces for later slices.

## 12. Acceptance Criteria

This slice is complete when:

- `backend` Go module exists.
- `portal-api` and `portal-worker` build.
- Goose migrations define all included Foundation tables.
- Domain models exist for included tables.
- Repository layer implements core Foundation transaction behavior.
- API key helper never stores plaintext secret.
- Ledger and balance transaction behavior is covered by tests or clearly skipped without DSN.
- `/healthz` returns a healthy response from `portal-api`.
- Product and architecture docs remain consistent with implementation choices.

