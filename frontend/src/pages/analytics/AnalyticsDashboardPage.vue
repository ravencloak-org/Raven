<script setup lang="ts">
import { onMounted, computed } from 'vue'
import { useAnalyticsStore } from '../../stores/analytics'
import type { DateRange } from '../../api/analytics'
import { map, firstBy, sumBy } from 'remeda'

const store = useAnalyticsStore()

onMounted(() => {
  store.fetchAll()
})

const rangeOptions: { value: DateRange; label: string }[] = [
  { value: '7d', label: 'Last 7 days' },
  { value: '30d', label: 'Last 30 days' },
  { value: '90d', label: 'Last 90 days' },
]

function onRangeChange(event: Event) {
  const value = (event.target as HTMLSelectElement).value as DateRange
  store.changeRange(value)
}

const maxVolume = computed(() => {
  if (store.conversationVolume.length === 0) return 1
  const top = firstBy(store.conversationVolume, [(p) => p.count, 'desc'])
  return top?.count ?? 1
})

const totalConversations = computed(() =>
  sumBy(store.conversationVolume, (p) => p.count),
)

const maxSourceHitCount = computed(() => {
  if (store.sourceHits.length === 0) return 1
  const top = firstBy(store.sourceHits, [(s) => s.hitCount, 'desc'])
  return top?.hitCount ?? 1
})

function formatDate(iso: string): string {
  const d = new Date(iso)
  return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' })
}

function formatDateTime(iso: string): string {
  const d = new Date(iso)
  return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' })
}

/** Show abbreviated labels on the volume chart x-axis. */
const volumeLabels = computed(() => {
  const pts = store.conversationVolume
  if (pts.length <= 14) return map(pts, (p) => formatDate(p.date))
  // For 30d/90d, show every Nth label
  const step = Math.ceil(pts.length / 12)
  return map(pts, (p, i) => (i % step === 0 ? formatDate(p.date) : ''))
})
</script>

