# Portal ClickHouse Usage Slice Design

Date: 2026-07-04

## 1. Goal

Make Portal Usage and Request Logs read from Gateway's structured ClickHouse access logs instead of adding a second Workspace usage counter path in Redis.

The first release should let a logged-in Portal user see current Workspace usage from real Gateway requests:

```text
Create API key -> Call Gateway -> Gateway writes ClickHouse access log -> Portal reads ClickHouse by Workspace -> Console shows usage and recent requests
```

This design covers the first read-only Portal usage slice. It does not implement ledger mutation, balance deduction, recharge review, export, or long-term reconciliation jobs.

## 2. Current State

### Portal

`tokenlive-portal` already owns:

- Portal users and sessions.
- Workspaces and Workspace membership.
- Workspace API keys.
- Console overview, API key, billing, and settings pages.
- `workspaces.tenant_code`, used to bind a Portal Workspace to an Admin Tenant.
- Runtime Redis sync for Portal API keys using `aigw:apikey_hash:{key_hash}`.

Portal currently has no durable usage query path and no Request Logs console page.

### Admin

`tokenlive-admin` already reads Gateway operational data for Admin-only dashboards:

- Redis minute counters:
  - `aigw:status:global:{minute}:s`
  - `aigw:status:global:{minute}:f`
  - `aigw:status:model:{model}:{minute}:s`
  - `aigw:status:model:{model}:{minute}:f`
  - `aigw:status:endpoint:{endpoint}:{minute}:s`
  - `aigw:status:endpoint:{endpoint}:{minute}:f`
- Redis natural-day counters:
  - `aigw:status:daily:req:{date}`
  - `aigw:status:daily:input_tokens:{date}`
  - `aigw:status:daily:output_tokens:{date}`
  - `aigw:status:daily:cached_tokens:{date}`
  - `aigw:status:daily:cache_creation_tokens:{date}`
  - `aigw:status:daily:cost:{date}`
- Redis Stream policy events at `aigw:events:policy`, consumed by Admin ops into `event_log`.

Those existing Redis keys are global, model, endpoint, or operational-policy scoped. They do not carry `workspace_id` or stable Portal API key identity, so they cannot be the source of truth for Portal user usage.

### Gateway

`tokenlive-gateway` already has ClickHouse access log support:

- `access_log.clickhouse.enabled` controls ClickHouse access-log writes.
- `AccessLogFilter` builds `AccessLogItem` from `GatewayContext`.
- A batcher writes rows to ClickHouse table `access_logs`.
- Failed ClickHouse writes fall back to the Redis compensation queue.
- `CONTEXT.md` states that billing and reconciliation must rely on structured ClickHouse access logs, not Prometheus or Redis metrics.

Gateway runtime already has `GatewayContext.WorkspaceID`. Portal API key runtime Redis records also include `key_id`, but Gateway does not yet propagate that key id into `GatewayContext` or `access_logs`.

## 3. Decision

Portal usage uses ClickHouse access logs as the first-release source of truth.

Rules:

- Gateway must not call Portal during the request path.
- Portal must not depend on Admin APIs for user-facing usage.
- Redis `aigw:status:*` keys remain Admin/global operational telemetry only.
- No new Portal Workspace usage Redis counters are added in this slice.
- Portal reads ClickHouse directly through its own backend using Workspace-scoped queries.
- A ClickHouse data-source outage makes Usage unavailable, but does not break login, API key management, billing views, model browsing, or Gateway request handling.

This keeps the runtime write path single-purpose: Gateway writes structured request facts once, and user-facing products query that durable fact table.

## 4. Gateway ClickHouse Contract

### Required `access_logs` Columns

The existing `access_logs` table must be extended for Portal use:

```sql
workspace_id LowCardinality(String)
api_key_id LowCardinality(String)
api_key_hash String
```

The table keeps its existing non-Portal fields:

