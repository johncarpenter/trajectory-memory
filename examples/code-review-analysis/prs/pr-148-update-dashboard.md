# PR #148: Update dashboard UI

**Author:** frank
**Branch:** feature/dashboard-v2
**Merged:** 2024-01-15
**Reviewers:** lisa, bob
**Files changed:** 24
**Additions:** 1,245, Deletions:** 890

## Description
Complete redesign of the dashboard UI based on new design specs. Updates charts, tables, and navigation.

## Review Comments

**lisa:** This PR is really large. Can we break it into smaller pieces?
> **frank:** The changes are interconnected, hard to split. I've organized commits logically though.

**bob:** The new chart library is 450KB. That's significant. Did we evaluate lighter alternatives?
> **frank:** Checked with design, they need the specific animations. Added lazy loading to mitigate.

**lisa:** I see a lot of inline styles. We should use the design system tokens.
> **frank:** Refactored to use CSS variables from our theme.

**bob:** No tests for the new dashboard components?
> **frank:** Added unit tests for the data transformation functions. UI tests are in a follow-up PR.

**lisa:** The API calls are made directly in components. Should use our data fetching hooks for consistency.
> **frank:** Migrated to useQuery hooks.

## Files Changed
- `components/Dashboard/*` - 15 files (new dashboard components)
- `hooks/useDashboardData.ts` - new data fetching hook
- `styles/dashboard.css` - dashboard-specific styles
- `utils/chartHelpers.ts` - chart data transformation
- `package.json` - added chart library dependency

## Tests
- Unit tests for `chartHelpers.ts`
- Integration tests deferred to follow-up PR

## CI Status
All checks passed. Bundle size increased by 125KB (lazy loaded).
