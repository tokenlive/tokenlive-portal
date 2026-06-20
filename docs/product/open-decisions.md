# TokenLive Portal Open Decisions

This file tracks decisions that are intentionally left open after the initial Portal product grilling session.

## 1. Trial Credit Numbers

Need final values for:

- Trial credit amount
- Daily trial spend limit
- Trial model whitelist
- Risk score thresholds
- Manual review process for high-risk trial users

Recommendation: choose conservative values for private beta, then tune from observed conversion and abuse data.

## 2. Default Workspace Budgets

Need final defaults for:

- Workspace daily budget
- Workspace monthly budget
- Low-balance alert threshold
- Budget alert thresholds

Recommendation: define defaults by customer type after pricing and trial amounts are finalized.

## 3. Model Performance Sample Thresholds

Need final thresholds for showing public service metrics:

- Minimum request count per model per window
- 24-hour and 7-day window definitions
- How to treat failed requests and user-side errors

Recommendation: exclude user-auth and client-validation failures from model health, but include upstream and gateway runtime failures.

## 4. Manual Recharge Workflow Details

Need final details for:

- Supported payment proof formats
- Minimum recharge amount
- Review SLA
- Rejection reasons
- Adjustment and refund policy

Recommendation: make recharge applications immutable after submission, with Admin-only review actions.

## 5. Internal Contract Format

Need final integration choice for first release:

- Internal REST only
- REST plus Redis Stream
- REST plus message queue

Recommendation: start with REST plus idempotency keys, add event queue when operational load justifies it.

## 6. Admin Ownership Of Shared Tables

Need final table ownership boundaries between `tokenlive-portal` and `tokenlive-admin`:

- Does Admin write Portal-owned tables directly?
- Or does Admin call Portal internal APIs?
- Which service owns migrations for shared business tables?

Recommendation: Portal Backend owns Portal user-facing business tables; Admin mutates them through internal APIs where possible.

## 7. Documentation Authoring Model

Need final choice:

- Markdown files in repository
- Database-backed CMS edited from Admin
- Hybrid model

Recommendation: start with repository-backed docs for developer API pages, and database-backed localized model catalog content.

## 8. OAuth Provider Configuration

Need final values for:

- Google OAuth app
- GitHub OAuth app
- Redirect domains
- Allowed organizations, if any
- Terms acceptance checkpoint

Recommendation: require explicit terms acceptance after OAuth callback before creating first Workspace.

## 9. CSV Export Limits

Need final limits:

- Maximum rows per export
- Synchronous vs asynchronous export
- Download expiry time

Recommendation: synchronous export for small ranges in beta, then background export when row limits become painful.

## 10. Brand System

Need final assets:

- Logo
- Brand color
- Typography preferences
- Dark and light theme tokens

Recommendation: create a small design token file before implementation to keep shadcn/ui customization consistent.

