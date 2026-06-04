<script setup lang="ts">
import { onMounted, ref, nextTick, onBeforeUnmount, watch } from 'vue'
import { usePasskeysStore } from '../../stores/passkeys'
import { defaultPasskeyLabel, formatRelativeTime } from '../../lib/passkey-label'

const store = usePasskeysStore()

// ─── inline relabel (mirrors M13 LlmProviderListPage pattern) ────────────────
const editingId = ref<string | null>(null)
const editingDraft = ref('')
const savedTickId = ref<string | null>(null)
let savedTickTimer: ReturnType<typeof setTimeout> | null = null
const labelInputRef = ref<HTMLInputElement | null>(null)

function flashSavedTick(credentialId: string) {
  savedTickId.value = credentialId
  if (savedTickTimer) clearTimeout(savedTickTimer)
  savedTickTimer = setTimeout(() => {
    savedTickId.value = null
    savedTickTimer = null
  }, 1500)
}

async function startEdit(credentialId: string, currentLabel: string) {
  editingId.value = credentialId
  editingDraft.value = currentLabel
  await nextTick()
  labelInputRef.value?.focus()
  labelInputRef.value?.select()
}

function cancelEdit() {
  editingId.value = null
  editingDraft.value = ''
}

async function commitEdit(credentialId: string, currentLabel: string) {
  const target = editingDraft.value.trim()
  if (!target || target === currentLabel) {
    cancelEdit()
    return
  }
  editingId.value = null
  editingDraft.value = ''
  try {
    await store.relabelPasskey(credentialId, target)
    flashSavedTick(credentialId)
  } catch (e) {
    toastError.value = e instanceof Error ? e.message : 'Could not save label'
  }
}

// ─── remove (confirm dialog) ─────────────────────────────────────────────────
const pendingRemoveId = ref<string | null>(null)
const removeBusy = ref(false)

function requestRemove(credentialId: string) {
  pendingRemoveId.value = credentialId
}

function cancelRemove() {
  pendingRemoveId.value = null
}

async function confirmRemove() {
  if (!pendingRemoveId.value) return
  removeBusy.value = true
  try {
    await store.removePasskey(pendingRemoveId.value)
    pendingRemoveId.value = null
  } catch (e) {
    toastError.value = e instanceof Error ? e.message : 'Could not remove passkey'
  } finally {
    removeBusy.value = false
  }
}

// ─── add (dialog with label input) ───────────────────────────────────────────
const showAddDialog = ref(false)
const addLabelDraft = ref('')
const addBusy = ref(false)
const addLabelInputRef = ref<HTMLInputElement | null>(null)

async function openAddDialog() {
  // Pre-fill from UA; user can edit before confirming.
  addLabelDraft.value = defaultPasskeyLabel(
    typeof navigator !== 'undefined' ? navigator.userAgent : '',
  )
  showAddDialog.value = true
  await nextTick()
  addLabelInputRef.value?.focus()
  addLabelInputRef.value?.select()
}

function closeAddDialog() {
  showAddDialog.value = false
  addLabelDraft.value = ''
}

async function confirmAdd() {
  const label = addLabelDraft.value.trim()
  if (!label) return
  addBusy.value = true
  try {
    await store.addPasskey(label)
    closeAddDialog()
  } catch (e) {
    toastError.value = e instanceof Error ? e.message : 'Could not add passkey'
  } finally {
    addBusy.value = false
  }
}

// ─── transient toast for failures ────────────────────────────────────────────
const toastError = ref<string | null>(null)
let toastTimer: ReturnType<typeof setTimeout> | null = null
watch(toastError, (v) => {
  if (toastTimer) clearTimeout(toastTimer)
  if (v) {
    toastTimer = setTimeout(() => {
      toastError.value = null
      toastTimer = null
    }, 4000)
  }
})

// ─── "current device" badge ──────────────────────────────────────────────────
// TODO(#773-followup): real heuristic. Today we have no client-side handle
// on which credential id was created on this browser, so we leave the badge
// off. When the add ceremony returns the credential id, persist it in
// localStorage (key: `raven.passkeys.lastLocalCredentialId`) and compare here.
// eslint-disable-next-line @typescript-eslint/no-unused-vars
function isCurrentDevice(_credentialId: string): boolean {
  return false
}

// ─── focus trap for dialogs (a11y) ───────────────────────────────────────────
function trapFocus(e: KeyboardEvent, root: HTMLElement | null) {
  if (!root || e.key !== 'Tab') return
  const focusables = root.querySelectorAll<HTMLElement>(
    'a[href], button:not([disabled]), input:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
  )
  if (focusables.length === 0) return
  const first = focusables[0]
  const last = focusables[focusables.length - 1]
  if (e.shiftKey && document.activeElement === first) {
    e.preventDefault()
    last.focus()
  } else if (!e.shiftKey && document.activeElement === last) {
    e.preventDefault()
    first.focus()
  }
}

