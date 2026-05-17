<script setup lang="ts">
/**
 * PlansPage — side-by-side comparison of Free / Pro / Enterprise plans.
 *
 * Route: /orgs/:orgId/billing/plans (name: `billing-plans`)
 *
 * - Plan data is fetched from GET /api/v1/billing/plans via getPlans().
 *   The Go backend returns the full Plan struct with `tier`, `max_users`,
 *   `max_workspaces`, `max_kbs`, `max_storage_mb`,
 *   `max_concurrent_voice_sessions`, `max_voice_minutes_monthly`.
 *   `-1` means unlimited and is rendered as "Unlimited".
 * - The current plan is read from the billing store and highlighted with a
 *   "Current plan" badge.
 * - Pro -> RouterLink to `billing-checkout` ('/orgs/:orgId/billing/upgrade').
 * - Enterprise -> mailto: sales@ravencloak.org (placeholder address; can be
 *   swapped out once a canonical sales inbox exists).
 * - The feature matrix uses a semantic <table> with <thead>/<tbody> for
 *   accessibility, and collapses to a stacked card layout on narrow viewports.
 */
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { getPlans, type Plan, type PlanName } from '../../api/billing'
import { useBillingStore } from '../../stores/billing'

const SALES_EMAIL = 'sales@ravencloak.org'

// Plan tiers we render, in the order they appear left-to-right.
const TIER_ORDER: PlanName[] = ['free', 'pro', 'enterprise']

const route = useRoute()
const store = useBillingStore()

const orgId = computed(() => route.params.orgId as string)

const plans = ref<Plan[]>([])
const loading = ref(false)
const error = ref<string | null>(null)

onMounted(async () => {
  loading.value = true
  error.value = null
  try {
    // Fire the plan fetch and ensure we know which plan the org is on.
    const [fetched] = await Promise.all([
      getPlans(),
      store.subscription ? Promise.resolve() : store.fetchSubscription(orgId.value),
    ])
    plans.value = fetched
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to load plans'
  } finally {
    loading.value = false
  }
})

// Resolve the current org plan; defaults to 'free' until the subscription
// loads (the store leaves `subscription` null for free-tier orgs).
const currentPlan = computed<PlanName>(() => store.subscription?.plan ?? 'free')

// Pull a plan by tier, falling back to matching on name when the backend
// omits the optional `tier` field on older deployments.
function planByTier(tier: PlanName): Plan | undefined {
  return plans.value.find(
    (p) => p.tier === tier || p.name?.toLowerCase() === tier,
  )
}

const orderedPlans = computed(() =>
  TIER_ORDER.map((tier) => ({ tier, plan: planByTier(tier) })),
)

function tierLabel(tier: PlanName): string {
  return tier.charAt(0).toUpperCase() + tier.slice(1)
}

// Formats the per-seat price in INR. Free renders as "Free"; everything
// else converts paise -> rupees using a localised number.
function priceLabel(tier: PlanName, plan: Plan | undefined): string {
  if (tier === 'free') return 'Free'
  if (!plan || !plan.price_per_seat_monthly) return '—'
  const rupees = plan.price_per_seat_monthly / 100
  return `₹${rupees.toLocaleString('en-IN')}`
}

function priceSuffix(tier: PlanName): string {
  return tier === 'free' ? '' : '/seat/month'
}

// `-1` -> "Unlimited"; anything else gets the unit appended.
function limitLabel(value: number | undefined, unit = ''): string {
  if (value === undefined || value === null) return '—'
  if (value === -1) return 'Unlimited'
  const formatted = value.toLocaleString()
  return unit ? `${formatted} ${unit}` : formatted
}

// Storage rendering: <1024 MB stays in MB, otherwise GB.
function storageLabel(mb: number | undefined): string {
  if (mb === undefined || mb === null) return '—'
  if (mb === -1) return 'Unlimited'
  if (mb >= 1024) return `${(mb / 1024).toLocaleString()} GB`
  return `${mb.toLocaleString()} MB`
}

