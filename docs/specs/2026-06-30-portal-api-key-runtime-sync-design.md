# Portal API Key Runtime Sync Design

Date: 2026-06-30

## 1. Goal

Define how Portal-owned API keys become usable by the existing TokenLive Gateway runtime without making Admin the storage owner for external user credentials.

The system has two deployment modes:

- `gateway + admin`: internal company mode. Admin owns users, tenants, and API keys.
- `gateway + admin + portal`: public platform mode. Portal owns external users, Workspaces, balances, and self-service API keys. Admin owns operations and runtime resource controls.

This design covers the public platform mode and preserves the existing internal mode.

## 2. Current State

### Portal

`tokenlive-portal` already owns:

- Portal users and OAuth/email login.
- Workspaces and Workspace members.
- Workspace API keys.
- API key `key_hash`, `key_prefix`, and `secret_last4`.
- One-time plaintext secret return on creation.
- `workspaces.tenant_code`, used to bind a Portal Workspace to an Admin Tenant.

Portal does not store API key plaintext after creation.

### Admin

`tokenlive-admin` already owns:

- Internal Admin users.
- Tenants.
- Tenant model and endpoint permissions.
- Tenant API keys for internal/toB mode.
- Internal user API keys.
- Redis sync for internal `aigw:apikey:<plaintext_api_key>` entries.
- Portal user/workspace lookup through Portal internal APIs.

Admin internal `user_api_key` currently stores plaintext API keys. Portal must not reuse that table.

### Gateway

`tokenlive-gateway` already owns runtime request processing and has:

- API key extraction from `Authorization`, `api-key`, `x-api-key`, and query string.
- `ApiKeyService`.
- `GatewayProvider`.
- Redis provider lookup using `aigw:apikey:<plaintext_api_key>`.
- Runtime fields for `user_id`, `tenant`, `workspace_id`, and `user_tenant`.
- Tenant model validation through Redis key `aigw:tenant:{tenantCode}:models`.
- Tenant endpoint routing through Redis key `aigw:tenant:{tenantCode}:model:{modelCode}:endpoints`.

Gateway already has a `workspace_id` field in its API key contract, so Portal integration can reuse the existing shape.

## 3. Ownership Decision

Portal is the business authority for Portal API keys.

Rules:

- Portal stores the durable API key business record.
- Portal stores only hash material, prefix, and last four characters.
- Portal never stores plaintext after creation.
- Admin does not store Portal API keys.
- Admin may view safe Portal API key metadata through Portal internal APIs.
- Gateway stores or reads only a runtime copy for hot-path authentication.
- Gateway runtime copies are derived data and can be rebuilt from Portal.

This keeps external user credentials out of the Admin internal user model and avoids inheriting Admin's plaintext `user_api_key` storage behavior.

## 4. Runtime Credential Contract

### Unified Hash Runtime Contract

Admin-owned credentials and Portal-owned credentials use the same Gateway runtime lookup shape:

```text
aigw:apikey_hash:{key_hash}
```

Hash fields:

```text
user_id
tenant
user_tenant
status
quota
expires_at
```

Gateway must continue to support this contract for existing internal mode.

### Portal Runtime Contract

Portal-owned credentials use a hash-keyed runtime copy:

```text
aigw:apikey_hash:{key_hash}
```

Hash fields:

```text
source        = portal
key_id        = Portal api_keys.id
user_id       = Portal users.id
workspace_id  = Portal workspaces.id
tenant        = Admin tenant_code when Workspace is bound
user_tenant   = same as tenant for first release when bound
status        = 1 enabled, 2 disabled
quota         = -1 for first release
expires_at    = Unix seconds, 0 means never
```

`key_hash` is the same HMAC-SHA256 value Portal stores in `api_keys.key_hash`.

The first release deliberately sets `quota = -1`. Portal API key daily/monthly limits are money limits in `micro_cny`, while Gateway's existing `quota` is token quota. They must not be mixed.

