# Portal/Admin/Gateway API Key Smoke Runbook

Date: 2026-07-01

## Goal

Verify the public Portal API key runtime path end to end:

1. Portal creates a Workspace API key.
2. Portal writes the runtime Redis hash key.
3. Admin can view only safe API key metadata and trigger runtime resync.
4. Gateway authenticates the Portal key through `aigw:apikey_hash:{key_hash}`.

## Required Local Services

- MySQL database for Portal.
- Redis shared by Portal runtime sync and Gateway.
- `tokenlive-portal` backend.
- `tokenlive-admin` backend/frontend.
- `tokenlive-gateway`.

## Required Configuration

Use one shared Redis instance for Portal and Gateway.

Portal:

```env
PORTAL_HTTP_ADDR=:8080
PORTAL_DATABASE_DSN=<mysql dsn>
PORTAL_AUTH_PEPPER=<shared api key pepper>
PORTAL_INTERNAL_API_TOKEN=<shared internal token>
PORTAL_GATEWAY_REDIS_ADDR=<redis host:port>
PORTAL_GATEWAY_REDIS_PASSWORD=<redis password if any>
PORTAL_GATEWAY_REDIS_DB=<redis db>
PORTAL_USAGE_CLICKHOUSE_ENABLED=<true to enable Portal usage pages>
PORTAL_CLICKHOUSE_ADDR=<clickhouse host:port>
PORTAL_CLICKHOUSE_DATABASE=tokenlive_gateway
PORTAL_CLICKHOUSE_USERNAME=<clickhouse user>
PORTAL_CLICKHOUSE_PASSWORD=<clickhouse password if any>
```

Admin:

```toml
[Portal]
BaseURL = "http://localhost:8080"
InternalAPIToken = "<same value as PORTAL_INTERNAL_API_TOKEN>"
```

Gateway:

```env
REDIS_ADDR=<same redis host:port>
REDIS_PASSWORD=<same redis password if any>
REDIS_DB=<same redis db>
GATEWAY_API_KEY_PEPPER=<same value as PORTAL_AUTH_PEPPER>
```

Gateway config must expose:

```yaml
llm:
  api_key_pepper: ${GATEWAY_API_KEY_PEPPER:}
gateway:
  config_source: redis
  state_store: redis
```

## Start Order

From `tokenlive-portal/backend`:

```bash
go run ./cmd/portal-api
```

From `tokenlive-admin`:

```bash
make start
```

From `tokenlive-admin/frontend` when using the separate Vite dev server:

```bash
npm run dev
```

From `tokenlive-gateway`:

```bash
go run ./cmd/server
```

## Smoke Steps

1. Create or reuse a Portal user session.
2. Ensure the user has accepted terms and has a default Workspace.
3. Create a Portal API key from the Portal console.
4. Bind the Portal Workspace to an Admin tenant through Portal internal operations.
5. In Admin, open `/monitor/portal-workspaces`.
6. Enter the Workspace ID and query API keys.
7. Confirm Admin shows only `name`, `key_prefix`, `secret_last4`, `status`, and timestamps.
8. Click runtime resync.
9. Call Gateway with the Portal API key.

Expected Redis key:

```text
aigw:apikey_hash:{HMAC-SHA256(api_key, PORTAL_AUTH_PEPPER)}
```

Expected Gateway behavior:

- Request with the Portal API key authenticates.
- Missing or mismatched `GATEWAY_API_KEY_PEPPER` causes Gateway Redis API key lookup to reject the key.
- Admin never receives plaintext API key or key hash.

## Current Local Readiness Notes

As of 2026-07-01, this workspace has no local `tokenlive-portal/.env` or `tokenlive-admin/.env` file. A full smoke run is blocked until the Portal database DSN, shared Redis, and shared pepper/internal-token values are provided locally.
