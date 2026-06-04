<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useAuthStore } from '../../stores/auth'
import { useLlmProviderHealthStore } from '../../stores/llm-provider-health'
import { useLlmProvidersStore } from '../../stores/llm-providers'
import type {
  CreateLlmProviderRequest,
  LlmProvider,
  UpdateLlmProviderRequest,
} from '../../api/llm-providers'
import CreateProviderDialog from './components/CreateProviderDialog.vue'
import EditCredentialsDialog from './components/EditCredentialsDialog.vue'
import ProviderCard from './components/ProviderCard.vue'

const store = useLlmProvidersStore()
const authStore = useAuthStore()
const healthStore = useLlmProviderHealthStore()

// Health failure mirrored locally so the banner reflects whatever the
// background cron last observed. We don't trigger our own probe here —
// the polling store running in DefaultLayout is the single source of
// truth and reaches into this page via shared pinia state.
const defaultProvider = computed(() =>
  store.providers.find((p) => p.is_default) ?? null,
)
const showHealthBanner = computed(
  () => defaultProvider.value !== null && !healthStore.isHealthy(),
)
const healthBannerDetail = computed(() => healthStore.failureReason())
const orgId = computed(() => authStore.orgId ?? sessionStorage.getItem('raven_org_id') ?? '')

const showCreateDialog = ref(false)

const showDeleteDialog = ref(false)
const providerToDelete = ref<string | null>(null)
const providerToDeleteName = ref('')
const deleting = ref(false)

// In-memory connection-test status keyed by provider id. The Create dialog
// (and the Edit dialog) writes here when a "Test connection" call comes back;
// the list page reads it to warn before switching default to a provider whose
// last test failed. Not persisted — session-only.
const testStatus = ref<Record<string, 'pass' | 'fail'>>({})

// Tracks which provider's "Make default" button is mid-flight so we can
// disable it and avoid duplicate clicks during the optimistic update.
const settingDefaultId = ref<string | null>(null)

// Non-blocking error toast for default-switch failures. Auto-dismissed
// after 4s; user can also click to close. Re-uses the existing Tailwind
// palette — no toast library to keep the page footprint small.
const defaultError = ref<string | null>(null)
let defaultErrorTimer: ReturnType<typeof setTimeout> | null = null

function showDefaultError(msg: string) {
  defaultError.value = msg
  if (defaultErrorTimer) clearTimeout(defaultErrorTimer)
  defaultErrorTimer = setTimeout(() => {
    defaultError.value = null
    defaultErrorTimer = null
  }, 4000)
}

async function handleMakeDefault(providerId: string) {
  if (settingDefaultId.value) return
  settingDefaultId.value = providerId
  try {
    await store.setDefault(orgId.value, providerId)
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : 'Failed to set default provider'
    showDefaultError(`Could not switch default: ${msg}`)
  } finally {
    settingDefaultId.value = null
  }
}

// ---------------------------------------------------------------------------
// Inline editing (#744)
//
// Two patches can fire from the card itself without opening the Edit dialog:
//   - `display_name` via click-to-edit input (Enter / blur saves, Esc cancels).
//   - `config.model` via inline <select> (change saves immediately).
// Both go through `store.editProvider` which wraps `updateLlmProvider` and
// sends a PARTIAL body. PR #749 guarantees `api_key` is preserved on the
// server when omitted, so neither inline flow ever ships a key over the wire.
//
// Optimistic UX: store mutates the cached provider on success; on failure we
// roll back the local mutation and surface a transient toast via the same
// `showDefaultError` channel (the toast slot is generic enough to re-use).
// ---------------------------------------------------------------------------
const savedTickId = ref<string | null>(null)
let savedTickTimer: ReturnType<typeof setTimeout> | null = null

function flashSavedTick(providerId: string) {
  savedTickId.value = providerId
  if (savedTickTimer) clearTimeout(savedTickTimer)
  savedTickTimer = setTimeout(() => {
    savedTickId.value = null
    savedTickTimer = null
  }, 1500)
}

