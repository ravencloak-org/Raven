# Billing & Subscription Management UI — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the complete billing frontend: plan selection page, usage dashboard, Hyperswitch payment flow, subscription management, and a reusable 402 upgrade-prompt component.

**Architecture:** Pinia store (`billing.ts`) is the single source of truth for subscription + usage data. API module (`api/billing.ts`) wraps all backend calls. The `UpgradePromptBanner` component is globally wired to the store's `quotaExceeded` flag. Feature-flagged via PostHog `billing_enabled`.

**Tech Stack:** Vue 3 (Composition API), Pinia, TypeScript, Tailwind CSS, Hyperswitch JS SDK, PostHog feature flags

---

## File Map

| Action | Path |
|--------|------|
| Create | `frontend/src/api/billing.ts` |
| Create | `frontend/src/stores/billing.ts` |
| Create | `frontend/src/pages/billing/BillingPage.vue` |
| Create | `frontend/src/components/UsageBar.vue` |
| Create | `frontend/src/components/UpgradePromptBanner.vue` |
| Create | `frontend/src/components/PlanCard.vue` |
| Modify | `frontend/src/api/client.ts` (or shared authFetch util) |
| Modify | `frontend/src/router/index.ts` |
| Modify | `frontend/src/components/AppSidebar.vue` |
| Modify | `frontend/src/layouts/DefaultLayout.vue` |

---

## Backend Reference

All billing endpoints are already implemented. Use these:
- `GET /api/v1/billing/plans` → `Plan[]`
- `GET /api/v1/billing/usage` → `UsageResponse`
- `POST /api/v1/billing/subscriptions` → `Subscription` (body: `{plan_id, payment_method_id}`)
- `DELETE /api/v1/billing/subscriptions/:id` → 204
- `POST /api/v1/billing/payment-intents` → `{client_secret, payment_intent_id}`

402 response shape from backend:
```json
{
  "code": 402,
  "message": "knowledge base limit reached",
  "upgrade_required": true,
  "limit": 3
}
```

---

### Task 1: Billing API module

**Files:**
- Create: `frontend/src/api/billing.ts`

Follow the exact same pattern as `frontend/src/api/apikeys.ts`.

- [ ] **Step 1: Define types and write the module**

Create `frontend/src/api/billing.ts`:
```typescript
import { useAuthStore } from '../stores/auth'

const API_BASE = import.meta.env.VITE_API_BASE_URL ?? '/api/v1'

async function authFetch(path: string, init?: RequestInit): Promise<Response> {
  const auth = useAuthStore()
  return fetch(API_BASE + path, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${auth.accessToken ?? ''}`,
      ...init?.headers,
    },
  })
}

export interface Plan {
  id: string
  name: string          // "free" | "pro" | "enterprise"
  display_name: string
  price_monthly: number
  max_users: number
  max_workspaces: number
  max_knowledge_bases: number
  max_storage_gb: number
  max_voice_sessions: number
  max_voice_minutes_monthly: number
}

export interface Subscription {
  id: string
  org_id: string
  plan_id: string
  status: 'active' | 'canceled' | 'past_due' | 'trialing' | 'paused' | 'expired'
  hyperswitch_subscription_id: string
  current_period_start: string
  current_period_end: string
  created_at: string
}

export interface UsageResponse {
  kbs_used: number
  kbs_limit: number
  seats_used: number
  seats_limit: number
  voice_minutes_used: number
  voice_minutes_limit: number
  concurrent_voice_limit: number
}

export interface CreatePaymentIntentResponse {
  client_secret: string
  payment_intent_id: string
}

export async function getPlans(): Promise<Plan[]> {
  const res = await authFetch('/billing/plans')
  if (!res.ok) throw new Error(`getPlans: ${res.status}`)
  return res.json()
}

export async function getUsage(): Promise<UsageResponse> {
  const res = await authFetch('/billing/usage')
  if (!res.ok) throw new Error(`getUsage: ${res.status}`)
  return res.json()
}

export async function createPaymentIntent(planId: string): Promise<CreatePaymentIntentResponse> {
  const res = await authFetch('/billing/payment-intents', {
    method: 'POST',
    body: JSON.stringify({ plan_id: planId }),
  })
  if (!res.ok) throw new Error(`createPaymentIntent: ${res.status}`)
  return res.json()
}

