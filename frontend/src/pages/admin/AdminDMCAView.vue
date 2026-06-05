<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  listDMCANotices,
  submitDMCANotice,
  submitCounterNotice,
  type DMCANotice,
  type DMCAStatus,
  type DMCASubmitInput,
} from '../../api/marketplace-admin'
import ResponsiveModal from '../../components/ResponsiveModal.vue'

// Admin DMCA inbox + two-stage counter-notice workflow (issue #736,
// launch blocker per ADR-0006).
//
// Three operations:
//   - List + filter notices by status pill.
//   - Record a DMCA notice received at dmca@ravencloak.org (modal form).
//     This atomically inserts the row + freezes the target KB into
//     `kb_status='dmca_pending'` for the 14-day counter-notice window.
//   - Record a publisher counter-notice on a `pending` row. MVP
//     simplification: the admin acts on behalf of the publisher who
//     replied to the inbox; there is no public counter-notice UI.
//
// What this page intentionally does NOT do (out of scope for #736):
//   - Manual auto-takedown trigger (the daily sweeper handles it).
//   - Restore button (resolved_keep_up). Deferred — the safe-harbour
//     window between counter-notice and restore requires legal review.

type StatusFilter = '' | DMCAStatus

const STATUSES: { value: StatusFilter; label: string }[] = [
  { value: '', label: 'All' },
  { value: 'pending', label: 'Pending' },
  { value: 'counter_filed', label: 'Counter-filed' },
  { value: 'resolved_take_down', label: 'Take-down' },
  { value: 'resolved_keep_up', label: 'Keep-up' },
  { value: 'withdrawn', label: 'Withdrawn' },
]

const notices = ref<DMCANotice[]>([])
const status = ref<StatusFilter>('pending')
const loading = ref(false)
const error = ref<string | null>(null)

const recordOpen = ref(false)
const counterOpen = ref(false)
const submitting = ref(false)
const targetNotice = ref<DMCANotice | null>(null)

const recordForm = ref<DMCASubmitInput>({
  target_kb_id: '',
  notice_text: '',
  claimant_email: '',
  claimant_name: '',
})
const counterText = ref('')

async function refresh() {
  loading.value = true
  error.value = null
  try {
    const res = await listDMCANotices(status.value || undefined, 100, 0)
    notices.value = res.notices
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    loading.value = false
  }
}

function openRecord() {
  recordForm.value = {
    target_kb_id: '',
    notice_text: '',
    claimant_email: '',
    claimant_name: '',
  }
  recordOpen.value = true
}

function closeRecord() {
  if (submitting.value) return
  recordOpen.value = false
}

async function submitRecord() {
  // Inline validation mirrors the server's DMCANoticeInput.Validate.
  // Catching here avoids a round-trip for the empty-field case.
  if (
    !recordForm.value.target_kb_id ||
    !recordForm.value.notice_text ||
    !recordForm.value.claimant_email ||
    !recordForm.value.claimant_name
  ) {
    error.value = 'All fields are required.'
    return
  }
  if (recordForm.value.notice_text.length > 8192) {
    error.value = 'Notice text must be 8 KiB or less.'
    return
  }
  submitting.value = true
  try {
    await submitDMCANotice(recordForm.value)
    recordOpen.value = false
    await refresh()
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    submitting.value = false
  }
}

function openCounter(notice: DMCANotice) {
  targetNotice.value = notice
  counterText.value = ''
  counterOpen.value = true
}

function closeCounter() {
  if (submitting.value) return
  counterOpen.value = false
  targetNotice.value = null
}

async function submitCounter() {
  if (!targetNotice.value) return
  if (!counterText.value.trim()) {
    error.value = 'Counter-notice text is required.'
    return
  }
  submitting.value = true
  try {
    await submitCounterNotice(targetNotice.value.id, counterText.value)
    counterOpen.value = false
    targetNotice.value = null
    await refresh()
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    submitting.value = false
  }
}

// Renders "Nd Nh remaining" for the 14-day window. Negative means the
// window has elapsed — the sweeper will pick it up at next 4am UTC.
function formatWindow(notice: DMCANotice): string {
  if (notice.status !== 'pending') return '—'
  const ends = new Date(notice.counter_notice_window_ends).getTime()
  const remaining = ends - Date.now()
  if (remaining < 0) return 'expired (awaiting sweep)'
  const days = Math.floor(remaining / 86_400_000)
  const hours = Math.floor((remaining % 86_400_000) / 3_600_000)
  return `${days}d ${hours}h remaining`
}

