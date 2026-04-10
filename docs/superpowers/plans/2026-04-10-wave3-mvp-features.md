# Wave 3: MVP Frontend Features Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver the final two MVP Launch features -- a billing/subscription management UI (#194) and Keycloak realm auto-provisioning with a tenant onboarding wizard (#197) -- to complete the MVP milestone.

**Architecture:** The billing UI is a Vue 3 frontend feature at `/settings/billing` that consumes the existing backend billing endpoints (`GET /billing/plans`, `POST /billing/subscriptions`, `GET /billing/usage`) and integrates Hyperswitch's client SDK for payment collection via Razorpay. The Keycloak onboarding feature adds a `POST /internal/provision-realm` backend endpoint that calls the Keycloak Admin REST API to create per-tenant realms, plus a frontend onboarding wizard shown on first login. Both features follow the existing handler/service/repository pattern on the backend and the Pinia store + API module + page component pattern on the frontend.

**Tech Stack:** Vue 3.5, Tailwind CSS 4, Pinia 3, TypeScript, vue-router 5, Go 1.24, Gin, pgx/v5, Keycloak Admin REST API, Hyperswitch Client SDK, Vitest, httptest

---

## File Structure

### #194 -- Billing and Subscription Management UI

| Action | Path | Responsibility |
|--------|------|----------------|
| Create | `frontend/src/api/billing.ts` | API client for billing endpoints (`getPlans`, `getUsage`, `createSubscription`, `cancelSubscription`, `createPaymentIntent`) |
| Create | `frontend/src/stores/billing.ts` | Pinia store for billing state (plans, current subscription, usage data) |
| Create | `frontend/src/pages/settings/BillingPage.vue` | Plan selection cards, usage dashboard, payment flow |
| Create | `frontend/src/components/billing/PlanCard.vue` | Individual plan card component (Free/Pro/Enterprise) |
| Create | `frontend/src/components/billing/UsageDashboard.vue` | Usage bars showing consumption vs limits |
| Create | `frontend/src/components/billing/UpgradePrompt.vue` | Global 402 upgrade prompt modal |
| Create | `frontend/src/components/billing/PaymentModal.vue` | Hyperswitch SDK payment collection modal |
| Create | `frontend/src/stores/billing.spec.ts` | Unit tests for billing store |
| Modify | `frontend/src/router/index.ts` | Add `/settings/billing` route |
| Modify | `frontend/src/api/billing.ts` | Add 402 response handling inside `authFetch` for upgrade prompts |
| Modify | `frontend/src/components/AppSidebar.vue` | Add Settings/Billing nav link |

### #197 -- Keycloak Realm Auto-Provisioning and Tenant Onboarding Wizard

| Action | Path | Responsibility |
|--------|------|----------------|
| Create | `internal/keycloak/admin.go` | Keycloak Admin API client (create realm, create client, set redirect URIs) |
| Create | `internal/keycloak/admin_test.go` | Unit tests for Keycloak admin client (httptest mock server) |
| Create | `internal/service/provisioning.go` | `ProvisioningService` -- orchestrates realm creation, client config, org update |
| Create | `internal/service/provisioning_test.go` | Unit tests for provisioning service (mock Keycloak client + mock repo) |
| Create | `internal/handler/provisioning.go` | `ProvisioningHandler` -- `POST /internal/provision-realm` endpoint |
| Create | `internal/handler/provisioning_test.go` | Handler-level tests (httptest) |
| Create | `internal/model/provisioning.go` | `ProvisionRealmRequest`, `ProvisionRealmResponse` types |
| Create | `frontend/src/pages/onboarding/OnboardingWizardPage.vue` | Multi-step onboarding wizard for new tenants |
| Create | `frontend/src/api/onboarding.ts` | API client for onboarding (check tenant status, trigger provisioning) |
| Create | `frontend/src/stores/onboarding.ts` | Pinia store for onboarding wizard state |
| Create | `frontend/src/stores/onboarding.spec.ts` | Unit tests for onboarding store |
| Modify | `internal/config/config.go` | Add `KeycloakAdminURL`, `KeycloakAdminUser`, `KeycloakAdminPassword` to `KeycloakConfig` |
| Modify | `internal/repository/org.go` | Add `UpdateKeycloakRealm(ctx, orgID, realm)` method |
| Modify | `cmd/api/main.go` | Wire provisioning handler, register on `/api/v1/internal` group |
| Modify | `frontend/src/router/index.ts` | Add `/onboarding` route with first-run guard |

---

## Feature A: Billing and Subscription Management UI (#194)

### Task 0: Write design spec for billing UI

**Purpose:** Document the UI/UX decisions, Hyperswitch SDK integration approach, and 402 interception strategy before writing code.

- [ ] **Step 1: Create design spec file**

Create `docs/superpowers/specs/194-billing-subscription-ui.md` with the following sections:

```markdown
# Design Spec: Billing and Subscription Management UI (#194)

## Overview
Frontend billing page at `/settings/billing` for plan management, payment collection, and usage monitoring.

## Dependencies
- #193 (billing subscription enforcement) must be merged -- provides:
  - `GET /api/v1/billing/plans` -- returns plan definitions
  - `GET /api/v1/billing/usage` -- returns current-period usage vs limits
  - `POST /api/v1/billing/subscriptions` -- create subscription (returns client_secret for paid plans)
  - `DELETE /api/v1/billing/subscriptions/:id` -- cancel subscription
  - `POST /api/v1/billing/payment-intents` -- create payment intent
  - 402 responses with `{"upgrade_required": true, "limit": N}` on quota exceeded

## UI Layout

### /settings/billing page
1. **Current Plan Banner** -- shows active plan name, status, billing period
2. **Plan Selection Cards** -- 3 cards (Free/Pro/Enterprise) in a responsive grid
   - Current plan highlighted with "Current Plan" badge
   - Upgrade/Downgrade CTA buttons
   - Feature comparison list per card
3. **Usage Dashboard** -- progress bars for each quota dimension:
   - Knowledge Bases: X / Y used
   - Team Members (seats): X / Y used
   - Voice Minutes: X / Y used this period
   - Concurrent Voice Sessions: X / Y active
4. **Billing History** (future -- out of MVP scope, show placeholder)

### Payment Flow (Hyperswitch SDK)
1. User clicks "Upgrade to Pro" or "Upgrade to Enterprise"
2. Frontend calls `POST /billing/subscriptions` with `{plan_id: "plan_pro"}`
3. Backend creates Hyperswitch payment, returns `{client_secret: "..."}`
4. Frontend opens Hyperswitch SDK checkout modal with the client_secret
5. Hyperswitch SDK handles payment (Razorpay as connector -- supports UPI, cards, etc.)
6. On success, SDK callback triggers page refresh to show new plan
7. Backend webhook confirms payment and activates subscription

### 402 Upgrade Prompt
- 402 handling is inlined in `billing.ts`'s own `authFetch` (not `client.ts`, which billing does not call through)
- Shows a modal with "You've reached your plan limit" message
- CTA button links to `/settings/billing`
- Displays which limit was hit (from response body)

### Responsive Design
- Mobile: cards stack vertically, usage bars full-width
- Desktop: 3-column card grid, usage in 2-column layout

## Environment Variables
- `VITE_HYPERSWITCH_PUBLISHABLE_KEY` -- Hyperswitch client-side publishable key
- `VITE_HYPERSWITCH_BASE_URL` -- Hyperswitch SDK base URL (default: https://api.hyperswitch.io)

## Out of Scope (MVP)
- Invoice PDF download
- Payment method management
- Billing history table
- Plan downgrade with prorated refund logic
```

- [ ] **Step 2: Review spec for completeness**

Verify the spec covers: plan cards, payment flow, usage dashboard, 402 handling, mobile responsiveness, and environment variables.

- [ ] **Step 3: Commit the spec**

```bash
git add docs/superpowers/specs/194-billing-subscription-ui.md
git commit -m "docs: add design spec for billing subscription UI (#194)"
```

---

### Task 1: Create billing API client module

**Files:** Create `frontend/src/api/billing.ts`

- [ ] **Step 1: Create the API client file**

Create `frontend/src/api/billing.ts`:

```typescript
import { useAuthStore } from '../stores/auth'

// --- Interfaces ---

export interface Plan {
  id: string
  tier: 'free' | 'pro' | 'enterprise'
  name: string
  price_monthly: number // cents
  max_users: number // -1 = unlimited
  max_workspaces: number
  max_kbs: number
  max_storage_mb: number
  max_concurrent_voice_sessions: number
  max_voice_minutes_monthly: number
}

export interface Subscription {
  id: string
  org_id: string
  plan_id: string
  status: 'active' | 'canceled' | 'past_due' | 'trialing' | 'paused' | 'expired'
  hyperswitch_subscription_id?: string
  current_period_start: string
  current_period_end: string
  created_at: string
  client_secret?: string
}

export interface UsageResponse {
  plan: Plan
  kbs_used: number
  kbs_limit: number
  seats_used: number
  seats_limit: number
  voice_minutes_used: number
  voice_minutes_limit: number
  concurrent_voice_used: number
  concurrent_voice_limit: number
}

export interface CreateSubscriptionRequest {
  plan_id: string
}

export interface CreatePaymentIntentRequest {
  amount: number
  currency: string
}

export interface PaymentIntent {
  id: string
  org_id: string
  amount: number
  currency: string
  status: string
  hyperswitch_payment_id?: string
  client_secret?: string
  created_at: string
}

// --- Auth helper (follows existing api/ module pattern) ---

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

// --- API functions ---

export async function getPlans(): Promise<Plan[]> {
  const res = await authFetch('/billing/plans')
  if (!res.ok) throw new Error(`getPlans failed: ${res.status}`)
  return res.json()
}

export async function getUsage(): Promise<UsageResponse> {
  const res = await authFetch('/billing/usage')
  if (!res.ok) throw new Error(`getUsage failed: ${res.status}`)
  return res.json()
}

export async function createSubscription(
  req: CreateSubscriptionRequest,
): Promise<Subscription> {
  const res = await authFetch('/billing/subscriptions', {
    method: 'POST',
    body: JSON.stringify(req),
  })
  if (!res.ok) throw new Error(`createSubscription failed: ${res.status}`)
  return res.json()
}

export async function cancelSubscription(subscriptionId: string): Promise<void> {
  const res = await authFetch(`/billing/subscriptions/${subscriptionId}`, {
    method: 'DELETE',
  })
  if (!res.ok) throw new Error(`cancelSubscription failed: ${res.status}`)
}

export async function createPaymentIntent(
  req: CreatePaymentIntentRequest,
): Promise<PaymentIntent> {
  const res = await authFetch('/billing/payment-intents', {
    method: 'POST',
    body: JSON.stringify(req),
  })
  if (!res.ok) throw new Error(`createPaymentIntent failed: ${res.status}`)
  return res.json()
}
```

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd /Users/jobinlawrance/Project/raven/frontend && npx vue-tsc --noEmit --pretty 2>&1 | head -20
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/api/billing.ts
git commit -m "feat(billing-ui): add billing API client module"
```

---

### Task 2: Create billing Pinia store

**Files:** Create `frontend/src/stores/billing.ts`, `frontend/src/stores/billing.spec.ts`

- [ ] **Step 1: Create the billing store**

Create `frontend/src/stores/billing.ts`:

```typescript
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import {
  getPlans,
  getUsage,
  createSubscription,
  cancelSubscription,
  type Plan,
  type Subscription,
  type UsageResponse,
} from '../api/billing'

