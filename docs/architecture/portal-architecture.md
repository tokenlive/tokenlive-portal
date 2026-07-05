# TokenLive Portal Architecture

## 1. System Boundary

TokenLive is split into three products:

- `tokenlive-portal`: public website, model marketplace, user console, and Portal Backend / BFF.
- `tokenlive-admin`: internal operations and management system.
- `tokenlive-gateway`: model request runtime gateway.

Portal does not implement runtime model routing. Gateway already handles OpenAI-compatible APIs, Anthropic Messages, Responses, Embeddings, authentication, policy, routing, settlement, and metrics. Portal consumes synchronized data and exposes user-facing management.

Current implemented and pending Portal slices are summarized in `docs/product/implementation-status.md`.

## 2. Ownership By Product

### 2.1 Portal

Portal owns:

- Public website
- Model marketplace presentation
- Developer console
- Workspace, members, invitations, and roles
- User authentication and account binding
- API key lifecycle user experience
- Trial credit state
- Balance display, manual recharge application, billing views
- Request metadata query and CSV export
- Aggregated usage query and CSV export
- Favorite models
- Support ticket user interface
- Public status page display
- Legal and policy pages

### 2.2 Admin

Admin owns:

- Model catalog editing and publishing
- Model visibility, featured state, and localized content management
- Runtime model / provider / endpoint configuration
- Recharge review and ledger crediting
- Support ticket processing
- User and Workspace risk controls
- User blocking / unblocking
- Workspace budget and model access adjustments
- Public status incident publishing
- Manual approvals for higher limits and restricted models

### 2.3 Gateway

Gateway owns:

- LLM request runtime
- Protocol compatibility
- API key authentication at runtime
- Model permission enforcement at runtime
- Routing, retry, fallback, circuit breaking, and load balancing
- Token settlement
- Runtime metrics and access logs

## 3. Technical Stack

Portal monorepo contains independently deployable Web and Backend services.

Web:

- Next.js
- TypeScript
- Tailwind CSS
- shadcn/ui
- SSR / SSG for public pages and SEO
- Simplified Chinese default, English supported

Backend:

- Go
- MySQL
- Redis
- goose SQL migrations
- GORM with explicit Repository layer

Deployment:

- Web and Backend are separate Docker images.
- Public pages use `/`.
- Console uses `/dashboard`.
- Portal API uses `/api` and is routed to Portal Backend by Ingress or edge routing.
- Static assets use CDN.
- MySQL and Redis use managed services when available.
- First release is single-region, multi-instance.

## 4. Data Strategy

Portal uses local read models for user-facing pages and dashboards.

Rules:

- Public pages and dashboard should read from Portal database.
- Portal should not depend on live Gateway availability for browsing historical usage, billing, docs, or model catalog.
- Runtime data is synchronized asynchronously into Portal.
- Write operations that affect runtime access call narrow internal APIs or publish narrow commands.
- Synchronization delay target is within one minute for user-facing operational data.

## 5. Model Catalog

Portal Model Catalog is separate from Gateway runtime Model.

Catalog is connected to Gateway by stable `model_id`.

Catalog stores:

- Public model identity
- Localized name and description
- Logo and tags
- Capability and modality metadata
- Context length
- Knowledge cutoff
- Public status
- Public prices and price versions
- Documentation and examples
- SEO content
- Featured state and sort weight

Gateway or Admin determines whether the model is actually callable. Portal can mark a model as paused or unavailable without deleting catalog content.

Provider and Endpoint details are not exposed to users in first release.

Model catalog and prices are maintained by Admin and published to Portal through `POST /internal/v1/model-catalogs/publish`. Portal persists the published snapshot and reads only published data.

History rules:

- Price versions are immutable and retained long-term.
- Catalog display content keeps only current published state in Portal.
- Drafts, revision history, and publication audit belong to Admin.
- Ledger and request records may store display snapshots for billing explanation.

## 6. Billing Architecture

The first release uses prepaid balance.

Authoritative records:

- Ledger entries are immutable.
- Every balance mutation creates a ledger entry.
- Requests reference the price version used during settlement.
- Historical billing is never recalculated using current price.
- Portal Backend is the authoritative system for ledger and balance.
- Gateway may use a runtime balance copy for high-performance checks, pre-deduction, and settlement.
- Gateway settlement events are written back to Portal idempotently.
- MySQL ledger and Workspace balance are updated in one transaction.
- Periodic reconciliation compares Portal ledger, Workspace balance, and Gateway settlement events.

Money unit:

- All stored and calculated amounts use integer `micro_cny`.
- `1 CNY = 1,000,000 micro_cny`.
- UI formats money as CNY yuan.

Ledger event types:

- Manual recharge credit
- Trial grant
- Consumption
- Refund or adjustment, if manually created by Admin

Consumption idempotency:

- Every Gateway settlement event has a unique `request_id`.
- `ledger_entries` enforces one consumption ledger per `request_id`.
- Duplicate events with identical content are treated as already processed.
- Duplicate events with conflicting content are flagged as reconciliation anomalies.

Public price:

- CNY per one million tokens.
- Separate input, output, and cache-read prices.
- Price version has effective time.

## 7. Usage And Logs

Portal stores only request metadata needed for user troubleshooting and billing explanation.

Stored request fields include:

