<script setup lang="ts">
import { computed, nextTick, onUnmounted, ref, watch } from 'vue'
import {
  getPlans,
  updateSeatCount,
  type PaymentIntent,
  type Plan,
  type Subscription,
} from '../../api/billing'
import {
  getHyperswitchPublishableKey,
  loadHyperswitchSDK,
  type HyperElements,
  type HyperInstance,
  type HyperPaymentElement,
} from '../../composables/useHyperswitch'

// ---------------------------------------------------------------------------
// Props / emits
// ---------------------------------------------------------------------------
interface Props {
  subscription: Subscription
}
const props = defineProps<Props>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'success'): void
}>()

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------
const FALLBACK_MIN_SEATS = 5
const MOUNT_SELECTOR = '#payment-element-seats'

// ---------------------------------------------------------------------------
// Seat input state
// ---------------------------------------------------------------------------
const plans = ref<Plan[]>([])
const plansLoading = ref(false)
const plansError = ref<string | null>(null)

const currentSeatCount = computed(() => props.subscription.seat_count ?? 0)

const planMinSeats = computed(() => {
  const planId = props.subscription.plan
  const match = plans.value.find((p) => p.name === planId || p.id === planId)
  return match?.min_seats ?? FALLBACK_MIN_SEATS
})

// Minimum the user can request: must be strictly greater than the current
// seat count, and at least the plan's MinSeats.
const minRequestable = computed(() =>
  Math.max(currentSeatCount.value + 1, planMinSeats.value),
)

const seatCount = ref<number>(minRequestable.value)

watch(minRequestable, (v) => {
  if (seatCount.value < v) seatCount.value = v
})