export const useBillingStore = defineStore('billing', () => {
  const plans = ref<Plan[]>([])
  const usage = ref<UsageResponse | null>(null)
  const currentSubscription = ref<Subscription | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)

  // Show/hide the global upgrade prompt modal
  const showUpgradePrompt = ref(false)
  const upgradePromptMessage = ref('')
  const upgradePromptLimit = ref(0)

  const currentPlan = computed(() => {
    if (!usage.value) return null
    return usage.value.plan
  })

  async function fetchPlans(): Promise<void> {
    loading.value = true
    error.value = null
    try {
      plans.value = await getPlans()
    } catch (e) {
      error.value = (e as Error).message
    } finally {
      loading.value = false
    }
  }

  async function fetchUsage(): Promise<void> {
    loading.value = true
    error.value = null
    try {
      usage.value = await getUsage()
    } catch (e) {
      error.value = (e as Error).message
    } finally {
      loading.value = false
    }
  }

  async function fetchAll(): Promise<void> {
    loading.value = true
    error.value = null
    try {
      const [plansData, usageData] = await Promise.all([getPlans(), getUsage()])
      plans.value = plansData
      usage.value = usageData
    } catch (e) {
      error.value = (e as Error).message
    } finally {
      loading.value = false
    }
  }

  async function subscribe(planId: string): Promise<Subscription> {
    error.value = null
    try {
      const sub = await createSubscription({ plan_id: planId })
      currentSubscription.value = sub
      return sub
    } catch (e) {
      error.value = (e as Error).message
      throw e
    }
  }

  async function cancel(subscriptionId: string): Promise<void> {
    error.value = null
    try {
      await cancelSubscription(subscriptionId)
      currentSubscription.value = null
      // Refresh usage to reflect downgrade
      await fetchUsage()
    } catch (e) {
      error.value = (e as Error).message
      throw e
    }
  }

  function triggerUpgradePrompt(message: string, limit: number): void {
    upgradePromptMessage.value = message
    upgradePromptLimit.value = limit
    showUpgradePrompt.value = true
  }

  function dismissUpgradePrompt(): void {
    showUpgradePrompt.value = false
    upgradePromptMessage.value = ''
    upgradePromptLimit.value = 0
  }

  return {
    plans,
    usage,
    currentSubscription,
    currentPlan,
    loading,
    error,
    showUpgradePrompt,
    upgradePromptMessage,
    upgradePromptLimit,
    fetchPlans,
    fetchUsage,
    fetchAll,
    subscribe,
    cancel,
    triggerUpgradePrompt,
    dismissUpgradePrompt,
  }
})
```

- [ ] **Step 2: Create store unit test**

Create `frontend/src/stores/billing.spec.ts`:

```typescript
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useBillingStore } from './billing'

// Mock the API module
vi.mock('../api/billing', () => ({
  getPlans: vi.fn(),
  getUsage: vi.fn(),
  createSubscription: vi.fn(),
  cancelSubscription: vi.fn(),
}))

import { getPlans, getUsage, createSubscription, cancelSubscription } from '../api/billing'

const mockPlans = [
  { id: 'plan_free', tier: 'free', name: 'Free', price_monthly: 0, max_users: 5, max_workspaces: 2, max_kbs: 3, max_storage_mb: 500, max_concurrent_voice_sessions: 1, max_voice_minutes_monthly: 60 },
  { id: 'plan_pro', tier: 'pro', name: 'Pro', price_monthly: 2900, max_users: 25, max_workspaces: 10, max_kbs: 50, max_storage_mb: 10240, max_concurrent_voice_sessions: 5, max_voice_minutes_monthly: 1200 },
]

const mockUsage = {
  plan: mockPlans[0],
  kbs_used: 2,
  kbs_limit: 3,
  seats_used: 3,
  seats_limit: 5,
  voice_minutes_used: 30,
  voice_minutes_limit: 60,
  concurrent_voice_used: 0,
  concurrent_voice_limit: 1,
}

describe('useBillingStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('fetchAll loads plans and usage', async () => {
    vi.mocked(getPlans).mockResolvedValue(mockPlans)
    vi.mocked(getUsage).mockResolvedValue(mockUsage)

    const store = useBillingStore()
    await store.fetchAll()

    expect(store.plans).toEqual(mockPlans)
    expect(store.usage).toEqual(mockUsage)
    expect(store.loading).toBe(false)
    expect(store.error).toBeNull()
  })

  it('sets error on fetchAll failure', async () => {
    vi.mocked(getPlans).mockRejectedValue(new Error('network error'))
    vi.mocked(getUsage).mockResolvedValue(mockUsage)

    const store = useBillingStore()
    await store.fetchAll()

    expect(store.error).toBe('network error')
  })

  it('subscribe calls API and stores result', async () => {
    const mockSub = { id: 'sub_1', org_id: 'org_1', plan_id: 'plan_pro', status: 'active' as const, current_period_start: '', current_period_end: '', created_at: '', client_secret: 'cs_test' }
    vi.mocked(createSubscription).mockResolvedValue(mockSub)

    const store = useBillingStore()
    const result = await store.subscribe('plan_pro')

    expect(createSubscription).toHaveBeenCalledWith({ plan_id: 'plan_pro' })
    expect(result.client_secret).toBe('cs_test')
    expect(store.currentSubscription).toEqual(mockSub)
  })

  it('triggerUpgradePrompt shows the modal', () => {
    const store = useBillingStore()
    store.triggerUpgradePrompt('KB limit reached', 3)

    expect(store.showUpgradePrompt).toBe(true)
    expect(store.upgradePromptMessage).toBe('KB limit reached')
    expect(store.upgradePromptLimit).toBe(3)
  })
})
```

- [ ] **Step 3: Run tests**

```bash
cd /Users/jobinlawrance/Project/raven/frontend && npx vitest run src/stores/billing.spec.ts
```

- [ ] **Step 4: Commit**

```bash
git add frontend/src/stores/billing.ts frontend/src/stores/billing.spec.ts
git commit -m "feat(billing-ui): add billing Pinia store with tests"
```

---

### Task 3: Add 402 response handling directly in billing.ts authFetch

**Files:** Modify `frontend/src/api/billing.ts`

> **Note:** The 402 interceptor must live in `billing.ts`'s own `authFetch`, not in `client.ts`.
> `client.ts` is a separate localStorage-based client that `billing.ts` does not call through —
> any interceptor added there would be dead code for billing flows.

- [ ] **Step 1: Add 402 handling inside billing.ts authFetch**

In `frontend/src/api/billing.ts`, update the `authFetch` helper to detect 402 responses and trigger the upgrade prompt:

```typescript
import { useAuthStore } from '../stores/auth'
import { useBillingStore } from '../stores/billing'