export async function createSubscription(planId: string, paymentMethodId: string): Promise<Subscription> {
  const res = await authFetch('/billing/subscriptions', {
    method: 'POST',
    body: JSON.stringify({ plan_id: planId, payment_method_id: paymentMethodId }),
  })
  if (!res.ok) throw new Error(`createSubscription: ${res.status}`)
  return res.json()
}

export async function cancelSubscription(subscriptionId: string): Promise<void> {
  const res = await authFetch(`/billing/subscriptions/${subscriptionId}`, {
    method: 'DELETE',
  })
  if (!res.ok && res.status !== 204) throw new Error(`cancelSubscription: ${res.status}`)
}
```

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd frontend && npx tsc --noEmit 2>&1 | tail -20
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/api/billing.ts
git commit -m "feat(billing-ui): billing API module with typed responses"
```

---

### Task 2: Billing Pinia store

**Files:**
- Create: `frontend/src/stores/billing.ts`

- [ ] **Step 1: Create the store**

Create `frontend/src/stores/billing.ts`:
```typescript
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import {
  getPlans, getUsage, createPaymentIntent, cancelSubscription,
  type Plan, type Subscription, type UsageResponse, type CreatePaymentIntentResponse,
} from '../api/billing'

export const useBillingStore = defineStore('billing', () => {
  const plans = ref<Plan[]>([])
  const subscription = ref<Subscription | null>(null)
  const usage = ref<UsageResponse | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)
  const quotaExceeded = ref(false)
  const quotaMessage = ref<string | null>(null)

  const currentPlan = computed(() =>
    plans.value.find(p => p.id === subscription.value?.plan_id) ?? null
  )

  async function fetchPlans() {
    try {
      plans.value = await getPlans()
    } catch (e) {
      error.value = (e as Error).message
    }
  }

  async function fetchUsage() {
    try {
      usage.value = await getUsage()
    } catch (e) {
      error.value = (e as Error).message
    }
  }

  async function startUsagePolling() {
    await fetchUsage()
    const id = setInterval(fetchUsage, 30_000)
    return () => clearInterval(id)
  }

  async function initiatePayment(planId: string): Promise<CreatePaymentIntentResponse> {
    return createPaymentIntent(planId)
  }

  async function cancel(subscriptionId: string) {
    loading.value = true
    error.value = null
    try {
      await cancelSubscription(subscriptionId)
      subscription.value = null
    } catch (e) {
      error.value = (e as Error).message
    } finally {
      loading.value = false
    }
  }

  function flagQuotaExceeded(message: string) {
    quotaExceeded.value = true
    quotaMessage.value = message
  }

  function clearQuotaExceeded() {
    quotaExceeded.value = false
    quotaMessage.value = null
  }

  return {
    plans, subscription, usage, loading, error,
    quotaExceeded, quotaMessage, currentPlan,
    fetchPlans, fetchUsage, startUsagePolling,
    initiatePayment, cancel,
    flagQuotaExceeded, clearQuotaExceeded,
  }
})
```

- [ ] **Step 2: Verify TypeScript**

```bash
cd frontend && npx tsc --noEmit 2>&1 | tail -20
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/stores/billing.ts
git commit -m "feat(billing-ui): Pinia billing store with usage polling and quota flag"
```

---

### Task 3: Global 402 error interceptor

**Files:**
- Modify: `frontend/src/api/client.ts`

Currently there is no global API error interceptor. We need to intercept 402 responses and call `billing.flagQuotaExceeded(message)`.

- [ ] **Step 1: Read current `client.ts`**

Read `/Users/jobinlawrance/Project/raven/frontend/src/api/client.ts` to understand the current structure.

- [ ] **Step 2: Add 402 handling**

The billing API functions each use their own `authFetch`. The cleanest approach is to create a shared `authFetch` utility that all API modules will use:

