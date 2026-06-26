# Frontend Activation Console Slice Design

Date: 2026-06-22

## 1. Goal

Build the first TokenLive Portal frontend slice as an independently deployable Web application under `web/`.

The slice should establish the frontend framework and deliver the first user-facing activation path:

```text
Open public entry -> Login -> Enter dashboard -> Create API key -> See first API call guidance
```

The frontend should connect directly to the existing Go Portal Backend / BFF rather than using mock data.

## 2. Confirmed Decisions

- Create `web/` as a Next.js application, parallel to the existing `backend/`.
- Use TypeScript, Tailwind CSS, shadcn/ui, and the Next.js App Router.
- Use Simplified Chinese as the default product language while keeping structure compatible with later English support.
- Support light, dark, and system themes from the first implementation.
- Keep `/` as a minimal public entry page, not a full marketing site.
- Put the main implementation focus on `/dashboard` and `/dashboard/api-keys`.
- Use real BFF endpoints via `NEXT_PUBLIC_API_BASE_URL`.
- Keep GitHub login visible but disabled until the backend endpoint exists.
- Keep model detail pages out of this slice; `/models` is a lightweight list only.

## 3. Scope

### 3.1 In Scope

Frontend foundation:

- `web/` project initialization.
- TypeScript configuration.
- Tailwind CSS setup.
- shadcn/ui setup.
- Shared theme tokens.
- Light / dark / system theme switching with persisted preference.
- Environment example for `NEXT_PUBLIC_API_BASE_URL`.
- API client wrapper with cookie credentials.
- Typed API modules for auth, public models, console overview, workspace, and API keys.

Pages:

- `/`
- `/login`
- `/terms`
- `/models`
- `/dashboard`
- `/dashboard/api-keys`

Layouts:

- Public layout.
- Auth layout.
- Dashboard layout.

Core components:

- `AppHeader`
- `DashboardShell`
- `ThemeToggle`
- `AuthPanel`
- `ActivationSummary`
- `ApiKeyTable`
- `CreateApiKeyDialog`
- `ModelList`
- `ModelCard`

### 3.2 Out Of Scope

- Full marketing website polish.
- Model detail pages.
- Usage dashboard.
- Billing and recharge pages.
- Workspace members and invitations.
- Workspace switcher beyond showing current Workspace.
- Favorite models.
- Support tickets.
- GitHub OAuth execution.
- Consumer chat Playground.
- Runtime gateway implementation.
- Admin or operations UI.

## 4. Routes And Behavior

### 4.1 `/`

Minimal public entry page.

Content:

- TokenLive product positioning.
- Primary CTA to `/dashboard`.
- Secondary CTA to `/models`.
- Header links for Models, disabled Docs and Pricing entries, Login / Dashboard.

The page should feel like a restrained developer product entry, not a large marketing landing page.

### 4.2 `/login`

Authentication entry page.

Supported actions:

- Start email verification with `POST /api/auth/email/start`.
- Verify email code with `POST /api/auth/email/verify`.
- Redirect to Google OAuth with `GET /api/auth/google/login`.
- Show GitHub OAuth as disabled until backend support exists.

Development behavior:

- If the email start response includes `dev_code`, show it in a small development-only hint.

Success behavior:

- After successful email verification, redirect to `/dashboard`.

### 4.3 `/terms`

Terms checkpoint page.

Behavior:

- Explain that terms must be accepted before console access.
- Call `POST /api/auth/accept-terms`.
- After success, refresh user/workspace state and redirect to `/dashboard`.

This page also handles users returned from OAuth who have a session but have not accepted terms.

### 4.4 `/models`

Lightweight public model marketplace list.

Endpoint:

- `GET /api/public/models?locale=zh-CN&limit=50`

Display:

- Model display name.
- Short description.
- Tags.
- Context length.
- Input and output modalities.
- Capabilities.
- Public price.
- Basic metrics when available.
- Graceful "collecting data" display when metrics are absent or low-signal.

No model detail page is included in this slice.

### 4.5 `/dashboard`

Activation overview page.

Endpoints:

- `GET /api/me`
- `GET /api/workspaces/current`
- `GET /api/console/overview`
- Optionally `GET /api/public/models?locale=zh-CN&limit=50` for recommended models.

Display:

- Current Workspace name.
- Trial credit state.
- API key creation state.
- First API call state.
- Primary CTA that changes by activation stage.
- API call guidance once an API key exists.
- Recommended model entry using public model data.

Guard behavior:

- Unauthenticated users redirect to `/login`.
- Terms-required users redirect to `/terms` or see a blocking terms acceptance panel.
- Authorized users see the dashboard content.

### 4.6 `/dashboard/api-keys`

API key management page.

Endpoints:

- `GET /api/api-keys`
- `POST /api/api-keys`
- `POST /api/api-keys/{id}/enable`
- `POST /api/api-keys/{id}/disable`
- `POST /api/api-keys/{id}/revoke`