async function authFetch(path: string, init?: RequestInit): Promise<Response> {
  const auth = useAuthStore()
  const base = import.meta.env.VITE_API_BASE_URL ?? '/api/v1'
  const response = await fetch(base + path, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${auth.accessToken ?? ''}`,
      ...init?.headers,
    },
  })

  // 402 Payment Required -- trigger the global upgrade prompt
  if (response.status === 402) {
    try {
      const errBody = await response.clone().json().catch(() => ({}))
      const billing = useBillingStore()
      const detail = errBody.detail ?? errBody.message ?? 'Plan limit reached'
      const limit = errBody.limit ?? 0
      billing.triggerUpgradePrompt(detail, limit)
    } catch {
      // Store may not be initialized yet -- ignore
    }
  }

  return response
}
```

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd /Users/jobinlawrance/Project/raven/frontend && npx vue-tsc --noEmit --pretty 2>&1 | head -20
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/api/billing.ts
git commit -m "feat(billing-ui): add 402 upgrade prompt handling in billing authFetch"
```

---

### Task 4: Create plan card and usage dashboard components

**Files:** Create `frontend/src/components/billing/PlanCard.vue`, `frontend/src/components/billing/UsageDashboard.vue`

- [ ] **Step 1: Create the PlanCard component**

Create `frontend/src/components/billing/PlanCard.vue`:

```vue
<script setup lang="ts">
import { computed } from 'vue'
import type { Plan } from '../../api/billing'

const props = defineProps<{
  plan: Plan
  isCurrent: boolean
  isUpgrade: boolean
  loading: boolean
}>()

const emit = defineEmits<{
  select: [planId: string]
}>()

function formatPrice(cents: number): string {
  if (cents === 0) return 'Free'
  return `$${(cents / 100).toFixed(0)}/mo`
}

function formatLimit(value: number): string {
  return value === -1 ? 'Unlimited' : String(value)
}

// Use computed so values stay reactive to prop changes
const features = computed(() => [
  { label: 'Team members', value: formatLimit(props.plan.max_users) },
  { label: 'Workspaces', value: formatLimit(props.plan.max_workspaces) },
  { label: 'Knowledge bases', value: formatLimit(props.plan.max_kbs) },
  { label: 'Storage', value: props.plan.max_storage_mb === -1 ? 'Unlimited' : `${props.plan.max_storage_mb / 1024} GB` },
  { label: 'Voice sessions', value: formatLimit(props.plan.max_concurrent_voice_sessions) },
])

const tierColors: Record<string, string> = {
  free: 'border-gray-200',
  pro: 'border-indigo-500 ring-2 ring-indigo-100',
  enterprise: 'border-amber-500 ring-2 ring-amber-100',
}
</script>

<template>
  <div
    class="relative flex flex-col rounded-2xl border bg-white p-6 shadow-sm transition-shadow hover:shadow-md"
    :class="tierColors[plan.tier] ?? 'border-gray-200'"
  >
    <!-- Current badge -->
    <span
      v-if="isCurrent"
      class="absolute -top-3 left-1/2 -translate-x-1/2 rounded-full bg-indigo-600 px-3 py-0.5 text-xs font-semibold text-white"
    >
      Current Plan
    </span>

    <!-- Plan name and price -->
    <h3 class="text-lg font-bold text-gray-900">{{ plan.name }}</h3>
    <p class="mt-1 text-3xl font-bold text-gray-900">
      {{ formatPrice(plan.price_monthly) }}
    </p>
    <p v-if="plan.price_monthly > 0" class="text-sm text-gray-500">per month</p>

    <!-- Feature list -->
    <ul class="mt-6 flex-1 space-y-3">
      <li
        v-for="feat in features"
        :key="feat.label"
        class="flex items-center justify-between text-sm"
      >
        <span class="text-gray-600">{{ feat.label }}</span>
        <span class="font-medium text-gray-900">{{ feat.value }}</span>
      </li>
    </ul>

    <!-- CTA button -->
    <button
      v-if="!isCurrent"
      :disabled="loading"
      class="mt-6 w-full rounded-lg px-4 py-2.5 text-sm font-semibold transition-colors min-h-[44px] disabled:opacity-50 disabled:cursor-not-allowed"
      :class="
        isUpgrade
          ? 'bg-indigo-600 text-white hover:bg-indigo-700'
          : 'border border-gray-300 text-gray-700 hover:bg-gray-50'
      "
      @click="emit('select', plan.id)"
    >
      {{ loading ? 'Processing...' : isUpgrade ? 'Upgrade' : 'Downgrade' }}
    </button>
    <div
      v-else
      class="mt-6 w-full rounded-lg border border-gray-200 bg-gray-50 px-4 py-2.5 text-center text-sm font-medium text-gray-500"
    >
      Your current plan
    </div>
  </div>
</template>
```

- [ ] **Step 2: Create the UsageDashboard component**

Create `frontend/src/components/billing/UsageDashboard.vue`:

```vue
<script setup lang="ts">
import type { UsageResponse } from '../../api/billing'

const props = defineProps<{
  usage: UsageResponse
}>()

interface UsageRow {
  label: string
  used: number
  limit: number
  unit: string
}

function getRows(): UsageRow[] {
  const u = props.usage
  return [
    { label: 'Knowledge Bases', used: u.kbs_used, limit: u.kbs_limit, unit: '' },
    { label: 'Team Members', used: u.seats_used, limit: u.seats_limit, unit: '' },
    { label: 'Voice Minutes', used: u.voice_minutes_used, limit: u.voice_minutes_limit, unit: 'min' },
    { label: 'Concurrent Voice', used: u.concurrent_voice_used, limit: u.concurrent_voice_limit, unit: '' },
  ]
}

function percentage(used: number, limit: number): number {
  if (limit <= 0) return 0 // unlimited
  return Math.min((used / limit) * 100, 100)
}

function barColor(used: number, limit: number): string {
  if (limit <= 0) return 'bg-gray-300' // unlimited
  const pct = (used / limit) * 100
  if (pct >= 90) return 'bg-red-500'
  if (pct >= 70) return 'bg-amber-500'
  return 'bg-indigo-500'
}

function formatLimit(limit: number, unit: string): string {
  if (limit < 0) return 'Unlimited'
  return unit ? `${limit} ${unit}` : String(limit)
}
</script>

<template>
  <div class="rounded-xl border border-gray-200 bg-white p-6 shadow-sm">
    <h2 class="mb-6 text-lg font-semibold text-gray-900">Current Usage</h2>
    <div class="grid grid-cols-1 gap-6 sm:grid-cols-2">
      <div v-for="row in getRows()" :key="row.label" class="space-y-2">
        <div class="flex items-center justify-between text-sm">
          <span class="font-medium text-gray-700">{{ row.label }}</span>
          <span class="text-gray-500">
            {{ row.used }}{{ row.unit ? ' ' + row.unit : '' }} / {{ formatLimit(row.limit, row.unit) }}
          </span>
        </div>
        <div class="h-2.5 w-full overflow-hidden rounded-full bg-gray-100">
          <div
            v-if="row.limit > 0"
            class="h-full rounded-full transition-all"
            :class="barColor(row.used, row.limit)"
            :style="{ width: percentage(row.used, row.limit) + '%' }"
          />
          <div
            v-else
            class="h-full w-full rounded-full bg-gray-200"
            title="Unlimited"
          />
        </div>
      </div>
    </div>
  </div>
</template>
```

- [ ] **Step 3: Verify TypeScript compiles**

```bash
cd /Users/jobinlawrance/Project/raven/frontend && npx vue-tsc --noEmit --pretty 2>&1 | head -20
```

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/billing/PlanCard.vue frontend/src/components/billing/UsageDashboard.vue
git commit -m "feat(billing-ui): add PlanCard and UsageDashboard components"
```

---

### Task 5: Create upgrade prompt and payment modal components

**Files:** Create `frontend/src/components/billing/UpgradePrompt.vue`, `frontend/src/components/billing/PaymentModal.vue`

- [ ] **Step 1: Create the UpgradePrompt component**

Create `frontend/src/components/billing/UpgradePrompt.vue`:

```vue
<script setup lang="ts">
import { useRouter } from 'vue-router'
import { useBillingStore } from '../../stores/billing'

const router = useRouter()
const billing = useBillingStore()

function goToBilling() {
  billing.dismissUpgradePrompt()
  router.push('/settings/billing')
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="billing.showUpgradePrompt"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
      @click.self="billing.dismissUpgradePrompt()"
    >
      <div class="w-full max-w-md rounded-2xl bg-white p-6 shadow-xl">
        <!-- Icon -->
        <div class="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-amber-100">
          <svg class="h-6 w-6 text-amber-600" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m9-.75a9 9 0 11-18 0 9 9 0 0118 0zm-9 3.75h.008v.008H12v-.008z" />
          </svg>
        </div>

        <h2 class="mt-4 text-center text-lg font-semibold text-gray-900">
          Plan Limit Reached
        </h2>
        <p class="mt-2 text-center text-sm text-gray-600">
          {{ billing.upgradePromptMessage }}
        </p>
        <p v-if="billing.upgradePromptLimit > 0" class="mt-1 text-center text-xs text-gray-400">
          Current limit: {{ billing.upgradePromptLimit }}
        </p>

        <div class="mt-6 flex flex-col gap-3 sm:flex-row sm:justify-end">
          <button
            class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 min-h-[44px]"
            @click="billing.dismissUpgradePrompt()"
          >
            Dismiss
          </button>
          <button
            class="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-700 min-h-[44px]"
            @click="goToBilling"
          >
            View Plans
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
```

- [ ] **Step 2: Create the PaymentModal component**

Create `frontend/src/components/billing/PaymentModal.vue`:

```vue
<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'

const props = defineProps<{
  clientSecret: string
  planName: string
}>()

const emit = defineEmits<{
  success: []
  cancel: []
  error: [message: string]
}>()

const paymentContainer = ref<HTMLDivElement | null>(null)
const loading = ref(true)
const paymentError = ref<string | null>(null)

// Hyperswitch SDK integration
// The SDK is loaded from the Hyperswitch CDN and initialized with the publishable key.
// See: https://docs.hyperswitch.io/hyperswitch-cloud/integration-guide
let hyper: any = null
let widgets: any = null

const PUBLISHABLE_KEY = import.meta.env.VITE_HYPERSWITCH_PUBLISHABLE_KEY ?? ''

onMounted(async () => {
  try {
    // Wait for Hyperswitch SDK to be available (loaded via script tag in index.html)
    if (typeof (window as any).Hyper === 'undefined') {
      throw new Error('Hyperswitch SDK not loaded. Add the SDK script to index.html.')
    }

    hyper = (window as any).Hyper(PUBLISHABLE_KEY)
    widgets = hyper.widgets({
      clientSecret: props.clientSecret,
      appearance: {
        theme: 'default',
        variables: {
          colorPrimary: '#4f46e5', // indigo-600
          fontFamily: 'Inter, system-ui, sans-serif',
          borderRadius: '8px',
        },
      },
    })

    const unifiedCheckout = widgets.create('payment')
    if (paymentContainer.value) {
      unifiedCheckout.mount(paymentContainer.value)
    }
    loading.value = false
  } catch (e) {
    paymentError.value = (e as Error).message
    loading.value = false
  }
})

onUnmounted(() => {
  // Unmount the payment container DOM element to clean up Hyperswitch SDK state.
  // widgets.destroy() is not a standard Hyperswitch SDK API; removing the mount
  // point from the DOM is the correct cleanup approach.
  if (paymentContainer.value) {
    paymentContainer.value.innerHTML = ''
  }
  hyper = null
  widgets = null
})

async function handleConfirmPayment() {
  if (!hyper) return
  loading.value = true
  paymentError.value = null

  try {
    const { error, status } = await hyper.confirmPayment({
      widgets,
      confirmParams: {
        return_url: `${window.location.origin}/settings/billing?payment=success`,
      },
      redirect: 'if_required',
    })

    if (error) {
      paymentError.value = error.message ?? 'Payment failed'
      emit('error', paymentError.value!)
    } else if (status === 'succeeded' || status === 'processing') {
      emit('success')
    }
  } catch (e) {
    paymentError.value = (e as Error).message
    emit('error', paymentError.value)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
    @click.self="emit('cancel')"
  >
    <div class="w-full max-w-lg rounded-2xl bg-white p-6 shadow-xl">
      <h2 class="text-lg font-semibold text-gray-900">
        Subscribe to {{ planName }}
      </h2>
      <p class="mt-1 text-sm text-gray-500">
        Complete your payment to activate your subscription.
      </p>

      <!-- Hyperswitch payment element mount point -->
      <div class="mt-6 min-h-[200px]">
        <div v-if="loading && !paymentError" class="flex items-center justify-center py-12">
          <div class="h-8 w-8 animate-spin rounded-full border-4 border-indigo-200 border-t-indigo-600" />
          <span class="ml-3 text-sm text-gray-500">Loading payment form...</span>
        </div>
        <div ref="paymentContainer" />
      </div>

      <!-- Error -->
      <div
        v-if="paymentError"
        class="mt-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700"
      >
        {{ paymentError }}
      </div>

      <!-- Actions -->
      <div class="mt-6 flex flex-col gap-3 sm:flex-row sm:justify-end">
        <button
          class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 min-h-[44px]"
          @click="emit('cancel')"
        >
          Cancel
        </button>
        <button
          :disabled="loading"
          class="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-700 disabled:opacity-50 disabled:cursor-not-allowed min-h-[44px]"
          @click="handleConfirmPayment"
        >
          {{ loading ? 'Processing...' : 'Pay Now' }}
        </button>
      </div>
    </div>
  </div>
</template>
```

- [ ] **Step 3: Verify TypeScript compiles**

```bash
cd /Users/jobinlawrance/Project/raven/frontend && npx vue-tsc --noEmit --pretty 2>&1 | head -20
```

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/billing/UpgradePrompt.vue frontend/src/components/billing/PaymentModal.vue
git commit -m "feat(billing-ui): add UpgradePrompt and PaymentModal components"
```

---

### Task 6: Create the billing page and register routes

**Files:** Create `frontend/src/pages/settings/BillingPage.vue`, modify `frontend/src/router/index.ts`, modify `frontend/src/components/AppSidebar.vue`

- [ ] **Step 1: Create the billing page**

Create `frontend/src/pages/settings/BillingPage.vue`:

```vue
<script setup lang="ts">
import { onMounted, computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useBillingStore } from '../../stores/billing'
import PlanCard from '../../components/billing/PlanCard.vue'
import UsageDashboard from '../../components/billing/UsageDashboard.vue'
import PaymentModal from '../../components/billing/PaymentModal.vue'

const store = useBillingStore()
const route = useRoute()

const subscribing = ref(false)
const showPayment = ref(false)
const pendingClientSecret = ref('')
const pendingPlanName = ref('')

onMounted(async () => {
  await store.fetchAll()

  // Handle return from Hyperswitch redirect
  if (route.query.payment === 'success') {
    await store.fetchUsage()
  }
})

const currentTierIndex = computed(() => {
  const tiers = ['free', 'pro', 'enterprise']
  const currentTier = store.currentPlan?.tier ?? 'free'
  return tiers.indexOf(currentTier)
})

function isCurrentPlan(planId: string): boolean {
  return store.currentPlan?.id === planId
}

function isUpgrade(planTier: string): boolean {
  const tiers = ['free', 'pro', 'enterprise']
  return tiers.indexOf(planTier) > currentTierIndex.value
}

async function handleSelectPlan(planId: string) {
  subscribing.value = true
  try {
    const sub = await store.subscribe(planId)

    // If the subscription has a client_secret, we need to collect payment
    if (sub.client_secret) {
      pendingClientSecret.value = sub.client_secret
      const plan = store.plans.find((p) => p.id === planId)
      pendingPlanName.value = plan?.name ?? 'Selected Plan'
      showPayment.value = true
    } else {
      // Free plan -- no payment needed, refresh usage
      await store.fetchUsage()
    }
  } finally {
    subscribing.value = false
  }
}

function handlePaymentSuccess() {
  showPayment.value = false
  pendingClientSecret.value = ''
  store.fetchUsage()
}

function handlePaymentCancel() {
  showPayment.value = false
  pendingClientSecret.value = ''
}

function handlePaymentError(message: string) {
  store.error = message
}
</script>

<template>
  <div class="space-y-8">
    <!-- Header -->
    <div>
      <h1 class="text-2xl font-bold text-gray-900">Billing & Subscription</h1>
      <p class="mt-1 text-sm text-gray-500">
        Manage your plan and monitor resource usage.
      </p>
    </div>

    <!-- Loading -->
    <div v-if="store.loading" class="flex items-center justify-center py-20">
      <div class="h-8 w-8 animate-spin rounded-full border-4 border-indigo-200 border-t-indigo-600" />
      <span class="ml-3 text-sm text-gray-500">Loading billing information...</span>
    </div>

    <!-- Error -->
    <div
      v-else-if="store.error"
      class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700"
    >
      {{ store.error }}
    </div>

    <!-- Content -->
    <template v-else>
      <!-- Plan cards -->
      <div>
        <h2 class="mb-4 text-lg font-semibold text-gray-900">Choose Your Plan</h2>
        <div class="grid grid-cols-1 gap-6 md:grid-cols-3">
          <PlanCard
            v-for="plan in store.plans"
            :key="plan.id"
            :plan="plan"
            :is-current="isCurrentPlan(plan.id)"
            :is-upgrade="isUpgrade(plan.tier)"
            :loading="subscribing"
            @select="handleSelectPlan"
          />
        </div>
      </div>

      <!-- Usage dashboard -->
      <UsageDashboard v-if="store.usage" :usage="store.usage" />

      <!-- Billing period info -->
      <div v-if="store.currentSubscription" class="rounded-xl border border-gray-200 bg-white p-6 shadow-sm">
        <h2 class="text-lg font-semibold text-gray-900">Billing Period</h2>
        <div class="mt-3 flex items-center gap-4 text-sm text-gray-600">
          <span>
            Current period: {{ new Date(store.currentSubscription.current_period_start).toLocaleDateString() }}
            &mdash; {{ new Date(store.currentSubscription.current_period_end).toLocaleDateString() }}
          </span>
          <span
            class="inline-flex rounded-full px-2 py-0.5 text-xs font-semibold"
            :class="store.currentSubscription.status === 'active' ? 'bg-green-100 text-green-800' : 'bg-amber-100 text-amber-800'"
          >
            {{ store.currentSubscription.status }}
          </span>
        </div>
      </div>
    </template>

    <!-- Payment modal -->
    <PaymentModal
      v-if="showPayment"
      :client-secret="pendingClientSecret"
      :plan-name="pendingPlanName"
      @success="handlePaymentSuccess"
      @cancel="handlePaymentCancel"
      @error="handlePaymentError"
    />
  </div>
</template>
```

- [ ] **Step 2: Add route to router**

In `frontend/src/router/index.ts`, add the billing route inside the `DefaultLayout` children array, after the existing `chatbot-config` route:

```typescript
        {
          path: 'settings/billing',
          name: 'billing',
          component: () => import('../pages/settings/BillingPage.vue'),
          meta: { requiresAuth: true },
        },
```

- [ ] **Step 3: Add billing nav link to sidebar**

In `frontend/src/components/AppSidebar.vue`, add a billing/settings link in the `<nav>` section before the closing `</nav>` tag. Add it after the Calls RouterLink block:

```vue
        <!-- Settings / Billing -->
        <RouterLink
          to="/settings/billing"
          :class="[
            'flex items-center rounded-lg text-slate-400 transition-colors hover:bg-slate-800 hover:text-white',
            mobile
              ? 'h-10 w-full gap-3 px-3'
              : 'h-10 w-10 justify-center',
          ]"
          active-class="bg-slate-800 text-white"
          title="Billing"
          @click="mobile && $emit('close')"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-5 w-5 shrink-0"
            viewBox="0 0 20 20"
            fill="currentColor"
          >
            <path d="M4 4a2 2 0 00-2 2v1h16V6a2 2 0 00-2-2H4z" />
            <path fill-rule="evenodd" d="M18 9H2v5a2 2 0 002 2h12a2 2 0 002-2V9zM4 13a1 1 0 011-1h1a1 1 0 110 2H5a1 1 0 01-1-1zm5-1a1 1 0 100 2h1a1 1 0 100-2H9z" clip-rule="evenodd" />
          </svg>
          <span v-if="mobile" class="text-sm font-medium">Billing</span>
        </RouterLink>
```

- [ ] **Step 4: Mount the UpgradePrompt globally**

In `frontend/src/layouts/DefaultLayout.vue`, import and render the upgrade prompt so it is available on every page. Add the component after `<MobileTabBar>`:

```vue
<template>
  <div class="flex h-screen bg-gray-100">
    <AppSidebar v-if="!isMobile" :mobile="false" :open="false" />
    <div class="flex flex-1 flex-col overflow-hidden">
      <AppHeader />
      <main class="flex-1 overflow-y-auto p-4 md:p-6" :class="isMobile ? 'pb-24' : ''">
        <RouterView />
      </main>
    </div>
    <MobileTabBar v-if="isMobile" />
    <UpgradePrompt />
  </div>
</template>

<script setup lang="ts">
import { RouterView } from 'vue-router'
import AppSidebar from '../components/AppSidebar.vue'
import AppHeader from '../components/AppHeader.vue'
import MobileTabBar from '../components/MobileTabBar.vue'
import UpgradePrompt from '../components/billing/UpgradePrompt.vue'
import { useMobile } from '../composables/useMediaQuery'

const { isMobile } = useMobile()
</script>
```

- [ ] **Step 5: Add Hyperswitch SDK script to index.html**

In `frontend/index.html`, add the Hyperswitch SDK script tag in the `<head>`:

```html
<script src="https://beta.hyperswitch.io/v1/HyperLoader.js"></script>
```

- [ ] **Step 6: Verify build**

```bash
cd /Users/jobinlawrance/Project/raven/frontend && npx vue-tsc --noEmit --pretty 2>&1 | head -20
```

- [ ] **Step 7: Commit**

```bash
git add frontend/src/pages/settings/BillingPage.vue \
       frontend/src/router/index.ts \
       frontend/src/components/AppSidebar.vue \
       frontend/src/layouts/DefaultLayout.vue \
       frontend/index.html
git commit -m "feat(billing-ui): add billing page, route, nav link, and global upgrade prompt"
```

---

### Task 7: Add lint and type-check pass

- [ ] **Step 1: Run ESLint**

```bash
cd /Users/jobinlawrance/Project/raven/frontend && npx eslint src/api/billing.ts src/stores/billing.ts src/pages/settings/BillingPage.vue src/components/billing/ --fix
```

- [ ] **Step 2: Run vue-tsc type check**

```bash
cd /Users/jobinlawrance/Project/raven/frontend && npx vue-tsc --noEmit --pretty
```

- [ ] **Step 3: Run vitest**

```bash
cd /Users/jobinlawrance/Project/raven/frontend && npx vitest run
```

- [ ] **Step 4: Fix any issues and commit**

```bash
git add -u
git commit -m "fix(billing-ui): resolve lint and type-check issues"
```

---

## Feature B: Keycloak Realm Auto-Provisioning and Tenant Onboarding Wizard (#197)

### Task 0: Write design spec for Keycloak provisioning

**Purpose:** Document the Keycloak Admin API integration, security model for the internal endpoint, realm configuration template, and onboarding wizard UX flow.

- [ ] **Step 1: Create design spec file**

Create `docs/superpowers/specs/197-keycloak-realm-provisioning.md`:

```markdown
# Design Spec: Keycloak Realm Auto-Provisioning and Tenant Onboarding Wizard (#197)

## Overview
Backend endpoint for creating per-tenant Keycloak realms and a frontend onboarding wizard for new tenant setup.

## Dependencies
- None (independent of billing features)
- Requires Keycloak instance with Admin REST API access

## Backend Architecture

### New Config Fields
Add to `KeycloakConfig` in `internal/config/config.go`:
- `AdminURL` (string): Keycloak Admin REST API base URL (e.g. `http://keycloak:8080/admin/realms`)
- `AdminUser` (string): Service account username for Admin API
- `AdminPassword` (string): Service account password

### Keycloak Admin Client (`internal/keycloak/admin.go`)
HTTP client wrapping the Keycloak Admin REST API:
- `GetAdminToken()` -- authenticate with admin credentials, cache token
- `CreateRealm(ctx, realmName, displayName)` -- POST /admin/realms
- `CreateClient(ctx, realmName, clientID, redirectURIs)` -- POST /admin/realms/{realm}/clients
- `ConfigureRealm(ctx, realmName, settings)` -- PUT /admin/realms/{realm} (set login theme, token lifespans, etc.)

### Provisioning Service (`internal/service/provisioning.go`)
Orchestrates the full provisioning flow:
1. Validate org exists and has no realm yet
2. Generate realm name from org slug (e.g. `raven-{org_slug}`)
3. Call Keycloak Admin to create realm
4. Call Keycloak Admin to create OIDC client with standard config
5. Update org record with `keycloak_realm` field
6. Return realm details to caller

### Internal Endpoint (`POST /internal/provision-realm`)
- Registered on the `/api/v1/internal` group (no JWT -- network-only access)
- Request body: `{"org_id": "...", "admin_email": "..."}`
- Response: `{"realm_name": "...", "client_id": "...", "issuer_url": "..."}`
- Idempotent: if realm already exists, return existing details

### Security Considerations
- Internal-only endpoint -- must not be exposed to public internet
- Keycloak admin credentials stored as secrets (env vars, not config file)
- Realm names derived from org slugs to prevent injection
- Rate limiting not needed (internal network only)

## Frontend Architecture

### Onboarding Wizard (`/onboarding`)
Multi-step wizard shown when:
1. User logs in for the first time (org has no workspaces)
2. Admin creates a new organisation

Steps:
1. **Welcome** -- "Welcome to Raven! Let's set up your workspace."
2. **Organization Details** -- Confirm org name, add description
3. **First Workspace** -- Create the first workspace name
4. **Complete** -- Success message with link to dashboard

### Router Guard
- After login, check if org has any workspaces
- If zero workspaces, redirect to `/onboarding`
- Store onboarding completion flag in org settings

## Realm Template Configuration
Default realm settings applied during provisioning:
- Login theme: `raven`
- Token lifespan: access=5min, refresh=30min, SSO=8hr
- Client protocol: openid-connect
- Client access type: public (PKCE)
- Redirect URIs: `{frontend_url}/*`
- Web origins: `{frontend_url}`
- Required actions: VERIFY_EMAIL

## Out of Scope (MVP)
- Custom realm branding/theming
- SAML federation
- Multi-realm per org
- Realm deletion/cleanup
```

- [ ] **Step 2: Review spec for security considerations**

Verify: internal-only endpoint, no credential exposure, slug-based realm naming, idempotency.

- [ ] **Step 3: Commit the spec**

```bash
git add docs/superpowers/specs/197-keycloak-realm-provisioning.md
git commit -m "docs: add design spec for Keycloak realm provisioning (#197)"
```

---

### Task 1: Add Keycloak admin config fields and AppURL

**Files:** Modify `internal/config/config.go`

- [ ] **Step 1: Add `AppURL` to the top-level `Config` struct**

In `internal/config/config.go`, add `AppURL` to the `Config` struct (after the existing `Meta` field):

```go
// Config holds all configuration for the application.
type Config struct {
    // ... existing fields ...
    Meta         MetaConfig
    // AppURL is the canonical frontend application URL used for Keycloak
    // redirect URIs and OIDC client configuration during realm provisioning.
    // Set via RAVEN_APP_URL (e.g. https://app.raven.example.com).
    AppURL string `mapstructure:"app_url"`
}
```

- [ ] **Step 2: Add admin fields to KeycloakConfig**

In `internal/config/config.go`, extend the `KeycloakConfig` struct:

```go
// KeycloakConfig holds Keycloak/OIDC settings for JWT validation.
type KeycloakConfig struct {
	IssuerURL string `mapstructure:"issuer_url"`
	Audience  string `mapstructure:"audience"`
	// APIKeyEnabled enables the unvalidated API-key stub (see issue-24).
	// Disabled by default; set RAVEN_KEYCLOAK_APIKEYENABLED=true only in
	// development environments until the real DB-backed lookup is implemented.
	APIKeyEnabled bool `mapstructure:"api_key_enabled"`
	// AdminURL is the Keycloak server base URL for Admin REST API calls.
	// Example: http://keycloak:8080
	AdminURL      string `mapstructure:"admin_url"`
	AdminUser     string `mapstructure:"admin_user"`
	AdminPassword string `mapstructure:"admin_password"`
}
```

- [ ] **Step 3: Add defaults and env bindings in Load()**

Add these defaults and bindings in the `Load()` function:

```go
v.SetDefault("app_url", "http://localhost:5173")
v.SetDefault("keycloak.admin_url", "http://localhost:8080")
v.SetDefault("keycloak.admin_user", "admin")
v.SetDefault("keycloak.admin_password", "")

_ = v.BindEnv("app_url", "RAVEN_APP_URL")
_ = v.BindEnv("keycloak.admin_url", "RAVEN_KEYCLOAK_ADMIN_URL")
_ = v.BindEnv("keycloak.admin_user", "RAVEN_KEYCLOAK_ADMIN_USER")
_ = v.BindEnv("keycloak.admin_password", "RAVEN_KEYCLOAK_ADMIN_PASSWORD")
```

- [ ] **Step 3: Verify compilation**

```bash
cd /Users/jobinlawrance/Project/raven && go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add internal/config/config.go
git commit -m "feat(keycloak): add admin API config fields to KeycloakConfig"
```

---

### Task 2: Create Keycloak Admin API client

**Files:** Create `internal/keycloak/admin.go`, `internal/keycloak/admin_test.go`

- [ ] **Step 1: Create the admin client**

Create `internal/keycloak/admin.go`:

```go
// Package keycloak provides an HTTP client for the Keycloak Admin REST API.
// Used for tenant realm provisioning during onboarding.
package keycloak

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// AdminClient communicates with the Keycloak Admin REST API.
type AdminClient struct {
	httpClient *http.Client
	baseURL    string
	username   string
	password   string

	mu    sync.RWMutex
	token string
	expAt time.Time
}

// NewAdminClient creates a new Keycloak Admin API client.
func NewAdminClient(baseURL, username, password string) *AdminClient {
	return &AdminClient{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    strings.TrimRight(baseURL, "/"),
		username:   username,
		password:   password,
	}
}

// tokenResponse is the OAuth2 token response from Keycloak.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// RealmRepresentation is the Keycloak realm payload for creation.
type RealmRepresentation struct {
	Realm       string `json:"realm"`
	DisplayName string `json:"displayName,omitempty"`
	Enabled     bool   `json:"enabled"`

	// Token lifespans (seconds).
	AccessTokenLifespan   int `json:"accessTokenLifespan,omitempty"`
	SsoSessionMaxLifespan int `json:"ssoSessionMaxLifespan,omitempty"`

	// Login settings.
	LoginWithEmailAllowed   bool     `json:"loginWithEmailAllowed,omitempty"`
	RegistrationAllowed     bool     `json:"registrationAllowed,omitempty"`
	RequiredActions         []string `json:"-"` // set separately via realm update
}

// ClientRepresentation is the Keycloak client payload for creation.
type ClientRepresentation struct {
	ClientID                string   `json:"clientId"`
	Name                    string   `json:"name,omitempty"`
	Enabled                 bool     `json:"enabled"`
	Protocol                string   `json:"protocol"`
	PublicClient            bool     `json:"publicClient"`
	RedirectUris            []string `json:"redirectUris"`
	WebOrigins              []string `json:"webOrigins"`
	StandardFlowEnabled     bool     `json:"standardFlowEnabled"`
	DirectAccessGrantsEnabled bool   `json:"directAccessGrantsEnabled"`

	// PKCE settings.
	Attributes map[string]string `json:"attributes,omitempty"`
}

// ProvisionResult contains the details of a provisioned realm.
type ProvisionResult struct {
	RealmName string `json:"realm_name"`
	ClientID  string `json:"client_id"`
	IssuerURL string `json:"issuer_url"`
}

// getToken retrieves or refreshes the admin access token.
func (c *AdminClient) getToken(ctx context.Context) (string, error) {
	c.mu.RLock()
	if c.token != "" && time.Now().Before(c.expAt) {
		t := c.token
		c.mu.RUnlock()
		return t, nil
	}
	c.mu.RUnlock()

	// Fetch new token.
	data := url.Values{
		"grant_type": {"password"},
		"client_id":  {"admin-cli"},
		"username":   {c.username},
		"password":   {c.password},
	}

	tokenURL := c.baseURL + "/realms/master/protocol/openid-connect/token"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("keycloak: create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("keycloak: token request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("keycloak: token request returned %d: %s", resp.StatusCode, string(body))
	}

	var tr tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return "", fmt.Errorf("keycloak: decode token response: %w", err)
	}

	c.mu.Lock()
	c.token = tr.AccessToken
	// Expire 30s early to avoid edge-case rejections.
	c.expAt = time.Now().Add(time.Duration(tr.ExpiresIn-30) * time.Second)
	c.mu.Unlock()

	return tr.AccessToken, nil
}

