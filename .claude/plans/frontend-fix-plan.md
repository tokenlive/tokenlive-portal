# Portal Frontend Fix Plan

## Current State Analysis

The portal frontend has a good foundation but has several mismatches between frontend types and backend API responses.

## Issues Identified

### 1. API Type Mismatches
- **Model List Response**: Frontend expects flat structure, backend returns nested `Price` and `Metrics` objects
- **Workspace Response**: Frontend expects simple fields, backend returns nested `Balance` structure
- **Activation Steps**: Frontend expects `title`/`description`, backend uses `label`

### 2. Missing Features
- No register page (only login)
- No accept terms flow
- No model detail page
- No proper session handling in console layout
- Missing API error handling

### 3. Minor Issues
- Console sidebar sign out button not functional
- Model card logo URL not accessible from nested structure

## Implementation Plan

### Phase 1: Fix Type Definitions
1. Update `types/api.ts` to match backend response structures
2. Update `lib/api.ts` API client functions
3. Update components to use new type structures

### Phase 2: Fix/Add Missing Features
1. Add registration page
2. Add terms acceptance page/flow
3. Add model detail page
4. Implement functional sign out
5. Add proper session/auth checks

### Phase 3: Polish & Test
1. Test all flows end-to-end
2. Add loading/error states
3. Polish UI details

## Files to Modify

- `src/types/api.ts` - Update type definitions
- `src/lib/api.ts` - Update API client
- `src/app/console/dashboard/page.tsx` - Update for new workspace structure
- `src/app/console/api-keys/page.tsx` - Verify API key handling
- `src/app/console/settings/page.tsx` - Fix sign out
- `src/app/console/layout.tsx` - Add session check
- `src/components/landing/model-card.tsx` - Update for nested price/metrics
- `src/app/register/page.tsx` - Add new
- `src/app/accept-terms/page.tsx` - Add new
- `src/app/models/[slug]/page.tsx` - Add new
