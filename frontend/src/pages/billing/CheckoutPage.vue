<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getPlans, createSubscription, type Plan, type BillingCycle } from '../../api/billing'

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------
const PRO_PLAN_ID = 'plan_pro'
const MIN_SEATS = 5
// Price per seat per month in paise (₹1,700 = 170,000 paise)
const PRICE_PER_SEAT_PAISE = 170_000
// Annual = 10× monthly (2 months free)
const ANNUAL_MONTHS = 10

const HYPERSWITCH_PUBLISHABLE_KEY =
  import.meta.env.VITE_HYPERSWITCH_PUBLISHABLE_KEY ?? ''

// ---------------------------------------------------------------------------
// Route / router
// ---------------------------------------------------------------------------
const route = useRoute()
const router = useRouter()

const orgId = computed(() => route.params.orgId as string)

// ---------------------------------------------------------------------------
// Step 1 & 2 state — seat count + billing cycle
// ---------------------------------------------------------------------------
const seatCount = ref(MIN_SEATS)
const billingCycle = ref<BillingCycle>('monthly')

function clampSeats(v: number): number {
  return Math.max(MIN_SEATS, Math.floor(isNaN(v) ? MIN_SEATS : v))
}

function onSeatInput(evt: Event) {
  const raw = parseInt((evt.target as HTMLInputElement).value, 10)
  seatCount.value = clampSeats(raw)
}

function incrementSeats() {
  seatCount.value += 1
}

function decrementSeats() {
  seatCount.value = clampSeats(seatCount.value - 1)
}

// ---------------------------------------------------------------------------
// Pricing helpers
// ---------------------------------------------------------------------------
const monthlyTotalPaise = computed(() => PRICE_PER_SEAT_PAISE * seatCount.value)
const annualTotalPaise = computed(() => PRICE_PER_SEAT_PAISE * seatCount.value * ANNUAL_MONTHS)
const annualMonthlySavingsPaise = computed(
  () => PRICE_PER_SEAT_PAISE * seatCount.value * 2,
)

function formatINR(paise: number): string {
  const rupees = paise / 100
  return new Intl.NumberFormat('en-IN', { style: 'currency', currency: 'INR', maximumFractionDigits: 0 }).format(rupees)
}

const displayPrice = computed(() =>
  billingCycle.value === 'monthly'
    ? formatINR(monthlyTotalPaise.value)
    : formatINR(annualTotalPaise.value),
)

const displayPeriod = computed(() =>
  billingCycle.value === 'monthly' ? '/month' : '/year',
)

const savingsLabel = computed(() =>
  `Save ${formatINR(annualMonthlySavingsPaise.value)} (2 months free)`,
)

// ---------------------------------------------------------------------------
// Step 3 — payment
// ---------------------------------------------------------------------------
const loadingPlans = ref(false)
const plansError = ref<string | null>(null)
const plans = ref<Plan[]>([])

const creatingSubscription = ref(false)
const subscriptionError = ref<string | null>(null)
const clientSecret = ref<string | null>(null)

// Hyperswitch SDK state
type HyperInstance = {
  elements: (opts: { clientSecret: string }) => HyperElements
}
type HyperElements = {
  create: (type: string) => HyperElement
  submit: () => Promise<{ error?: { message: string } }>
}
type HyperElement = {
  mount: (selector: string) => void
  unmount: () => void
}

declare global {
  interface Window {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    Hyper?: (key: string) => HyperInstance
  }
}

const hyperInstance = ref<HyperInstance | null>(null)
const hyperElements = ref<HyperElements | null>(null)
const paymentElement = ref<HyperElement | null>(null)
const sdkLoaded = ref(false)
const sdkLoadError = ref<string | null>(null)
const paymentProcessing = ref(false)
const paymentError = ref<string | null>(null)

// Success state for redirect
const checkoutSuccess = ref(false)

// ---------------------------------------------------------------------------
// Load Hyperswitch SDK from CDN
// ---------------------------------------------------------------------------
function loadHyperswitchSDK(): Promise<void> {
  return new Promise((resolve, reject) => {
    if (window.Hyper) {
      resolve()
      return
    }
    const existing = document.getElementById('hyperswitch-sdk')
    if (existing) {
      existing.addEventListener('load', () => resolve())
      existing.addEventListener('error', () => reject(new Error('Hyperswitch SDK failed to load')))
      return
    }
    const script = document.createElement('script')
    script.id = 'hyperswitch-sdk'
    script.src = 'https://beta.hyperswitch.io/v1/HyperLoader.js'
    script.onload = () => resolve()
    script.onerror = () => reject(new Error('Hyperswitch SDK failed to load from CDN'))
    document.head.appendChild(script)
  })
}