// CreateRealm creates a new Keycloak realm.
func (c *AdminClient) CreateRealm(ctx context.Context, realm RealmRepresentation) error {
	token, err := c.getToken(ctx)
	if err != nil {
		return err
	}

	body, err := json.Marshal(realm)
	if err != nil {
		return fmt.Errorf("keycloak: marshal realm: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/admin/realms", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("keycloak: create realm request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("keycloak: create realm failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	// 409 Conflict means realm already exists -- treat as success (idempotent).
	if resp.StatusCode == http.StatusConflict {
		return nil
	}
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("keycloak: create realm returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// CreateClient creates an OIDC client in the given realm.
func (c *AdminClient) CreateClient(ctx context.Context, realmName string, client ClientRepresentation) error {
	token, err := c.getToken(ctx)
	if err != nil {
		return err
	}

	body, err := json.Marshal(client)
	if err != nil {
		return fmt.Errorf("keycloak: marshal client: %w", err)
	}

	url := fmt.Sprintf("%s/admin/realms/%s/clients", c.baseURL, realmName)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("keycloak: create client request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("keycloak: create client failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	// 409 Conflict means client already exists -- treat as success (idempotent).
	if resp.StatusCode == http.StatusConflict {
		return nil
	}
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("keycloak: create client returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}
```

- [ ] **Step 2: Create tests for the admin client**

Create `internal/keycloak/admin_test.go`:

```go
package keycloak_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ravencloak-org/Raven/internal/keycloak"
)

func TestCreateRealm_Success(t *testing.T) {
	mux := http.NewServeMux()

	// Token endpoint
	mux.HandleFunc("/realms/master/protocol/openid-connect/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "test-token",
			"expires_in":   300,
		})
	})

	// Create realm endpoint
	mux.HandleFunc("/admin/realms", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("missing or wrong Authorization header")
		}

		var realm keycloak.RealmRepresentation
		if err := json.NewDecoder(r.Body).Decode(&realm); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if realm.Realm != "raven-test-org" {
			t.Errorf("expected realm 'raven-test-org', got %q", realm.Realm)
		}
		if !realm.Enabled {
			t.Error("expected realm to be enabled")
		}
		w.WriteHeader(http.StatusCreated)
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := keycloak.NewAdminClient(ts.URL, "admin", "password")
	err := client.CreateRealm(context.Background(), keycloak.RealmRepresentation{
		Realm:       "raven-test-org",
		DisplayName: "Test Org",
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("CreateRealm: %v", err)
	}
}

func TestCreateRealm_Conflict_Idempotent(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/realms/master/protocol/openid-connect/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"access_token": "test-token", "expires_in": 300})
	})

	mux.HandleFunc("/admin/realms", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := keycloak.NewAdminClient(ts.URL, "admin", "password")
	err := client.CreateRealm(context.Background(), keycloak.RealmRepresentation{
		Realm:   "raven-existing",
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("expected nil error for 409 Conflict, got: %v", err)
	}
}

func TestCreateClient_Success(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/realms/master/protocol/openid-connect/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"access_token": "test-token", "expires_in": 300})
	})

	mux.HandleFunc("/admin/realms/raven-test/clients", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		var client keycloak.ClientRepresentation
		if err := json.NewDecoder(r.Body).Decode(&client); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if client.ClientID != "raven-admin" {
			t.Errorf("expected clientId 'raven-admin', got %q", client.ClientID)
		}
		if !client.PublicClient {
			t.Error("expected public client")
		}
		w.WriteHeader(http.StatusCreated)
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := keycloak.NewAdminClient(ts.URL, "admin", "password")
	err := client.CreateClient(context.Background(), "raven-test", keycloak.ClientRepresentation{
		ClientID:            "raven-admin",
		Enabled:             true,
		Protocol:            "openid-connect",
		PublicClient:        true,
		RedirectUris:        []string{"http://localhost:5173/*"},
		WebOrigins:          []string{"http://localhost:5173"},
		StandardFlowEnabled: true,
	})
	if err != nil {
		t.Fatalf("CreateClient: %v", err)
	}
}
```

- [ ] **Step 3: Run tests**

```bash
cd /Users/jobinlawrance/Project/raven && go test ./internal/keycloak/ -v
```

- [ ] **Step 4: Run linter**

```bash
cd /Users/jobinlawrance/Project/raven && golangci-lint run ./internal/keycloak/
```

- [ ] **Step 5: Commit**

```bash
git add internal/keycloak/admin.go internal/keycloak/admin_test.go
git commit -m "feat(keycloak): add Admin REST API client for realm provisioning"
```

---

### Task 3: Create provisioning model types

**Files:** Create `internal/model/provisioning.go`

- [ ] **Step 1: Create model file**

Create `internal/model/provisioning.go`:

```go
package model