```sql
request_id String
time DateTime64(3)
tenant_id LowCardinality(String)
user_id LowCardinality(String)
session_id String
api_key String
client_ip String
original_model LowCardinality(String)
model LowCardinality(String)
provider LowCardinality(String)
endpoint_id LowCardinality(String)
is_stream UInt8
attempts UInt8
fallback_chain Array(String)
status_code Int16
latency_ms UInt32
ttft_ms UInt32
error_message String
input_tokens UInt32
output_tokens UInt32
cached_tokens UInt32
cache_creation_tokens UInt32
cost Decimal(18, 9)
```

`api_key` remains a redacted display value. Portal must not use it for joins or authorization.

### Column Semantics

`workspace_id`:

- Set from `GatewayContext.WorkspaceID`.
- Empty string for internal Gateway/Admin-only keys.
- Required for Portal user queries.

`api_key_id`:

- Set from Portal runtime Redis field `key_id`.
- Empty string for legacy internal keys or when the runtime record does not contain a key id.
- Used for grouping and filtering by Portal API key.

`api_key_hash`:

- Set from the HMAC key hash computed during API key validation.
- Empty string if the runtime provider cannot expose it.
- Used only as a stable technical correlation value. It is not returned to Portal frontend.

`tenant_id`:

- Continues to represent the Admin Tenant or user tenant used by Gateway routing.
- Portal must not use `tenant_id` alone to scope user usage because multiple Workspaces can share an Admin Tenant.

### Table Ordering

The first ClickHouse table update should keep the existing primary read pattern and add `workspace_id` to the sort key:

```sql
ORDER BY (workspace_id, tenant_id, model, request_id, time)
```

If production ClickHouse already has the old table, the migration must be handled as an additive schema change first:

```sql
ALTER TABLE access_logs ADD COLUMN IF NOT EXISTS workspace_id LowCardinality(String) DEFAULT '';
ALTER TABLE access_logs ADD COLUMN IF NOT EXISTS api_key_id LowCardinality(String) DEFAULT '';
ALTER TABLE access_logs ADD COLUMN IF NOT EXISTS api_key_hash String DEFAULT '';
```

Changing `ORDER BY` for existing ClickHouse tables requires a separate operational migration and is outside this first Portal slice.

## 5. Portal Backend Query Design

Portal adds a ClickHouse read client that is separate from its MySQL repositories.

Configuration:

```text
PORTAL_CLICKHOUSE_ADDR
PORTAL_CLICKHOUSE_DATABASE
PORTAL_CLICKHOUSE_USERNAME
PORTAL_CLICKHOUSE_PASSWORD
PORTAL_USAGE_CLICKHOUSE_ENABLED
```

Development default:

- ClickHouse usage is disabled unless `PORTAL_USAGE_CLICKHOUSE_ENABLED=true`.
- When disabled, Usage APIs return a stable unavailable response instead of failing the whole Portal API server.

### Usage Summary API

Add:

```text
GET /api/usage/summary
```

Authentication:

- Requires a valid Portal session.
- Resolves the current Workspace through existing Workspace membership logic.

Response:

```json
{
  "data_source": "clickhouse",
  "available": true,
  "workspace_id": "wsp_xxx",
  "today": {
    "request_count": 42,
    "success_count": 40,
    "error_count": 2,
    "input_tokens": 12000,
    "output_tokens": 3400,
    "cached_tokens": 800,
    "cache_creation_tokens": 100,
    "cost_cny": "1.234567",
    "avg_latency_ms": 921,
    "avg_ttft_ms": 311
  },
  "models": [
    {
      "model": "gpt-4o",
      "request_count": 20,
      "success_count": 19,
      "error_count": 1,
      "input_tokens": 9000,
      "output_tokens": 2300,
      "cost_cny": "0.923456"
    }
  ]
}
```

If ClickHouse is disabled or unavailable:

