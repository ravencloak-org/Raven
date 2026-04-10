<template>
  <div class="space-y-1">
    <div class="flex items-center justify-between text-sm">
      <span class="font-medium text-gray-700">{{ label }}</span>
      <span class="text-gray-500">
        <template v-if="limit === -1">
          {{ used.toLocaleString() }} {{ unit }} / Unlimited
        </template>
        <template v-else>
          {{ used.toLocaleString() }} / {{ limit.toLocaleString() }} {{ unit }}
        </template>
      </span>
    </div>

    <div class="h-2 w-full overflow-hidden rounded-full bg-gray-200">
      <div
        v-if="limit === -1"
        class="h-full w-full rounded-full bg-gray-400"
      />
      <div
        v-else
        class="h-full rounded-full transition-all duration-300"
        :class="barColor"
        :style="{ width: `${Math.min(percentage, 100)}%` }"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    label: string
    used: number
    limit: number
    unit?: string
  }>(),
  {
    unit: '',
  },
)

const percentage = computed<number>(() => {
  if (props.limit <= 0) return 0
  return (props.used / props.limit) * 100
})

const barColor = computed<string>(() => {
  const pct = percentage.value
  if (pct >= 90) return 'bg-red-500'
  if (pct >= 70) return 'bg-yellow-500'
  return 'bg-green-500'
})
</script>