## 5. Gateway Lookup Behavior

Gateway API key validation uses the shared hash runtime contract:

1. Extract plaintext API key from the request as it does today.
2. Compute `key_hash` using the Portal-compatible HMAC pepper.
3. Try Redis hash lookup at `aigw:apikey_hash:{key_hash}`.
4. If not found, reject the API key.
5. Validate `status` and `expires_at`.
6. Inject `user_id`, `tenant`, `workspace_id`, and `user_tenant` into request context.

Existing Admin and Tenant API keys keep working through the same hash Redis contract written by Admin.

The HMAC pepper must be configured consistently between Portal and Gateway. For the first release, Portal continues to compute `api_keys.key_hash` with the existing Portal API key hashing input, currently sourced from `PORTAL_AUTH_PEPPER`. Gateway gets the same value through `GATEWAY_API_KEY_PEPPER` / `llm.api_key_pepper`. If Gateway does not have this pepper configured, Redis-backed API key lookup rejects keys instead of falling back to plaintext Redis lookup.

## 6. Portal Sync Behavior

Portal adds a runtime sync component with methods:

```text
SyncAPIKey(key_id)
DisableAPIKey(key_id)
RevokeAPIKey(key_id)
SyncWorkspaceAPIKeys(workspace_id)
```

Sync input comes from Portal database:

- `api_keys.id`
- `api_keys.key_hash`
- `api_keys.status`
- `api_keys.expires_at`
- `api_keys.created_by_user_id`
- `api_keys.workspace_id`
- `workspaces.tenant_code`

Sync output writes the Portal runtime Redis hash.

State mapping:

```text
enabled  -> status 1
disabled -> status 2
revoked  -> delete aigw:apikey_hash:{key_hash}
```

If a Workspace has no `tenant_code`, Portal must not leave an enabled runtime key in Gateway. For the first release, sync deletes `aigw:apikey_hash:{key_hash}` for unbound Workspaces. The user-facing console should show that the Workspace is waiting for runtime activation.

## 7. Admin Integration

Admin does not create or store Portal API keys.

Admin responsibilities:

- Search Portal users.
- Search Portal Workspaces.
- Bind or unbind a Portal Workspace to an Admin Tenant.
- Review recharge requests.
- Adjust Workspace model permissions through Portal internal APIs.
- View safe Portal API key metadata when needed.

Existing Portal internal APIs already cover:

```text
GET  /internal/v1/users/search
GET  /internal/v1/workspaces/search
POST /internal/v1/workspaces/{id}/bind-tenant
POST /internal/v1/workspaces/{id}/unbind-tenant
```

New Portal internal APIs needed for operations:

```text
GET  /internal/v1/workspaces/{id}/api-keys
POST /internal/v1/workspaces/{id}/runtime-sync
```

`GET /internal/v1/workspaces/{id}/api-keys` returns only safe metadata:

```json
{
  "api_keys": [
    {
      "id": "ak_xxx",
      "name": "production",
      "key_prefix": "tl_live_xxx",
      "secret_last4": "abcd",
      "status": "enabled",
      "created_at": "2026-06-30T00:00:00Z",
      "updated_at": "2026-06-30T00:00:00Z",
      "last_used_at": null
    }
  ]
}
```

No plaintext secret and no key hash are returned to Admin UI.

## 8. Model Permission Strategy

First release uses Tenant-level runtime permission as the coarse gate:

- A Portal Workspace may be bound to an Admin Tenant through `workspaces.tenant_code`.
- Gateway uses `tenant` or `user_tenant` to apply existing Tenant model and endpoint controls.
- Portal API key whitelist editing remains a later slice.

This means first release Portal runtime access is:

```text
Workspace bound tenant models
```

Later, add Workspace/API-key model intersection:

```text
Admin Tenant permission
AND Portal Workspace model permission
AND Portal API key whitelist
```

The later slice should add runtime Redis keys for Workspace and API-key model permissions rather than overloading Admin Tenant permissions.