Create `frontend/src/api/utils.ts`:
```typescript
import { useAuthStore } from '../stores/auth'
import { useBillingStore } from '../stores/billing'

const API_BASE = import.meta.env.VITE_API_BASE_URL ?? '/api/v1'

export async function authFetch(path: string, init?: RequestInit): Promise<Response> {
  const auth = useAuthStore()
  const res = await fetch(API_BASE + path, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${auth.accessToken ?? ''}`,
      ...init?.headers,
    },
  })

  if (res.status === 402) {
    // Clone to read body without consuming
    const body = await res.clone().json().catch(() => ({}))
    const billing = useBillingStore()
    billing.flagQuotaExceeded(body.message ?? 'Plan limit reached. Upgrade to continue.')
  }

  return res
}
```

Then update `frontend/src/api/billing.ts` to import from `./utils` instead of defining its own `authFetch`.

- [ ] **Step 3: TypeScript check**

```bash
cd frontend && npx tsc --noEmit 2>&1 | tail -20
```

- [ ] **Step 4: Commit**

```bash
git add frontend/src/api/utils.ts frontend/src/api/billing.ts
git commit -m "feat(billing-ui): shared authFetch with global 402 quota-exceeded interceptor"
```

---

### Task 4: UsageBar component

**Files:**
- Create: `frontend/src/components/UsageBar.vue`

Reusable progress bar showing used/limit with label.

- [ ] **Step 1: Create component**

Create `frontend/src/components/UsageBar.vue`:
```vue
<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  label: string
  used: number
  limit: number  // -1 means unlimited
  unit?: string
}>()

const pct = computed(() => {
  if (props.limit <= 0) return 0
  return Math.min(100, Math.round((props.used / props.limit) * 100))
})

const barColor = computed(() => {
  if (props.limit <= 0) return 'bg-gray-300 dark:bg-gray-600'
  if (pct.value >= 90) return 'bg-red-500'
  if (pct.value >= 70) return 'bg-yellow-500'
  return 'bg-indigo-500'
})

const displayLimit = computed(() =>
  props.limit <= 0 ? '∞' : String(props.limit)
)
</script>

<template>
  <div class="space-y-1">
    <div class="flex justify-between text-sm">
      <span class="text-gray-700 dark:text-gray-300 font-medium">{{ label }}</span>
      <span class="text-gray-500 dark:text-gray-400">
        {{ used }}{{ unit ? ` ${unit}` : '' }} / {{ displayLimit }}{{ limit > 0 && unit ? ` ${unit}` : '' }}
      </span>
    </div>
    <div class="h-2 bg-gray-100 dark:bg-gray-700 rounded-full overflow-hidden">
      <div
        v-if="limit > 0"
        class="h-full rounded-full transition-all duration-500"
        :class="barColor"
        :style="{ width: `${pct}%` }"
      />
      <div v-else class="h-full w-full bg-gray-300 dark:bg-gray-600 rounded-full" />
    </div>
  </div>
</template>
```

- [ ] **Step 2: TypeScript check**

```bash
cd frontend && npx tsc --noEmit 2>&1 | tail -10
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/UsageBar.vue
git commit -m "feat(billing-ui): UsageBar progress component with color thresholds"
```

---

### Task 5: PlanCard component

**Files:**
- Create: `frontend/src/components/PlanCard.vue`

Displays a single plan with feature list and CTA button.

- [ ] **Step 1: Create component**

Create `frontend/src/components/PlanCard.vue`:
```vue
<script setup lang="ts">
import type { Plan } from '../api/billing'

const props = defineProps<{
  plan: Plan
  isCurrent: boolean
  loading?: boolean
}>()

const emit = defineEmits<{
  select: [planId: string]
}>()

const features = [
  { label: 'Users', value: props.plan.max_users <= 0 ? 'Unlimited' : String(props.plan.max_users) },
  { label: 'Workspaces', value: props.plan.max_workspaces <= 0 ? 'Unlimited' : String(props.plan.max_workspaces) },
  { label: 'Knowledge Bases', value: props.plan.max_knowledge_bases <= 0 ? 'Unlimited' : String(props.plan.max_knowledge_bases) },
  { label: 'Storage', value: props.plan.max_storage_gb <= 0 ? 'Unlimited' : `${props.plan.max_storage_gb} GB` },
  { label: 'Voice Sessions', value: props.plan.max_voice_sessions <= 0 ? 'Unlimited' : String(props.plan.max_voice_sessions) },
  { label: 'Voice Minutes/mo', value: props.plan.max_voice_minutes_monthly <= 0 ? 'Unlimited' : String(props.plan.max_voice_minutes_monthly) },
]
</script>

