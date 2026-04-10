<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useMobile } from '../composables/useMediaQuery'
import ResponsiveModal from './ResponsiveModal.vue'

const props = defineProps<{
  open: boolean
  feature?: string
}>()

const emit = defineEmits<{
  close: []
}>()

const route = useRoute()
const { isMobile } = useMobile()

const orgId = computed(() => route.params.orgId as string | undefined)

const billingPath = computed(() =>
  orgId.value ? `/orgs/${orgId.value}/billing` : '/dashboard',
)

const featureMessage = computed(() =>
  props.feature
    ? `You've reached the limit for "${props.feature}".`
    : "You've reached a plan limit.",
)
</script>

<template>
  <ResponsiveModal :open="open" title="Upgrade Required" @close="emit('close')">
    <div class="p-6 space-y-4">
      <!-- Desktop close button -->
      <div class="hidden sm:flex items-center justify-between mb-2">
        <h2 class="text-lg font-bold text-white">Upgrade Required</h2>
        <button
          class="flex h-11 w-11 items-center justify-center rounded-lg text-slate-400 hover:bg-slate-700 hover:text-slate-200"
          aria-label="Close"
          @click="emit('close')"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-5 w-5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            stroke-width="2"
          >
            <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>

      <!-- Icon -->
      <div class="flex justify-center">
        <div class="flex h-14 w-14 items-center justify-center rounded-full bg-amber-500/20">
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-7 w-7 text-amber-400"
            viewBox="0 0 20 20"
            fill="currentColor"
          >
            <path
              fill-rule="evenodd"
              d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z"
              clip-rule="evenodd"
            />
          </svg>
        </div>
      </div>

      <p class="text-center text-sm text-slate-300">{{ featureMessage }}</p>
      <p class="text-center text-sm text-slate-400">
        Upgrade your plan to unlock higher limits and additional features.
      </p>

      <div :class="isMobile ? 'flex flex-col gap-2 pt-2' : 'hidden sm:flex gap-3 pt-2'">
        <RouterLink
          :to="billingPath"
          class="flex flex-1 items-center justify-center min-h-[44px] rounded-lg bg-indigo-600 px-4 text-sm font-semibold text-white hover:bg-indigo-500 transition-colors"
          @click="emit('close')"
        >
          View Billing
        </RouterLink>
        <button
          class="flex-1 min-h-[44px] rounded-lg border border-slate-600 text-sm font-medium text-slate-300 hover:bg-slate-700"
          @click="emit('close')"
        >
          Dismiss
        </button>
      </div>
    </div>

    <template #actions>
      <div class="flex flex-col gap-2 sm:hidden">
        <RouterLink
          :to="billingPath"
          class="flex items-center justify-center w-full min-h-[48px] rounded-xl bg-indigo-600 text-sm font-semibold text-white"
          @click="emit('close')"
        >
          View Billing
        </RouterLink>
        <button
          class="w-full min-h-[48px] rounded-xl bg-slate-700 text-sm text-slate-200"
          @click="emit('close')"
        >
          Dismiss
        </button>
      </div>
    </template>
  </ResponsiveModal>
</template>
