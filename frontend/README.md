# Raven Frontend

Vue.js 3 admin dashboard for the Raven platform. Mobile-first, Tailwind CSS, TypeScript.

## Tech Stack

- **Vue 3** — Composition API + `<script setup>`
- **Pinia** — state management
- **Vue Router** — client-side routing
- **Tailwind CSS** — utility-first styling
- **TypeScript** — strict mode
- **Vite** — build tool
- **Vitest** — unit tests
- **Playwright** — end-to-end tests
- **keycloak-js** — OIDC authentication

## Getting Started

```bash
npm install
npm run dev       # http://localhost:3000
```

Requires the Go API and Keycloak running. Easiest way:

```bash
# From repo root
docker compose up -d postgres valkey keycloak go-api
```

Then set your `.env` in the repo root (copy `.env.example`). Vite proxies `/api/v1` to `localhost:8080` in dev.

## Environment Variables

Vite env vars are prefixed with `VITE_`. Copy `frontend/.env.example` if it exists, otherwise set:

```bash
VITE_API_BASE_URL=http://localhost:8080/api/v1
VITE_KEYCLOAK_URL=http://localhost:8081
VITE_KEYCLOAK_REALM=raven
VITE_KEYCLOAK_CLIENT_ID=raven-admin
VITE_POSTHOG_API_KEY=         # leave empty to disable analytics
VITE_HYPERSWITCH_PUBLISHABLE_KEY=  # only needed for billing flow
```

## Scripts

```bash
npm run dev          # Vite dev server with HMR
npm run build        # Production build (vue-tsc + vite)
npm run preview      # Preview production build locally
npm run lint         # ESLint
npm run lint:fix     # ESLint with auto-fix
npm run format       # Prettier
npm run test:unit    # Vitest (fast, no browser)
npm run test:e2e     # Playwright (requires running stack)
npm run test         # unit + e2e
```

## Project Structure

```
src/
├── api/            # Typed fetch wrappers (one file per backend resource)
│   ├── utils.ts    # Shared authFetch with 402 interceptor
│   ├── billing.ts
│   ├── knowledge-bases.ts
│   └── ...
├── components/     # Reusable components
│   ├── AppSidebar.vue
│   ├── AppHeader.vue
│   ├── UsageBar.vue
│   ├── PlanCard.vue
│   ├── UpgradePromptBanner.vue
│   └── ...
├── composables/    # Composition API utilities
│   ├── useFeatureFlag.ts   # PostHog feature flags
│   ├── useOnboarding.ts    # First-run wizard state
│   └── useMobile.ts
├── layouts/
│   ├── DefaultLayout.vue   # Authenticated pages (sidebar + header)
│   └── AuthLayout.vue      # Login / legal pages
├── pages/          # Route views (mirrored in router/index.ts)
│   ├── billing/
│   ├── knowledge-bases/
│   ├── onboarding/
│   ├── voice/
│   └── ...
├── router/
│   └── index.ts    # All routes + beforeEach guard
├── stores/         # Pinia stores
│   ├── auth.ts     # Keycloak session, user identity
│   ├── billing.ts  # Subscription, plans, usage, quota flag
│   └── ...
└── types/          # Shared TypeScript interfaces
```

## Authentication

Auth is handled by `stores/auth.ts` using `keycloak-js`. On app init, Keycloak checks for an existing SSO session silently. Unauthenticated users are redirected to Keycloak login.

The JWT contains custom claims set by the Keycloak `raven-org` client scope:
- `org_id` — the user's organisation UUID
- `org_role` — `org_admin` or `org_member`

Access token is available as `useAuthStore().accessToken` and is automatically included by `authFetch`.

## Feature Flags

PostHog feature flags gate optional features (e.g. billing UI):

```typescript
import { useFeatureFlag } from '../composables/useFeatureFlag'

const { isEnabled: billingEnabled } = useFeatureFlag('billing_enabled')
```

Leave `VITE_POSTHOG_API_KEY` empty during local development — all flags default to `false`, which safely hides billing/premium UI.

## Adding a New Page

1. `src/api/<resource>.ts` — API functions using `authFetch` from `./utils`
2. `src/stores/<resource>.ts` — Pinia store (copy the pattern from `stores/billing.ts`)
3. `src/pages/<feature>/YourPage.vue` — Vue SFC with `<script setup>`
4. Add route in `src/router/index.ts` inside the `DefaultLayout` children
5. Add nav item to `src/components/AppSidebar.vue` if needed

## Testing

Unit tests use Vitest and live alongside the source (`*.spec.ts` or in `tests/unit/`).

E2E tests use Playwright and live in `e2e/`. They require a running stack:

```bash
# Start everything
docker compose up -d

# Run E2E
npm run test:e2e

# Run a specific spec
npx playwright test e2e/billing/billing.spec.ts --headed
```