// Feature rows for the comparison table. Each row pulls a value out of the
// fetched Plan and runs it through the appropriate formatter.
interface FeatureRow {
  label: string
  value: (plan: Plan | undefined) => string
}

const featureRows: FeatureRow[] = [
  { label: 'Users', value: (p) => limitLabel(p?.max_users) },
  { label: 'Workspaces', value: (p) => limitLabel(p?.max_workspaces) },
  { label: 'Knowledge bases', value: (p) => limitLabel(p?.max_kbs) },
  { label: 'Storage', value: (p) => storageLabel(p?.max_storage_mb) },
  {
    label: 'Concurrent voice sessions',
    value: (p) => limitLabel(p?.max_concurrent_voice_sessions),
  },
  {
    label: 'Voice minutes / month',
    value: (p) => limitLabel(p?.max_voice_minutes_monthly, 'min'),
  },
  { label: 'Minimum seats', value: (p) => (p ? p.min_seats.toString() : '—') },
]

function isCurrent(tier: PlanName): boolean {
  return tier === currentPlan.value
}

const enterpriseMailto = computed(() => {
  const subject = encodeURIComponent('Raven Enterprise plan enquiry')
  const body = encodeURIComponent(
    `Hi Raven team,\n\nWe'd like to talk about the Enterprise plan for our org (id: ${orgId.value}).\n\nThanks,`,
  )
  return `mailto:${SALES_EMAIL}?subject=${subject}&body=${body}`
})
</script>