// ProvisionRealmRequest is the payload for POST /internal/provision-realm.
type ProvisionRealmRequest struct {
	OrgID      string `json:"org_id" binding:"required"`
	AdminEmail string `json:"admin_email" binding:"required,email"`
}

// ProvisionRealmResponse is returned after successful realm provisioning.
type ProvisionRealmResponse struct {
	RealmName string `json:"realm_name"`
	ClientID  string `json:"client_id"`
	IssuerURL string `json:"issuer_url"`
}
```

- [ ] **Step 2: Verify compilation**

```bash
cd /Users/jobinlawrance/Project/raven && go build ./internal/model/
```

- [ ] **Step 3: Commit**

```bash
git add internal/model/provisioning.go
git commit -m "feat(keycloak): add ProvisionRealmRequest/Response model types"
```

---

### Task 4: Add UpdateKeycloakRealm to org repository

**Files:** Modify `internal/repository/org.go`

- [ ] **Step 1: Add the update method**

Add to `internal/repository/org.go`:

```go
const sqlOrgUpdateKeycloakRealm = `
	UPDATE organizations SET keycloak_realm = $2, updated_at = now()
	WHERE id = $1
	RETURNING ` + orgColumns

// UpdateKeycloakRealm sets the keycloak_realm field on an org record.
func (r *OrgRepository) UpdateKeycloakRealm(ctx context.Context, orgID, realm string) (*model.Organization, error) {
	row := r.pool.QueryRow(ctx, sqlOrgUpdateKeycloakRealm, orgID, realm)
	org, err := scanOrg(row)
	if err != nil {
		return nil, fmt.Errorf("OrgRepository.UpdateKeycloakRealm: %w", err)
	}
	return org, nil
}
```

- [ ] **Step 2: Verify compilation**

```bash
cd /Users/jobinlawrance/Project/raven && go build ./internal/repository/
```

- [ ] **Step 3: Commit**

```bash
git add internal/repository/org.go
git commit -m "feat(keycloak): add UpdateKeycloakRealm to org repository"
```

---

### Task 5: Create provisioning service

**Files:** Create `internal/service/provisioning.go`, `internal/service/provisioning_test.go`

- [ ] **Step 1: Create the service**

Create `internal/service/provisioning.go`:

```go
package service

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/ravencloak-org/Raven/internal/keycloak"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// KeycloakAdminClient defines the interface for Keycloak Admin API operations.
type KeycloakAdminClient interface {
	CreateRealm(ctx context.Context, realm keycloak.RealmRepresentation) error
	CreateClient(ctx context.Context, realmName string, client keycloak.ClientRepresentation) error
}

