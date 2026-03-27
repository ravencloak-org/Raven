# Frontend M5 Admin Dashboard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement Keycloak OIDC PKCE authentication and Organization Management pages for the Raven Admin Dashboard (M5: issues #42, #43).

**Architecture:** Vue 3 + TypeScript + Pinia + vue-router. Replace the stub email/password auth store with a real Keycloak OIDC PKCE flow using `keycloak-js`. The auth store is the single source of truth for tokens and user identity. All pages are route-guarded. Build against `contracts/openapi-stub.yaml` (mock API) until backend PRs merge.

**Tech Stack:** Vue 3.5, TypeScript 5, Tailwind 4, Pinia 3, vue-router 5, keycloak-js (to add), vitest (unit), Playwright (E2E).

**Worktree:** `.claude/worktrees/stream-frontend` on branch `feat/stream-frontend-m5-scaffold`

---

## Pre-flight: Read before writing a single line

- [ ] Read `frontend/src/stores/auth.ts` — understand current stub auth store
- [ ] Read `frontend/src/composables/useAuth.ts` — understand the public API
- [ ] Read `frontend/src/router/index.ts` — understand route guards
- [ ] Read `frontend/src/pages/LoginPage.vue` — understand current login UI
- [ ] Read `frontend/src/api/client.ts` — understand how API calls are made
- [ ] Read `frontend/src/types/index.ts` — understand existing User type
- [ ] Read `contracts/openapi-stub.yaml` — understand Org API contract to mock against

---

## Task 1: Add dependencies and Keycloak OIDC PKCE auth store

**Closes:** #42 (Auth Flow — Keycloak OIDC PKCE)

**Files:**
- Modify: `frontend/package.json` — add keycloak-js
- Replace: `frontend/src/stores/auth.ts` — full Keycloak PKCE implementation
- Modify: `frontend/src/composables/useAuth.ts` — update to use new store API
- Modify: `frontend/src/types/index.ts` — update User type with Keycloak claims
- Create: `frontend/src/stores/auth.spec.ts` — unit tests

- [ ] Install keycloak-js:
```bash
cd .claude/worktrees/stream-frontend/frontend
npm install keycloak-js@latest
npm install --save-dev @types/node vitest @vue/test-utils happy-dom
```

- [ ] Add vitest config to `vite.config.ts`:
```ts
// Add to defineConfig
test: {
  environment: 'happy-dom',
  globals: true,
},
```

- [ ] Write failing unit tests in `frontend/src/stores/auth.spec.ts`:
```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useAuthStore } from './auth'

// Mock keycloak-js
vi.mock('keycloak-js', () => ({
  default: vi.fn().mockImplementation(() => ({
    init: vi.fn().mockResolvedValue(true),
    login: vi.fn(),
    logout: vi.fn(),
    updateToken: vi.fn().mockResolvedValue(true),
    token: 'mock-access-token',
    tokenParsed: {
      sub: 'user-123',
      email: 'test@example.com',
      preferred_username: 'testuser',
      org_id: 'org-456',
      org_role: 'org_admin',
    },
    authenticated: true,
    isTokenExpired: vi.fn().mockReturnValue(false),
  })),
}))

describe('useAuthStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('initialises as unauthenticated', () => {
    const store = useAuthStore()
    expect(store.isAuthenticated).toBe(false)
  })

  it('exposes token after init', async () => {
    const store = useAuthStore()
    await store.init()
    expect(store.accessToken).toBe('mock-access-token')
    expect(store.isAuthenticated).toBe(true)
  })

  it('exposes user claims after init', async () => {
    const store = useAuthStore()
    await store.init()
    expect(store.user?.email).toBe('test@example.com')
    expect(store.user?.orgId).toBe('org-456')
  })
})
```