<template>
  <div class="p-4 sm:p-6 max-w-6xl mx-auto">
    <!-- Header -->
    <div class="mb-6 sm:mb-8">
      <RouterLink
        :to="`/orgs/${orgId}/billing`"
        class="mb-3 inline-flex items-center gap-1 text-sm text-slate-400 hover:text-slate-200 transition-colors"
        aria-label="Back to billing"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-4 w-4"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          stroke-width="2"
          aria-hidden="true"
        >
          <path stroke-linecap="round" stroke-linejoin="round" d="M15 19l-7-7 7-7" />
        </svg>
        Back to Billing
      </RouterLink>
      <h1 class="text-2xl font-bold text-white">Compare plans</h1>
      <p class="mt-1 text-sm text-slate-400">
        Pick the plan that fits your team. You can upgrade or downgrade any time.
      </p>
    </div>

    <!-- Loading / error -->
    <div v-if="loading" class="text-slate-400 text-sm" role="status">Loading plans…</div>
    <div
      v-else-if="error"
      class="rounded-lg bg-red-900/40 border border-red-500/50 p-4 text-sm text-red-300"
      role="alert"
    >
      {{ error }}
    </div>

    <template v-else>
      <!-- Plan cards -->
      <section
        class="grid grid-cols-1 md:grid-cols-3 gap-4 sm:gap-6"
        aria-label="Plan comparison"
      >
        <article
          v-for="{ tier, plan } in orderedPlans"
          :key="tier"
          class="relative flex flex-col rounded-xl bg-slate-800 p-5 ring-1 ring-slate-700"
          :class="{ 'ring-2 ring-indigo-500': isCurrent(tier) }"
          :aria-current="isCurrent(tier) ? 'true' : undefined"
        >
          <!-- Current-plan badge -->
          <span
            v-if="isCurrent(tier)"
            class="absolute -top-3 right-4 inline-flex items-center rounded-full bg-indigo-600 px-3 py-1 text-xs font-semibold text-white"
          >
            Current plan
          </span>

          <!-- Tier + price -->
          <header class="mb-4">
            <p
              class="text-xs font-semibold uppercase tracking-wider text-slate-400 mb-1"
            >
              {{ tierLabel(tier) }}
            </p>
            <p class="text-3xl font-bold text-white">
              {{ priceLabel(tier, plan) }}<span
                v-if="priceSuffix(tier)"
                class="text-sm font-normal text-slate-400"
              >
                {{ priceSuffix(tier) }}</span>
            </p>
            <p v-if="plan && tier !== 'free'" class="mt-1 text-xs text-slate-500">
              Minimum {{ plan.min_seats }} seats · billed in INR
            </p>
          </header>

          <!-- CTA -->
          <div class="mb-5">
            <span
              v-if="tier === 'free'"
              class="inline-flex w-full items-center justify-center min-h-[44px] rounded-lg border border-slate-600 px-6 text-sm font-medium text-slate-400 cursor-default"
              aria-label="Free plan — no action required"
            >
              {{ isCurrent('free') ? 'Your plan' : 'Included' }}
            </span>

            <RouterLink
              v-else-if="tier === 'pro'"
              :to="`/orgs/${orgId}/billing/upgrade`"
              class="inline-flex w-full items-center justify-center min-h-[44px] rounded-lg bg-indigo-600 px-6 text-sm font-semibold text-white hover:bg-indigo-500 transition-colors focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 focus:ring-offset-slate-800"
              :aria-label="isCurrent('pro') ? 'Manage Pro subscription' : 'Upgrade to Pro plan'"
            >
              {{ isCurrent('pro') ? 'Manage plan' : 'Upgrade' }}
            </RouterLink>

            <a
              v-else
              :href="enterpriseMailto"
              class="inline-flex w-full items-center justify-center min-h-[44px] rounded-lg border border-indigo-500 px-6 text-sm font-semibold text-indigo-300 hover:bg-indigo-500/10 transition-colors focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 focus:ring-offset-slate-800"
              aria-label="Contact Raven sales about the Enterprise plan"
            >
              Contact Sales
            </a>
          </div>

          <!-- Mobile/stacked feature list (hidden on md+ in favour of the table) -->
          <ul class="space-y-2 text-sm text-slate-300 md:hidden">
            <li
              v-for="row in featureRows"
              :key="row.label"
              class="flex justify-between gap-2"
            >
              <span class="text-slate-400">{{ row.label }}</span>
              <span class="font-medium text-white text-right">{{ row.value(plan) }}</span>
            </li>
          </ul>
        </article>
      </section>

      <!-- Desktop feature matrix -->
      <section class="mt-8 hidden md:block" aria-label="Feature comparison">
        <div class="overflow-hidden rounded-xl bg-slate-800 ring-1 ring-slate-700">
          <table class="w-full text-sm">
            <caption class="sr-only">
              Side-by-side comparison of Free, Pro, and Enterprise plan limits
            </caption>
            <thead>
              <tr class="border-b border-slate-700 text-left">
                <th
                  scope="col"
                  class="px-5 py-3 text-xs font-semibold uppercase tracking-wider text-slate-400"
                >
                  Feature
                </th>
                <th
                  v-for="{ tier } in orderedPlans"
                  :key="`th-${tier}`"
                  scope="col"
                  class="px-5 py-3 text-xs font-semibold uppercase tracking-wider text-slate-400"
                  :class="{ 'text-indigo-300': isCurrent(tier) }"
                >
                  {{ tierLabel(tier) }}
                </th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="(row, idx) in featureRows"
                :key="row.label"
                :class="idx % 2 === 0 ? 'bg-slate-800' : 'bg-slate-800/60'"
              >
                <th
                  scope="row"
                  class="px-5 py-3 text-left font-medium text-slate-300"
                >
                  {{ row.label }}
                </th>
                <td
                  v-for="{ tier, plan } in orderedPlans"
                  :key="`${row.label}-${tier}`"
                  class="px-5 py-3 text-white"
                  :class="{ 'font-semibold': isCurrent(tier) }"
                >
                  {{ row.value(plan) }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </template>
  </div>
</template>
