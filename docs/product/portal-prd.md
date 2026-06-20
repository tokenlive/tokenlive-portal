# TokenLive Portal PRD

## 1. Product Positioning

TokenLive Portal is the public website, model marketplace, and self-service developer console for TokenLive users. It is inspired by OpenRouter, but positioned as developer infrastructure rather than a consumer chat product.

The first release optimizes for one activation goal:

```text
Browse models -> Sign up -> Receive trial credit -> Create API key -> Make first real API call
```

The runtime gateway is already implemented in `tokenlive-gateway`; this PRD covers only user-facing Portal capabilities.

Current implementation status is tracked in `docs/product/implementation-status.md`.

## 2. Target Users

Primary users:

- Developers and engineering teams integrating LLM APIs.
- Workspace owners who manage balance, billing, API keys, and members.
- Billing operators inside a customer workspace.

Non-primary users for the first release:

- Consumer chat users.
- Internal operations staff. They use `tokenlive-admin`.
- Model providers managing their own supply-side presence.

## 3. Core Principles

- Public model discovery should work without login.
- Login is required for API key creation, trial credit, usage, billing, support tickets, and workspace management.
- Portal should expose aggregated product-facing information, not internal gateway routing complexity.
- Workspace is the asset, billing, API key, and usage boundary.
- The first release should avoid high-risk automation such as automatic payment, automatic refunds, and automatic approval of restricted access.

## 4. Confirmed Scope

### 4.1 Public Website

Public navigation:

- Home
- Models
- Docs
- Pricing
- Status
- Login / sign up

Public pages:

- Marketing home page
- Public model marketplace
- Model detail page
- Pricing page
- Documentation center
- Public status page at `/status`
- Legal and rules pages:
  - Terms of service
  - Privacy policy
  - Billing and refund rules
  - Acceptable use policy
  - Contact page
  - ICP / company information placeholder

### 4.2 Model Marketplace

Model marketplace is model-centric, not provider-centric.

Model list supports filtering and sorting by:

- Input and output modality
- Capabilities
- Context length
- Input and output price
- Release date
- Popularity and usage

Model detail page shows:

- Model name, description, tags, and capabilities
- Context length
- Knowledge cutoff date, when available
- Input, output, and cache-read prices
- Supported parameters and protocols
- API call examples
- Aggregated service performance
- Data freshness and statistics window

Provider and Endpoint details are hidden from public users in the first release. The page shows TokenLive aggregated price and service quality.

Aggregated service performance should include:

- Availability
- Median and P95 time to first token
- Overall response speed
- User request success rate
- Updated-at timestamp and statistics window

Metrics are shown only when sample size reaches a threshold. New models should show "collecting data" instead of misleading low-sample metrics.

Logged-in users can favorite models. The first release does not include model comparison tables, benchmark rankings, or user comments.

### 4.3 Developer Console

Authenticated console navigation:

- Overview
- API Keys
- Usage
- Billing
- Favorite models
- Support tickets
- Settings

Workspace settings:

- Members
- Invitations
- Roles
- Budget alerts
- Account binding

Dashboard behavior:

- Before first successful API call, the dashboard focuses on activation:
  - Create API key
  - Choose a model
  - Run API example
- After activation, the dashboard focuses on account health:
  - Balance
  - Today's spend
  - Today's requests
  - Token usage
  - Recent error rate
  - Recent models
  - Recent request records
  - Recharge entry

Runtime details such as Provider health, routing internals, and Endpoint diagnostics belong to Admin or Gateway operations, not Portal.

### 4.4 Workspace Model

Workspace is the asset and billing subject.

Rules:

- A new user automatically gets a personal Workspace.
- API keys, balance, usage, billing, and limits belong to Workspace.
- A user can join multiple Workspaces.
- A user may create multiple Workspaces, with a first-release soft limit of 3 self-created Workspaces per user.
- Invited Workspaces do not count toward the self-created Workspace limit.
- Trial credit is granted only to the first default Workspace created during registration.
- New Workspaces created later do not receive trial credit.
- Workspace creation requires verified email.
- High-risk accounts cannot self-create additional Workspaces and must request review through support.
- Workspace switcher is global in the console.

