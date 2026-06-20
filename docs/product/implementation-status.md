# TokenLive Portal Implementation Status

Date: 2026-06-20

This document summarizes what has been implemented in `tokenlive-portal` and what is still pending for the first user-facing Portal release.

## 1. Current Product Boundary

`tokenlive-portal` is responsible for the public website, model marketplace, authenticated developer console, Workspace-facing billing views, usage views, support entry points, and user-facing account flows.

Runtime gateway behavior is out of scope for this repository. The runtime gateway already lives in `tokenlive-gateway`.

Internal operations belong to `tokenlive-admin`. Portal may expose internal APIs later, but user-facing Portal work should not put operations UI into this repository.

## 2. Implemented Backend Slices

### Foundation

Implemented:

- MySQL-backed Go backend skeleton.
- Goose SQL migrations.
- Domain models for users, Workspaces, Workspace members, invitations, API keys, model permissions, balances, ledgers, audit logs, model catalog, model prices, and model metrics.
- Repository layer with explicit transaction boundaries.
- Money helpers using integer `micro_cny`.
- Health endpoint.
- Portal API and worker command entry points.

Still pending:

- Redis-backed background workers.
- Production deployment packaging.
- Admin-owned publication workflows.

### Public Model Catalog

Implemented:

- Public model catalog tables and repository queries.
- Public model list and detail APIs.
- Published model filtering.
- Public price and metrics response shaping.
- Provider and Endpoint details are hidden from user-facing APIs.

Still pending:

- Frontend model marketplace pages.
- Admin publication workflow.
- Favorites.
- Model comparison.
- Metrics sample-threshold policy.

### Auth And Session

Implemented:

- Passwordless email-code login skeleton.
- Atomic email-code verification, user/default Workspace creation, session creation, and code consumption.
- Configurable trial credit grant during first default Workspace registration.
- Session cookie support.
- Current-user API.
- Logout.
- Development/test code return behavior.

Still pending:

- Real email delivery integration.
- Google OAuth login.
- GitHub OAuth login.
- Account linking.
- Terms acceptance checkpoint.
- CSRF protection and rate limiting.

### Workspace API Key Console

Implemented:

- `GET /api/console/overview`.
- Current Workspace resolution for the logged-in user.
- `GET /api/workspaces/current`.
- `GET /api/api-keys`.
- `POST /api/api-keys`.
- `POST /api/api-keys/{id}/enable`.
- `POST /api/api-keys/{id}/disable`.
- `POST /api/api-keys/{id}/revoke`.
- Owner/developer permission checks for API key operations.
- Billing role is blocked from API key management.
- API key creation returns the full secret once.
- API key plaintext is never stored.
- API key state changes are scoped to Workspace.
- API key state changes write audit logs.
- Repeated enable, disable, or revoke calls are no-op updates when already in the requested state.
- Overview activation state for trial credit, API key creation, and first API call placeholder.

Still pending:

- Gateway synchronization for API key lifecycle.
- API key model whitelist editing.
- API key spend enforcement.
- Workspace switcher.
- Workspace member and invitation APIs.

### Trial Credit And Activation Overview

Design spec:

- `docs/specs/2026-06-20-trial-credit-activation-overview-slice-design.md`

Implemented:

- Grant trial credit when registration creates the first default Workspace.
- Default trial amount: 10 yuan.
- Default trial window: 7 days.
- Store the grant as a `trial_grant` ledger entry and update Workspace balance in one transaction.
- Set `workspaces.trial_granted_at`.
- Add `GET /api/console/overview`.
- Return activation status for trial credit, API key creation, and first API call.

Still pending:

- Runtime trial model whitelist enforcement.
- Daily trial spend limit enforcement.
- Automatic trial expiration or clawback.
- Gateway runtime synchronization.
- Usage/request-log ingestion.
- Frontend pages.

## 3. Major Pending Product Areas

Public website:

- Marketing home page.
- Documentation center.
- Pricing page.
- Public status page.
- Legal and policy pages.

Developer console:

- Overview frontend.
- API keys frontend.
- Usage dashboard.
- Request logs with 15-day retention.
- Billing and manual recharge request flow.
- Favorite models.
- Support tickets.
- Settings.
- Workspace members and invitations.
- Workspace switcher.

Billing and usage:

- Manual recharge application.
- Admin recharge review integration.
- Consumption ledger from Gateway settlement events.
- Usage aggregation by day/hour/model/API key.
- Balance reconciliation.
- Low-balance and budget alerts.

Authentication:

- Google OAuth.
- GitHub OAuth.
- Explicit account linking.
- Email delivery provider.
- Terms acceptance after OAuth callback.

Runtime integration:

- API key lifecycle synchronization to Gateway.
- Workspace balance/runtime copy synchronization.
- Model permission synchronization.
- Gateway settlement event ingestion.
- Reconciliation jobs.

Operations and Admin integration:

- Model catalog editing and publication from `tokenlive-admin`.
- Recharge review from `tokenlive-admin`.
- Support ticket processing from `tokenlive-admin`.
- User and Workspace risk controls from `tokenlive-admin`.

## 4. Deliberately Deferred For First Release

- Consumer chat Playground.
- Provider-facing supply-side portal.
- Automatic online payment.
- Automatic refunds.
- User comments and public reviews.
- Benchmark ranking pages.
- Custom Workspace roles.
- Public join links.
- Prompt/output content storage.
