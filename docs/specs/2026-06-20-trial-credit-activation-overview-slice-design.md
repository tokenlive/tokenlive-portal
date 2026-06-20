# Trial Credit And Activation Overview Slice Design

Date: 2026-06-20

## 1. Goal

Close the next gap in the Portal activation path:

```text
Browse models -> Sign up -> Receive trial credit -> Create API key -> Make first real API call
```

Previous slices let users browse public models, log in, resolve their current Workspace, and create API keys. This slice makes a newly registered Workspace immediately usable by granting a small trial credit and exposing an activation-focused console overview.

The slice remains Portal-only. It does not redesign or implement Gateway runtime behavior.

## 2. Scope

### Included

- Grant trial credit when email login creates a new user and their first default Workspace.
- Store the grant as a `trial_grant` ledger entry and update `workspace_balances` in the same transaction.
- Mark `workspaces.trial_granted_at`.
- Make the trial amount configurable, defaulting to 10 yuan.
- Record the 7-day trial window in ledger metadata for user-facing explanation and future enforcement.
- Add `GET /api/console/overview`.
- Return activation steps for the console dashboard:
  - trial credit granted
  - API key created
  - first call made
- Add repository, service, handler, route, and config tests.

### Excluded

- Frontend pages.
- Gateway runtime synchronization.
- Runtime balance checks.
- Automatic trial expiration or balance clawback.
- Trial model whitelist enforcement.
- Daily trial spend limit enforcement.
- Abuse/risk scoring.
- Manual recharge workflow.
- Usage and request log ingestion.
- Multi-Workspace switching.
- OAuth login.

## 3. Trial Credit Rules

Rules:

- Trial credit is granted only when `CompleteEmailLogin` creates a brand-new user and that user's first default Workspace.
- Existing users who log in again do not receive another trial grant.
- Additional Workspace creation does not receive trial credit.
- Invited Workspaces do not receive trial credit.
- A Workspace with non-null `trial_granted_at` must never receive another trial grant through this flow.
- The default trial amount is `10.000000` CNY, stored as `10_000_000` micro-CNY.
- The default trial window is 7 days from grant time.
- Trial credit is denominated in CNY.

Configuration:

- `PORTAL_TRIAL_CREDIT_MICRO_CNY`
  - Default: `10000000`.
  - Must be greater than or equal to zero.
  - `0` disables automatic trial credit while still allowing registration.
- `PORTAL_TRIAL_CREDIT_TTL_DAYS`
  - Default: `7`.
  - Must be greater than zero.

The first release stores money as integer micro-CNY and returns display strings in yuan for user-facing responses.

## 4. Data Model Usage

No new table is required.

Existing fields used by this slice:

- `workspaces.trial_granted_at`
- `workspace_balances.available_micro_cny`
- `workspace_balances.version`
- `ledger_entries.type`
- `ledger_entries.direction`
- `ledger_entries.amount_micro_cny`
- `ledger_entries.balance_after_micro_cny`
- `ledger_entries.idempotency_key`
- `ledger_entries.metadata`

Ledger entry for a grant:

```text
type: trial_grant
direction: credit
amount_micro_cny: configured trial amount
currency: CNY
idempotency_key: trial-grant:<workspace_id>
```

Metadata:

```json
{
  "source": "email_registration",
  "trial_expires_at": "2026-06-27T00:00:00Z",
  "trial_ttl_days": 7
}
```

The expiry metadata is explanatory in this slice. Enforcement will belong to the future usage/billing/runtime integration slices.

## 5. Registration Data Flow

`CompleteEmailLogin` already performs code verification, user/default Workspace creation, session creation, and code consumption in one transaction. This slice extends that transaction when a new default Workspace is created.

Flow:

1. Lock the latest pending email code.
2. Verify the submitted code.
3. Find existing user by email.
4. If user exists:
   - mark email verified
   - create session
   - consume code
   - do not grant trial credit
5. If user does not exist:
   - create user
   - create default Workspace
   - create owner membership
   - create zero balance
   - if configured trial amount is greater than zero:
     - lock the balance row
     - create `trial_grant` ledger entry with idempotency key `trial-grant:<workspace_id>`
     - increment balance
     - set `workspaces.trial_granted_at`
   - mark email verified
   - create session
   - consume code

Any failure rolls back the full login transaction so a one-time email code is not consumed without the matching user/session/trial state.

## 6. Repository Design

Add a small internal helper that grants trial credit using an existing transaction handle.

Suggested shape:

```go
type TrialCreditConfig struct {
    AmountMicroCNY int64
    TTLDays        int
}

func grantTrialCreditInTx(tx *gorm.DB, workspaceID string, now time.Time, cfg TrialCreditConfig) error
```

Behavior:

- Return without mutation when `AmountMicroCNY == 0`.
- Lock the Workspace row.
- If `trial_granted_at` is already set, return without mutation.
- Lock the Workspace balance row.
- Insert the ledger entry.
- Update the balance.
- Set `trial_granted_at`.

