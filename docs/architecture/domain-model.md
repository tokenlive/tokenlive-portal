# TokenLive Portal Domain Model

This document is a first implementation-oriented domain model. It captures ownership, relationships, and constraints before database migrations are written.

## 1. Identity And Workspace

### User

Global user account. User is not scoped to Workspace.

Key fields:

- `id`
- `display_name`
- `primary_email`
- `email_verified_at`
- `avatar_url`
- `status`
- `created_at`
- `updated_at`
- `deleted_at`

Rules:

- One user can join many Workspaces.
- OAuth identities and verified emails attach to User.
- User must resolve owned Workspaces before deleting the account.

### AccountIdentity

External or email-based login identity.

Key fields:

- `id`
- `user_id`
- `provider`
- `provider_subject`
- `email`
- `email_verified`
- `created_at`
- `updated_at`

Rules:

- Providers include `google`, `github`, and `email`.
- Do not auto-merge accounts by email alone.
- Explicit binding requires the user to be logged in and pass secondary verification.

### Workspace

Asset, billing, API key, and usage boundary.

Key fields:

- `id`
- `name`
- `slug`
- `owner_user_id`
- `status`
- `trial_granted_at`
- `created_by_user_id`
- `created_at`
- `updated_at`
- `deleted_at`

Rules:

- Registration creates one default Workspace.
- User may self-create up to 3 Workspaces in the first release.
- Invited Workspaces do not count toward the self-created limit.
- Trial credit is granted only to the first default Workspace.
- Additional Workspace creation requires verified email.

### WorkspaceMember

Association between User and Workspace.

Key fields:

- `workspace_id`
- `user_id`
- `role`
- `status`
- `joined_at`
- `created_at`
- `updated_at`

Roles:

- `owner`
- `developer`
- `billing`

Rules:

- Owner can manage members, recharge, billing, API keys, and settings.
- Developer can manage API keys and view usage/request logs.
- Billing can recharge and view balance/billing.
- Custom roles are out of scope for first release.

### WorkspaceInvitation

Email invitation into Workspace.

Key fields:

- `id`
- `workspace_id`
- `email`
- `role`
- `token_hash`
- `status`
- `invited_by_user_id`
- `accepted_by_user_id`
- `expires_at`
- `accepted_at`
- `revoked_at`
- `created_at`

Rules:

- Invitation token is single-use.
- Invitation expires after 7 days.
- The accepting user must verify the invited email before joining.

## 2. API Key And Runtime Access

### ApiKey

Portal-owned API key business record.

Key fields:

- `id`
- `workspace_id`
- `name`
- `key_prefix`
- `secret_last4`
- `key_hash`
- `status`
- `created_by_user_id`
- `expires_at`
- `daily_limit_micro_cny`
- `monthly_limit_micro_cny`
- `last_used_at`
- `total_spend_micro_cny`
- `created_at`
- `updated_at`
- `revoked_at`

Rules:

- Full key is shown only once at creation.
- Plaintext key is never stored.
- `key_hash` is used for verification with salted hash or HMAC.
- `key_prefix` and `secret_last4` are for UI identification.
- New keys are enabled by default.
- No default expiration.

### ApiKeyModelWhitelist

Optional per-key model whitelist.

Key fields:

- `api_key_id`
- `model_id`
- `created_at`

Rules:

- Empty whitelist means inherit all Workspace-allowed models.
- Effective access is Workspace model permission intersected with key whitelist.

### WorkspaceModelPermission

Workspace-level model permission.

Key fields:

- `workspace_id`
- `model_id`
- `source`
- `granted_by_user_id`
- `created_at`

Rules:

- Restricted models must be granted to Workspace before API keys can use them.
- Admin can adjust permissions through internal API.

## 3. Model Catalog And Pricing

### ModelCatalog

Current published model catalog record read by Portal.

Key fields:

- `model_id`
- `slug`
- `status`
- `visibility`
- `logo_url`
- `context_length`
- `knowledge_cutoff`
- `input_modalities`
- `output_modalities`
- `capabilities`
- `featured`
- `sort_weight`
- `published_at`
- `updated_at`

Rules:

- Admin owns editing and publishing.
- Portal reads only published data.
- Provider and Endpoint details are hidden from users.

### ModelCatalogI18n

Localized model content.

Key fields:

- `model_id`
- `locale`
- `display_name`
- `short_description`
- `long_description`
- `seo_title`
- `seo_description`
- `tags`
- `updated_at`

Rules:

- First release supports `zh-CN` and `en`.
- Content is language-modeled from the beginning.

### ModelPriceVersion

Immutable public price version.

Key fields:

- `id`
- `model_id`
- `currency`
- `input_micro_cny_per_1m_tokens`
- `output_micro_cny_per_1m_tokens`
- `cache_read_micro_cny_per_1m_tokens`
- `effective_from`
- `effective_until`
- `status`
- `published_by_user_id`
- `published_at`

Rules:

- Prices are displayed as CNY per one million tokens.
- Price versions are immutable after publication.
- Requests reference the exact price version used for settlement.

### ModelServiceMetric

Aggregated model service quality for public display.

Key fields:

- `model_id`
- `window`
- `availability`
- `ttft_p50_ms`
- `ttft_p95_ms`
- `response_speed`
- `success_rate`
- `sample_count`
- `updated_at`

Rules:

- Show public metrics only above sample threshold.
- Do not expose Provider or Endpoint details.

## 4. Billing And Ledger

### WorkspaceBalance

Fast balance read model updated with ledger entries.

Key fields:

