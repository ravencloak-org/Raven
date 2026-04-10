<template>
  <div class="mx-auto max-w-5xl space-y-8 py-2">
    <h1 class="text-2xl font-bold text-gray-900">Billing & Subscription</h1>

    <!-- Current plan summary -->
    <section class="rounded-xl border border-gray-200 bg-white p-6 shadow-sm">
      <h2 class="mb-4 text-lg font-semibold text-gray-900">Current Plan</h2>
      <div v-if="billing.subscription" class="flex flex-wrap items-center gap-6">
        <div>
          <p class="text-sm text-gray-500">Plan</p>
          <p class="font-medium text-gray-900">{{ billing.currentPlan?.name ?? 'Unknown' }}</p>
        </div>
        <div>
          <p class="text-sm text-gray-500">Status</p>
          <span
            class="inline-block rounded-full px-2 py-0.5 text-xs font-semibold capitalize"
            :class="statusBadgeClass"
          >
            {{ billing.subscription.status }}
          </span>
        </div>
        <div v-if="billing.subscription.current_period_end">
          <p class="text-sm text-gray-500">Renews / Expires</p>
          <p class="font-medium text-gray-900">{{ formatDate(billing.subscription.current_period_end) }}</p>
        </div>
      </div>
      <p v-else class="text-sm text-gray-500">No active subscription. Choose a plan below.</p>
    </section>

    <!-- Usage dashboard -->
    <section class="rounded-xl border border-gray-200 bg-white p-6 shadow-sm">
      <h2 class="mb-4 text-lg font-semibold text-gray-900">Usage</h2>
      <div v-if="billing.usage" class="space-y-4">
        <UsageBar
          label="Knowledge Bases"
          :used="billing.usage.knowledge_bases_used"
          :limit="billing.usage.knowledge_bases_limit"
        />
        <UsageBar
          label="Seats"
          :used="billing.usage.seats_used"
          :limit="billing.usage.seats_limit"
          unit="users"
        />
        <UsageBar
          label="Voice Minutes"
          :used="billing.usage.voice_minutes_used"
          :limit="billing.usage.voice_minutes_limit"
          unit="min"
        />
        <p class="text-sm text-gray-600">
          Concurrent sessions:
          <span class="font-medium">{{ billing.usage.concurrent_sessions }}</span>
          <span v-if="billing.usage.concurrent_sessions_limit !== -1">
            / {{ billing.usage.concurrent_sessions_limit }}
          </span>
          <span v-else> (Unlimited)</span>
        </p>
      </div>
      <p v-else class="text-sm text-gray-400">Loading usage data...</p>
    </section>

    <!-- Plan selection grid -->
    <section class="rounded-xl border border-gray-200 bg-white p-6 shadow-sm">
      <h2 class="mb-4 text-lg font-semibold text-gray-900">Available Plans</h2>
      <div v-if="billing.loading" class="flex items-center justify-center py-8">
        <svg class="h-8 w-8 animate-spin text-indigo-600" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z" />
        </svg>
      </div>
      <div
        v-else
        class="grid gap-6"
        :class="billing.plans.length > 2 ? 'md:grid-cols-3' : 'md:grid-cols-2'"
      >
        <PlanCard
          v-for="plan in billing.plans"
          :key="plan.id"
          :plan="plan"
          :is-current="billing.subscription?.plan_id === plan.id"
          :loading="selectingPlanId === plan.id"
          @select="handleSelectPlan"
        />
      </div>
      <p v-if="billing.error" class="mt-4 text-sm text-red-600">{{ billing.error }}</p>
    </section>

    <!-- Hyperswitch checkout container -->
    <div v-if="checkoutVisible" id="hyperswitch-checkout" class="rounded-xl border border-gray-200 bg-white p-6 shadow-sm" />

    <!-- Cancel subscription section -->
    <section
      v-if="billing.subscription?.status === 'active'"
      class="rounded-xl border border-red-100 bg-red-50 p-6"
    >
      <h2 class="mb-2 text-lg font-semibold text-red-800">Cancel Subscription</h2>
      <p class="mb-4 text-sm text-red-700">
        Cancelling will end your subscription at the current billing period. You won't be charged again.
      </p>
      <button
        class="rounded-lg border border-red-300 px-4 py-2 text-sm font-semibold text-red-700 transition-colors hover:bg-red-100"
        @click="showCancelModal = true"
      >
        Cancel Subscription
      </button>
    </section>

    <!-- Cancel confirmation modal via Teleport -->
    <Teleport to="body">
      <Transition name="modal">
        <div
          v-if="showCancelModal"
          class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
          @click.self="showCancelModal = false"
        >
          <div class="w-full max-w-md rounded-xl bg-white p-6 shadow-xl">
            <h3 class="mb-2 text-lg font-bold text-gray-900">Confirm Cancellation</h3>
            <p class="mb-6 text-sm text-gray-600">
              Are you sure you want to cancel your subscription? This action cannot be undone.
            </p>
            <div class="flex justify-end gap-3">
              <button
                class="rounded-lg px-4 py-2 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-100"
                @click="showCancelModal = false"
              >
                Keep Subscription
              </button>
              <button
                class="flex items-center gap-2 rounded-lg bg-red-600 px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-red-700 disabled:opacity-60"
                :disabled="cancelling"
                @click="handleCancelSubscription"
              >
                <svg
                  v-if="cancelling"
                  class="h-4 w-4 animate-spin"
                  xmlns="http://www.w3.org/2000/svg"
                  fill="none"
                  viewBox="0 0 24 24"
                >
                  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
                  <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z" />
                </svg>
                Yes, Cancel
              </button>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useBillingStore } from '../../stores/billing'