The helper should share the same money mutation semantics as `CreateLedgerEntry`. If duplication becomes awkward, extract a private ledger/balance mutation helper used by both paths, but avoid a broad billing refactor.

## 7. Overview API

Endpoint:

```text
GET /api/console/overview
```

Requires valid `tl_session` cookie.

All active Workspace roles may view the overview:

- `owner`
- `developer`
- `billing`

Response:

```json
{
  "workspace": {
    "id": "wsp_xxx",
    "name": "dev",
    "slug": "personal-xxx",
    "role": "owner",
    "status": "active",
    "trial_granted_at": "2026-06-20T00:00:00Z",
    "balance": {
      "available_micro_cny": 10000000,
      "frozen_micro_cny": 0,
      "available_cny": "10.000000",
      "frozen_cny": "0.000000"
    }
  },
  "activation": {
    "trial_credit_granted": true,
    "trial_expires_at": "2026-06-27T00:00:00Z",
    "api_key_created": true,
    "first_call_made": false,
    "steps": [
      {
        "key": "trial_credit",
        "label": "Receive trial credit",
        "status": "completed"
      },
      {
        "key": "api_key",
        "label": "Create API key",
        "status": "completed"
      },
      {
        "key": "first_call",
        "label": "Make first API call",
        "status": "pending"
      }
    ]
  }
}
```

`first_call_made` returns `false` until the usage/request-log slice adds a durable signal. This keeps the contract stable without inventing runtime integration.

## 8. Service Design

Extend the console service with:

```go
Overview(ctx context.Context, user CurrentUser) (ConsoleOverviewResponse, error)
```

The service should:

- Resolve current Workspace using the existing `ResolveCurrentWorkspace`.
- Count API keys in that Workspace or reuse `ListAPIKeysByWorkspace`.
- Derive `trial_credit_granted` from `workspace.trial_granted_at != nil`.
- Derive `trial_expires_at` from `trial_granted_at + configured TTL`.
- Derive `api_key_created` from key count greater than zero.
- Return `first_call_made = false` for now.
- Build ordered activation steps from those booleans.

The API key count should be scoped to the current Workspace. Revoked keys still count as "created" because the activation milestone is about whether the user has learned the API key creation path.

## 9. Error Handling

Use the existing API error envelope.

Expected errors:

- Missing or invalid session: `auth.unauthorized`.
- No active Workspace: `workspace.not_found`.
- Repository failure: internal error with request ID.
- Invalid trial config at startup: service construction should fail in production-like command startup rather than silently grant the wrong amount.

Trial credit grant idempotency:

- Replaying the same user creation transaction is not expected because the transaction either commits or rolls back.
- The ledger idempotency key is still deterministic, `trial-grant:<workspace_id>`, so future repair jobs or manual replay paths can safely detect duplicates.

## 10. Testing

Repository tests:

- New email login creates user, default Workspace, trial ledger, updated balance, and `trial_granted_at`.
- Existing user login does not create a second trial ledger.
- Trial amount `0` creates user/default Workspace with zero balance and null `trial_granted_at`.
- Grant helper is idempotent when `trial_granted_at` is already present.
- Conflicting duplicate ledger payload is not silently accepted.

Config tests:

- Defaults are 10 yuan and 7 days.
- Environment variables override defaults.
- Negative trial amount is rejected.
- Zero or negative TTL is rejected.

Service tests:

- Overview maps Workspace, balance, trial state, and API key-created state.
- Billing role can view overview.
- Missing Workspace maps to `workspace.not_found`.
- Empty user maps to `auth.unauthorized`.

Handler tests:

- `GET /api/console/overview` requires session.
- Successful response shape includes activation steps.
- Route is registered in `cmd/portal-api`.

Verification:

```text
GOCACHE=/private/tmp/go-build-cache go test ./...
GOCACHE=/private/tmp/go-build-cache go build -o /private/tmp/tokenlive-portal-api ./cmd/portal-api
GOCACHE=/private/tmp/go-build-cache go build -o /private/tmp/tokenlive-portal-worker ./cmd/portal-worker
```

## 11. Rollout Notes

- Existing databases already have the required columns and tables from the foundation migration.
- Existing users are not backfilled automatically in this slice.
- If beta users should receive trial credit retroactively, implement a separate explicit repair script with dry-run output.
- Gateway enforcement of the trial window and allowed trial models is intentionally deferred.
- Frontend can use the overview API immediately to render the activation dashboard.

## 12. Open Follow-Ups

These are intentionally outside this slice:

- Whether beta users receive retroactive trial credit.
- Which model whitelist applies to trial credit.
- Daily trial spend limit.
- First-call signal from usage/request logs.
- Automatic expiration behavior.
- Recharge CTA behavior after balance reaches zero.