Display:

- API key name.
- Key prefix and last four characters.
- Status.
- Created time.
- Last used time, if available.
- Operations for enable, disable, and revoke.

Create flow:

- User enters a required key name.
- Backend creates the key.
- Frontend displays the full secret exactly once.
- Secret panel supports copy.
- After closing the secret panel, the full secret is no longer available in UI state.

Safety:

- Revoke requires confirmation.
- Mutating action buttons are disabled while requests are in flight.
- After mutations, refresh the list from the backend.

## 5. Architecture

### 5.1 Project Layout

Expected structure:

```text
web/
  app/
    (public)/
    (auth)/
    dashboard/
  components/
    auth/
    dashboard/
    models/
    shell/
    ui/
  lib/
    api/
    format/
    theme/
  styles/
  public/
```

The exact route group names can change during implementation if Next.js conventions make another shape cleaner.

### 5.2 API Client

`lib/api/client.ts` owns fetch behavior:

- Prefix paths with `NEXT_PUBLIC_API_BASE_URL` when configured.
- Use `credentials: "include"`.
- Send and receive JSON.
- Parse backend error responses into a typed frontend error.
- Preserve backend request id when available.

Domain API modules:

- `lib/api/auth.ts`
- `lib/api/models.ts`
- `lib/api/console.ts`
- `lib/api/workspaces.ts`
- `lib/api/api-keys.ts`

Components and pages should call these modules rather than constructing raw fetch URLs.

### 5.3 Session And Guarding

Dashboard pages use a shared guard that calls `/api/me`.

States:

- Loading: show stable skeleton or loading region.
- Unauthorized: redirect to `/login`.
- Terms required: redirect to `/terms` or show a blocking acceptance panel.
- Authenticated: render dashboard content.

The guard must avoid blank pages during auth or network failures.

### 5.4 Theme

Use `next-themes` with `light`, `dark`, and `system`.

Rules:

- Default to `system`.
- Persist user choice.
- Use Tailwind and shadcn CSS variables.
- Avoid hard-coded component colors.
- Public pages, auth pages, dashboard pages, dialogs, empty states, errors, disabled buttons, and secret reveal panels must work in both light and dark themes.

## 6. Visual Direction

The UI should feel like a professional developer console:

- Quiet and work-focused.
- Information-dense but readable.
- Minimal decorative imagery.
- No large purple or blue gradient-dominated theme.
- No card-within-card layouts.
- Cards only for repeated items, status panels, dialogs, and framed tool surfaces.
- Dashboard first screen centers on activation status and the next action.
- Public home stays lightweight and directs users into models or dashboard.

## 7. Error Handling

Authentication errors:

- Unauthorized users are routed to `/login`.
- Terms-required users are routed to `/terms` or shown a blocking terms panel.

API errors:

- Show human-readable message.
- Include request id when available.
- Keep form input after failed mutations.
- Provide retry affordance for list or overview load failures.

Network errors:

- Show a retry block.
- Do not collapse layout or render a blank page.

API key errors:

- Creation failure keeps the entered name.
- Mutation failure leaves the row in its previous known state and allows retry.

Model errors:

- `/models` can show a full-page retry state.
- Dashboard model recommendation failure should not block activation overview.

## 8. Testing And Verification

Automated checks:

- TypeScript check.
- Lint.
- Production build.
- Unit tests for API error mapping.
- Unit tests for money / price formatting.
- Unit tests for API key secret masking helpers.
- Component tests for login form, activation summary, and API key table if the test stack is added in this slice.

Manual browser verification:

- `/` renders in light and dark themes.
- `/login` can complete email-code login against the backend in development.
- `/dashboard` redirects unauthenticated users to `/login`.
- `/dashboard` renders activation state for an authenticated user.
- `/dashboard/api-keys` can create a key and reveal the secret once.
- `/dashboard/api-keys` can enable, disable, and revoke keys.
- `/models` renders real public model data.
- Theme toggle works on public, auth, and dashboard pages.

E2E tests are deferred until backend local seed data and browser test fixtures are stable.

## 9. Implementation Order

1. Initialize `web/` with Next.js, TypeScript, Tailwind CSS, shadcn/ui, and theme support.
2. Add environment example and API client foundation.
3. Add public, auth, and dashboard layouts.
4. Implement `/`, `/login`, and `/terms`.
5. Implement dashboard guard and `/dashboard`.
6. Implement `/dashboard/api-keys`.
7. Implement `/models`.
8. Run checks and browser verification in light and dark themes.

## 10. Open Follow-Ups

The following are intentionally outside this slice but should be revisited soon:

- Whether model detail pages should be the next public-site slice.
- Whether `/docs` and `/pricing` should be static lightweight pages or backed by content files.
- Whether to add Playwright once backend fixture data is stable.
- Whether GitHub OAuth should be the next auth backend slice.