const addDialogRoot = ref<HTMLElement | null>(null)
const removeDialogRoot = ref<HTMLElement | null>(null)

function onAddDialogKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    closeAddDialog()
    return
  }
  trapFocus(e, addDialogRoot.value)
}

function onRemoveDialogKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    cancelRemove()
    return
  }
  trapFocus(e, removeDialogRoot.value)
}

onMounted(() => {
  void store.fetchPasskeys()
})

onBeforeUnmount(() => {
  if (savedTickTimer) clearTimeout(savedTickTimer)
  if (toastTimer) clearTimeout(toastTimer)
})
</script>

<template>
  <section data-test="panel-authentication" class="passkeys-section">
    <!-- Heading row -->
    <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between mb-2">
      <div>
        <h2 class="text-lg font-semibold text-neutral-900 dark:text-white">Passkeys</h2>
        <p class="mt-1 text-sm text-neutral-600 dark:text-neutral-400 max-w-xl">
          Sign in with your fingerprint, face, or a security key &mdash; no Google round-trip.
        </p>
      </div>
      <button
        type="button"
        class="shrink-0 bg-amber-500 hover:bg-amber-600 disabled:opacity-50 text-white font-semibold px-4 py-2 rounded-lg transition-colors"
        data-test="passkey-add-button"
        :disabled="store.loading"
        @click="openAddDialog"
      >
        Add passkey
      </button>
    </div>

    <!-- Loading -->
    <p v-if="store.loading" class="text-sm text-neutral-500 mt-6">Loading passkeys…</p>

    <!-- Empty state -->
    <div
      v-else-if="store.passkeys.length === 0"
      data-test="passkey-empty-state"
      class="mt-6 rounded-lg border border-dashed border-neutral-300 dark:border-neutral-700 px-4 py-8 text-center"
    >
      <p class="text-sm text-neutral-700 dark:text-neutral-300">
        No passkeys yet. Add one to skip Google sign-in next time.
      </p>
      <button
        type="button"
        class="mt-4 bg-amber-500 hover:bg-amber-600 text-white font-semibold px-4 py-2 rounded-lg transition-colors"
        @click="openAddDialog"
      >
        Add your first passkey
      </button>
    </div>

    <!-- Populated list (unified-card pattern from M13 LlmProviderListPage) -->
    <ul v-else data-test="passkey-list" class="mt-6 space-y-3">
      <li
        v-for="pk in store.passkeys"
        :key="pk.credential_id"
        class="rounded-lg border border-neutral-200 dark:border-neutral-800 bg-white dark:bg-neutral-900 p-4 shadow-sm flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between"
      >
        <div class="min-w-0 flex-1">
          <div class="flex items-center gap-2">
            <!-- Inline-editable label -->
            <template v-if="editingId === pk.credential_id">
              <input
                ref="labelInputRef"
                v-model="editingDraft"
                type="text"
                class="flex-1 min-w-0 rounded border border-amber-300 dark:border-amber-700 bg-white dark:bg-neutral-950 text-neutral-900 dark:text-white px-2 py-1 text-sm"
                :aria-label="`Rename passkey ${pk.label}`"
                @keydown.enter.prevent="commitEdit(pk.credential_id, pk.label)"
                @keydown.esc.prevent="cancelEdit"
                @blur="commitEdit(pk.credential_id, pk.label)"
              />
            </template>
            <button
              v-else
              type="button"
              class="text-left font-semibold text-neutral-900 dark:text-white hover:underline focus:underline focus:outline-none truncate"
              :aria-label="`Edit label for ${pk.label}`"
              @click="startEdit(pk.credential_id, pk.label)"
            >
              {{ pk.label }}
            </button>

            <span
              v-if="isCurrentDevice(pk.credential_id)"
              class="shrink-0 inline-flex items-center rounded-full bg-emerald-100 dark:bg-emerald-900/40 px-2 py-0.5 text-xs font-medium text-emerald-700 dark:text-emerald-300"
            >
              Current device
            </span>
            <span
              v-if="savedTickId === pk.credential_id"
              class="shrink-0 text-xs text-emerald-600 dark:text-emerald-400"
              data-test="passkey-saved-tick"
            >
              Saved ✓
            </span>
          </div>

          <div class="mt-1 text-xs text-neutral-500 dark:text-neutral-400 flex flex-wrap gap-x-3 gap-y-1">
            <span>Added {{ formatRelativeTime(pk.created_at) }}</span>
            <span aria-hidden="true">·</span>
            <span v-if="pk.last_used_at">Last used {{ formatRelativeTime(pk.last_used_at) }}</span>
            <span v-else>Never used</span>
          </div>
        </div>

        <button
          type="button"
          class="self-start sm:self-auto text-sm font-medium text-red-600 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300 px-3 py-1.5 rounded-lg border border-red-200 dark:border-red-900 hover:bg-red-50 dark:hover:bg-red-950 transition-colors"
          :aria-label="`Remove passkey ${pk.label}`"
          data-test="passkey-remove-button"
          @click="requestRemove(pk.credential_id)"
        >
          Remove
        </button>
      </li>
    </ul>

    <p v-if="store.error" class="mt-4 text-sm text-red-600 dark:text-red-400" role="alert">
      {{ store.error }}
    </p>

    <!-- Add dialog -->
    <div
      v-if="showAddDialog"
      ref="addDialogRoot"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      role="dialog"
      aria-modal="true"
      aria-labelledby="passkey-add-title"
      data-test="passkey-add-dialog"
      @keydown="onAddDialogKeydown"
    >
      <div class="w-full max-w-md rounded-lg bg-white dark:bg-neutral-900 p-6 shadow-xl">
        <h3 id="passkey-add-title" class="text-lg font-semibold text-neutral-900 dark:text-white">
          Add a passkey
        </h3>
        <p class="mt-1 text-sm text-neutral-600 dark:text-neutral-400">
          Your device will prompt for biometrics or a security key.
        </p>

        <label class="block mt-4">
          <span class="text-sm font-medium text-neutral-700 dark:text-neutral-300">Label</span>
          <input
            ref="addLabelInputRef"
            v-model="addLabelDraft"
            type="text"
            required
            class="mt-1 block w-full rounded-lg border border-neutral-300 dark:border-neutral-700 bg-white dark:bg-neutral-950 text-neutral-900 dark:text-white px-3 py-2"
            data-test="passkey-add-label-input"
            @keydown.enter.prevent="confirmAdd"
          />
          <span class="mt-1 block text-xs text-neutral-500">
            Pick something you'll recognise later, e.g. your laptop's name.
          </span>
        </label>

        <div class="mt-6 flex justify-end gap-2">
          <button
            type="button"
            class="px-4 py-2 rounded-lg border border-neutral-300 dark:border-neutral-700 text-neutral-700 dark:text-neutral-300 hover:bg-neutral-100 dark:hover:bg-neutral-800 transition-colors"
            :disabled="addBusy"
            @click="closeAddDialog"
          >
            Cancel
          </button>
          <button
            type="button"
            class="px-4 py-2 rounded-lg bg-amber-500 hover:bg-amber-600 disabled:opacity-50 text-white font-semibold transition-colors"
            :disabled="addBusy || addLabelDraft.trim().length === 0"
            data-test="passkey-add-confirm"
            @click="confirmAdd"
          >
            {{ addBusy ? 'Adding…' : 'Add passkey' }}
          </button>
        </div>
      </div>
    </div>

    <!-- Remove confirm dialog -->
    <div
      v-if="pendingRemoveId !== null"
      ref="removeDialogRoot"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      role="dialog"
      aria-modal="true"
      aria-labelledby="passkey-remove-title"
      data-test="passkey-remove-dialog"
      @keydown="onRemoveDialogKeydown"
    >
      <div class="w-full max-w-md rounded-lg bg-white dark:bg-neutral-900 p-6 shadow-xl">
        <h3 id="passkey-remove-title" class="text-lg font-semibold text-neutral-900 dark:text-white">
          Remove this passkey?
        </h3>
        <p class="mt-2 text-sm text-neutral-600 dark:text-neutral-400">
          You won't be able to sign in with it again. You can always add a new one.
        </p>
        <div class="mt-6 flex justify-end gap-2">
          <button
            type="button"
            class="px-4 py-2 rounded-lg border border-neutral-300 dark:border-neutral-700 text-neutral-700 dark:text-neutral-300 hover:bg-neutral-100 dark:hover:bg-neutral-800 transition-colors"
            :disabled="removeBusy"
            @click="cancelRemove"
          >
            Cancel
          </button>
          <button
            type="button"
            class="px-4 py-2 rounded-lg bg-red-600 hover:bg-red-700 disabled:opacity-50 text-white font-semibold transition-colors"
            :disabled="removeBusy"
            data-test="passkey-remove-confirm"
            @click="confirmRemove"
          >
            {{ removeBusy ? 'Removing…' : 'Remove' }}
          </button>
        </div>
      </div>
    </div>

    <!-- Failure toast -->
    <div
      v-if="toastError"
      class="fixed bottom-4 right-4 z-50 max-w-sm rounded-lg border border-red-300 bg-red-50 px-4 py-3 text-sm text-red-800 shadow-lg"
      role="alert"
      data-test="passkey-error-toast"
      @click="toastError = null"
    >
      {{ toastError }}
    </div>
  </section>
</template>
