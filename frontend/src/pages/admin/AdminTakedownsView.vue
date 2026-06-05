<script setup lang="ts">
// Admin-only Marketplace takedown audit log (issue #735).
//
// Read-only paginated table. The backend gates the endpoint with the
// global raven-admin allowlist; this view shows a "forbidden" empty
// state on 403 so a curious non-admin who guesses the URL sees a clear
// message rather than the loading spinner spinning forever.

import { ref, computed, onMounted } from 'vue'
import {
  listAdminTakedowns,
  AdminTakedownsHTTPError,
  type AdminTakedownRow,
  type AdminTakedownSource,
} from '../../api/admin-takedowns'

const PAGE_SIZE = 25

const rows = ref<AdminTakedownRow[]>([])
const nextCursor = ref<string | undefined>(undefined)
const cursorStack = ref<string[]>([])
const loading = ref(false)
const errorStatus = ref<number | null>(null)
const errorMessage = ref<string>('')

async function load(cursor?: string) {
  loading.value = true
  errorStatus.value = null
  errorMessage.value = ''
  try {
    const page = await listAdminTakedowns({ limit: PAGE_SIZE, cursor })
    rows.value = page.items
    nextCursor.value = page.next_cursor
  } catch (err) {
    if (err instanceof AdminTakedownsHTTPError) {
      errorStatus.value = err.status
      errorMessage.value = err.message
    } else {
      errorStatus.value = 0
      errorMessage.value = err instanceof Error ? err.message : 'unknown error'
    }
  } finally {
    loading.value = false
  }
}

onMounted(() => load())

function loadNext() {
  if (!nextCursor.value) return
  // Push the cursor that opened the *current* page onto the stack so
  // "back" returns to it. The first page is represented by undefined.
  cursorStack.value.push(cursorStack.value.length === 0 ? '' : cursorStack.value[cursorStack.value.length - 1])
  void load(nextCursor.value)
}

function loadPrev() {
  if (cursorStack.value.length === 0) return
  const prev = cursorStack.value.pop()
  void load(prev === '' ? undefined : prev)
}

const canGoBack = computed(() => cursorStack.value.length > 0)
const canGoNext = computed(() => !!nextCursor.value)

function sourceLabel(s: AdminTakedownSource): string {
  switch (s) {
    case 'publisher':
      return 'Publisher'
    case 'admin':
      return 'Admin'
    case 'dmca':
      return 'DMCA'
  }
}

function sourceClass(s: AdminTakedownSource): string {
  switch (s) {
    case 'publisher':
      return 'pill pill-publisher'
    case 'admin':
      return 'pill pill-admin'
    case 'dmca':
      return 'pill pill-dmca'
  }
}

function formatTimestamp(iso: string): string {
  try {
    return new Date(iso).toLocaleString()
  } catch {
    return iso
  }
}

const forbidden = computed(
  () => errorStatus.value === 401 || errorStatus.value === 403,
)
</script>

