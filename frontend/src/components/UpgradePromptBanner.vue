<template>
  <Transition name="banner">
    <div
      v-if="visible"
      class="flex items-center justify-between gap-4 bg-amber-50 px-4 py-3 text-amber-900 shadow-sm border-b border-amber-200"
      role="alert"
    >
      <div class="flex items-center gap-2 min-w-0">
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-5 w-5 shrink-0 text-amber-500"
          viewBox="0 0 20 20"
          fill="currentColor"
        >
          <path
            fill-rule="evenodd"
            d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z"
            clip-rule="evenodd"
          />
        </svg>
        <p class="truncate text-sm font-medium">
          {{ billing.quotaMessage ?? 'You have reached your plan limit.' }}
        </p>
      </div>

      <div class="flex shrink-0 items-center gap-2">
        <RouterLink
          to="/billing"
          class="rounded-md bg-amber-600 px-3 py-1.5 text-xs font-semibold text-white transition-colors hover:bg-amber-700"
        >
          Upgrade Plan
        </RouterLink>
        <button
          class="flex h-6 w-6 items-center justify-center rounded text-amber-700 transition-colors hover:bg-amber-100"
          title="Dismiss"
          @click="dismiss"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" viewBox="0 0 20 20" fill="currentColor">
            <path
              fill-rule="evenodd"
              d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z"
              clip-rule="evenodd"
            />
          </svg>
        </button>
      </div>
    </div>
  </Transition>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import { useBillingStore } from '../stores/billing'
import { useFeatureFlag } from '../composables/useFeatureFlag'

const billing = useBillingStore()
const { isEnabled: billingEnabled } = useFeatureFlag('billing_enabled')

const visible = computed(() => billingEnabled.value && billing.quotaExceeded)

function dismiss(): void {
  billing.clearQuotaExceeded()
}
</script>

<style scoped>
.banner-enter-active,
.banner-leave-active {
  transition:
    opacity 0.25s ease,
    max-height 0.25s ease;
  max-height: 80px;
  overflow: hidden;
}

.banner-enter-from,
.banner-leave-to {
  opacity: 0;
  max-height: 0;
}
</style>