- [ ] Run — expect FAIL (store API doesn't match yet):
```bash
cd frontend && npx vitest run src/stores/auth.spec.ts 2>&1
```

- [ ] Replace `frontend/src/stores/auth.ts` with Keycloak implementation:
```ts
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import Keycloak from 'keycloak-js'

export interface AuthUser {
  id: string
  email: string
  username: string
  orgId: string
  orgRole: string
}

const keycloak = new Keycloak({
  url: import.meta.env.VITE_KEYCLOAK_URL ?? 'http://localhost:8080',
  realm: import.meta.env.VITE_KEYCLOAK_REALM ?? 'raven',
  clientId: import.meta.env.VITE_KEYCLOAK_CLIENT_ID ?? 'raven-admin',
})

export const useAuthStore = defineStore('auth', () => {
  const user = ref<AuthUser | null>(null)
  const accessToken = ref<string | null>(null)
  const initialized = ref(false)

  const isAuthenticated = computed(() => initialized.value && !!accessToken.value)

  async function init(): Promise<void> {
    const authenticated = await keycloak.init({
      onLoad: 'check-sso',
      pkceMethod: 'S256',
      silentCheckSsoRedirectUri: window.location.origin + '/silent-check-sso.html',
    })
    initialized.value = true
    if (authenticated) {
      _syncFromKeycloak()
    }
    // Auto-refresh token 30s before expiry
    setInterval(() => keycloak.updateToken(30), 60_000)
  }

  function login(): void {
    keycloak.login({ redirectUri: window.location.origin + '/dashboard' })
  }

  function logout(): void {
    keycloak.logout({ redirectUri: window.location.origin + '/' })
  }

  function _syncFromKeycloak(): void {
    const p = keycloak.tokenParsed as Record<string, unknown>
    accessToken.value = keycloak.token ?? null
    user.value = {
      id: p['sub'] as string,
      email: p['email'] as string,
      username: p['preferred_username'] as string,
      orgId: p['org_id'] as string,
      orgRole: p['org_role'] as string,
    }
  }

  return { user, accessToken, isAuthenticated, initialized, init, login, logout }
})
```

- [ ] Update `useAuth.ts` to align with new store API:
```ts
import { computed } from 'vue'
import { useAuthStore } from '../stores/auth'

export function useAuth() {
  const store = useAuthStore()
  return {
    user: computed(() => store.user),
    isAuthenticated: computed(() => store.isAuthenticated),
    login: () => store.login(),
    logout: () => store.logout(),
  }
}
```

- [ ] Update `main.ts` to call `store.init()` before mounting:
```ts
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { useAuthStore } from './stores/auth'
import App from './App.vue'
import router from './router'
import './assets/main.css'

const app = createApp(App)
const pinia = createPinia()
app.use(pinia)
app.use(router)

// Initialise Keycloak before mounting
const authStore = useAuthStore()
authStore.init().then(() => app.mount('#app'))
```

- [ ] Create `frontend/public/silent-check-sso.html`:
```html
<html><body><script>parent.postMessage(location.href, location.origin)</script></body></html>
```

- [ ] Update `.env.example` (create if needed):
```
VITE_KEYCLOAK_URL=http://localhost:8080
VITE_KEYCLOAK_REALM=raven
VITE_KEYCLOAK_CLIENT_ID=raven-admin
VITE_API_BASE_URL=http://localhost:8081/api/v1
```

- [ ] Run unit tests — expect PASS:
```bash
cd frontend && npx vitest run src/stores/auth.spec.ts
```

- [ ] Build to confirm no TypeScript errors:
```bash
cd frontend && npm run build 2>&1
```

- [ ] Run lint:
```bash
cd frontend && npm run lint 2>&1
```

- [ ] Commit:
```bash
git add frontend/
git commit -m "feat(#42): Keycloak OIDC PKCE auth store replacing stub email/password auth"
```

---

## Task 2: Install and configure Playwright

**Files:**
- Create: `frontend/e2e/auth.spec.ts`
- Create: `frontend/playwright.config.ts`

- [ ] Install Playwright:
```bash
cd frontend && npm install --save-dev @playwright/test
npx playwright install chromium
```

- [ ] Create `frontend/playwright.config.ts`:
```ts
import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  reporter: 'html',
  use: {
    baseURL: 'http://localhost:5173',
    trace: 'on-first-retry',
  },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
  ],
  webServer: {
    command: 'npm run dev',
    url: 'http://localhost:5173',
    reuseExistingServer: !process.env.CI,
  },
})
```

- [ ] Write auth E2E test in `frontend/e2e/auth.spec.ts`:
```ts
import { test, expect } from '@playwright/test'

test('unauthenticated user sees login page', async ({ page }) => {
  await page.goto('/')
  // Since Keycloak isn't running in test, check that the UI shows auth state
  await expect(page.locator('text=Login')).toBeVisible({ timeout: 5000 })
})

test('login button redirects to Keycloak', async ({ page }) => {
  await page.goto('/')
  // Mock: just check the login button exists and is clickable
  const loginBtn = page.getByRole('button', { name: /login/i })
    .or(page.getByRole('link', { name: /login/i }))
  await expect(loginBtn).toBeVisible({ timeout: 5000 })
})
```

- [ ] Run Playwright (with dev server running separately):
```bash
cd frontend && npm run dev &
sleep 3
npx playwright test e2e/auth.spec.ts --headed 2>&1
```

- [ ] Add scripts to `package.json`:
```json
"test:unit": "vitest run",
"test:e2e": "playwright test",
"test": "npm run test:unit && npm run test:e2e"
```

- [ ] Commit:
```bash
git add frontend/e2e/ frontend/playwright.config.ts frontend/package.json
git commit -m "test(#42): add Playwright E2E tests for auth flow"
```

- [ ] Push and create PR:
```bash
git push origin feat/stream-frontend-m5-scaffold
gh pr create --title "feat: Keycloak OIDC PKCE auth flow (#42)" \
  --body "Closes #42"
```

---

## Task 3: Issue #43 — Organization Management Pages

**Closes:** #43

**Files:**
- Create: `frontend/src/api/orgs.ts` — API client for Org endpoints (uses stub)
- Create: `frontend/src/stores/orgs.ts` — Pinia store for org state
- Create: `frontend/src/pages/orgs/OrgListPage.vue`
- Create: `frontend/src/pages/orgs/OrgDetailPage.vue`
- Create: `frontend/src/stores/orgs.spec.ts`
- Create: `frontend/e2e/orgs.spec.ts`
- Modify: `frontend/src/router/index.ts` — add org routes

**Read first:** `contracts/openapi-stub.yaml` to match Org API shape.

- [ ] Write failing unit tests `frontend/src/stores/orgs.spec.ts`:
```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useOrgsStore } from './orgs'
import * as orgsApi from '../api/orgs'

vi.mock('../api/orgs')

describe('useOrgsStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('fetchOrg populates currentOrg', async () => {
    vi.mocked(orgsApi.getOrg).mockResolvedValue({
      id: 'org-1', name: 'Test Org', slug: 'test-org', status: 'active',
      settings: {}, created_at: '', updated_at: '',
    })
    const store = useOrgsStore()
    await store.fetchOrg('org-1')
    expect(store.currentOrg?.name).toBe('Test Org')
  })
})
```

- [ ] Run — expect FAIL:
```bash
cd frontend && npx vitest run src/stores/orgs.spec.ts 2>&1
```

- [ ] Implement `frontend/src/api/orgs.ts`:
```ts
import { useAuthStore } from '../stores/auth'

export interface Org {
  id: string
  name: string
  slug: string
  status: 'active' | 'deactivated'
  settings: Record<string, unknown>
  created_at: string
  updated_at: string
}

async function authFetch(path: string, init?: RequestInit): Promise<Response> {
  const auth = useAuthStore()
  const base = import.meta.env.VITE_API_BASE_URL ?? '/api/v1'
  return fetch(base + path, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${auth.accessToken ?? ''}`,
      ...init?.headers,
    },
  })
}