function formatAge(iso: string): string {
  const ms = Date.now() - new Date(iso).getTime()
  const hours = Math.floor(ms / 3_600_000)
  if (hours < 1) return 'just now'
  if (hours < 24) return `${hours}h ago`
  return `${Math.floor(hours / 24)}d ago`
}

const counterTitle = computed(() => {
  if (!targetNotice.value) return ''
  return `Record counter-notice on ${targetNotice.value.id.slice(0, 8)}`
})

onMounted(refresh)
</script>

<template>
  <div class="p-4 sm:p-6">
    <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-xl font-semibold text-slate-100">DMCA inbox</h1>
        <p class="text-sm text-slate-400">
          Records DMCA notices received at
          <code>dmca&#64;ravencloak.org</code> and their counter-notices
          (ADR-0006). Pending notices freeze the target KB for the 14-day
          counter-notice window; the daily sweeper auto-resolves expired
          notices to take-down.
        </p>
      </div>
      <div class="flex gap-2">
        <button
          class="rounded border border-slate-600 px-3 py-1.5 text-sm text-slate-100 hover:bg-slate-700"
          :disabled="loading"
          @click="refresh"
        >
          Refresh
        </button>
        <button
          class="rounded bg-indigo-600 px-3 py-1.5 text-sm text-white hover:bg-indigo-500"
          data-testid="admin-dmca-record-button"
          @click="openRecord"
        >
          Record DMCA notice
        </button>
      </div>
    </div>

    <!-- Status filter pills -->
    <div class="mb-4 flex flex-wrap gap-2" role="tablist" aria-label="DMCA notice status filter">
      <button
        v-for="opt in STATUSES"
        :key="opt.value"
        role="tab"
        :aria-selected="status === opt.value"
        class="rounded-full border px-3 py-1 text-sm"
        :class="
          status === opt.value
            ? 'border-indigo-400 bg-indigo-500/20 text-indigo-200'
            : 'border-slate-600 text-slate-300 hover:bg-slate-700'
        "
        @click="status = opt.value; refresh()"
      >
        {{ opt.label }}
      </button>
    </div>

    <div
      v-if="error"
      class="mb-4 rounded border border-red-500/40 bg-red-500/10 p-3 text-sm text-red-200"
    >
      {{ error }}
    </div>

    <div v-if="loading" class="text-sm text-slate-400">Loading…</div>

    <div
      v-else-if="notices.length === 0"
      class="rounded border border-slate-700 p-6 text-center text-sm text-slate-400"
      data-testid="admin-dmca-empty"
    >
      No DMCA notices in <strong>{{ status || 'any status' }}</strong>.
    </div>

    <div v-else class="overflow-x-auto rounded border border-slate-700">
      <table class="min-w-full text-left text-sm">
        <thead class="bg-slate-800 text-xs uppercase text-slate-400">
          <tr>
            <th class="px-3 py-2">Notice</th>
            <th class="px-3 py-2">Target KB</th>
            <th class="px-3 py-2">Claimant</th>
            <th class="px-3 py-2">Status</th>
            <th class="px-3 py-2">Window</th>
            <th class="px-3 py-2">Filed</th>
            <th class="px-3 py-2 text-right">Actions</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-700 bg-slate-900 text-slate-200">
          <tr
            v-for="n in notices"
            :key="n.id"
            data-testid="admin-dmca-row"
            :data-notice-id="n.id"
          >
            <td class="px-3 py-2 font-mono text-xs">{{ n.id.slice(0, 8) }}</td>
            <td class="px-3 py-2 font-mono text-xs">{{ n.target_kb_id.slice(0, 8) }}</td>
            <td class="px-3 py-2 text-slate-300">
              <div>{{ n.claimant_name }}</div>
              <div class="text-xs text-slate-500">{{ n.claimant_email }}</div>
            </td>
            <td class="px-3 py-2">
              <span
                class="rounded-full px-2 py-0.5 text-xs"
                :class="{
                  'bg-amber-500/20 text-amber-200': n.status === 'pending',
                  'bg-sky-500/20 text-sky-200': n.status === 'counter_filed',
                  'bg-red-500/20 text-red-200': n.status === 'resolved_take_down',
                  'bg-emerald-500/20 text-emerald-200': n.status === 'resolved_keep_up',
                  'bg-slate-500/20 text-slate-200': n.status === 'withdrawn',
                }"
              >
                {{ n.status }}
              </span>
            </td>
            <td class="px-3 py-2 text-slate-400">{{ formatWindow(n) }}</td>
            <td class="px-3 py-2 text-slate-400">{{ formatAge(n.created_at) }}</td>
            <td class="px-3 py-2 text-right">
              <button
                v-if="n.status === 'pending'"
                class="rounded border border-sky-500/50 px-2 py-1 text-xs text-sky-200 hover:bg-sky-500/20"
                :data-testid="`admin-dmca-counter-${n.id}`"
                @click="openCounter(n)"
              >
                Counter-notice
              </button>
              <span v-else class="text-xs text-slate-500">—</span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Record DMCA notice modal -->
    <ResponsiveModal :open="recordOpen" title="Record DMCA notice" @close="closeRecord">
      <form class="space-y-3 text-sm text-slate-200" @submit.prevent="submitRecord">
        <p class="text-xs text-slate-400">
          Recording a DMCA notice freezes the target KB for 14 days. The
          sweeper auto-resolves to take-down at the end of the window if
          no counter-notice is filed.
        </p>
        <label class="block">
          <span class="text-xs text-slate-400">Target KB ID (UUID)</span>
          <input
            v-model="recordForm.target_kb_id"
            type="text"
            required
            class="mt-1 w-full rounded border border-slate-600 bg-slate-800 px-2 py-1 text-sm font-mono"
            data-testid="admin-dmca-target-kb-id"
          />
        </label>
        <label class="block">
          <span class="text-xs text-slate-400">Claimant name</span>
          <input
            v-model="recordForm.claimant_name"
            type="text"
            required
            class="mt-1 w-full rounded border border-slate-600 bg-slate-800 px-2 py-1 text-sm"
            data-testid="admin-dmca-claimant-name"
          />
        </label>
        <label class="block">
          <span class="text-xs text-slate-400">Claimant email</span>
          <input
            v-model="recordForm.claimant_email"
            type="email"
            required
            class="mt-1 w-full rounded border border-slate-600 bg-slate-800 px-2 py-1 text-sm"
            data-testid="admin-dmca-claimant-email"
          />
        </label>
        <label class="block">
          <span class="text-xs text-slate-400">Notice text (max 8 KiB)</span>
          <textarea
            v-model="recordForm.notice_text"
            required
            rows="6"
            maxlength="8192"
            class="mt-1 w-full rounded border border-slate-600 bg-slate-800 px-2 py-1 text-xs font-mono"
            data-testid="admin-dmca-notice-text"
          ></textarea>
        </label>
        <div class="flex justify-end gap-2 pt-2">
          <button
            type="button"
            class="rounded border border-slate-600 px-3 py-1.5 text-sm text-slate-200 hover:bg-slate-700"
            :disabled="submitting"
            @click="closeRecord"
          >
            Cancel
          </button>
          <button
            type="submit"
            class="rounded bg-indigo-600 px-3 py-1.5 text-sm text-white hover:bg-indigo-500"
            :disabled="submitting"
            data-testid="admin-dmca-record-submit"
          >
            {{ submitting ? 'Submitting…' : 'Record notice' }}
          </button>
        </div>
      </form>
    </ResponsiveModal>

    <!-- Counter-notice modal -->
    <ResponsiveModal :open="counterOpen" :title="counterTitle" @close="closeCounter">
      <form class="space-y-3 text-sm text-slate-200" @submit.prevent="submitCounter">
        <p class="text-xs text-slate-400">
          Records the publisher's counter-notice verbatim. The KB stays
          in <code>dmca_pending</code> until an admin makes the final
          keep-up/take-down decision (safe-harbour rules require a hold
          window between counter-notice and restoration).
        </p>
        <label class="block">
          <span class="text-xs text-slate-400">Counter-notice text (max 8 KiB)</span>
          <textarea
            v-model="counterText"
            required
            rows="8"
            maxlength="8192"
            class="mt-1 w-full rounded border border-slate-600 bg-slate-800 px-2 py-1 text-xs font-mono"
            data-testid="admin-dmca-counter-text"
          ></textarea>
        </label>
        <div class="flex justify-end gap-2 pt-2">
          <button
            type="button"
            class="rounded border border-slate-600 px-3 py-1.5 text-sm text-slate-200 hover:bg-slate-700"
            :disabled="submitting"
            @click="closeCounter"
          >
            Cancel
          </button>
          <button
            type="submit"
            class="rounded bg-sky-600 px-3 py-1.5 text-sm text-white hover:bg-sky-500"
            :disabled="submitting"
            data-testid="admin-dmca-counter-submit"
          >
            {{ submitting ? 'Submitting…' : 'Record counter-notice' }}
          </button>
        </div>
      </form>
    </ResponsiveModal>
  </div>
</template>