import UsageBar from '../../components/UsageBar.vue'
import PlanCard from '../../components/PlanCard.vue'

const billing = useBillingStore()

const showCancelModal = ref(false)
const cancelling = ref(false)
const selectingPlanId = ref<string | null>(null)
const checkoutVisible = ref(false)

let stopPolling: (() => void) | null = null

onMounted(async () => {
  await Promise.all([billing.fetchPlans(), billing.fetchUsage()])
  stopPolling = billing.startUsagePolling()
})

onUnmounted(() => {
  stopPolling?.()
})

const statusBadgeClass = computed(() => {
  const status = billing.subscription?.status
  switch (status) {
    case 'active':
      return 'bg-green-100 text-green-800'
    case 'trialing':
      return 'bg-blue-100 text-blue-800'
    case 'past_due':
      return 'bg-yellow-100 text-yellow-800'
    case 'cancelled':
      return 'bg-red-100 text-red-800'
    default:
      return 'bg-gray-100 text-gray-800'
  }
})

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
  })
}

async function handleSelectPlan(planId: string): Promise<void> {
  selectingPlanId.value = planId
  try {
    const { client_secret } = await billing.initiatePayment(planId)
    await mountHyperswitchCheckout(client_secret)
  } catch {
    // Error already set in store
  } finally {
    selectingPlanId.value = null
  }
}

async function mountHyperswitchCheckout(clientSecret: string): Promise<void> {
  checkoutVisible.value = true

  // Dynamically load the Hyperswitch SDK
  await loadScript('https://checkout.hyperswitch.io/v0/HyperLoader.js')

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const Hyper = (window as any).Hyper
  if (!Hyper) {
    billing.error = 'Hyperswitch SDK failed to load.'
    return
  }

  const publishableKey = import.meta.env.VITE_HYPERSWITCH_PUBLISHABLE_KEY as string
  const hyper = Hyper(publishableKey)
  const elements = hyper.elements({ clientSecret })
  const paymentElement = elements.create('payment')

  // Wait for the DOM element to be available
  await nextTick()
  paymentElement.mount('#hyperswitch-checkout')
}

function loadScript(src: string): Promise<void> {
  return new Promise((resolve, reject) => {
    if (document.querySelector(`script[src="${src}"]`)) {
      resolve()
      return
    }
    const script = document.createElement('script')
    script.src = src
    script.onload = () => resolve()
    script.onerror = () => reject(new Error(`Failed to load script: ${src}`))
    document.head.appendChild(script)
  })
}

// Import nextTick for DOM readiness
import { nextTick } from 'vue'

async function handleCancelSubscription(): Promise<void> {
  if (!billing.subscription) return
  cancelling.value = true
  try {
    await billing.cancel(billing.subscription.id)
    showCancelModal.value = false
  } catch {
    // Error already set in store
  } finally {
    cancelling.value = false
  }
}
</script>

<style scoped>
.modal-enter-active,
.modal-leave-active {
  transition: opacity 0.2s ease;
}

.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}
</style>