<template>
  <section class="admin-takedowns" data-test="admin-takedowns-view">
    <header class="header">
      <h1>Marketplace Takedowns</h1>
      <p class="subtitle">
        Audit log of every Public KB removed from the Marketplace. Read-only;
        gated to Raven admins (the
        <code>RAVEN_ADMIN_EMAILS</code> allowlist).
      </p>
    </header>

    <div v-if="forbidden" class="state state-forbidden" data-test="admin-takedowns-forbidden">
      <h2>Access denied</h2>
      <p>
        You must be a Raven admin to view the takedown audit log. If you
        believe this is wrong, contact the platform team.
      </p>
    </div>

    <div
      v-else-if="errorStatus !== null"
      class="state state-error"
      data-test="admin-takedowns-error"
    >
      <h2>Couldn't load takedowns</h2>
      <p>{{ errorMessage || 'Unknown error.' }}</p>
      <button type="button" @click="load()">Retry</button>
    </div>

    <div v-else-if="loading" class="state state-loading">Loading…</div>

    <div
      v-else-if="rows.length === 0"
      class="state state-empty"
      data-test="admin-takedowns-empty"
    >
      <h2>No takedowns yet</h2>
      <p>When the Marketplace flags or removes a Public KB, it shows up here.</p>
    </div>

    <table v-else class="takedowns-table" data-test="admin-takedowns-table">
      <thead>
        <tr>
          <th>Takedown ID</th>
          <th>Target KB</th>
          <th>Source</th>
          <th>Reason / Notes</th>
          <th>Created</th>
          <th>Org strikes</th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="row in rows"
          :key="row.takedown_id"
          data-test="admin-takedowns-row"
        >
          <td class="mono">{{ row.takedown_id.slice(0, 8) }}…</td>
          <td>
            <div class="kb-cell">
              <span class="kb-name">{{ row.target_kb_name }}</span>
              <span class="kb-org"
                >{{ row.target_org_display_name }} ({{ row.target_org_slug }})</span
              >
            </div>
          </td>
          <td>
            <span :class="sourceClass(row.source)">{{ sourceLabel(row.source) }}</span>
          </td>
          <td class="notes">{{ row.notes || '—' }}</td>
          <td>{{ formatTimestamp(row.created_at) }}</td>
          <td class="strikes">{{ row.strikes_after_org_total }}</td>
        </tr>
      </tbody>
    </table>

    <nav v-if="!forbidden && errorStatus === null" class="pagination" aria-label="pagination">
      <button
        type="button"
        :disabled="!canGoBack || loading"
        data-test="admin-takedowns-prev"
        @click="loadPrev"
      >
        ← Previous
      </button>
      <button
        type="button"
        :disabled="!canGoNext || loading"
        data-test="admin-takedowns-next"
        @click="loadNext"
      >
        Next →
      </button>
    </nav>
  </section>
</template>

<style scoped>
.admin-takedowns {
  padding: 1.5rem;
  max-width: 100%;
}
.header h1 {
  margin: 0 0 0.25rem;
  font-size: 1.5rem;
}
.subtitle {
  color: var(--color-text-muted, #666);
  margin: 0 0 1.5rem;
}
.state {
  padding: 2rem;
  text-align: center;
  border: 1px dashed var(--color-border, #ccc);
  border-radius: 8px;
  color: var(--color-text-muted, #666);
}
.state-forbidden,
.state-error {
  border-style: solid;
  background: var(--color-bg-warning, #fff8f0);
}
.takedowns-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.95rem;
}
.takedowns-table th,
.takedowns-table td {
  text-align: left;
  padding: 0.75rem 0.5rem;
  border-bottom: 1px solid var(--color-border, #eee);
  vertical-align: top;
}
.takedowns-table th {
  font-weight: 600;
  background: var(--color-bg-subtle, #fafafa);
}
.mono {
  font-family: 'SFMono-Regular', 'Consolas', monospace;
  font-size: 0.85em;
}
.kb-cell {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
}
.kb-name {
  font-weight: 500;
}
.kb-org {
  font-size: 0.85em;
  color: var(--color-text-muted, #666);
}
.notes {
  max-width: 24rem;
  white-space: pre-wrap;
  word-break: break-word;
}
.strikes {
  text-align: right;
  font-variant-numeric: tabular-nums;
  font-weight: 600;
}
.pill {
  display: inline-block;
  padding: 0.15rem 0.55rem;
  border-radius: 999px;
  font-size: 0.8em;
  font-weight: 600;
}
.pill-publisher {
  background: #e9eefb;
  color: #2748a3;
}
.pill-admin {
  background: #fff4d9;
  color: #8a5a00;
}
.pill-dmca {
  background: #fde2e2;
  color: #9c1c1c;
}
.pagination {
  display: flex;
  gap: 0.5rem;
  justify-content: flex-end;
  margin-top: 1rem;
}
.pagination button {
  padding: 0.4rem 0.9rem;
  border-radius: 4px;
  border: 1px solid var(--color-border, #ccc);
  background: var(--color-bg-button, #fff);
  cursor: pointer;
}
.pagination button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