<template>
  <div
    class="relative flex flex-col border-2 rounded-2xl p-6 transition-all"
    :class="isCurrent
      ? 'border-indigo-600 bg-indigo-50 dark:bg-indigo-900/20'
      : 'border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800'"
  >
    <div v-if="isCurrent"
      class="absolute top-3 right-3 text-xs font-semibold bg-indigo-600 text-white px-2 py-0.5 rounded-full">
      Current
    </div>

    <h3 class="text-xl font-bold text-gray-900 dark:text-white mb-1">{{ plan.display_name }}</h3>
    <p class="text-3xl font-bold text-gray-900 dark:text-white mb-1">
      <span v-if="plan.price_monthly === 0">Free</span>
      <span v-else>${{ plan.price_monthly }}<span class="text-base font-normal text-gray-500">/mo</span></span>
    </p>

    <ul class="mt-4 space-y-2 flex-1">
      <li v-for="f in features" :key="f.label" class="flex items-center gap-2 text-sm">
        <span class="text-green-500">✓</span>
        <span class="text-gray-700 dark:text-gray-300">{{ f.label }}</span>
        <span class="ml-auto font-medium text-gray-900 dark:text-white">{{ f.value }}</span>
      </li>
    </ul>

    <button
      v-if="!isCurrent"
      class="mt-6 w-full py-2.5 bg-indigo-600 text-white font-medium rounded-lg hover:bg-indigo-700 transition-colors disabled:opacity-50"
      :disabled="loading"
      @click="emit('select', plan.id)"
    >
      {{ loading ? 'Processing…' : 'Upgrade to ' + plan.display_name }}
    </button>
    <div v-else class="mt-6 w-full py-2.5 text-center text-indigo-600 font-medium text-sm">
      ✓ Your current plan
    </div>
  </div>
</template>
```

- [ ] **Step 2: TypeScript check**

```bash
cd frontend && npx tsc --noEmit 2>&1 | tail -10
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/PlanCard.vue
git commit -m "feat(billing-ui): PlanCard component with feature list and upgrade CTA"
```

---

### Task 6: UpgradePromptBanner component

**Files:**
- Create: `frontend/src/components/UpgradePromptBanner.vue`
- Modify: `frontend/src/layouts/DefaultLayout.vue`

Triggered when `billing.quotaExceeded === true`. Shown at the top of all authenticated pages.

- [ ] **Step 1: Create component**

Create `frontend/src/components/UpgradePromptBanner.vue`:
```vue
<script setup lang="ts">
import { useRouter } from 'vue-router'
import { useBillingStore } from '../stores/billing'
import { useFeatureFlag } from '../composables/useFeatureFlag'

const billing = useBillingStore()
const router = useRouter()
const { isEnabled: billingEnabled } = useFeatureFlag('billing_enabled')

function goToBilling() {
  billing.clearQuotaExceeded()
  router.push('/billing')
}
</script>

<template>
  <Transition name="banner">
    <div
      v-if="billingEnabled && billing.quotaExceeded"
      class="bg-amber-50 dark:bg-amber-900/20 border-b border-amber-200 dark:border-amber-700 px-4 py-3 flex items-center gap-3"
    >
      <span class="text-amber-600 dark:text-amber-400 text-lg">⚠️</span>
      <p class="text-sm text-amber-800 dark:text-amber-200 flex-1">
        {{ billing.quotaMessage ?? 'You\'ve reached your plan limit.' }}
      </p>
      <button
        class="shrink-0 px-4 py-1.5 bg-amber-600 text-white text-sm font-medium rounded-lg hover:bg-amber-700 transition-colors"
        @click="goToBilling"
      >
        Upgrade Plan
      </button>
      <button
        class="text-amber-600 dark:text-amber-400 hover:text-amber-800"
        aria-label="Dismiss"
        @click="billing.clearQuotaExceeded()"
      >
        ✕
      </button>
    </div>
  </Transition>
</template>