// ---------------------------------------------------------------------------
// Initialise payment element whenever clientSecret changes
// ---------------------------------------------------------------------------
watch(clientSecret, async (secret) => {
  if (!secret) return

  // Unmount previous element if any
  paymentElement.value?.unmount()
  paymentElement.value = null

  if (!HYPERSWITCH_PUBLISHABLE_KEY) {
    sdkLoadError.value =
      'Hyperswitch publishable key is not configured. Set VITE_HYPERSWITCH_PUBLISHABLE_KEY in your environment.'
    return
  }

  try {
    await loadHyperswitchSDK()
    sdkLoaded.value = true

    if (!window.Hyper) throw new Error('Hyperswitch SDK not available after load')

    hyperInstance.value = window.Hyper(HYPERSWITCH_PUBLISHABLE_KEY)
    hyperElements.value = hyperInstance.value.elements({ clientSecret: secret })
    paymentElement.value = hyperElements.value.create('payment')

    // Give Vue a tick to render #payment-element before mounting
    await nextTick()
    paymentElement.value.mount('#payment-element')
  } catch (e) {
    sdkLoadError.value = e instanceof Error ? e.message : 'Failed to initialise payment form'
  }
})

// ---------------------------------------------------------------------------
// onMounted — fetch plans
// ---------------------------------------------------------------------------
onMounted(async () => {
  loadingPlans.value = true
  plansError.value = null
  try {
    plans.value = await getPlans()
  } catch (e) {
    plansError.value = e instanceof Error ? e.message : 'Failed to load plans'
  } finally {
    loadingPlans.value = false
  }
})

onUnmounted(() => {
  paymentElement.value?.unmount()
})

// ---------------------------------------------------------------------------
// Step: create subscription → get client_secret → init payment element
// ---------------------------------------------------------------------------
async function proceedToPayment() {
  creatingSubscription.value = true
  subscriptionError.value = null
  clientSecret.value = null
  paymentError.value = null

  try {
    const result = await createSubscription({
      plan_id: PRO_PLAN_ID,
      seat_count: seatCount.value,
      billing_cycle: billingCycle.value,
    })
    clientSecret.value = result.client_secret
  } catch (e) {
    subscriptionError.value = e instanceof Error ? e.message : 'Failed to create subscription'
  } finally {
    creatingSubscription.value = false
  }
}

// ---------------------------------------------------------------------------
// Submit payment
// ---------------------------------------------------------------------------
async function submitPayment() {
  if (!hyperElements.value) return

  paymentProcessing.value = true
  paymentError.value = null

  try {
    const { error } = await hyperElements.value.submit()
    if (error) {
      paymentError.value = error.message
      return
    }
    // Payment succeeded
    checkoutSuccess.value = true
    // Redirect to dashboard after brief moment for the success state to render
    setTimeout(() => {
      router.push({
        name: 'dashboard',
        query: { checkout: 'success' },
      })
    }, 1500)
  } catch (e) {
    paymentError.value = e instanceof Error ? e.message : 'Payment failed. Please try again.'
  } finally {
    paymentProcessing.value = false
  }
}

// ---------------------------------------------------------------------------
// Retry — reset payment element
// ---------------------------------------------------------------------------
function retryPayment() {
  paymentError.value = null
  sdkLoadError.value = null
  clientSecret.value = null
  paymentElement.value?.unmount()
  paymentElement.value = null
}
</script>

