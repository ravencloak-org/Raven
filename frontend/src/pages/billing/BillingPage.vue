<script setup lang="ts">
import { onMounted, onUnmounted, computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useBillingStore } from '../../stores/billing'
import { useMobile } from '../../composables/useMediaQuery'
import BottomSheet from '../../components/BottomSheet.vue'

const route = useRoute()
const store = useBillingStore()
const { isMobile } = useMobile()

const orgId = route.params.orgId as string

const showCancelConfirm = ref(false)

onMounted(async () => {
  await Promise.all([store.fetchUsage(orgId), store.fetchSubscription(orgId)])
  store.startPolling(orgId)
})

onUnmounted(() => {
  store.stopPolling()
})

function usagePercent(current: number, max: number): number {
  if (max <= 0) return 0
  return Math.min(100, Math.round((current / max) * 100))
}

function barColor(current: number, max: number): string {
  const pct = usagePercent(current, max)
  return pct > 80 ? 'bg-amber-500' : 'bg-indigo-500'
}

function textColor(current: number, max: number): string {
  const pct = usagePercent(current, max)
  return pct > 80 ? 'text-amber-400' : 'text-slate-300'
}

const planLabel = computed(() => {
  const plan = store.subscription?.plan
  if (!plan) return 'Free'
  return plan.charAt(0).toUpperCase() + plan.slice(1)
})

const renewalDate = computed(() => {
  const end = store.subscription?.current_period_end
  if (!end) return null
  return new Date(end).toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
  })
})

function handleCancelRequest() {
  showCancelConfirm.value = true
}

function handleCancelConfirmed() {
  // Placeholder: wire to POST /billing/cancel once backend endpoint exists
  showCancelConfirm.value = false
}
</script>