<style scoped>
.banner-enter-active, .banner-leave-active { transition: all 0.2s ease; }
.banner-enter-from, .banner-leave-to { opacity: 0; transform: translateY(-100%); }
</style>
```

- [ ] **Step 2: Add banner to DefaultLayout**

In `frontend/src/layouts/DefaultLayout.vue`, import and add `<UpgradePromptBanner />` at the top of the main content area (above the `<RouterView />`).

Read the file first to find the right insertion point.

- [ ] **Step 3: TypeScript + build check**

```bash
cd frontend && npx tsc --noEmit 2>&1 | tail -10
```

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/UpgradePromptBanner.vue frontend/src/layouts/DefaultLayout.vue
git commit -m "feat(billing-ui): global upgrade prompt banner wired into DefaultLayout"
```

---

### Task 7: Billing page (main UI)

**Files:**
- Create: `frontend/src/pages/billing/BillingPage.vue`

The page has three sections:
1. **Current plan & usage** — usage bars for KBs, seats, voice minutes
2. **Plan selection** — PlanCard grid (only show Pro and Enterprise cards if on Free)
3. **Subscription management** — cancel button with confirmation modal

Payment flow: clicking "Upgrade" on a PlanCard calls `billing.initiatePayment(planId)` to get a `client_secret`, then opens Hyperswitch embedded checkout. On success, page refreshes subscription.

**Hyperswitch SDK**: Load dynamically from CDN — `https://checkout.hyperswitch.io/v0/HyperLoader.js`. Initialize with `clientSecret` from backend, mount to `#hyperswitch-checkout` div, listen for `paymentSuccess` event.

- [ ] **Step 1: Create the page**