- `workspace_id`
- `available_micro_cny`
- `frozen_micro_cny`
- `version`
- `updated_at`

Rules:

- Updated in the same transaction as ledger insertion.
- Reconciled periodically against ledger.

### LedgerEntry

Immutable financial record.

Key fields:

- `id`
- `workspace_id`
- `type`
- `direction`
- `amount_micro_cny`
- `balance_after_micro_cny`
- `currency`
- `idempotency_key`
- `request_id`
- `api_key_id`
- `api_key_name_snapshot`
- `model_id`
- `model_display_name_snapshot`
- `price_version_id`
- `unit_price_snapshot`
- `metadata`
- `created_at`

Types:

- `recharge`
- `trial_grant`
- `consumption`
- `refund`
- `adjustment`

Rules:

- Every balance mutation creates a ledger entry.
- Consumption ledger uses `request_id` as unique idempotency key.
- Duplicate settlement with conflicting content is a reconciliation anomaly.
- Amounts use integer `micro_cny`.

### RechargeRequest

Manual recharge workflow record.

Key fields:

- `id`
- `workspace_id`
- `amount_micro_cny`
- `status`
- `payment_proof_url`
- `submitted_by_user_id`
- `reviewed_by_user_id`
- `review_note`
- `ledger_entry_id`
- `created_at`
- `reviewed_at`

Rules:

- Approval creates a recharge ledger entry and updates balance in one transaction.
- A request can be credited at most once.
- Rejection does not create ledger.

## 5. Usage And Request Records

### RequestLog

Short-retention request metadata.

Key fields:

- `request_id`
- `workspace_id`
- `api_key_id`
- `api_key_name_snapshot`
- `model_id`
- `model_display_name_snapshot`
- `status`
- `error_type`
- `safe_error_message`
- `latency_ms`
- `ttft_ms`
- `input_tokens`
- `output_tokens`
- `cost_micro_cny`
- `price_version_id`
- `unit_price_snapshot`
- `created_at`
- `expires_at`

Rules:

- Retain for 15 days.
- Do not store prompt or output body.
- Request ID links logs, support tickets, and ledger entries.

### UsageAggregate

Hourly and daily usage rollup.

Key fields:

- `workspace_id`
- `api_key_id`
- `model_id`
- `bucket`
- `bucket_start_at`
- `request_count`
- `success_count`
- `error_count`
- `input_tokens`
- `output_tokens`
- `cost_micro_cny`
- `updated_at`

Rules:

- Retained long-term.
- Used by dashboard, usage charts, and export.

## 6. Support, Status, And Notifications

### SupportTicket

User-created support request owned by Portal.

Key fields:

- `id`
- `workspace_id`
- `created_by_user_id`
- `category`
- `priority`
- `status`
- `subject`
- `request_id`
- `created_at`
- `updated_at`
- `closed_at`

Rules:

- Admin processes tickets through Portal internal API.
- Tickets can be used for limit increase and restricted model requests.

### SupportTicketMessage

Ticket conversation message.

Key fields:

- `id`
- `ticket_id`
- `author_type`
- `author_user_id`
- `body`
- `created_at`

Rules:

- Messages are append-only for auditability.

### StatusIncident

Public status page incident.

Key fields:

- `id`
- `status`
- `severity`
- `started_at`
- `resolved_at`
- `created_by_user_id`
- `updated_at`

Rules:

- Portal stores public status data.
- Admin publishes and updates incidents.
- No Provider or Endpoint details are exposed.

### StatusIncidentI18n

Localized status incident content.

Key fields:

- `incident_id`
- `locale`
- `title`
- `summary`
- `updates`

### NotificationEvent

Asynchronous email notification event.

Key fields:

- `id`
- `workspace_id`
- `user_id`
- `type`
- `locale`
- `payload`
- `status`
- `retry_count`
- `last_error`
- `scheduled_at`
- `sent_at`
- `created_at`

Rules:

- `portal-worker` sends emails from notification events.
- Templates support Chinese and English.
- First release does not support SMS, station inbox, Slack, or webhook notifications.

## 7. Audit

### AuditLog

Long-retention record of sensitive changes.

Key fields:

- `id`
- `workspace_id`
- `actor_user_id`
- `action`
- `resource_type`
- `resource_id`
- `before`
- `after`
- `ip`
- `user_agent`
- `created_at`

Audited actions include:

- Login and OAuth binding
- Workspace creation, deletion, and ownership transfer
- Member invitation, join, removal, and role change
- API key creation, disable, revoke, and limit change
- Recharge request submission and review result
- Balance adjustment
- Model permission adjustment
- High-risk secondary verification
- User deletion and Workspace soft deletion

## 8. API And Worker Conventions

API style:

- REST plus OpenAPI.
- Public API under `/api/public/...`.
- Current-user API under `/api/me/...`.
- Workspace API under `/api/workspaces/{workspace_id}/...`.
- Internal API under `/api/internal/...`.
- Critical writes accept `Idempotency-Key`.
- All responses include `request_id`.

Frontend contract:

- TypeScript client and DTO types are generated from OpenAPI.
- Frontend uses Zod for user experience validation.
- Backend validation remains authoritative.

Error shape:

```json
{
  "error": {
    "code": "workspace.insufficient_balance",
    "message": "Insufficient balance",
    "request_id": "req_xxx"
  }
}
```

Worker:

- `portal-api` handles HTTP.
- `portal-worker` handles email, synchronization, reconciliation, cleanup, aggregation, and alerts.
- Both processes share the same Go codebase and database models.