```json
{
  "data_source": "clickhouse",
  "available": false,
  "workspace_id": "wsp_xxx",
  "today": null,
  "models": []
}
```

### Request Logs API

Add:

```text
GET /api/request-logs?limit=50
```

Scope:

- Current Workspace only.
- First release supports `limit`, capped at 100.
- No prompt or output content is queried or returned.

Response:

```json
{
  "logs": [
    {
      "request_id": "req_xxx",
      "time": "2026-07-04T12:34:56.789Z",
      "model": "gpt-4o",
      "api_key_id": "ak_xxx",
      "api_key_display": "tl_l***abcd",
      "status_code": 200,
      "latency_ms": 1234,
      "ttft_ms": 321,
      "input_tokens": 1000,
      "output_tokens": 200,
      "cached_tokens": 0,
      "cache_creation_tokens": 0,
      "cost_cny": "0.123456",
      "error_message": ""
    }
  ]
}
```

`api_key_display` uses the redacted `api_key` value already stored in ClickHouse. If the value is empty, frontend displays `-`.

## 6. Portal Frontend

Add `/console/usage`.

First release layout:

- Summary cards:
  - Requests today
  - Success rate
  - Tokens today
  - Cost today
  - Average latency
  - Average TTFT
- Model usage table:
  - Model
  - Requests
  - Success rate
  - Tokens
  - Cost
- Recent request table:
  - Time
  - Request ID
  - Model
  - API key
  - Status
  - Latency
  - TTFT
  - Tokens
  - Cost

The existing console navigation gets a Usage item.

When ClickHouse is unavailable, the page shows a concise unavailable state and leaves the rest of Console usable.

## 7. Security And Privacy

- Portal Usage APIs must always scope by the current Workspace ID resolved from the Portal session.
- Frontend must never receive `api_key_hash`.
- Frontend must never receive plaintext API key.
- Prompt and output content remain out of scope and are not stored in the current Gateway access log table.
- Error messages come from Gateway `error_message`; Portal should treat them as safe operational strings and truncate long values in UI.

## 8. Reliability And Performance

Gateway:

- Keeps ClickHouse writes inside existing `AccessLogFilter`.
- Uses existing batch writes and Redis compensation behavior.
- Does not add a Portal-specific runtime filter or Portal-specific Redis counters in this slice.

Portal:

- Uses bounded queries by Workspace and time.
- First release defaults to today and recent logs.
- Request log `limit` is capped at 100.
- Backend should set a short ClickHouse query timeout, such as 3 seconds.
- Query failure returns an unavailable response for Usage instead of a generic 500 when possible.

## 9. Testing Strategy

Gateway tests:

- API key validation propagates `key_id`, `workspace_id`, and `key_hash` into the runtime identity.
- `AccessLogFilter` writes `workspace_id`, `api_key_id`, and `api_key_hash` into `AccessLogItem`.
- ClickHouse batch append includes the new columns in the same order as DDL.
- Internal/Admin-only API keys keep empty Portal fields.

Portal backend tests:

- Usage service rejects unauthenticated requests through existing auth behavior.
- Usage service scopes queries to the current Workspace.
- Summary response maps ClickHouse aggregate rows to JSON with decimal cost strings.
- Request logs response maps ClickHouse rows without exposing `api_key_hash`.
- Disabled ClickHouse config returns `available=false`.

Portal frontend tests:

- Usage helper formats empty data as unavailable.
- Usage helper formats token totals, success rate, latency, and cost.
- Usage page routes auth errors through existing console auth redirect helper.

## 10. Deferred Work

- Ledger entries from ClickHouse request rows.
- Workspace balance mutation from Gateway settlement.
- Reconciliation between ClickHouse, MySQL ledger, and Workspace balance.
- CSV export.
- Date range filters beyond today.
- API key filter UI.
- Model drill-down.
- Request-log retention controls in Portal.
- ClickHouse table re-ordering migration for existing production tables.
