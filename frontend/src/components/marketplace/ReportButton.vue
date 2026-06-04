<script setup lang="ts">
// ReportButton — opens an inline form, POSTs to /marketplace/reports.
// Inline confirmation (no toast system in repo). Used on both Marketplace
// cards and the KB detail page.
import { ref } from 'vue'
import { submitMarketplaceReport, MarketplaceApiError } from '../../api/marketplace'

const props = defineProps<{
  kbId: string
  /** Optional compact rendering for use inside cards. */
  compact?: boolean
}>()

const open = ref(false)
const reason = ref('')
const submitting = ref(false)
const errorMessage = ref('')
const confirmation = ref('')

function toggle(event: Event) {
  // Cards wrap the whole row in a router-link; the report flow must not
  // navigate. Stop propagation defensively at the button click.
  event.stopPropagation()
  event.preventDefault()
  open.value = !open.value
  if (!open.value) reset()
}

function reset() {
  reason.value = ''
  errorMessage.value = ''
  confirmation.value = ''
}

async function submit(event: Event) {
  event.preventDefault()
  event.stopPropagation()
  const trimmed = reason.value.trim()
  if (!trimmed) return
  submitting.value = true
  errorMessage.value = ''
  try {
    await submitMarketplaceReport(props.kbId, trimmed)
    confirmation.value = 'Report submitted. Our team will review it.'
    reason.value = ''
  } catch (err) {
    if (err instanceof MarketplaceApiError && err.status === 429) {
      errorMessage.value = 'Rate limit reached — try again later.'
    } else {
      errorMessage.value = err instanceof Error ? err.message : 'Failed to submit report.'
    }
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="relative inline-block" @click.stop>
    <button
      type="button"
      class="rounded-md border border-gray-300 bg-white text-gray-700 hover:bg-gray-50"
      :class="compact ? 'px-2 py-1 text-xs' : 'px-3 py-1.5 text-sm'"
      data-test="marketplace-report-button"
      @click="toggle"
    >
      Report
    </button>

    <div
      v-if="open"
      class="absolute right-0 z-10 mt-2 w-72 rounded-lg border border-gray-200 bg-white p-3 shadow-lg"
      data-test="marketplace-report-form"
    >
      <p v-if="confirmation" class="text-sm text-green-700" data-test="marketplace-report-confirm">
        {{ confirmation }}
      </p>
      <form v-else class="space-y-2" @submit="submit">
        <label class="block text-xs font-medium text-gray-700" for="report-reason">
          Why are you reporting this KB?
        </label>
        <textarea
          id="report-reason"
          v-model="reason"
          rows="3"
          maxlength="2000"
          required
          class="w-full resize-none rounded-md border border-gray-300 px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
          data-test="marketplace-report-reason"
        />
        <p v-if="errorMessage" class="text-xs text-red-600">{{ errorMessage }}</p>
        <div class="flex justify-end gap-2">
          <button
            type="button"
            class="rounded-md px-2 py-1 text-xs text-gray-600 hover:bg-gray-100"
            @click="toggle"
          >
            Cancel
          </button>
          <button
            type="submit"
            :disabled="submitting || !reason.trim()"
            class="rounded-md bg-indigo-600 px-3 py-1 text-xs font-medium text-white hover:bg-indigo-700 disabled:cursor-not-allowed disabled:opacity-50"
            data-test="marketplace-report-submit"
          >
            {{ submitting ? 'Submitting…' : 'Submit' }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>
