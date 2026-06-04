<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  listReports,
  approveReport,
  dismissReport,
  type MarketplaceReport,
  type ReportStatus,
} from '../../api/marketplace-admin'
import ResponsiveModal from '../../components/ResponsiveModal.vue'

// Admin review queue for marketplace reports (issue #734, ADR-0008).
// Two binary admin actions per report: Approve (escalate to takedown)
// and Dismiss (close without action). Status filter pill defaults to
// the live work-list (`open`); operators can flip to any status to
// inspect the queue's history.

type StatusFilter = ReportStatus

const STATUSES: { value: StatusFilter; label: string }[] = [
  { value: 'open', label: 'Open' },
  { value: 'reviewing', label: 'Reviewing' },
  { value: 'resolved', label: 'Resolved' },
  { value: 'dismissed', label: 'Dismissed' },
]

const reports = ref<MarketplaceReport[]>([])
const status = ref<StatusFilter>('open')
const loading = ref(false)
const error = ref<string | null>(null)

const confirmAction = ref<'approve' | 'dismiss' | null>(null)
const targetReport = ref<MarketplaceReport | null>(null)
const submitting = ref(false)

const lastApproveResult = ref<{ kbId: string; strikes: number } | null>(null)

async function refresh() {
  loading.value = true
  error.value = null
  try {
    const res = await listReports(status.value, 100, 0)
    reports.value = res.reports
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    loading.value = false
  }
}

function openApprove(report: MarketplaceReport) {
  targetReport.value = report
  confirmAction.value = 'approve'
}

function openDismiss(report: MarketplaceReport) {
  targetReport.value = report
  confirmAction.value = 'dismiss'
}

function closeConfirm() {
  if (submitting.value) return
  confirmAction.value = null
  targetReport.value = null
}

async function submitConfirm() {
  if (!targetReport.value || !confirmAction.value) return
  submitting.value = true
  try {
    if (confirmAction.value === 'approve') {
      const result = await approveReport(targetReport.value.id)
      lastApproveResult.value = {
        kbId: result.target_kb_id,
        strikes: result.strikes_after,
      }
    } else {
      await dismissReport(targetReport.value.id)
    }
    closeConfirm()
    await refresh()
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    submitting.value = false
  }
}

function formatAge(iso: string): string {
  const ms = Date.now() - new Date(iso).getTime()
  const hours = Math.floor(ms / 3_600_000)
  if (hours < 1) return 'just now'
  if (hours < 24) return `${hours}h ago`
  const days = Math.floor(hours / 24)
  return `${days}d ago`
}

function formatReporter(report: MarketplaceReport): string {
  // ADR-0006: reporter id may be NULL when the user was deleted (FK
  // ON DELETE SET NULL). Surface that as anonymous in the UI rather
  // than showing the empty string.
  return report.reporter_user_id ? report.reporter_user_id.slice(0, 8) : 'anonymous'
}

const confirmTitle = computed(() => {
  if (confirmAction.value === 'approve') return 'Approve report and unpublish KB?'
  if (confirmAction.value === 'dismiss') return 'Dismiss report?'
  return ''
})

onMounted(refresh)
</script>