<template>
  <div class="p-4 sm:p-6 max-w-3xl mx-auto">
    <h1 class="text-2xl font-bold text-white mb-6">Billing &amp; Subscription</h1>

    <!-- Loading -->
    <div v-if="store.loading && !store.usage" class="text-slate-400">Loading...</div>

    <!-- Error -->
    <div v-else-if="store.error && !store.usage" class="text-red-400">{{ store.error }}</div>

    <template v-else>
      <!-- Plan card -->
      <div class="rounded-xl bg-slate-800 p-5 mb-6">
        <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
          <div>
            <p class="text-xs font-semibold uppercase tracking-wider text-slate-400 mb-1">
              Current Plan
            </p>
            <p class="text-2xl font-bold text-white">{{ planLabel }}</p>
            <p v-if="renewalDate" class="text-sm text-slate-400 mt-1">
              Renews {{ renewalDate }}
            </p>
          </div>

          <RouterLink
            :to="`/orgs/${orgId}/billing/upgrade`"
            class="inline-flex items-center justify-center min-h-[44px] rounded-lg bg-indigo-600 px-6 text-sm font-semibold text-white hover:bg-indigo-500 transition-colors"
          >
            Upgrade Plan
          </RouterLink>
        </div>
      </div>

      <!-- Usage section -->
      <div v-if="store.usage" class="rounded-xl bg-slate-800 p-5 mb-6">
        <h2 class="text-base font-semibold text-white mb-4">Usage this period</h2>

        <div class="space-y-5">
          <!-- Knowledge Base storage -->
          <div>
            <div class="flex justify-between items-baseline mb-1">
              <span class="text-sm font-medium text-slate-300">Knowledge Base Storage</span>
              <span
                class="text-xs font-medium"
                :class="textColor(store.usage.knowledge_base_kbs.current, store.usage.knowledge_base_kbs.max)"
              >
                {{ store.usage.knowledge_base_kbs.current.toLocaleString() }} /
                {{ store.usage.knowledge_base_kbs.max.toLocaleString() }} KB
              </span>
            </div>
            <div class="h-2 w-full rounded-full bg-slate-700 overflow-hidden">
              <div
                class="h-full rounded-full transition-all duration-500"
                :class="barColor(store.usage.knowledge_base_kbs.current, store.usage.knowledge_base_kbs.max)"
                :style="{ width: `${usagePercent(store.usage.knowledge_base_kbs.current, store.usage.knowledge_base_kbs.max)}%` }"
              />
            </div>
          </div>

          <!-- Seats -->
          <div>
            <div class="flex justify-between items-baseline mb-1">
              <span class="text-sm font-medium text-slate-300">Seats</span>
              <span
                class="text-xs font-medium"
                :class="textColor(store.usage.seats.current, store.usage.seats.max)"
              >
                {{ store.usage.seats.current }} / {{ store.usage.seats.max }}
              </span>
            </div>
            <div class="h-2 w-full rounded-full bg-slate-700 overflow-hidden">
              <div
                class="h-full rounded-full transition-all duration-500"
                :class="barColor(store.usage.seats.current, store.usage.seats.max)"
                :style="{ width: `${usagePercent(store.usage.seats.current, store.usage.seats.max)}%` }"
              />
            </div>
          </div>

          <!-- Voice minutes -->
          <div>
            <div class="flex justify-between items-baseline mb-1">
              <span class="text-sm font-medium text-slate-300">Voice Minutes</span>
              <span
                class="text-xs font-medium"
                :class="textColor(store.usage.voice_minutes.current, store.usage.voice_minutes.max)"
              >
                {{ store.usage.voice_minutes.current.toLocaleString() }} /
                {{ store.usage.voice_minutes.max.toLocaleString() }} min
              </span>
            </div>
            <div class="h-2 w-full rounded-full bg-slate-700 overflow-hidden">
              <div
                class="h-full rounded-full transition-all duration-500"
                :class="barColor(store.usage.voice_minutes.current, store.usage.voice_minutes.max)"
                :style="{ width: `${usagePercent(store.usage.voice_minutes.current, store.usage.voice_minutes.max)}%` }"
              />
            </div>
          </div>

          <!-- API calls -->
          <div>
            <div class="flex justify-between items-baseline mb-1">
              <span class="text-sm font-medium text-slate-300">API Calls</span>
              <span
                class="text-xs font-medium"
                :class="textColor(store.usage.api_calls.current, store.usage.api_calls.max)"
              >
                {{ store.usage.api_calls.current.toLocaleString() }} /
                {{ store.usage.api_calls.max.toLocaleString() }}
              </span>
            </div>
            <div class="h-2 w-full rounded-full bg-slate-700 overflow-hidden">
              <div
                class="h-full rounded-full transition-all duration-500"
                :class="barColor(store.usage.api_calls.current, store.usage.api_calls.max)"
                :style="{ width: `${usagePercent(store.usage.api_calls.current, store.usage.api_calls.max)}%` }"
              />
            </div>
          </div>
        </div>
      </div>

      <!-- Cancel subscription -->
      <div v-if="store.subscription && store.subscription.plan !== 'free'" class="mt-2">
        <button
          class="min-h-[44px] rounded-lg border border-red-500/50 px-4 py-2 text-sm font-medium text-red-400 hover:bg-red-500/10 transition-colors"
          @click="handleCancelRequest"
        >
          Cancel Subscription
        </button>
      </div>
    </template>

    <!-- Mobile: bottom-sheet cancel confirm -->
    <BottomSheet
      v-if="isMobile"
      :open="showCancelConfirm"
      title="Cancel Subscription"
      @close="showCancelConfirm = false"
    >
      <div class="px-4 pb-4 pt-2 space-y-4">
        <p class="text-sm text-slate-300">
          Are you sure you want to cancel your subscription? Your plan will revert to Free at the
          end of the billing period.
        </p>
        <div class="flex flex-col gap-2">
          <button
            class="w-full min-h-[48px] rounded-xl bg-red-600 text-sm font-semibold text-white hover:bg-red-500"
            @click="handleCancelConfirmed"
          >
            Yes, cancel subscription
          </button>
          <button
            class="w-full min-h-[48px] rounded-xl bg-slate-700 text-sm text-slate-200"
            @click="showCancelConfirm = false"
          >
            Keep subscription
          </button>
        </div>
      </div>
    </BottomSheet>

    <!-- Desktop: centered dialog cancel confirm -->
    <Teleport v-else to="body">
      <div
        v-if="showCancelConfirm"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm"
        @click.self="showCancelConfirm = false"
        @keydown.escape="showCancelConfirm = false"
      >
        <div class="w-full max-w-sm rounded-xl bg-slate-800 mx-4 p-6 space-y-4">
          <h3 class="text-lg font-bold text-white">Cancel Subscription</h3>
          <p class="text-sm text-slate-300">
            Are you sure you want to cancel your subscription? Your plan will revert to Free at the
            end of the billing period.
          </p>
          <div class="flex gap-3 pt-2">
            <button
              class="flex-1 min-h-[44px] rounded-lg border border-slate-600 text-sm font-medium text-slate-300 hover:bg-slate-700"
              @click="showCancelConfirm = false"
            >
              Keep subscription
            </button>
            <button
              class="flex-1 min-h-[44px] rounded-lg bg-red-600 text-sm font-semibold text-white hover:bg-red-500"
              @click="handleCancelConfirmed"
            >
              Cancel
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