function clampSeats(v: number): number {
  if (Number.isNaN(v)) return minRequestable.value
  return Math.max(minRequestable.value, Math.floor(v))
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

const seatInputError = computed<string | null>(() => {
  if (seatCount.value <= currentSeatCount.value) {
    return `Must be greater than your current ${currentSeatCount.value} seat${
      currentSeatCount.value === 1 ? '' : 's'
    }.`
  }
  if (seatCount.value < planMinSeats.value) {
    return `Plan requires at least ${planMinSeats.value} seats.`
  }
  return null
})

const canRequest = computed(() => seatInputError.value === null)

// ---------------------------------------------------------------------------
// API call: PATCH /seats → PaymentIntent
// ---------------------------------------------------------------------------
const requesting = ref(false)
const requestError = ref<string | null>(null)
const paymentIntent = ref<PaymentIntent | null>(null)

async function requestUpgrade() {
  if (!canRequest.value) return
  requesting.value = true
  requestError.value = null
  paymentError.value = null
  try {
    paymentIntent.value = await updateSeatCount(
      props.subscription.id,
      seatCount.value,
    )
  } catch (e) {
    requestError.value =
      e instanceof Error ? e.message : 'Failed to start seat upgrade'
  } finally {
    requesting.value = false
  }
}

// ---------------------------------------------------------------------------
// Hyperswitch payment element
// ---------------------------------------------------------------------------
const hyperInstance = ref<HyperInstance | null>(null)
const hyperElements = ref<HyperElements | null>(null)
const paymentElement = ref<HyperPaymentElement | null>(null)
const sdkLoaded = ref(false)
const sdkLoadError = ref<string | null>(null)
const paymentProcessing = ref(false)
const paymentError = ref<string | null>(null)
const paymentSuccess = ref(false)

watch(
  () => paymentIntent.value?.client_secret ?? null,
  async (secret) => {
    if (!secret) return
    paymentElement.value?.unmount()
    paymentElement.value = null
    sdkLoadError.value = null

    const key = getHyperswitchPublishableKey()
    if (!key) {
      sdkLoadError.value =
        'Hyperswitch publishable key is not configured. Set VITE_HYPERSWITCH_PUBLISHABLE_KEY in your environment.'
      return
    }

    try {
      await loadHyperswitchSDK()
      sdkLoaded.value = true
      if (!window.Hyper) throw new Error('Hyperswitch SDK not available after load')

      hyperInstance.value = window.Hyper(key)
      hyperElements.value = hyperInstance.value.elements({ clientSecret: secret })
      paymentElement.value = hyperElements.value.create('payment')

      await nextTick()
      paymentElement.value.mount(MOUNT_SELECTOR)
    } catch (e) {
      sdkLoadError.value =
        e instanceof Error ? e.message : 'Failed to initialise payment form'
    }
  },
)

async function submitPayment() {
  if (!hyperElements.value) return
  paymentProcessing.value = true
  paymentError.value = null
  try {
    const { error } = await hyperElements.value.submit()
    if (error) {
      // Inline error: do NOT unmount the form so the user can retry.
      paymentError.value = error.message
      return
    }
    paymentSuccess.value = true
    emit('success')
  } catch (e) {
    paymentError.value =
      e instanceof Error ? e.message : 'Payment failed. Please try again.'
  } finally {
    paymentProcessing.value = false
  }
}

function resetToSeatStep() {
  paymentElement.value?.unmount()
  paymentElement.value = null
  hyperInstance.value = null
  hyperElements.value = null
  sdkLoaded.value = false
  sdkLoadError.value = null
  paymentError.value = null
  paymentIntent.value = null
}

onUnmounted(() => {
  paymentElement.value?.unmount()
})

// ---------------------------------------------------------------------------
// Eagerly fetch plans so we know `min_seats` for client-side validation.
// Failure here is non-fatal: we fall back to FALLBACK_MIN_SEATS.
// ---------------------------------------------------------------------------
async function loadPlans() {
  plansLoading.value = true
  plansError.value = null
  try {
    plans.value = await getPlans()
  } catch (e) {
    plansError.value =
      e instanceof Error ? e.message : 'Failed to load plan limits'
  } finally {
    plansLoading.value = false
  }
}
loadPlans()

// ---------------------------------------------------------------------------
// Pricing display helpers
// ---------------------------------------------------------------------------
function formatINR(paise: number): string {
  const rupees = paise / 100
  return new Intl.NumberFormat('en-IN', {
    style: 'currency',
    currency: 'INR',
    maximumFractionDigits: 0,
  }).format(rupees)
}

const proratedDisplay = computed(() => {
  const pi = paymentIntent.value
  if (!pi) return null
  // Backend returns amount in minor units (paise for INR).
  if (pi.currency.toUpperCase() === 'INR') return formatINR(pi.amount)
  return `${(pi.amount / 100).toFixed(2)} ${pi.currency.toUpperCase()}`
})
</script>

<template>
  <div class="space-y-5">
    <header>
      <h3 id="seat-upgrade-title" class="text-lg font-bold text-white">
        Add more seats
      </h3>
      <p class="mt-1 text-sm text-slate-400">
        You'll be charged a prorated amount today for the rest of this billing
        period. Your seat count updates immediately on payment.
      </p>
    </header>

    <!-- Step 1: Seat selector (hidden once we have a clientSecret) -->
    <section v-if="!paymentIntent" aria-labelledby="seat-step-title">
      <h4 id="seat-step-title" class="sr-only">Choose new seat count</h4>

      <div class="mb-2 flex items-baseline justify-between">
        <span class="text-xs font-semibold uppercase tracking-wider text-slate-400">
          Current
        </span>
        <span class="text-sm text-slate-300">
          {{ currentSeatCount }} seat{{ currentSeatCount === 1 ? '' : 's' }}
        </span>
      </div>

      <label for="seat-upgrade-input" class="sr-only">New seat count</label>
      <div class="flex items-center gap-3">
        <button
          type="button"
          class="flex h-10 w-10 items-center justify-center rounded-lg border border-slate-600 text-slate-300 hover:bg-slate-700 transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
          :disabled="seatCount <= minRequestable"
          aria-label="Decrease new seat count"
          @click="decrementSeats"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2" aria-hidden="true">
            <path stroke-linecap="round" stroke-linejoin="round" d="M20 12H4" />
          </svg>
        </button>
        <input
          id="seat-upgrade-input"
          :value="seatCount"
          type="number"
          inputmode="numeric"
          :min="minRequestable"
          class="w-24 rounded-lg border border-slate-600 bg-slate-700 px-3 py-2 text-center text-sm text-white focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
          aria-label="New seat count"
          :aria-invalid="seatInputError !== null"
          aria-describedby="seat-upgrade-help seat-upgrade-error"
          @change="onSeatInput"
          @input="onSeatInput"
        />
        <button
          type="button"
          class="flex h-10 w-10 items-center justify-center rounded-lg border border-slate-600 text-slate-300 hover:bg-slate-700 transition-colors"
          aria-label="Increase new seat count"
          @click="incrementSeats"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2" aria-hidden="true">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 4v16m8-8H4" />
          </svg>
        </button>
        <span id="seat-upgrade-help" class="text-xs text-slate-400">
          Minimum {{ minRequestable }}
        </span>
      </div>

      <p
        id="seat-upgrade-error"
        class="mt-2 min-h-[1rem] text-xs"
        :class="seatInputError ? 'text-amber-400' : 'text-transparent'"
        role="status"
      >
        {{ seatInputError ?? '\u00A0' }}
      </p>

      <p v-if="plansError" class="mt-1 text-xs text-slate-500">
        Couldn't load plan limits; using safe defaults.
      </p>

      <!-- Request error -->
      <p
        v-if="requestError"
        class="mt-3 rounded-lg bg-red-900/40 border border-red-500/50 px-3 py-2 text-sm text-red-300"
        role="alert"
      >
        {{ requestError }}
      </p>

      <div class="mt-5 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
        <button
          type="button"
          class="min-h-[44px] rounded-lg border border-slate-600 px-4 text-sm font-medium text-slate-300 hover:bg-slate-700 transition-colors"
          @click="emit('close')"
        >
          Cancel
        </button>
        <button
          type="button"
          class="min-h-[44px] rounded-lg bg-indigo-600 px-6 text-sm font-semibold text-white hover:bg-indigo-500 transition-colors focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 focus:ring-offset-slate-800 disabled:opacity-60 disabled:cursor-not-allowed flex items-center justify-center gap-2"
          :disabled="!canRequest || requesting || plansLoading"
          aria-label="Continue to payment for additional seats"
          @click="requestUpgrade"
        >
          <svg
            v-if="requesting"
            class="h-4 w-4 animate-spin"
            xmlns="http://www.w3.org/2000/svg"
            fill="none"
            viewBox="0 0 24 24"
            aria-hidden="true"
          >
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8z" />
          </svg>
          {{ requesting ? 'Calculating…' : 'Continue to payment' }}
        </button>
      </div>
    </section>

    <!-- Step 2: Payment -->
    <section v-else aria-labelledby="payment-step-title" class="space-y-4">
      <h4 id="payment-step-title" class="sr-only">Complete payment</h4>

      <!-- Prorated amount summary -->
      <dl
        class="rounded-lg bg-slate-900/60 border border-slate-700 p-4 text-sm"
        aria-label="Prorated charge summary"
      >
        <div class="flex justify-between">
          <dt class="text-slate-400">New seat count</dt>
          <dd class="font-medium text-white">{{ seatCount }}</dd>
        </div>
        <div class="mt-2 flex justify-between border-t border-slate-700 pt-2">
          <dt class="text-slate-300 font-medium">Prorated charge today</dt>
          <dd class="text-base font-bold text-white">
            {{ proratedDisplay ?? '—' }}
          </dd>
        </div>
      </dl>

      <!-- SDK load error -->
      <div
        v-if="sdkLoadError"
        class="rounded-lg bg-red-900/40 border border-red-500/50 px-4 py-3 text-sm text-red-300 space-y-2"
        role="alert"
      >
        <p>{{ sdkLoadError }}</p>
        <button
          type="button"
          class="underline text-red-300 hover:text-red-200 text-xs"
          @click="resetToSeatStep"
        >
          Back to seat selection
        </button>
      </div>

      <template v-else>
        <!-- Hyperswitch mount point — distinct id so it doesn't collide with CheckoutPage -->
        <div
          id="payment-element-seats"
          class="min-h-[160px]"
          role="region"
          aria-label="Payment form for additional seats"
        />

        <!-- Inline payment error: form remains mounted so the user can retry -->
        <div
          v-if="paymentError"
          class="rounded-lg bg-red-900/40 border border-red-500/50 px-4 py-3 text-sm text-red-300"
          role="alert"
        >
          {{ paymentError }}
        </div>

        <div
          v-if="paymentSuccess"
          class="rounded-lg bg-green-900/40 border border-green-500/50 px-4 py-3 text-sm text-green-300"
          role="status"
        >
          Payment successful! Your seat count will update momentarily.
        </div>

        <div class="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <button
            type="button"
            class="min-h-[44px] rounded-lg border border-slate-600 px-4 text-sm font-medium text-slate-300 hover:bg-slate-700 transition-colors"
            :disabled="paymentProcessing"
            @click="resetToSeatStep"
          >
            Change seats
          </button>
          <button
            type="button"
            class="min-h-[44px] rounded-lg bg-indigo-600 px-6 text-sm font-semibold text-white hover:bg-indigo-500 transition-colors focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 focus:ring-offset-slate-800 disabled:opacity-60 disabled:cursor-not-allowed flex items-center justify-center gap-2"
            :disabled="paymentProcessing || paymentSuccess || !sdkLoaded"
            :aria-label="`Pay ${proratedDisplay ?? ''} for additional seats`"
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
            {{ paymentProcessing ? 'Processing…' : `Pay ${proratedDisplay ?? ''}` }}
          </button>
        </div>
      </template>
    </section>
  </div>
</template>
