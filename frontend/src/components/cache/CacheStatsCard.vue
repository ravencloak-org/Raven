<script setup lang="ts">
/**
 * CacheStatsCard — displays semantic-cache metrics and exposes the operator
 * controls described in issue #256:
 *
 *   * Toggle: enable / disable the cache per KB.
 *   * Slider: cosine-similarity threshold (0.80 – 0.99).
 *   * Button: flush every cached answer for this KB.
 *
 * Props: `orgId`, `kbId`, `wsId` and the current `KnowledgeBase` row.
 * Emits: `updated` with the refreshed KB when the toggle/slider is saved.
 *
 * Everything is optimistic-light: we let the parent own the KB state and
 * only emit on successful save, so cancelling a slider drag cannot leak
 * stale values into the store.
 */
import { computed, ref, watch } from 'vue'
import {
  flushCache,
  getCacheStats,
  updateKnowledgeBase,
  type CacheStats,
  type KnowledgeBase,
} from '../../api/knowledge-bases'

const props = defineProps<{
  orgId: string
  wsId: string
  kbId: string
  kb: KnowledgeBase
}>()

const emit = defineEmits<{
  (e: 'updated', kb: KnowledgeBase): void
}>()

const stats = ref<CacheStats | null>(null)
const loading = ref(false)
const saving = ref(false)
const flushing = ref(false)
const error = ref<string | null>(null)

// Local drafts for the toggle and slider so we can debounce and validate.
const cacheEnabled = ref(props.kb.cache_enabled)
const threshold = ref(props.kb.cache_similarity_threshold)

watch(
  () => [props.kb.cache_enabled, props.kb.cache_similarity_threshold],
  ([enabled, thr]) => {
    cacheEnabled.value = enabled as boolean
    threshold.value = thr as number
  },
)

// Estimated hit rate: hits over (hits + entries), where entries is a proxy
// for misses on the assumption each entry was inserted on a miss. A precise
// rate will land in #256 follow-up when we record hit+miss counters directly.
const hitRate = computed(() => {
  if (!stats.value) return null
  const hits = stats.value.hit_count
  const entries = stats.value.total_entries
  const total = hits + entries
  return total === 0 ? 0 : Math.round((hits / total) * 100)
})

async function loadStats() {
  loading.value = true
  error.value = null
  try {
    stats.value = await getCacheStats(props.orgId, props.kbId)
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    loading.value = false
  }
}

async function saveSettings() {
  saving.value = true
  error.value = null
  try {
    const updated = await updateKnowledgeBase(props.orgId, props.wsId, props.kbId, {
      cache_enabled: cacheEnabled.value,
      cache_similarity_threshold: threshold.value,
    })
    emit('updated', updated)
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    saving.value = false
  }
}

async function onFlush() {
  const ok = window.confirm(
    'Flush every cached answer for this knowledge base? This cannot be undone.',
  )
  if (!ok) return
  flushing.value = true
  error.value = null
  try {
    await flushCache(props.orgId, props.kbId)
    await loadStats()
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    flushing.value = false
  }
}

// Initial load on mount.
void loadStats()
</script>

<template>
  <section
    class="cache-stats-card rounded-lg border border-gray-200 bg-white p-5 shadow-sm"
    aria-label="Semantic response cache"
  >
    <header class="mb-4 flex items-center justify-between">
      <h3 class="text-base font-semibold text-gray-900">Semantic cache</h3>
      <button
        class="text-xs text-blue-600 hover:underline disabled:text-gray-400"
        :disabled="loading"
        @click="loadStats"
      >
        Refresh
      </button>
    </header>

    <div v-if="error" class="mb-3 rounded bg-red-50 px-3 py-2 text-sm text-red-700">
      {{ error }}
    </div>

    <dl class="mb-4 grid grid-cols-3 gap-3 text-sm">
      <div>
        <dt class="text-gray-500">Entries</dt>
        <dd class="font-mono text-lg">{{ stats?.total_entries ?? '—' }}</dd>
      </div>
      <div>
        <dt class="text-gray-500">Hits</dt>
        <dd class="font-mono text-lg">{{ stats?.hit_count ?? '—' }}</dd>
      </div>
      <div>
        <dt class="text-gray-500">Hit rate</dt>
        <dd class="font-mono text-lg">
          {{ hitRate === null ? '—' : `${hitRate}%` }}
        </dd>
      </div>
      <div class="col-span-3">
        <dt class="text-gray-500">Estimated tokens saved</dt>
        <dd class="font-mono text-lg">
          {{ stats?.estimated_tokens_saved?.toLocaleString() ?? '—' }}
        </dd>
      </div>
    </dl>

    <fieldset class="mb-4 space-y-3">
      <label class="flex items-center justify-between gap-3">
        <span class="text-sm font-medium">Cache enabled</span>
        <input
          v-model="cacheEnabled"
          type="checkbox"
          class="h-4 w-4"
          :disabled="saving"
        />
      </label>

      <label class="block">
        <span class="mb-1 flex items-center justify-between text-sm font-medium">
          Similarity threshold
          <span class="font-mono text-xs text-gray-500">
            {{ threshold.toFixed(2) }}
          </span>
        </span>
        <input
          v-model.number="threshold"
          type="range"
          min="0.80"
          max="0.99"
          step="0.01"
          :disabled="saving || !cacheEnabled"
          class="w-full"
        />
        <span class="mt-1 block text-xs text-gray-500">
          Stricter = fewer hits, higher accuracy
        </span>
      </label>
    </fieldset>

    <div class="flex items-center gap-2">
      <button
        class="rounded bg-blue-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-blue-700 disabled:bg-blue-300"
        :disabled="saving"
        @click="saveSettings"
      >
        {{ saving ? 'Saving…' : 'Save' }}
      </button>
      <button
        class="rounded border border-red-300 px-3 py-1.5 text-sm font-medium text-red-700 hover:bg-red-50 disabled:opacity-50"
        :disabled="flushing"
        @click="onFlush"
      >
        {{ flushing ? 'Flushing…' : 'Flush cache' }}
      </button>
    </div>
  </section>
</template>