Roles:

- `Owner`: manages members, recharge, billing, API keys, and workspace settings.
- `Developer`: manages API keys, views request logs and usage, cannot recharge or manage members.
- `Billing`: recharges and views balance and billing, cannot view API keys or request logs.

No custom roles in the first release.

### 4.5 Workspace Invitation

The first release supports email invitations, not public join links.

Rules:

- Owner invites an email and assigns `Developer` or `Billing`.
- Invitation link is single-use and expires after 7 days.
- Unregistered users can accept after logging in via email, Google, or GitHub.
- The accepting account does not need to use the invited email as its primary login, but must verify the invited email before joining.
- Owner can revoke invitations, remove members, and transfer ownership.

### 4.6 Authentication

Supported login methods:

- Google OAuth
- GitHub OAuth
- Passwordless email verification code as fallback

Account linking:

- Do not auto-merge accounts by matching email alone.
- Third-party accounts can be linked only explicitly.
- If a provider returns a verified email, linking still requires the user to be logged in to the existing account and pass secondary verification.

High-risk operations require email verification.

### 4.7 Trial Credit

Registration grants limited trial credit to help users complete their first API call.

Rules:

- One trial grant per Workspace.
- Trial credit expires after 7 days.
- Trial credit is limited to a low-risk model whitelist.
- Trial usage has daily spend limits.
- Trial credit cannot be transferred, refunded, or withdrawn.
- Abuse control uses risk scoring with signals such as email, OAuth account, device fingerprint, IP range, and behavior.
- A single shared IP match should not directly ban a user.
- High-risk users may require secondary email verification or manual review.

### 4.8 API Key Management

API keys belong to a Workspace and can be independently configured.

Each key supports:

- Required name
- Creator
- Model whitelist
- Daily spend limit
- Monthly spend limit
- Expiration time
- Enabled / disabled / revoked state
- Full secret shown only once at creation time
- Last used time
- Cumulative spend

Default behavior:

- No expiration by default.
- Inherits Workspace model access by default.
- Daily spend limit defaults to 50% of Workspace daily budget.
- Monthly spend limit is empty by default and falls back to Workspace monthly budget.
- New key is enabled by default.
- Effective model access is the intersection of Workspace model permission and API key whitelist.
- An empty API key whitelist means inheriting all Workspace-allowed models.
- Restricted models must first be granted to the Workspace before any API key can use them.
- API key full secret is shown once and never stored in plaintext.

No IP allowlist, domain restriction, or fine-grained endpoint permission in the first release.

### 4.9 Usage And Request Logs

Portal shows request-level metadata but does not store prompts or model outputs.

Each request record shows:

- Request time
- Model
- API key name
- Status
- Latency
- Time to first token
- Input and output tokens
- Cost
- Request ID
- Error type and safe error message

Data retention:

- Request metadata: 15 days
- Error details: 15 days
- Hourly and daily aggregated usage: long-term
- Ledger entries, recharge records, and audit logs: long-term
- Prompt and output text: never stored
- Request detail CSV export: last 15 days

### 4.10 Billing And Recharge

The first release uses prepaid balance and usage-based billing.

Rules:

- No overdraft.
- Requests are charged based on actual model, token usage, and price version.
- Balance changes must create immutable ledger entries.
- Amounts are stored and calculated as integer `micro_cny`.
- `1 CNY = 1,000,000 micro_cny`.
- User-facing UI displays amounts as CNY yuan.
- Workspace balance is stored for fast reads and updated in the same transaction as ledger entries.
- Recharge, grants, consumption, and refunds are separate ledger types.
- Historical bills keep their original price version.
- Each request consumption creates one long-term ledger entry linked to `request_id`.
- Request metadata is retained for 15 days, while consumption ledger entries are retained long-term.

Price display:

- Public price is the TokenLive price, not Provider cost.
- Prices are shown in CNY per one million tokens.
- Input, output, and cache-read prices are modeled separately.
- Price changes are versioned and affect only requests after the effective time.

Recharge:

