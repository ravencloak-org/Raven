<template>
  <div
    class="relative flex flex-col rounded-xl border bg-white p-6 shadow-sm transition-shadow hover:shadow-md"
    :class="isCurrent ? 'border-indigo-500 ring-2 ring-indigo-500' : 'border-gray-200'"
  >
    <!-- Current plan badge -->
    <div v-if="isCurrent" class="absolute -top-3 left-1/2 -translate-x-1/2">
      <span class="rounded-full bg-indigo-600 px-3 py-1 text-xs font-semibold text-white">
        Current plan
      </span>
    </div>

    <!-- Plan name -->
    <h3 class="text-lg font-bold text-gray-900">{{ plan.name }}</h3>

    <!-- Price -->
    <div class="mt-2 flex items-baseline gap-1">
      <span class="text-3xl font-extrabold text-gray-900">
        {{ plan.price_monthly === 0 ? 'Free' : `${plan.currency} ${plan.price_monthly.toLocaleString()}` }}
      </span>
      <span v-if="plan.price_monthly > 0" class="text-sm text-gray-500">/month</span>
    </div>

    <!-- Feature list -->
    <ul class="mt-4 flex-1 space-y-2">
      <li class="flex items-center gap-2 text-sm text-gray-600">
        <CheckIcon class="h-4 w-4 shrink-0 text-green-500" />
        <span>{{ formatLimit(plan.max_users) }} users</span>
      </li>
      <li class="flex items-center gap-2 text-sm text-gray-600">
        <CheckIcon class="h-4 w-4 shrink-0 text-green-500" />
        <span>{{ formatLimit(plan.max_workspaces) }} workspaces</span>
      </li>
      <li class="flex items-center gap-2 text-sm text-gray-600">
        <CheckIcon class="h-4 w-4 shrink-0 text-green-500" />
        <span>{{ formatLimit(plan.max_knowledge_bases) }} knowledge bases</span>
      </li>
      <li class="flex items-center gap-2 text-sm text-gray-600">
        <CheckIcon class="h-4 w-4 shrink-0 text-green-500" />
        <span>{{ formatStorageLimit(plan.max_storage_gb) }} storage</span>
      </li>
      <li class="flex items-center gap-2 text-sm text-gray-600">
        <CheckIcon class="h-4 w-4 shrink-0 text-green-500" />
        <span>{{ formatLimit(plan.max_voice_sessions) }} voice sessions</span>
      </li>
      <li class="flex items-center gap-2 text-sm text-gray-600">
        <CheckIcon class="h-4 w-4 shrink-0 text-green-500" />
        <span>{{ formatLimit(plan.max_voice_minutes) }} voice minutes</span>
      </li>
    </ul>

    <!-- CTA -->
    <div class="mt-6">
      <button
        v-if="isCurrent"
        disabled
        class="w-full cursor-default rounded-lg bg-gray-100 px-4 py-2 text-sm font-medium text-gray-500"
      >
        Current plan
      </button>
      <button
        v-else
        class="flex w-full items-center justify-center rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-indigo-700 disabled:opacity-60"
        :disabled="loading"
        @click="emit('select', plan.id)"
      >
        <svg
          v-if="loading"
          class="mr-2 h-4 w-4 animate-spin"
          xmlns="http://www.w3.org/2000/svg"
          fill="none"
          viewBox="0 0 24 24"
        >
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z" />
        </svg>
        Upgrade to {{ plan.name }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { Plan } from '../api/billing'

// Inline check icon to avoid external icon dependencies
const CheckIcon = {
  template: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor">
    <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd" />
  </svg>`,
}

defineProps<{
  plan: Plan
  isCurrent: boolean
  loading?: boolean
}>()

const emit = defineEmits<{
  select: [planId: string]
}>()

function formatLimit(value: number): string {
  return value === -1 ? 'Unlimited' : value.toLocaleString()
}

function formatStorageLimit(gb: number): string {
  if (gb === -1) return 'Unlimited'
  if (gb >= 1024) return `${(gb / 1024).toFixed(0)} TB`
  return `${gb} GB`
}
</script>