- Workspace ID
- API key ID / display name
- API key name snapshot
- Model ID
- Model display name snapshot
- Request ID
- Status
- Error type and safe error message
- Latency
- Time to first token
- Input and output tokens
- Cost
- Price version
- Unit price snapshot
- Created time

Retention:

- Request metadata: 15 days
- Error details: 15 days
- Aggregated hourly and daily usage: long-term
- Ledger, recharge, and audit records: long-term
- Prompt and output content: never stored

Aggregations should support dashboard queries by:

- Workspace
- Model
- API key
- Day / hour
- Status

Request logs and consumption ledgers should keep display snapshots so historical records do not drift when model names, API key names, or prices change.

## 8. Internal Integration Contract

Portal and Admin should define narrow contracts early. First release prefers internal REST APIs; message queues can be added later.

Required contract areas:

- Model catalog publish and unpublish
- Model aggregated status update
- Price version publish
- Recharge application review result
- Ticket status and reply updates
- User block / unblock
- Workspace budget adjustment
- Workspace model permission adjustment
- API key lifecycle propagation to runtime
- Status incident publish and update

Gateway integration areas:

- API key creation, disable, revoke, and runtime permission propagation
- API key effective model permission synchronization
- Workspace balance / limit runtime copy synchronization
- Runtime request metadata ingestion
- Token settlement and cost ingestion
- Model availability and service performance ingestion

API key authority:

- Portal Backend owns API key lifecycle and business records.
- Gateway verifies against a synchronized runtime copy, not Portal MySQL.
- Portal stores only API key hash material, prefix, and last four characters.

Balance authority:

- Portal Backend owns ledger and balance.
- Gateway uses runtime copies for hot-path checks and emits settlement events back to Portal.

## 9. Authentication And Authorization

Login methods:

- Google OAuth
- GitHub OAuth
- Email verification code

Authorization model:

- User is global, not scoped to a Workspace.
- User can belong to many Workspaces.
- Workspace membership is represented by a `WorkspaceMember` association.
- Workspace roles are `Owner`, `Developer`, and `Billing`.
- API keys belong to Workspace.
- Balance belongs to Workspace.
- API key model access is the intersection of Workspace model permission and key whitelist.
- Empty API key whitelist means inheriting all Workspace-allowed models.

Account linking:

- No automatic account merge by email.
- Explicit binding only.
- High-risk operations require email verification.

## 10. Security And Compliance Notes

Security-sensitive requirements:

- API key secret is shown only once.
- Store only hashed API key secret material.
- API key UI may display prefix and last four characters for identification.
- Full API key validation uses salted hash or HMAC, never plaintext comparison.
- Sensitive operations create audit logs.
- Audit logs are stored separately from request logs and ledgers and retained long-term.
- Prompt and output bodies are not stored.
- Recharge proof upload must be private.
- Manual review actions must be auditable.
- OAuth account linking must avoid email-only account takeover.
- Trial abuse detection should be risk-scored, not based on a single signal.

Legal pages required at launch:

- Terms of service
- Privacy policy
- Billing and refund rules
- Acceptable use policy
- Contact page
- ICP / entity information placeholder

## 11. Deployment Shape

Recommended first release deployment:

```text
User
  -> CDN / Edge
  -> Next.js Web
  -> /api routed to Portal Backend
  -> MySQL / Redis
  -> Internal APIs to Admin and Gateway
```

Public website and console share the same primary domain.

First release does not require:

- Multi-region active-active deployment
- Independent status subdomain
- Dedicated public data API

## 12. Suggested Initial Modules

Portal Backend modules:

- Auth
- User account
- Workspace
- Invitation
- Role and permission
- Model catalog read API
- Favorites
- API key management
- Trial credit
- Balance and ledger
- Recharge application
- Usage query
- Request log query
- CSV export
- Budget alerts
- Support ticket user API
- Status page public API
- Audit log
- Internal integration API

Worker modules:

- Email notification sender
- Runtime data synchronization
- Ledger and balance reconciliation
- Request log retention cleanup
- Usage aggregation
- Invitation expiry cleanup
- Soft delete finalization
- Budget alert evaluation

The API service and worker service share the same Go codebase but run as independent processes:

- `portal-api`
- `portal-worker`

Web modules:

- Public marketing pages
- Model marketplace
- Model details
- Docs
- Pricing
- Status
- Auth flow
- Dashboard
- API keys
- Usage
- Billing
- Favorites
- Tickets
- Workspace settings

## 13. API Style

Portal Backend uses REST plus OpenAPI.

Conventions:

- Public API: `GET /api/public/...`
- Authenticated user API: `/api/me/...`
- Workspace API: `/api/workspaces/{workspace_id}/...`
- Internal API: `/api/internal/...`
- All responses include a `request_id`.
- Critical writes, especially financial and synchronization writes, support `Idempotency-Key`.
- A separate internal OpenAPI document may be used for Admin and Gateway integration.

Frontend TypeScript client and types are generated from OpenAPI. The frontend should not hand-maintain duplicate DTO types.

Validation:

- Backend validation is authoritative.
- Frontend uses Zod for immediate user feedback.
- Financial, permission, balance, and idempotency rules are enforced only by backend authority.

Error response shape:

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
- Frontend localizes user-facing copy by error code.
- Internal errors do not expose stack traces.