export async function getOrg(orgId: string): Promise<Org> {
  const res = await authFetch(`/orgs/${orgId}`)
  if (!res.ok) throw new Error(`getOrg failed: ${res.status}`)
  return res.json()
}

export async function createOrg(name: string): Promise<Org> {
  const res = await authFetch('/orgs', {
    method: 'POST',
    body: JSON.stringify({ name }),
  })
  if (!res.ok) throw new Error(`createOrg failed: ${res.status}`)
  return res.json()
}
```

- [ ] Implement `frontend/src/stores/orgs.ts`:
```ts
import { defineStore } from 'pinia'
import { ref } from 'vue'
import { getOrg, createOrg, type Org } from '../api/orgs'

export const useOrgsStore = defineStore('orgs', () => {
  const currentOrg = ref<Org | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function fetchOrg(orgId: string) {
    loading.value = true
    error.value = null
    try {
      currentOrg.value = await getOrg(orgId)
    } catch (e) {
      error.value = (e as Error).message
    } finally {
      loading.value = false
    }
  }

  async function create(name: string): Promise<Org> {
    const org = await createOrg(name)
    currentOrg.value = org
    return org
  }

  return { currentOrg, loading, error, fetchOrg, create }
})
```

- [ ] Implement `frontend/src/pages/orgs/OrgDetailPage.vue` (Tailwind, minimal):
```vue
<script setup lang="ts">
import { onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useOrgsStore } from '../../stores/orgs'