// OrgLookup defines the interface for org repository operations needed by provisioning.
type OrgLookup interface {
	GetByID(ctx context.Context, orgID string) (*model.Organization, error)
	UpdateKeycloakRealm(ctx context.Context, orgID, realm string) (*model.Organization, error)
}

// ProvisioningService orchestrates Keycloak realm provisioning for new tenants.
type ProvisioningService struct {
	kcClient    KeycloakAdminClient
	orgRepo     OrgLookup
	kcBaseURL   string // Keycloak base URL for constructing issuer URLs
	frontendURL string // Frontend URL for redirect URIs
}

// NewProvisioningService creates a new ProvisioningService.
func NewProvisioningService(
	kcClient KeycloakAdminClient,
	orgRepo OrgLookup,
	kcBaseURL string,
	frontendURL string,
) *ProvisioningService {
	return &ProvisioningService{
		kcClient:    kcClient,
		orgRepo:     orgRepo,
		kcBaseURL:   strings.TrimRight(kcBaseURL, "/"),
		frontendURL: strings.TrimRight(frontendURL, "/"),
	}
}

// slugRegex strips non-alphanumeric characters from org slugs.
var slugRegex = regexp.MustCompile(`[^a-z0-9-]`)

// realmNameFromSlug generates a safe Keycloak realm name from an org slug.
func realmNameFromSlug(slug string) string {
	safe := slugRegex.ReplaceAllString(strings.ToLower(slug), "")
	if safe == "" {
		safe = "default"
	}
	return "raven-" + safe
}

// ProvisionRealm creates a Keycloak realm and OIDC client for the given org.
// The operation is idempotent: if the org already has a realm, the existing
// details are returned without making Keycloak API calls.
func (s *ProvisioningService) ProvisionRealm(ctx context.Context, req model.ProvisionRealmRequest) (*model.ProvisionRealmResponse, error) {
	// Look up the org.
	org, err := s.orgRepo.GetByID(ctx, req.OrgID)
	if err != nil {
		slog.ErrorContext(ctx, "provisioning: failed to look up org", "error", err, "org_id", req.OrgID)
		return nil, apierror.NewInternal("failed to look up organisation")
	}
	if org == nil {
		return nil, apierror.NewNotFound("organisation not found: " + req.OrgID)
	}

	// Idempotent: if realm already provisioned, return existing.
	if org.KeycloakRealm != "" {
		return &model.ProvisionRealmResponse{
			RealmName: org.KeycloakRealm,
			ClientID:  "raven-admin",
			IssuerURL: fmt.Sprintf("%s/realms/%s", s.kcBaseURL, org.KeycloakRealm),
		}, nil
	}

	realmName := realmNameFromSlug(org.Slug)
	clientID := "raven-admin"

	// Create realm.
	realm := keycloak.RealmRepresentation{
		Realm:                   realmName,
		DisplayName:             org.Name,
		Enabled:                 true,
		AccessTokenLifespan:     300,       // 5 minutes
		SsoSessionMaxLifespan:   28800,     // 8 hours
		LoginWithEmailAllowed:   true,
		RegistrationAllowed:     false,
	}
	if err := s.kcClient.CreateRealm(ctx, realm); err != nil {
		slog.ErrorContext(ctx, "provisioning: failed to create realm", "error", err, "realm", realmName)
		return nil, apierror.NewInternal("failed to create Keycloak realm")
	}

	// Create OIDC client.
	client := keycloak.ClientRepresentation{
		ClientID:                  clientID,
		Name:                     "Raven Admin Console",
		Enabled:                  true,
		Protocol:                 "openid-connect",
		PublicClient:             true,
		RedirectUris:             []string{s.frontendURL + "/*"},
		WebOrigins:               []string{s.frontendURL},
		StandardFlowEnabled:      true,
		DirectAccessGrantsEnabled: false,
		Attributes: map[string]string{
			"pkce.code.challenge.method": "S256",
		},
	}
	if err := s.kcClient.CreateClient(ctx, realmName, client); err != nil {
		slog.ErrorContext(ctx, "provisioning: failed to create client", "error", err, "realm", realmName)
		return nil, apierror.NewInternal("failed to create Keycloak client")
	}

	// Update org record.
	if _, err := s.orgRepo.UpdateKeycloakRealm(ctx, req.OrgID, realmName); err != nil {
		slog.ErrorContext(ctx, "provisioning: failed to update org realm", "error", err, "org_id", req.OrgID)
		return nil, apierror.NewInternal("failed to update organisation with realm")
	}

	slog.InfoContext(ctx, "provisioning: realm created", "realm", realmName, "org_id", req.OrgID)

	return &model.ProvisionRealmResponse{
		RealmName: realmName,
		ClientID:  clientID,
		IssuerURL: fmt.Sprintf("%s/realms/%s", s.kcBaseURL, realmName),
	}, nil
}
```

- [ ] **Step 2: Create unit tests**

Create `internal/service/provisioning_test.go`:

```go
package service_test

import (
	"context"
	"testing"

	"github.com/ravencloak-org/Raven/internal/keycloak"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/service"
)

// mockKCClient implements service.KeycloakAdminClient.
type mockKCClient struct {
	createRealmCalled  bool
	createClientCalled bool
	createRealmErr     error
	createClientErr    error
}

func (m *mockKCClient) CreateRealm(_ context.Context, _ keycloak.RealmRepresentation) error {
	m.createRealmCalled = true
	return m.createRealmErr
}

func (m *mockKCClient) CreateClient(_ context.Context, _ string, _ keycloak.ClientRepresentation) error {
	m.createClientCalled = true
	return m.createClientErr
}

// mockOrgLookup implements service.OrgLookup.
type mockOrgLookup struct {
	org             *model.Organization
	getErr          error
	updateRealmErr  error
	updatedRealm    string
}

func (m *mockOrgLookup) GetByID(_ context.Context, _ string) (*model.Organization, error) {
	return m.org, m.getErr
}

func (m *mockOrgLookup) UpdateKeycloakRealm(_ context.Context, _ string, realm string) (*model.Organization, error) {
	m.updatedRealm = realm
	if m.updateRealmErr != nil {
		return nil, m.updateRealmErr
	}
	org := *m.org
	org.KeycloakRealm = realm
	return &org, nil
}

func TestProvisionRealm_Success(t *testing.T) {
	kc := &mockKCClient{}
	orgRepo := &mockOrgLookup{
		org: &model.Organization{
			ID:   "org-1",
			Name: "Test Corp",
			Slug: "test-corp",
		},
	}

	svc := service.NewProvisioningService(kc, orgRepo, "http://keycloak:8080", "http://localhost:5173")

	resp, err := svc.ProvisionRealm(context.Background(), model.ProvisionRealmRequest{
		OrgID:      "org-1",
		AdminEmail: "admin@test.com",
	})
	if err != nil {
		t.Fatalf("ProvisionRealm: %v", err)
	}

	if resp.RealmName != "raven-test-corp" {
		t.Errorf("expected realm 'raven-test-corp', got %q", resp.RealmName)
	}
	if resp.ClientID != "raven-admin" {
		t.Errorf("expected client 'raven-admin', got %q", resp.ClientID)
	}
	if resp.IssuerURL != "http://keycloak:8080/realms/raven-test-corp" {
		t.Errorf("unexpected issuer URL: %q", resp.IssuerURL)
	}
	if !kc.createRealmCalled {
		t.Error("expected CreateRealm to be called")
	}
	if !kc.createClientCalled {
		t.Error("expected CreateClient to be called")
	}
	if orgRepo.updatedRealm != "raven-test-corp" {
		t.Errorf("expected org realm updated to 'raven-test-corp', got %q", orgRepo.updatedRealm)
	}
}

