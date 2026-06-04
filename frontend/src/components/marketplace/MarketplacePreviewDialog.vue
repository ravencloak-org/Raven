<script setup lang="ts">
// MarketplacePreviewDialog — modal that renders up to 3 sample Chunks
// from `/marketplace/{org}/{kb}/preview` (ADR-0007). No "load more"; the
// cap is enforced server-side and we deliberately mirror that limit here.
import { onMounted, ref, watch } from 'vue'
import {
  previewMarketplaceKb,
  type PreviewChunk,
  MarketplaceApiError,
} from '../../api/marketplace'

const props = defineProps<{
  open: boolean
  orgSlug: string
  kbSlug: string
}>()

const emit = defineEmits<{
  close: []
  import: []
}>()

const chunks = ref<PreviewChunk[]>([])
const loading = ref(false)
const errorMessage = ref('')
const errorStatus = ref<number | null>(null)

async function load() {
  if (!props.open) return
  loading.value = true
  errorMessage.value = ''
  errorStatus.value = null
  chunks.value = []
  try {
    const res = await previewMarketplaceKb(props.orgSlug, props.kbSlug)
    chunks.value = res.chunks
  } catch (err) {
    if (err instanceof MarketplaceApiError) {
      errorStatus.value = err.status
      errorMessage.value =
        err.status === 410
          ? 'This KB has been unpublished.'
          : err.message
    } else {
      errorMessage.value = err instanceof Error ? err.message : 'Failed to load preview.'
    }
  } finally {
    loading.value = false
  }
}

onMounted(load)
watch(() => [props.open, props.orgSlug, props.kbSlug], load)
</script>

<template>
  <div
    v-if="open"
    class="fixed inset-0 z-40 flex items-center justify-center bg-black/40 p-4"
    role="dialog"
    aria-modal="true"
    aria-labelledby="preview-title"
    data-test="marketplace-preview-dialog"
    @click.self="emit('close')"
  >
    <div class="w-full max-w-2xl rounded-xl bg-white p-6 shadow-xl">
      <header class="mb-4 flex items-start justify-between">
        <div>
          <h2 id="preview-title" class="text-lg font-semibold text-gray-900">
            Preview
          </h2>
          <p class="text-xs text-gray-500">Up to 3 sample chunks from this KB.</p>
        </div>
        <button
          type="button"
          class="rounded-md p-1 text-gray-500 hover:bg-gray-100"
          aria-label="Close preview"
          data-test="marketplace-preview-close"
          @click="emit('close')"
        >
          ×
        </button>
      </header>

      <div v-if="loading" class="py-12 text-center text-sm text-gray-500">
        Loading preview…
      </div>
      <div
        v-else-if="errorMessage"
        class="rounded-md bg-red-50 border border-red-200 px-4 py-3 text-sm text-red-700"
        data-test="marketplace-preview-error"
      >
        {{ errorMessage }}
      </div>
      <div v-else-if="chunks.length === 0" class="py-12 text-center text-sm text-gray-500">
        No preview chunks are available for this KB yet.
      </div>
      <ul v-else class="space-y-3" data-test="marketplace-preview-chunks">
        <li
          v-for="chunk in chunks"
          :key="chunk.chunk_id"
          class="rounded-md border border-gray-200 bg-gray-50 p-3"
        >
          <div class="mb-1 text-xs font-medium text-gray-500">
            Chunk #{{ chunk.ordinal }}
          </div>
          <pre class="whitespace-pre-wrap text-sm text-gray-800">{{ chunk.text }}</pre>
        </li>
      </ul>

      <footer class="mt-6 flex justify-end gap-2">
        <button
          type="button"
          class="rounded-md border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
          @click="emit('close')"
        >
          Close
        </button>
        <button
          type="button"
          class="rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:cursor-not-allowed disabled:opacity-50"
          :disabled="!!errorMessage"
          data-test="marketplace-preview-import-cta"
          @click="emit('import')"
        >
          Import
        </button>
      </footer>
    </div>
  </div>
</template>