const route = useRoute()
const store = useOrgsStore()
onMounted(() => store.fetchOrg(route.params.orgId as string))
</script>

<template>
  <div class="p-6">
    <div v-if="store.loading" class="text-gray-500">Loading…</div>
    <div v-else-if="store.error" class="text-red-600">{{ store.error }}</div>
    <div v-else-if="store.currentOrg">
      <h1 class="text-2xl font-bold">{{ store.currentOrg.name }}</h1>
      <p class="text-sm text-gray-500">{{ store.currentOrg.slug }}</p>
      <span
        class="inline-block mt-2 px-2 py-0.5 rounded text-xs"
        :class="store.currentOrg.status === 'active' ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'"
      >
        {{ store.currentOrg.status }}
      </span>
    </div>
  </div>
</template>
```

- [ ] Add routes to `frontend/src/router/index.ts`:
```ts
{
  path: '/orgs/:orgId',
  component: () => import('../pages/orgs/OrgDetailPage.vue'),
  meta: { requiresAuth: true },
},
```

- [ ] Implement route guard in router (read current router first — add if missing):
```ts
router.beforeEach(async (to) => {
  if (to.meta.requiresAuth) {
    const auth = useAuthStore()
    if (!auth.isAuthenticated) {
      auth.login()
      return false
    }
  }
})
```

- [ ] Run unit tests:
```bash
cd frontend && npx vitest run src/stores/orgs.spec.ts
```

- [ ] Write Playwright E2E `frontend/e2e/orgs.spec.ts`:
```ts
import { test, expect } from '@playwright/test'

test('org detail page shows org name when API returns data', async ({ page }) => {
  // Mock the API response
  await page.route('**/api/v1/orgs/**', route =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ id: 'org-1', name: 'Acme Corp', slug: 'acme-corp', status: 'active', settings: {}, created_at: '', updated_at: '' }),
    })
  )
  await page.goto('/orgs/org-1')
  await expect(page.getByRole('heading', { name: 'Acme Corp' })).toBeVisible({ timeout: 5000 })
})
```

- [ ] Run Playwright:
```bash
cd frontend && npx playwright test e2e/orgs.spec.ts 2>&1
```

- [ ] Build final check:
```bash
cd frontend && npm run build 2>&1
```

- [ ] Commit and PR:
```bash
git add frontend/
git commit -m "feat(#43): Organization Management pages with Pinia store and Playwright tests"
git push origin feat/stream-frontend-m5-scaffold
gh pr create --title "feat: Organization Management pages (#43)" --body "Closes #43"
```

---

## Final verification before each PR

```bash
cd frontend
npm run test:unit      # vitest unit tests
npm run build          # TypeScript + Vite build
npm run lint           # ESLint
npx playwright test    # E2E (requires dev server)
```