Create `frontend/src/pages/billing/BillingPage.vue`:
```vue
<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useBillingStore } from '../../stores/billing'
import UsageBar from '../../components/UsageBar.vue'
import PlanCard from '../../components/PlanCard.vue'

const billing = useBillingStore()
const showCancelModal = ref(false)
const paymentLoading = ref(false)
const paymentError = ref<string | null>(null)
let stopPolling: (() => void) | null = null

onMounted(async () => {
  await Promise.all([billing.fetchPlans(), billing.fetchUsage()])
  stopPolling = await billing.startUsagePolling()
})
onUnmounted(() => stopPolling?.())

async function handleUpgrade(planId: string) {
  paymentLoading.value = true
  paymentError.value = null
  try {
    const { client_secret } = await billing.initiatePayment(planId)
    await openHyperswitchCheckout(client_secret)
  } catch (e) {
    paymentError.value = (e as Error).message
  } finally {
    paymentLoading.value = false
  }
}

async function openHyperswitchCheckout(clientSecret: string): Promise<void> {
  return new Promise((resolve, reject) => {
    // Dynamically load Hyperswitch SDK
    const existing = document.getElementById('hyperswitch-sdk')
    if (existing) existing.remove()
    const script = document.createElement('script')
    script.id = 'hyperswitch-sdk'
    script.src = 'https://checkout.hyperswitch.io/v0/HyperLoader.js'
    script.onload = async () => {
      try {
        // @ts-expect-error Hyperswitch SDK is loaded dynamically
        const hyper = window.Hyper(import.meta.env.VITE_HYPERSWITCH_PUBLISHABLE_KEY)
        const elements = hyper.elements({ clientSecret })
        const paymentElement = elements.create('payment')
        const container = document.getElementById('hyperswitch-checkout')
        if (container) {
          container.innerHTML = ''
          paymentElement.mount('#hyperswitch-checkout')
        }

        // Listen for completion — in real Hyperswitch integration this is via
        // confirmPayment call + redirect. For now, expose a resolve hook.
        // Hyperswitch SDK redirects on success; handle returnUrl on the page.
        resolve()
      } catch (err) {
        reject(err)
      }
    }
    script.onerror = () => reject(new Error('Failed to load payment SDK'))
    document.head.appendChild(script)
  })
}

async function confirmCancel() {
  if (!billing.subscription) return
  await billing.cancel(billing.subscription.id)
  showCancelModal.value = false
}
</script>

<template>
  <div class="max-w-4xl mx-auto px-4 py-8 space-y-10">
    <!-- Header -->
    <div>
      <h1 class="text-2xl font-bold text-gray-900 dark:text-white">Billing & Subscription</h1>
      <p class="text-gray-500 dark:text-gray-400 text-sm mt-1">
        Manage your plan and track usage.
      </p>
    </div>

    <!-- Current Plan Summary -->
    <section class="bg-white dark:bg-gray-800 rounded-2xl border border-gray-200 dark:border-gray-700 p-6 space-y-4">
      <div class="flex items-center justify-between">
        <div>
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ billing.currentPlan?.display_name ?? 'Free' }} Plan
          </h2>
          <p v-if="billing.subscription" class="text-sm text-gray-500 dark:text-gray-400">
            Renews {{ new Date(billing.subscription.current_period_end).toLocaleDateString() }}
          </p>
          <p v-else class="text-sm text-gray-500 dark:text-gray-400">No active subscription</p>
        </div>
        <span
          v-if="billing.subscription"
          class="px-3 py-1 text-xs font-semibold rounded-full"
          :class="{
            'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400': billing.subscription.status === 'active',
            'bg-yellow-100 text-yellow-700': billing.subscription.status === 'past_due',
            'bg-red-100 text-red-700': billing.subscription.status === 'canceled',
          }"
        >
          {{ billing.subscription.status }}
        </span>
      </div>
    </section>

    <!-- Usage Dashboard -->
    <section class="bg-white dark:bg-gray-800 rounded-2xl border border-gray-200 dark:border-gray-700 p-6 space-y-4">
      <h2 class="text-lg font-semibold text-gray-900 dark:text-white">Current Usage</h2>

      <div v-if="billing.usage" class="space-y-4">
        <UsageBar
          label="Knowledge Bases"
          :used="billing.usage.kbs_used"
          :limit="billing.usage.kbs_limit"
        />
        <UsageBar
          label="Team Members"
          :used="billing.usage.seats_used"
          :limit="billing.usage.seats_limit"
        />
        <UsageBar
          label="Voice Minutes"
          :used="billing.usage.voice_minutes_used"
          :limit="billing.usage.voice_minutes_limit"
          unit="min"
        />
        <div class="text-sm text-gray-600 dark:text-gray-400">
          Concurrent voice sessions: <strong>{{ billing.usage.concurrent_voice_limit }}</strong>
        </div>
      </div>
      <div v-else class="animate-pulse space-y-3">
        <div class="h-4 bg-gray-200 dark:bg-gray-700 rounded w-3/4" />
        <div class="h-4 bg-gray-200 dark:bg-gray-700 rounded w-2/3" />
        <div class="h-4 bg-gray-200 dark:bg-gray-700 rounded w-1/2" />
      </div>
    </section>

    <!-- Plan Selection -->
    <section>
      <h2 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">Choose a Plan</h2>

      <div v-if="paymentError" class="mb-4 p-3 bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-400 text-sm rounded-lg">
        {{ paymentError }}
      </div>

      <div class="grid md:grid-cols-3 gap-4">
        <PlanCard
          v-for="plan in billing.plans"
          :key="plan.id"
          :plan="plan"
          :is-current="plan.id === billing.subscription?.plan_id"
          :loading="paymentLoading"
          @select="handleUpgrade"
        />
      </div>

      <!-- Hyperswitch checkout container -->
      <div id="hyperswitch-checkout" class="mt-6 min-h-[200px]" />
    </section>

    <!-- Subscription Management -->
    <section v-if="billing.subscription && billing.subscription.status === 'active'"
      class="bg-white dark:bg-gray-800 rounded-2xl border border-gray-200 dark:border-gray-700 p-6">
      <h2 class="text-lg font-semibold text-gray-900 dark:text-white mb-1">Cancel Subscription</h2>
      <p class="text-sm text-gray-500 dark:text-gray-400 mb-4">
        Your access will continue until the end of the current billing period.
      </p>
      <button
        class="px-4 py-2 text-sm font-medium text-red-600 border border-red-300 rounded-lg hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors"
        @click="showCancelModal = true"
      >
        Cancel Subscription
      </button>
    </section>
  </div>

  <!-- Cancel confirmation modal -->
  <Teleport to="body">
    <Transition name="modal">
      <div v-if="showCancelModal" class="fixed inset-0 z-50 flex items-center justify-center p-4">
        <div class="absolute inset-0 bg-black/50" @click="showCancelModal = false" />
        <div class="relative bg-white dark:bg-gray-800 rounded-2xl shadow-xl p-6 w-full max-w-sm">
          <h3 class="text-lg font-bold text-gray-900 dark:text-white mb-2">Cancel subscription?</h3>
          <p class="text-sm text-gray-500 dark:text-gray-400 mb-6">
            You'll be downgraded to the Free plan at the end of your billing period. This cannot be undone.
          </p>
          <div class="flex gap-3">
            <button
              class="flex-1 py-2 text-sm text-gray-700 dark:text-gray-300 border border-gray-200 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700"
              @click="showCancelModal = false"
            >
              Keep Plan
            </button>
            <button
              class="flex-1 py-2 text-sm font-medium text-white bg-red-600 rounded-lg hover:bg-red-700 transition-colors"
              :disabled="billing.loading"
              @click="confirmCancel"
            >
              {{ billing.loading ? 'Canceling…' : 'Yes, Cancel' }}
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.modal-enter-active, .modal-leave-active { transition: all 0.2s ease; }
.modal-enter-from, .modal-leave-to { opacity: 0; }
</style>
```