<template>
  <div class="space-y-8">
    <!-- Header -->
    <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
      <div>
        <h1 class="text-2xl font-bold text-gray-900">Analytics</h1>
        <p class="mt-1 text-sm text-gray-500">Conversation insights and knowledge base performance</p>
      </div>
      <select
        :value="store.selectedRange"
        class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 shadow-sm focus:border-indigo-500 focus:ring-2 focus:ring-indigo-500 focus:outline-none"
        @change="onRangeChange"
      >
        <option v-for="opt in rangeOptions" :key="opt.value" :value="opt.value">
          {{ opt.label }}
        </option>
      </select>
    </div>

    <!-- Loading state -->
    <div v-if="store.loading" class="flex items-center justify-center py-20">
      <div class="h-8 w-8 animate-spin rounded-full border-4 border-indigo-200 border-t-indigo-600"></div>
      <span class="ml-3 text-sm text-gray-500">Loading analytics...</span>
    </div>

    <!-- Error state -->
    <div
      v-else-if="store.error"
      class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700"
    >
      Failed to load analytics: {{ store.error }}
    </div>

    <!-- Dashboard content -->
    <template v-else>
      <!-- Summary cards -->
      <div class="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-4">
        <div class="rounded-xl border border-gray-200 bg-white p-6 shadow-sm">
          <h3 class="text-sm font-semibold text-gray-500">Total Conversations</h3>
          <p class="mt-2 text-3xl font-bold text-gray-900">{{ totalConversations.toLocaleString() }}</p>
        </div>
        <div class="rounded-xl border border-gray-200 bg-white p-6 shadow-sm">
          <h3 class="text-sm font-semibold text-gray-500">Unique Queries</h3>
          <p class="mt-2 text-3xl font-bold text-gray-900">{{ store.topQueries.length }}</p>
        </div>
        <div class="rounded-xl border border-gray-200 bg-white p-6 shadow-sm">
          <h3 class="text-sm font-semibold text-gray-500">Sources Used</h3>
          <p class="mt-2 text-3xl font-bold text-gray-900">{{ store.sourceHits.length }}</p>
        </div>
        <div class="rounded-xl border border-gray-200 bg-white p-6 shadow-sm">
          <h3 class="text-sm font-semibold text-gray-500">Unanswered</h3>
          <p class="mt-2 text-3xl font-bold text-indigo-600">{{ store.unansweredQuestions.length }}</p>
        </div>
      </div>

      <!-- Conversation Volume Chart -->
      <div class="rounded-xl border border-gray-200 bg-white p-6 shadow-sm">
        <h2 class="mb-4 text-lg font-semibold text-gray-900">Conversation Volume</h2>
        <div class="flex items-end gap-px" style="height: 200px">
          <div
            v-for="(point, idx) in store.conversationVolume"
            :key="point.date"
            class="group relative flex flex-1 flex-col items-center justify-end"
          >
            <!-- Tooltip -->
            <div
              class="pointer-events-none absolute -top-10 z-10 hidden rounded bg-gray-800 px-2 py-1 text-xs whitespace-nowrap text-white group-hover:block"
            >
              {{ formatDate(point.date) }}: {{ point.count }}
            </div>
            <!-- Bar -->
            <div
              class="w-full min-w-[2px] rounded-t bg-indigo-500 transition-all hover:bg-indigo-400"
              :style="{ height: (point.count / maxVolume) * 100 + '%' }"
            ></div>
            <!-- X-axis label -->
            <span
              v-if="volumeLabels[idx]"
              class="mt-2 block text-[10px] text-gray-400"
            >
              {{ volumeLabels[idx] }}
            </span>
          </div>
        </div>
      </div>

      <!-- Two-column: Top Queries + Source Hits -->
      <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <!-- Top Queries -->
        <div class="rounded-xl border border-gray-200 bg-white p-6 shadow-sm">
          <h2 class="mb-4 text-lg font-semibold text-gray-900">Top Queries</h2>
          <div class="space-y-3">
            <div
              v-for="q in store.topQueries"
              :key="q.query"
              class="space-y-1"
            >
              <div class="flex items-center justify-between text-sm">
                <span class="truncate pr-4 text-gray-700">{{ q.query }}</span>
                <span class="shrink-0 font-medium text-gray-900">{{ q.count }}</span>
              </div>
              <div class="h-2 w-full overflow-hidden rounded-full bg-gray-100">
                <div
                  class="h-full rounded-full bg-indigo-500"
                  :style="{ width: (q.count / (store.topQueries[0]?.count || 1)) * 100 + '%' }"
                ></div>
              </div>
            </div>
          </div>
        </div>

        <!-- Source Hit Frequency -->
        <div class="rounded-xl border border-gray-200 bg-white p-6 shadow-sm">
          <h2 class="mb-4 text-lg font-semibold text-gray-900">Source Hit Frequency</h2>
          <div class="space-y-3">
            <div
              v-for="src in store.sourceHits"
              :key="src.sourceId"
              class="space-y-1"
            >
              <div class="flex items-center justify-between text-sm">
                <span class="truncate pr-4 text-gray-700">{{ src.sourceName }}</span>
                <span class="shrink-0 font-medium text-gray-900">{{ src.hitCount }}</span>
              </div>
              <div class="h-2 w-full overflow-hidden rounded-full bg-gray-100">
                <div
                  class="h-full rounded-full bg-emerald-500"
                  :style="{ width: (src.hitCount / maxSourceHitCount) * 100 + '%' }"
                ></div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Unanswered Questions -->
      <div class="rounded-xl border border-gray-200 bg-white p-6 shadow-sm">
        <h2 class="mb-4 text-lg font-semibold text-gray-900">Unanswered Questions</h2>
        <div v-if="store.unansweredQuestions.length === 0" class="py-8 text-center text-sm text-gray-400">
          No unanswered questions in this period.
        </div>
        <div v-else class="divide-y divide-gray-100">
          <div
            v-for="uq in store.unansweredQuestions"
            :key="uq.id"
            class="flex items-start justify-between gap-4 py-3"
          >
            <div class="min-w-0 flex-1">
              <p class="text-sm font-medium text-gray-900">{{ uq.question }}</p>
              <p class="mt-0.5 text-xs text-gray-400">Asked {{ formatDateTime(uq.askedAt) }}</p>
            </div>
            <span
              class="shrink-0 rounded-full bg-amber-100 px-2.5 py-0.5 text-xs font-medium text-amber-800"
            >
              Needs answer
            </span>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>