func TestProvisionRealm_AlreadyProvisioned(t *testing.T) {
	kc := &mockKCClient{}
	orgRepo := &mockOrgLookup{
		org: &model.Organization{
			ID:            "org-1",
			Slug:          "test-corp",
			KeycloakRealm: "raven-test-corp",
		},
	}

	svc := service.NewProvisioningService(kc, orgRepo, "http://keycloak:8080", "http://localhost:5173")

	resp, err := svc.ProvisionRealm(context.Background(), model.ProvisionRealmRequest{
		OrgID:      "org-1",
		AdminEmail: "admin@test.com",
	})
	if err != nil {
		t.Fatalf("ProvisionRealm: %v", err)
	}

	if resp.RealmName != "raven-test-corp" {
		t.Errorf("expected realm 'raven-test-corp', got %q", resp.RealmName)
	}
	if kc.createRealmCalled {
		t.Error("CreateRealm should NOT be called for already-provisioned org")
	}
}

func TestProvisionRealm_OrgNotFound(t *testing.T) {
	kc := &mockKCClient{}
	orgRepo := &mockOrgLookup{org: nil}

	svc := service.NewProvisioningService(kc, orgRepo, "http://keycloak:8080", "http://localhost:5173")

	_, err := svc.ProvisionRealm(context.Background(), model.ProvisionRealmRequest{
		OrgID:      "org-nonexistent",
		AdminEmail: "admin@test.com",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent org")
	}
}
```

- [ ] **Step 3: Run tests**

```bash
cd /Users/jobinlawrance/Project/raven && go test ./internal/service/ -run TestProvisionRealm -v
```

- [ ] **Step 4: Run linter**

```bash
cd /Users/jobinlawrance/Project/raven && golangci-lint run ./internal/service/provisioning.go ./internal/service/provisioning_test.go
```

- [ ] **Step 5: Commit**

```bash
git add internal/service/provisioning.go internal/service/provisioning_test.go
git commit -m "feat(keycloak): add ProvisioningService for realm creation"
```

---

### Task 6: Create provisioning handler and register route

**Files:** Create `internal/handler/provisioning.go`, `internal/handler/provisioning_test.go`, modify `cmd/api/main.go`

- [ ] **Step 1: Create the handler**

Create `internal/handler/provisioning.go`:

```go
package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// ProvisioningServicer is the interface the handler requires from the provisioning service.
type ProvisioningServicer interface {
	ProvisionRealm(ctx context.Context, req model.ProvisionRealmRequest) (*model.ProvisionRealmResponse, error)
}

// ProvisioningHandler handles internal provisioning requests.
type ProvisioningHandler struct {
	svc ProvisioningServicer
}

// NewProvisioningHandler creates a new ProvisioningHandler.
func NewProvisioningHandler(svc ProvisioningServicer) *ProvisioningHandler {
	return &ProvisioningHandler{svc: svc}
}

// ProvisionRealm handles POST /api/v1/internal/provision-realm.
//
// @Summary     Provision Keycloak realm for tenant
// @Description Internal-only. Creates a Keycloak realm and OIDC client for the given org.
// @Tags        internal
// @Accept      json
// @Produce     json
// @Param       request body model.ProvisionRealmRequest true "Provisioning payload"
// @Success     201 {object} model.ProvisionRealmResponse
// @Failure     400 {object} apierror.AppError
// @Failure     404 {object} apierror.AppError
// @Failure     500 {object} apierror.AppError
// @Router      /internal/provision-realm [post]
func (h *ProvisioningHandler) ProvisionRealm(c *gin.Context) {
	var req model.ProvisionRealmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, apierror.AppError{
			Code:    http.StatusBadRequest,
			Message: "Bad Request",
			Detail:  err.Error(),
		})
		return
	}

	resp, err := h.svc.ProvisionRealm(c.Request.Context(), req)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}

	c.JSON(http.StatusCreated, resp)
}
```

- [ ] **Step 2: Create handler tests**

Create `internal/handler/provisioning_test.go`:

```go
package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ravencloak-org/Raven/internal/handler"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

type mockProvisioningService struct {
	resp *model.ProvisionRealmResponse
	err  error
}

func (m *mockProvisioningService) ProvisionRealm(_ context.Context, _ model.ProvisionRealmRequest) (*model.ProvisionRealmResponse, error) {
	return m.resp, m.err
}

// newProvisioningRouter creates a test Gin engine with the provisioning route.
// It uses apierror.ErrorHandler() as middleware, consistent with other handler tests.
func newProvisioningRouter(svc handler.ProvisioningServicer) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())

	h := handler.NewProvisioningHandler(svc)
	r.POST("/api/v1/internal/provision-realm", h.ProvisionRealm)

	return r
}