async function handleRename(provider: LlmProvider, target: string) {
  const previousName = provider.display_name
  // Optimistic local mutation: paint the new name immediately.
  const idx = store.providers.findIndex((p) => p.id === provider.id)
  if (idx !== -1) store.providers[idx] = { ...store.providers[idx], display_name: target }
  try {
    await store.editProvider(orgId.value, provider.id, { display_name: target })
    flashSavedTick(provider.id)
  } catch (e: unknown) {
    // Roll back the optimistic name.
    const rollbackIdx = store.providers.findIndex((p) => p.id === provider.id)
    if (rollbackIdx !== -1) {
      store.providers[rollbackIdx] = {
        ...store.providers[rollbackIdx],
        display_name: previousName,
      }
    }
    const msg = e instanceof Error ? e.message : 'Failed to save name'
    showDefaultError(`Could not save name: ${msg}`)
  }
}

async function handleChangeModel(provider: LlmProvider, nextModel: string) {
  const previousConfig = provider.config ?? {}
  const previousModel = typeof previousConfig.model === 'string' ? previousConfig.model : ''
  if (nextModel === previousModel) return
  // Optimistic local mutation.
  const idx = store.providers.findIndex((p) => p.id === provider.id)
  if (idx !== -1) {
    store.providers[idx] = {
      ...store.providers[idx],
      config: { ...(previousConfig as Record<string, unknown>), model: nextModel },
    }
  }
  try {
    await store.editProvider(orgId.value, provider.id, {
      config: { ...(previousConfig as Record<string, unknown>), model: nextModel },
    })
    flashSavedTick(provider.id)
  } catch (e: unknown) {
    const rollbackIdx = store.providers.findIndex((p) => p.id === provider.id)
    if (rollbackIdx !== -1) {
      store.providers[rollbackIdx] = {
        ...store.providers[rollbackIdx],
        config: previousConfig as Record<string, unknown>,
      }
    }
    const msg = e instanceof Error ? e.message : 'Failed to save model'
    showDefaultError(`Could not save model: ${msg}`)
  }
}

// Create handler — wired to the dialog's onSubmit prop so the dialog
// can await it and self-close on success.
async function handleCreate(payload: CreateLlmProviderRequest) {
  await store.addProvider(orgId.value, payload)
}

// Edit dialog state. We pass the selected provider down — the dialog
// builds its own form state from it.
const showEditDialog = ref(false)
const editingProvider = ref<LlmProvider | null>(null)

function openEditDialog(provider: LlmProvider) {
  editingProvider.value = provider
  showEditDialog.value = true
}

function closeEditDialog() {
  showEditDialog.value = false
  editingProvider.value = null
}

async function handleEditSave(payload: UpdateLlmProviderRequest) {
  if (!editingProvider.value) return
  await store.editProvider(orgId.value, editingProvider.value.id, payload)
}

function confirmDelete(id: string, name: string) {
  providerToDelete.value = id
  providerToDeleteName.value = name
  showDeleteDialog.value = true
}

async function handleDelete() {
  if (!providerToDelete.value) return
  deleting.value = true
  try {
    await store.removeProvider(orgId.value, providerToDelete.value)
    showDeleteDialog.value = false
  } finally {
    deleting.value = false
  }
}

onMounted(() => store.fetchProviders(orgId.value))
</script>