## 9. Activation Flow

The first end-to-end flow is:

```text
Portal user signs up
-> accepts terms
-> Portal creates default Workspace and trial credit
-> Admin binds Workspace to tenant_code
-> Portal user creates API key
-> Portal syncs aigw:apikey_hash:{key_hash}
-> User calls Gateway with plaintext key
-> Gateway validates via hash lookup
-> Gateway injects user_id, workspace_id, tenant/user_tenant
-> Gateway routes using existing Tenant model and endpoint controls
```

The console activation status should distinguish:

- Trial credit granted.
- API key created.
- Runtime activated by Admin Tenant binding.
- First successful Gateway request observed.

## 10. Failure And Retry

Runtime sync failure must not roll back the Portal DB transaction.

First release behavior:

- API key database mutation succeeds or fails independently.
- Runtime sync runs after transaction commit.
- Sync failures are logged.
- A manual internal sync endpoint can repair runtime copies.

Later behavior:

- Add outbox table for API key runtime sync jobs.
- `portal-worker` retries failed sync jobs.
- Store last sync status and timestamp for user-facing diagnostics.

## 11. Security Requirements

- Portal API key plaintext is returned only during creation.
- Portal never writes plaintext API keys to Redis keys or values.
- Gateway logs must mask plaintext API keys.
- Admin UI never receives plaintext or hash values for Portal keys.
- `PORTAL_AUTH_PEPPER` or its dedicated API-key pepper equivalent must be treated as a secret.
- Gateway hash lookup must use the same pepper as Portal for Portal-owned keys.
- Existing Admin plaintext key compatibility remains only for legacy/internal credentials.

## 12. Implementation Tasks

### Task 1: Gateway hash lookup compatibility

Repository: `tokenlive-gateway`

Changes:

- Add API key hash computation support.
- Add configurable API key pepper, for example `GATEWAY_API_KEY_PEPPER`.
- Update Redis provider to check `aigw:apikey_hash:{key_hash}`.
- Reject Redis-backed API key lookup when `llm.api_key_pepper` is empty.
- Add unit tests for hash lookup, legacy plaintext rejection, disabled key, expired key, and missing pepper.

### Task 2: Portal runtime sync component

Repository: `tokenlive-portal`

Changes:

- Add Redis runtime sync configuration.
- Add runtime sync service.
- Add repository read methods for API key plus Workspace runtime metadata.
- Wire create/enable/disable/revoke API key flows to sync after commit.
- Add tests with a fake runtime sync target.

### Task 3: Portal internal operations APIs

Repository: `tokenlive-portal`

Changes:

- Add safe API key list internal endpoint.
- Add Workspace runtime resync internal endpoint.
- Reuse existing internal bearer-token authentication.
- Add tests for auth, safe response fields, and resync behavior.

### Task 4: Admin Portal Workspace operations

Repository: `tokenlive-admin`

Changes:

- Extend Portal Workspace operations UI/API to show bound Tenant state.
- Add safe Portal API key metadata view.
- Add "runtime resync" action that calls Portal internal API.
- Do not store Portal API keys in Admin DB.

### Task 5: Activation and smoke verification

Repositories: `tokenlive-portal`, `tokenlive-gateway`, `tokenlive-admin`

Checks:

- Create Portal API key.
- Bind Workspace to Tenant.
- Confirm Redis contains `aigw:apikey_hash:{key_hash}`.
- Call Gateway with the plaintext one-time key.
- Confirm Gateway context has `workspace_id`.
- Disable the key and verify Gateway rejects it after cache expiry or explicit cache purge.
- Revoke the key and verify runtime copy is removed.

## 13. Out Of Scope

- Prompt or output storage.
- Online payment.
- Consumption ledger ingestion.
- Workspace/API-key model whitelist runtime enforcement.
- API key IP allowlist or domain restrictions.
- Replacing existing Admin internal API key storage.