func TestProvisionRealm_Success(t *testing.T) {
	svc := &mockProvisioningService{
		resp: &model.ProvisionRealmResponse{
			RealmName: "raven-test-corp",
			ClientID:  "raven-admin",
			IssuerURL: "http://keycloak:8080/realms/raven-test-corp",
		},
	}

	r := newProvisioningRouter(svc)

	body, _ := json.Marshal(model.ProvisionRealmRequest{
		OrgID:      "org-1",
		AdminEmail: "admin@test.com",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/internal/provision-realm", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}

	var resp model.ProvisionRealmResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.RealmName != "raven-test-corp" {
		t.Errorf("expected realm 'raven-test-corp', got %q", resp.RealmName)
	}
}

func TestProvisionRealm_BadRequest(t *testing.T) {
	svc := &mockProvisioningService{}
	r := newProvisioningRouter(svc)

	// Missing required fields
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/internal/provision-realm", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestProvisionRealm_NotFound(t *testing.T) {
	svc := &mockProvisioningService{
		err: apierror.NewNotFound("organisation not found"),
	}

	r := newProvisioningRouter(svc)

	body, _ := json.Marshal(model.ProvisionRealmRequest{
		OrgID:      "org-nonexistent",
		AdminEmail: "admin@test.com",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/internal/provision-realm", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
```

- [ ] **Step 3: Run handler tests**

```bash
cd /Users/jobinlawrance/Project/raven && go test ./internal/handler/ -run TestProvisionRealm -v
```

- [ ] **Step 4: Wire into main.go**

In `cmd/api/main.go`, add the following:

1. In the imports section, the `keycloak` package is already indirectly available. No new import needed if it's referenced through the service.

2. After the existing service wiring (around line 295), add:

```go
	// Keycloak Admin client for realm provisioning.
	kcAdmin := keycloak.NewAdminClient(cfg.Keycloak.AdminURL, cfg.Keycloak.AdminUser, cfg.Keycloak.AdminPassword)
	provisioningSvc := service.NewProvisioningService(kcAdmin, orgRepo, cfg.Keycloak.AdminURL, cfg.AppURL)
```

3. After the existing handler wiring (around line 404), add:

```go
	provisioningHandler := handler.NewProvisioningHandler(provisioningSvc)
```

4. In the internal routes group (around line 714), add:

```go
		internal.POST("/provision-realm", provisioningHandler.ProvisionRealm)
```

5. Add the import for the keycloak package:

```go
	"github.com/ravencloak-org/Raven/internal/keycloak"
```

- [ ] **Step 5: Verify compilation**

```bash
cd /Users/jobinlawrance/Project/raven && go build ./cmd/api/
```

- [ ] **Step 6: Run linter**

```bash
cd /Users/jobinlawrance/Project/raven && golangci-lint run ./internal/handler/provisioning.go ./internal/handler/provisioning_test.go ./cmd/api/
```

- [ ] **Step 7: Commit**

```bash
git add internal/handler/provisioning.go internal/handler/provisioning_test.go cmd/api/main.go
git commit -m "feat(keycloak): add provisioning handler and wire into internal routes"
```

---

### Task 7: Create frontend onboarding wizard

**Files:** Create `frontend/src/api/onboarding.ts`, `frontend/src/stores/onboarding.ts`, `frontend/src/pages/onboarding/OnboardingWizardPage.vue`, modify `frontend/src/router/index.ts`

- [ ] **Step 1: Create the onboarding API client**

Create `frontend/src/api/onboarding.ts`:

```typescript
import { useAuthStore } from '../stores/auth'

export interface TenantStatus {
  has_workspaces: boolean
  // Note: keycloak_realm and org_name are not available from the workspace list
  // endpoint. If the org's keycloak_realm is needed, fetch it from a separate
  // org endpoint (e.g. GET /orgs/:id).
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

export async function getTenantStatus(orgId: string): Promise<TenantStatus> {
  // The workspace list endpoint returns a flat []Workspace array and uses
  // offset/limit pagination (not page/page_size).
  const res = await authFetch(`/orgs/${orgId}/workspaces?offset=0&limit=1`)
  if (!res.ok) throw new Error(`getTenantStatus failed: ${res.status}`)
  const data: unknown[] = await res.json()
  return {
    has_workspaces: data.length > 0,
  }
}
```

- [ ] **Step 2: Create the onboarding store**

Create `frontend/src/stores/onboarding.ts`:

```typescript
import { defineStore } from 'pinia'
import { ref } from 'vue'

export type OnboardingStep = 'welcome' | 'org-details' | 'first-workspace' | 'complete'

export const useOnboardingStore = defineStore('onboarding', () => {
  const currentStep = ref<OnboardingStep>('welcome')
  const orgName = ref('')
  const orgDescription = ref('')
  const workspaceName = ref('')
  const loading = ref(false)
  const error = ref<string | null>(null)

  const steps: OnboardingStep[] = ['welcome', 'org-details', 'first-workspace', 'complete']

  function nextStep(): void {
    const idx = steps.indexOf(currentStep.value)
    if (idx < steps.length - 1) {
      currentStep.value = steps[idx + 1]
    }
  }

  function prevStep(): void {
    const idx = steps.indexOf(currentStep.value)
    if (idx > 0) {
      currentStep.value = steps[idx - 1]
    }
  }

  function reset(): void {
    currentStep.value = 'welcome'
    orgName.value = ''
    orgDescription.value = ''
    workspaceName.value = ''
    error.value = null
  }

  return {
    currentStep,
    orgName,
    orgDescription,
    workspaceName,
    loading,
    error,
    steps,
    nextStep,
    prevStep,
    reset,
  }
})
```

- [ ] **Step 3: Create the onboarding wizard page**

Create `frontend/src/pages/onboarding/OnboardingWizardPage.vue`:

```vue
<script setup lang="ts">
import { useRouter } from 'vue-router'
import { useOnboardingStore } from '../../stores/onboarding'
import { useAuthStore } from '../../stores/auth'

const router = useRouter()
const store = useOnboardingStore()
const auth = useAuthStore()

// Pre-fill org name from auth context
if (auth.user?.orgId && !store.orgName) {
  store.orgName = 'My Organization'
}

async function handleCreateWorkspace() {
  if (!store.workspaceName.trim()) return
  store.loading = true
  store.error = null
  try {
    const base = import.meta.env.VITE_API_BASE_URL ?? '/api/v1'
    const orgId = auth.user?.orgId ?? ''
    const res = await fetch(`${base}/orgs/${orgId}/workspaces`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${auth.accessToken ?? ''}`,
      },
      body: JSON.stringify({ name: store.workspaceName.trim() }),
    })
    if (!res.ok) throw new Error(`Failed to create workspace: ${res.status}`)
    store.nextStep()
  } catch (e) {
    store.error = (e as Error).message
  } finally {
    store.loading = false
  }
}

function goToDashboard() {
  store.reset()
  router.push('/dashboard')
}
</script>

<template>
  <div class="flex min-h-screen items-center justify-center bg-gray-50 p-4">
    <div class="w-full max-w-lg">
      <!-- Progress indicator -->
      <div class="mb-8 flex items-center justify-center gap-2">
        <div
          v-for="(step, idx) in store.steps"
          :key="step"
          class="h-2 w-12 rounded-full transition-colors"
          :class="idx <= store.steps.indexOf(store.currentStep) ? 'bg-indigo-600' : 'bg-gray-200'"
        />
      </div>

      <!-- Card -->
      <div class="rounded-2xl border border-gray-200 bg-white p-8 shadow-sm">

        <!-- Step 1: Welcome -->
        <div v-if="store.currentStep === 'welcome'" class="text-center">
          <div class="mx-auto flex h-16 w-16 items-center justify-center rounded-2xl bg-indigo-100">
            <span class="text-2xl font-bold text-indigo-600">R</span>
          </div>
          <h1 class="mt-6 text-2xl font-bold text-gray-900">Welcome to Raven</h1>
          <p class="mt-3 text-gray-600">
            Let's get your workspace set up. This will only take a minute.
          </p>
          <button
            class="mt-8 w-full rounded-lg bg-indigo-600 px-4 py-3 text-sm font-semibold text-white hover:bg-indigo-700 min-h-[44px]"
            @click="store.nextStep()"
          >
            Get Started
          </button>
        </div>

        <!-- Step 2: Organization Details -->
        <div v-else-if="store.currentStep === 'org-details'">
          <h2 class="text-xl font-bold text-gray-900">Your Organization</h2>
          <p class="mt-2 text-sm text-gray-500">Confirm your organization details.</p>

          <form class="mt-6 space-y-4" @submit.prevent="store.nextStep()">
            <div>
              <label for="org-name" class="block text-sm font-medium text-gray-700">Organization Name</label>
              <input
                id="org-name"
                v-model="store.orgName"
                type="text"
                required
                class="mt-1 block w-full rounded-lg border border-gray-300 px-3 py-2.5 text-sm shadow-sm focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 focus:outline-none min-h-[44px]"
              />
            </div>
            <div>
              <label for="org-desc" class="block text-sm font-medium text-gray-700">Description (optional)</label>
              <textarea
                id="org-desc"
                v-model="store.orgDescription"
                rows="3"
                class="mt-1 block w-full rounded-lg border border-gray-300 px-3 py-2.5 text-sm shadow-sm focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 focus:outline-none"
                placeholder="What does your organization do?"
              />
            </div>
            <div class="flex gap-3 pt-2">
              <button
                type="button"
                class="flex-1 rounded-lg border border-gray-300 bg-white px-4 py-2.5 text-sm font-medium text-gray-700 hover:bg-gray-50 min-h-[44px]"
                @click="store.prevStep()"
              >
                Back
              </button>
              <button
                type="submit"
                class="flex-1 rounded-lg bg-indigo-600 px-4 py-2.5 text-sm font-semibold text-white hover:bg-indigo-700 min-h-[44px]"
              >
                Continue
              </button>
            </div>
          </form>
        </div>

        <!-- Step 3: First Workspace -->
        <div v-else-if="store.currentStep === 'first-workspace'">
          <h2 class="text-xl font-bold text-gray-900">Create Your First Workspace</h2>
          <p class="mt-2 text-sm text-gray-500">
            Workspaces organize your knowledge bases and team members.
          </p>

          <form class="mt-6 space-y-4" @submit.prevent="handleCreateWorkspace">
            <div>
              <label for="ws-name" class="block text-sm font-medium text-gray-700">Workspace Name</label>
              <input
                id="ws-name"
                v-model="store.workspaceName"
                type="text"
                required
                placeholder="e.g. Customer Support, Engineering"
                class="mt-1 block w-full rounded-lg border border-gray-300 px-3 py-2.5 text-sm shadow-sm focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 focus:outline-none min-h-[44px]"
              />
            </div>

            <div v-if="store.error" class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">
              {{ store.error }}
            </div>

            <div class="flex gap-3 pt-2">
              <button
                type="button"
                class="flex-1 rounded-lg border border-gray-300 bg-white px-4 py-2.5 text-sm font-medium text-gray-700 hover:bg-gray-50 min-h-[44px]"
                @click="store.prevStep()"
              >
                Back
              </button>
              <button
                type="submit"
                :disabled="store.loading || !store.workspaceName.trim()"
                class="flex-1 rounded-lg bg-indigo-600 px-4 py-2.5 text-sm font-semibold text-white hover:bg-indigo-700 disabled:opacity-50 disabled:cursor-not-allowed min-h-[44px]"
              >
                {{ store.loading ? 'Creating...' : 'Create Workspace' }}
              </button>
            </div>
          </form>
        </div>

        <!-- Step 4: Complete -->
        <div v-else-if="store.currentStep === 'complete'" class="text-center">
          <div class="mx-auto flex h-16 w-16 items-center justify-center rounded-full bg-green-100">
            <svg class="h-8 w-8 text-green-600" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" d="M4.5 12.75l6 6 9-13.5" />
            </svg>
          </div>
          <h2 class="mt-6 text-2xl font-bold text-gray-900">You're All Set!</h2>
          <p class="mt-3 text-gray-600">
            Your workspace is ready. Start adding knowledge bases and inviting your team.
          </p>
          <button
            class="mt-8 w-full rounded-lg bg-indigo-600 px-4 py-3 text-sm font-semibold text-white hover:bg-indigo-700 min-h-[44px]"
            @click="goToDashboard"
          >
            Go to Dashboard
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
```

- [ ] **Step 4: Add onboarding route to router**

In `frontend/src/router/index.ts`, add the onboarding route. Add it as a top-level route (not inside `DefaultLayout`) since the wizard has its own full-page layout:

```typescript
    {
      path: '/onboarding',
      name: 'onboarding',
      component: () => import('../pages/onboarding/OnboardingWizardPage.vue'),
      meta: { requiresAuth: true },
    },
```

Place it after the login route block and before the DefaultLayout block.

- [ ] **Step 5: Verify TypeScript compiles**

```bash
cd /Users/jobinlawrance/Project/raven/frontend && npx vue-tsc --noEmit --pretty 2>&1 | head -20
```

- [ ] **Step 6: Run ESLint**

```bash
cd /Users/jobinlawrance/Project/raven/frontend && npx eslint src/api/onboarding.ts src/stores/onboarding.ts src/pages/onboarding/OnboardingWizardPage.vue --fix
```

- [ ] **Step 7: Commit**

```bash
git add frontend/src/api/onboarding.ts \
       frontend/src/stores/onboarding.ts \
       frontend/src/pages/onboarding/OnboardingWizardPage.vue \
       frontend/src/router/index.ts
git commit -m "feat(onboarding): add tenant onboarding wizard with multi-step flow"
```

---

### Task 7b: Add unit tests for onboarding store

**Files:** Create `frontend/src/stores/onboarding.spec.ts`

- [ ] **Step 1: Create the store spec**

Create `frontend/src/stores/onboarding.spec.ts`:

```typescript
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useOnboardingStore } from './onboarding'

describe('useOnboardingStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('initial state starts at welcome step', () => {
    const store = useOnboardingStore()
    expect(store.currentStep).toBe('welcome')
    expect(store.loading).toBe(false)
    expect(store.error).toBeNull()
  })

  it('nextStep advances through the step sequence', () => {
    const store = useOnboardingStore()
    expect(store.currentStep).toBe('welcome')
    store.nextStep()
    expect(store.currentStep).toBe('org-details')
    store.nextStep()
    expect(store.currentStep).toBe('first-workspace')
    store.nextStep()
    expect(store.currentStep).toBe('complete')
  })

  it('nextStep does not advance past complete', () => {
    const store = useOnboardingStore()
    store.currentStep = 'complete'
    store.nextStep()
    expect(store.currentStep).toBe('complete')
  })

  it('prevStep moves back through the step sequence', () => {
    const store = useOnboardingStore()
    store.currentStep = 'first-workspace'
    store.prevStep()
    expect(store.currentStep).toBe('org-details')
  })

  it('prevStep does not go before welcome', () => {
    const store = useOnboardingStore()
    store.prevStep()
    expect(store.currentStep).toBe('welcome')
  })

  it('reset clears all state back to welcome', () => {
    const store = useOnboardingStore()
    store.currentStep = 'complete'
    store.orgName = 'Acme'
    store.workspaceName = 'Support'
    store.error = 'some error'
    store.reset()
    expect(store.currentStep).toBe('welcome')
    expect(store.orgName).toBe('')
    expect(store.workspaceName).toBe('')
    expect(store.error).toBeNull()
  })

})
```

- [ ] **Step 2: Run tests**

```bash
cd /Users/jobinlawrance/Project/raven/frontend && npx vitest run src/stores/onboarding.spec.ts
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/stores/onboarding.spec.ts
git commit -m "test(onboarding): add unit tests for onboarding store"
```

---

### Task 8: Final verification pass

- [ ] **Step 1: Run all Go tests**

```bash
cd /Users/jobinlawrance/Project/raven && go test ./internal/keycloak/ ./internal/service/ ./internal/handler/ -v -count=1 2>&1 | tail -30
```

- [ ] **Step 2: Run Go linter**

```bash
cd /Users/jobinlawrance/Project/raven && golangci-lint run ./...
```

- [ ] **Step 3: Run all frontend tests**

```bash
cd /Users/jobinlawrance/Project/raven/frontend && npx vitest run
```

- [ ] **Step 4: Run frontend lint**

```bash
cd /Users/jobinlawrance/Project/raven/frontend && npx eslint src/ --fix
```

- [ ] **Step 5: Run full TypeScript check**

```bash
cd /Users/jobinlawrance/Project/raven/frontend && npx vue-tsc --noEmit --pretty
```

- [ ] **Step 6: Fix any remaining issues and commit**

```bash
git add -u
git commit -m "fix: resolve lint and type-check issues for wave 3 features"
```

---

## PR and Merge Strategy

### For #194 (Billing UI):
1. Create branch: `feat/194-billing-subscription-ui`
2. Accumulate tasks 0-7 as commits on this branch
3. Open PR targeting `main`
4. Queue auto-merge: `gh pr merge <PR> --auto --squash`

### For #197 (Keycloak Onboarding):
1. Create branch: `feat/197-keycloak-realm-provisioning`
2. Accumulate tasks 0-8 as commits on this branch
3. Open PR targeting `main`
4. Queue auto-merge: `gh pr merge <PR> --auto --squash`

**Note:** #194 is blocked by #193. #197 is independent and can begin immediately (design spec phase can start during Wave 2). Both PRs should be opened and merged sequentially to avoid merge conflicts in shared files (`router/index.ts`, `cmd/api/main.go`).