<template>
  <div class="p-4 sm:p-6 max-w-4xl mx-auto">
    <!-- Header -->
    <div class="mb-8">
      <RouterLink
        :to="`/orgs/${orgId}/billing`"
        class="mb-4 inline-flex items-center gap-1 text-sm text-slate-400 hover:text-slate-200 transition-colors"
        aria-label="Back to billing"
      >
        <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2" aria-hidden="true">
          <path stroke-linecap="round" stroke-linejoin="round" d="M15 19l-7-7 7-7" />
        </svg>
        Back to Billing
      </RouterLink>
      <h1 class="text-2xl font-bold text-white">Upgrade to Pro</h1>
      <p class="mt-1 text-sm text-slate-400">Configure your seats and billing cycle to get started.</p>
    </div>

    <!-- Plans loading / error -->
    <div v-if="loadingPlans" class="text-slate-400 text-sm">Loading plan details…</div>
    <div v-else-if="plansError" class="rounded-lg bg-red-900/40 border border-red-500/50 p-4 text-sm text-red-300">
      {{ plansError }}
    </div>

    <template v-else>
      <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <!-- Left: configuration + payment -->
        <div class="lg:col-span-2 space-y-6">

          <!-- Step 1: Seat selector -->
          <section class="rounded-xl bg-slate-800 p-5">
            <h2 class="text-base font-semibold text-white mb-4">
              <span class="mr-2 inline-flex h-6 w-6 items-center justify-center rounded-full bg-indigo-600 text-xs font-bold" aria-hidden="true">1</span>
              Number of Seats
            </h2>
            <div class="flex items-center gap-3">
              <button
                class="flex h-10 w-10 items-center justify-center rounded-lg border border-slate-600 text-slate-300 hover:bg-slate-700 transition-colors disabled:opacity-40"
                :disabled="seatCount <= MIN_SEATS"
                aria-label="Decrease seat count"
                @click="decrementSeats"
              >
                <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2" aria-hidden="true">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M20 12H4" />
                </svg>
              </button>
              <input
                :value="seatCount"
                type="number"
                :min="MIN_SEATS"
                class="w-20 rounded-lg border border-slate-600 bg-slate-700 px-3 py-2 text-center text-white text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                aria-label="Number of seats"
                @change="onSeatInput"
              />
              <button
                class="flex h-10 w-10 items-center justify-center rounded-lg border border-slate-600 text-slate-300 hover:bg-slate-700 transition-colors"
                aria-label="Increase seat count"
                @click="incrementSeats"
              >
                <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2" aria-hidden="true">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M12 4v16m8-8H4" />
                </svg>
              </button>
              <span class="text-sm text-slate-400">seats (minimum {{ MIN_SEATS }})</span>
            </div>
            <p class="mt-2 text-xs text-slate-500">
              ₹1,700 per seat / month &nbsp;·&nbsp; {{ formatINR(PRICE_PER_SEAT_PAISE) }} per seat/mo
            </p>
          </section>

          <!-- Step 2: Billing cycle -->
          <section class="rounded-xl bg-slate-800 p-5">
            <h2 class="text-base font-semibold text-white mb-4">
              <span class="mr-2 inline-flex h-6 w-6 items-center justify-center rounded-full bg-indigo-600 text-xs font-bold" aria-hidden="true">2</span>
              Billing Cycle
            </h2>
            <div class="flex gap-3" role="group" aria-label="Billing cycle">
              <button
                class="flex-1 rounded-lg border px-4 py-3 text-sm font-medium transition-colors focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 focus:ring-offset-slate-800"
                :class="billingCycle === 'monthly'
                  ? 'border-indigo-500 bg-indigo-600/20 text-white'
                  : 'border-slate-600 text-slate-300 hover:bg-slate-700'"
                :aria-pressed="billingCycle === 'monthly'"
                @click="billingCycle = 'monthly'"
              >
                Monthly
                <span class="block text-xs font-normal text-slate-400 mt-0.5">
                  {{ formatINR(monthlyTotalPaise) }}/mo
                </span>
              </button>
              <button
                class="flex-1 rounded-lg border px-4 py-3 text-sm font-medium transition-colors focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 focus:ring-offset-slate-800 relative"
                :class="billingCycle === 'annual'
                  ? 'border-indigo-500 bg-indigo-600/20 text-white'
                  : 'border-slate-600 text-slate-300 hover:bg-slate-700'"
                :aria-pressed="billingCycle === 'annual'"
                @click="billingCycle = 'annual'"
              >
                Annual
                <span class="block text-xs font-normal text-slate-400 mt-0.5">
                  {{ formatINR(annualTotalPaise) }}/year
                </span>
                <!-- Savings badge -->
                <span
                  class="absolute -top-2 -right-2 rounded-full bg-green-500 px-2 py-0.5 text-xs font-semibold text-white"
                  aria-label="Save approximately 17 percent with annual billing"
                >
                  Save ~17%
                </span>
              </button>
            </div>
            <p v-if="billingCycle === 'annual'" class="mt-3 text-xs text-green-400">
              {{ savingsLabel }}
            </p>
          </section>

          <!-- Step 3: Payment -->
          <section class="rounded-xl bg-slate-800 p-5">
            <h2 class="text-base font-semibold text-white mb-4">
              <span class="mr-2 inline-flex h-6 w-6 items-center justify-center rounded-full bg-indigo-600 text-xs font-bold" aria-hidden="true">3</span>
              Payment
            </h2>

            <!-- Not yet initiated payment -->
            <template v-if="!clientSecret && !creatingSubscription">
              <p v-if="subscriptionError" class="mb-3 rounded-lg bg-red-900/40 border border-red-500/50 px-4 py-3 text-sm text-red-300">
                {{ subscriptionError }}
              </p>
              <button
                class="min-h-[44px] w-full rounded-lg bg-indigo-600 px-6 py-3 text-sm font-semibold text-white hover:bg-indigo-500 transition-colors focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 focus:ring-offset-slate-800"
                @click="proceedToPayment"
              >
                Continue to Payment
              </button>
            </template>

            <!-- Creating subscription (loading) -->
            <div v-else-if="creatingSubscription" class="flex items-center gap-2 text-slate-400 text-sm">
              <svg class="h-4 w-4 animate-spin" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" aria-hidden="true">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8z" />
              </svg>
              Setting up payment…
            </div>

            <!-- Payment element -->
            <template v-else-if="clientSecret">
              <!-- SDK load error -->
              <div v-if="sdkLoadError" class="rounded-lg bg-red-900/40 border border-red-500/50 px-4 py-3 text-sm text-red-300 space-y-2">
                <p>{{ sdkLoadError }}</p>
                <button
                  class="underline text-red-300 hover:text-red-200 text-xs"
                  @click="retryPayment"
                >
                  Try again
                </button>
              </div>

              <template v-else>
                <!-- Hyperswitch payment element mount point -->
                <div
                  id="payment-element"
                  class="mb-4 min-h-[160px]"
                  role="region"
                  aria-label="Payment form"
                />

                <!-- Payment error -->
                <div
                  v-if="paymentError"
                  class="mb-4 rounded-lg bg-red-900/40 border border-red-500/50 px-4 py-3 text-sm text-red-300"
                  role="alert"
                >
                  <p>{{ paymentError }}</p>
                  <button
                    class="mt-1 underline text-red-300 hover:text-red-200 text-xs"
                    @click="retryPayment"
                  >
                    Change payment details
                  </button>
                </div>

                <!-- Success state -->
                <div
                  v-if="checkoutSuccess"
                  class="mb-4 rounded-lg bg-green-900/40 border border-green-500/50 px-4 py-3 text-sm text-green-300"
                  role="status"
                >
                  Payment successful! Redirecting to dashboard…
                </div>

                <!-- Submit button -->
                <button
                  v-else
                  class="min-h-[44px] w-full rounded-lg bg-indigo-600 px-6 py-3 text-sm font-semibold text-white hover:bg-indigo-500 transition-colors focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 focus:ring-offset-slate-800 disabled:opacity-60 disabled:cursor-not-allowed flex items-center justify-center gap-2"
                  :disabled="paymentProcessing || !sdkLoaded"
                  @click="submitPayment"
                >
                  <svg
                    v-if="paymentProcessing"
                    class="h-4 w-4 animate-spin"
                    xmlns="http://www.w3.org/2000/svg"
                    fill="none"
                    viewBox="0 0 24 24"
                    aria-hidden="true"
                  >
                    <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
                    <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8z" />
                  </svg>
                  {{ paymentProcessing ? 'Processing…' : `Pay ${displayPrice}` }}
                </button>
              </template>
            </template>
          </section>
        </div>

        <!-- Right: summary sidebar -->
        <aside class="rounded-xl bg-slate-800 p-5 h-fit sticky top-6" aria-label="Order summary">
          <h2 class="text-base font-semibold text-white mb-4">Order Summary</h2>

          <dl class="space-y-3 text-sm">
            <div class="flex justify-between">
              <dt class="text-slate-400">Plan</dt>
              <dd class="font-medium text-white">Pro</dd>
            </div>
            <div class="flex justify-between">
              <dt class="text-slate-400">Seats</dt>
              <dd class="font-medium text-white">{{ seatCount }}</dd>
            </div>
            <div class="flex justify-between">
              <dt class="text-slate-400">Per seat</dt>
              <dd class="font-medium text-white">{{ formatINR(PRICE_PER_SEAT_PAISE) }}/mo</dd>
            </div>
            <div class="flex justify-between">
              <dt class="text-slate-400">Cycle</dt>
              <dd class="font-medium text-white capitalize">{{ billingCycle }}</dd>
            </div>

            <div class="border-t border-slate-700 pt-3">
              <div class="flex justify-between items-baseline">
                <dt class="text-slate-300 font-medium">Total</dt>
                <dd class="text-lg font-bold text-white">
                  {{ displayPrice }}<span class="text-xs font-normal text-slate-400">{{ displayPeriod }}</span>
                </dd>
              </div>
              <p v-if="billingCycle === 'annual'" class="mt-1 text-xs text-green-400">
                {{ savingsLabel }}
              </p>
            </div>
          </dl>

          <p class="mt-4 text-xs text-slate-500">
            Billed in INR. Cancel anytime from your billing settings.
          </p>
        </aside>
      </div>
    </template>
  </div>
</template>