<template>
  <div class="mx-auto max-w-4xl p-4 sm:p-6">
    <div class="mb-6 flex items-center justify-between">
      <h1 class="text-2xl font-bold text-gray-900">LLM Providers</h1>
      <button class="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700" @click="showCreateDialog = true">
        Add Provider
      </button>
    </div>

    <!--
      Connection-error banner driven by the background health cron. Shows
      whenever the cron's last probe of the default provider returned
      ok=false. Sits above the cards so it reads as page-level, not
      per-card. The toast in DefaultLayout hides itself when this page
      is open to avoid duplication.
    -->
    <div
      v-if="showHealthBanner"
      role="alert"
      data-testid="llm-health-banner"
      class="mb-4 rounded-lg border border-red-200 bg-red-50 p-4"
    >
      <div class="flex items-start gap-3">
        <svg class="h-5 w-5 shrink-0 text-red-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" aria-hidden="true">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01M5 19h14a2 2 0 001.84-2.75L13.74 4a2 2 0 00-3.48 0L3.16 16.25A2 2 0 005 19z" />
        </svg>
        <div class="min-w-0 flex-1">
          <p class="text-sm font-semibold text-red-900">Connection error</p>
          <p class="mt-1 break-words text-sm text-red-700">{{ healthBannerDetail }}</p>
        </div>
      </div>
    </div>

    <div v-if="store.loading" class="py-12 text-center text-gray-500">Loading providers...</div>

    <div v-else-if="store.providers.length === 0" class="rounded-lg border-2 border-dashed border-gray-300 py-12 text-center">
      <p class="text-gray-500">No LLM providers configured yet.</p>
      <button class="mt-2 text-sm text-indigo-600 hover:underline" @click="showCreateDialog = true">Add your first provider</button>
    </div>

    <!--
      Unified responsive provider card (#744). One markup tree drives both
      desktop and mobile layouts via Tailwind breakpoints — no `isMobile`
      branching. Card body collapses gracefully on narrow viewports; the
      action row wraps. Default-pill + Make-default button data-testids
      from #748 are preserved so e2e tests don't break.
    -->
    <div v-else class="space-y-4" data-testid="provider-list">
      <ProviderCard
        v-for="provider in store.providers"
        :key="provider.id"
        :provider="provider"
        :is-healthy="healthStore.isHealthy()"
        :health-failure-reason="healthStore.failureReason()"
        :setting-default-id="settingDefaultId"
        :test-failed-for-id="testStatus[provider.id] === 'fail'"
        :show-saved-tick="savedTickId === provider.id"
        @make-default="handleMakeDefault"
        @edit-credentials="openEditDialog"
        @delete="confirmDelete"
        @rename="handleRename"
        @change-model="handleChangeModel"
      />
    </div>

    <!-- Default-switch error toast (non-blocking, auto-dismisses) -->
    <div
      v-if="defaultError"
      data-testid="default-error-toast"
      class="fixed bottom-4 right-4 z-50 max-w-sm rounded-lg border border-red-300 bg-red-50 px-4 py-3 text-sm text-red-800 shadow-lg"
      role="alert"
      @click="defaultError = null"
    >
      {{ defaultError }}
    </div>

    <CreateProviderDialog
      v-if="showCreateDialog"
      :org-id="orgId"
      :is-first-provider="store.providers.length === 0"
      :on-submit="handleCreate"
      @close="showCreateDialog = false"
    />

    <EditCredentialsDialog
      v-if="showEditDialog && editingProvider"
      :provider="editingProvider"
      :org-id="orgId"
      :on-submit="handleEditSave"
      @close="closeEditDialog"
    />

    <!-- Delete Confirmation -->
    <div v-if="showDeleteDialog" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div class="w-full max-w-sm rounded-lg bg-white p-6 shadow-xl">
        <h2 class="mb-2 text-lg font-semibold text-gray-900">Delete Provider</h2>
        <p class="text-sm text-gray-600">Are you sure you want to delete <strong>{{ providerToDeleteName }}</strong>?</p>
        <div class="mt-4 flex justify-end gap-2">
          <button class="rounded px-4 py-2 text-sm text-gray-700 hover:bg-gray-100" @click="showDeleteDialog = false">Cancel</button>
          <button :disabled="deleting" class="rounded bg-red-600 px-4 py-2 text-sm text-white hover:bg-red-700 disabled:opacity-50" @click="handleDelete">
            {{ deleting ? 'Deleting...' : 'Delete' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