<template>
  <div class="p-4 sm:p-6">
    <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-xl font-semibold text-slate-100">Marketplace report queue</h1>
        <p class="text-sm text-slate-400">
          Reactive moderation queue (ADR-0006). Approve unpublishes the KB and
          increments the publisher Org's strike counter.
        </p>
      </div>
      <button
        class="rounded border border-slate-600 px-3 py-1.5 text-sm text-slate-100 hover:bg-slate-700"
        :disabled="loading"
        @click="refresh"
      >
        Refresh
      </button>
    </div>

    <!-- Status filter pills -->
    <div class="mb-4 flex flex-wrap gap-2" role="tablist" aria-label="Report status filter">
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
      v-if="lastApproveResult"
      class="mb-4 rounded border border-emerald-500/40 bg-emerald-500/10 p-3 text-sm text-emerald-200"
      data-testid="admin-reports-approve-toast"
    >
      KB <code class="font-mono">{{ lastApproveResult.kbId.slice(0, 8) }}</code> unpublished.
      Publisher Org is now at <strong>{{ lastApproveResult.strikes }}</strong>
      {{ lastApproveResult.strikes === 1 ? 'strike' : 'strikes' }}.
      <button class="ml-2 text-emerald-100 underline" @click="lastApproveResult = null">
        Dismiss
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
      v-else-if="reports.length === 0"
      class="rounded border border-slate-700 p-6 text-center text-sm text-slate-400"
      data-testid="admin-reports-empty"
    >
      No reports in <strong>{{ status }}</strong>.
    </div>

    <div v-else class="overflow-x-auto rounded border border-slate-700">
      <table class="min-w-full text-left text-sm">
        <thead class="bg-slate-800 text-xs uppercase text-slate-400">
          <tr>
            <th class="px-3 py-2">Report</th>
            <th class="px-3 py-2">Target KB</th>
            <th class="px-3 py-2">Reporter</th>
            <th class="px-3 py-2">Reason</th>
            <th class="px-3 py-2">Age</th>
            <th class="px-3 py-2">Status</th>
            <th class="px-3 py-2 text-right">Actions</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-700 bg-slate-900 text-slate-200">
          <tr
            v-for="r in reports"
            :key="r.id"
            data-testid="admin-reports-row"
            :data-report-id="r.id"
          >
            <td class="px-3 py-2 font-mono text-xs">{{ r.id.slice(0, 8) }}</td>
            <td class="px-3 py-2 font-mono text-xs">{{ r.reported_kb_id.slice(0, 8) }}</td>
            <td class="px-3 py-2 font-mono text-xs">{{ formatReporter(r) }}</td>
            <td class="max-w-md px-3 py-2 text-slate-300">
              <span class="line-clamp-2">{{ r.reason }}</span>
            </td>
            <td class="px-3 py-2 text-slate-400">{{ formatAge(r.created_at) }}</td>
            <td class="px-3 py-2">
              <span
                class="rounded-full px-2 py-0.5 text-xs"
                :class="{
                  'bg-amber-500/20 text-amber-200': r.status === 'open',
                  'bg-sky-500/20 text-sky-200': r.status === 'reviewing',
                  'bg-emerald-500/20 text-emerald-200': r.status === 'resolved',
                  'bg-slate-500/20 text-slate-200': r.status === 'dismissed',
                }"
              >
                {{ r.status }}
              </span>
            </td>
            <td class="px-3 py-2 text-right">
              <div class="flex justify-end gap-2">
                <button
                  v-if="r.status === 'open' || r.status === 'reviewing'"
                  class="rounded border border-red-500/50 px-2 py-1 text-xs text-red-200 hover:bg-red-500/20"
                  :data-testid="`admin-reports-approve-${r.id}`"
                  @click="openApprove(r)"
                >
                  Approve
                </button>
                <button
                  v-if="r.status === 'open' || r.status === 'reviewing'"
                  class="rounded border border-slate-500/50 px-2 py-1 text-xs text-slate-300 hover:bg-slate-700"
                  :data-testid="`admin-reports-dismiss-${r.id}`"
                  @click="openDismiss(r)"
                >
                  Dismiss
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <ResponsiveModal
      :open="confirmAction !== null"
      :title="confirmTitle"
      @close="closeConfirm"
    >
      <div class="space-y-3 text-sm text-slate-200">
        <p v-if="confirmAction === 'approve'">
          This will unpublish the KB and increment the publisher Org's
          <strong>takedown strikes</strong> by one. The action is auditable but
          cannot be silently undone — recovery requires a manual republish.
        </p>
        <p v-if="confirmAction === 'dismiss'">
          This will close the report without action. The reporter is not
          notified (ADR-0006 — reporter-confirms-takedown loop is a known
          harassment vector).
        </p>
        <p v-if="targetReport" class="rounded bg-slate-800/60 p-2 font-mono text-xs">
          Report {{ targetReport.id.slice(0, 8) }} · KB
          {{ targetReport.reported_kb_id.slice(0, 8) }}
        </p>
        <div class="flex justify-end gap-2 pt-2">
          <button
            class="rounded border border-slate-600 px-3 py-1.5 text-sm text-slate-200 hover:bg-slate-700"
            :disabled="submitting"
            @click="closeConfirm"
          >
            Cancel
          </button>
          <button
            class="rounded px-3 py-1.5 text-sm text-white"
            :class="
              confirmAction === 'approve'
                ? 'bg-red-600 hover:bg-red-500'
                : 'bg-slate-600 hover:bg-slate-500'
            "
            :disabled="submitting"
            data-testid="admin-reports-confirm"
            @click="submitConfirm"
          >
            {{ submitting ? 'Submitting…' : 'Confirm' }}
          </button>
        </div>
      </div>
    </ResponsiveModal>
  </div>
</template>