- First release uses manual recharge review.
- User submits amount and payment proof.
- Admin reviews and credits balance.
- Recharge request approval creates a ledger entry; only the ledger entry changes balance.
- A recharge request can be credited at most once through idempotency controls.
- No automatic Alipay / WeChat Pay in the first release.
- No automatic refund, withdrawal, invoice, or contract support in the first release.

### 4.11 Budget And Alerts

First release includes email-only alerts:

- Low balance alerts to Owner and Billing.
- Daily or monthly budget threshold alerts.
- API key limit reached alerts to creator and Owner.

Users may disable non-critical alerts. Balance insufficient and settlement failure alerts cannot be disabled.

### 4.12 Support And Status

Support includes:

- Public documentation center
- Ticket system for logged-in users
- Public status page

Tickets support:

- Category
- Priority
- Request ID association
- Replies
- Status workflow
- Access-limit requests

Users can request through tickets:

- Higher Workspace budget
- Higher API key limits
- Access to restricted models
- Higher trial credit

All approvals are manual in Admin.

Status page:

- Public path `/status`
- Shows aggregated TokenLive service status and historical incidents
- Does not disclose Provider, Endpoint, or internal routing details
- Incidents are published and updated by Admin
- Portal stores public status page data; Admin publishes and updates it through internal APIs.
- Email subscription is deferred

### 4.13 Documentation

Docs are public and current-version only.

First release docs:

- Quick start
- Authentication
- Model list
- API examples
- Billing
- Errors
- Migration guide
- Changelog

No documentation version selector in the first release.

### 4.14 Localization

Primary market is China.

First release:

- Simplified Chinese default
- English available
- CNY balance and public pricing
- Content is modeled by language from the first version

Localized content includes:

- Model names and descriptions
- Tags
- Quick start and API example explanations
- Error code docs
- Billing rules
- SEO title and description

### 4.15 Data Export

CSV export is supported:

- Request details for the last 15 days
- Consumption ledger by time range
- Recharge records by time range
- Aggregated usage by model, API key, and date

No PDF bill, automatic monthly report email, or open data API in the first release.

### 4.16 Account And Workspace Deletion

Rules:

- User can leave Workspaces they do not own.
- Owner must transfer ownership before leaving.
- Workspace cannot be deleted while it has balance, pending recharge requests, or disputed billing.
- Workspace deletion is a 30-day soft delete with restore window.
- After the window, API keys and personal profile data are deleted where possible.
- Ledger, audit logs, and necessary request metadata are retained as required.
- User must resolve all owned Workspaces before deleting their personal account.

### 4.17 Responsive Design

Public pages are mobile-first:

- Home
- Model list
- Model detail
- Docs
- Pricing
- Status

Auth, recharge request, and ticket submission should work well on mobile.

Console table-heavy pages are desktop-first but must remain usable on mobile.

No app, mini program, or PWA in the first release.

### 4.18 Visual Direction

Style:

- Developer infrastructure, not exchange / crypto trading UI.
- Dark technology feel as primary direction.
- Light mode support.
- Black, white, and gray base palette with restrained brand accent.
- Dense model cards.
- Clear code examples.
- Tool-like billing and usage screens.
- Minimal motion, no heavy 3D.

## 5. Explicit Non-Goals

The first release does not include:

- Consumer chat product.
- Playground.
- Provider / Endpoint selection by users.
- Runtime routing configuration.
- Internal operations UI inside Portal.
- Automatic Alipay / WeChat Pay integration.
- Automatic refunds or withdrawals.
- Invoice and contract management.
- PDF bills.
- Automatic monthly reports.
- Documentation versioning.
- Public join links for Workspaces.
- Custom roles.
- IP allowlist or domain restrictions for API keys.
- Online live chat support.
- Community forum.
- Status page email subscription.
- Multi-region disaster recovery.
- Automatic account merging by email.
- Floating-point money calculations.
- Direct Portal editing of model catalog or model prices.

## 6. Success Metrics

Suggested launch metrics:

- Visitor to signup conversion.
- Signup to API key creation conversion.
- API key creation to first successful API call conversion.
- Time to first successful API call.
- Trial credit activation rate.
- First-week retained active Workspaces.
- Recharge request conversion.
- Support ticket volume per active Workspace.
- API key error rate after first integration.