- [ ] **Step 2: TypeScript + build check**

```bash
cd frontend && npx tsc --noEmit 2>&1 | tail -20
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/pages/billing/BillingPage.vue
git commit -m "feat(billing-ui): billing page with plan cards, usage bars, payment flow, and cancel modal"
```

---

### Task 8: Routes + Sidebar navigation

**Files:**
- Modify: `frontend/src/router/index.ts`
- Modify: `frontend/src/components/AppSidebar.vue`

- [ ] **Step 1: Add billing route**

In `frontend/src/router/index.ts`, inside the `DefaultLayout` children array, add:
```typescript
{
  path: 'billing',
  name: 'billing',
  component: () => import('../pages/billing/BillingPage.vue'),
  meta: { requiresAuth: true },
},
```

- [ ] **Step 2: Read AppSidebar.vue**

Read `frontend/src/components/AppSidebar.vue` to understand the nav item structure (icon, label, route).

- [ ] **Step 3: Add billing nav item**

Add a "Billing" navigation entry to the sidebar that links to `/billing`. Use a credit card icon (existing icon library or inline SVG). Add it near the bottom of the settings/admin links group.

The nav item should be wrapped in a `v-if` using `useFeatureFlag('billing_enabled').isEnabled` so it only appears when billing is active.

- [ ] **Step 4: Build check**

```bash
cd frontend && npm run build 2>&1 | tail -20
```

- [ ] **Step 5: Commit**

```bash
git add frontend/src/router/index.ts frontend/src/components/AppSidebar.vue
git commit -m "feat(billing-ui): add /billing route and sidebar navigation item"
```

---

### Task 9: Full verification

- [ ] **Step 1: TypeScript strict check**

```bash
cd frontend && npx tsc --noEmit 2>&1 | tail -30
```

- [ ] **Step 2: Lint**

```bash
cd frontend && npm run lint 2>&1 | tail -30
```

- [ ] **Step 3: Production build**

```bash
cd frontend && npm run build 2>&1 | tail -20
```

- [ ] **Step 4: Fix all issues**

Fix any TypeScript or lint errors. Commit fixes.

---

## Push + PR

```bash
git push -u origin feat/issue-194-billing-ui
gh pr create \
  --title "feat(frontend): billing and subscription management UI" \
  --body "Closes #194

## Summary
- Billing Pinia store with usage polling (30s interval)
- Plan selection page with PlanCard components and feature comparison
- Hyperswitch JS SDK payment flow integration
- UsageBar components with color thresholds (green/yellow/red)
- Reusable UpgradePromptBanner triggered by 402 responses (feature-flagged behind \`billing_enabled\`)
- Cancel subscription flow with confirmation modal
- Global 402 interceptor in shared authFetch utility

## Test plan
- [ ] Navigate to /billing — verify plan cards render correctly
- [ ] Usage bars display with correct percentages
- [ ] Clicking upgrade opens Hyperswitch checkout
- [ ] Cancel subscription shows confirmation modal
- [ ] Triggering a 402 error (e.g. exceed KB limit) shows upgrade banner
- [ ] Banner only shows when PostHog \`billing_enabled\` flag is true" \
  --base main
gh pr merge <PR_NUMBER> --auto --squash
```
