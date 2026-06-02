<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useAuthStore } from "../../stores/auth"
import { useLlmProvidersStore } from '../../stores/llm-providers'
import { useMobile } from '../../composables/useMediaQuery'
import {
  PROVIDER_MODELS,
  testLlmProviderConnection,
  type LlmProvider,
  type ProviderType,
  type CreateLlmProviderRequest,
  type UpdateLlmProviderRequest,
} from '../../api/llm-providers'

const store = useLlmProvidersStore()
const { isMobile } = useMobile()
const authStore = useAuthStore()
const orgId = computed(() => authStore.orgId ?? sessionStorage.getItem("raven_org_id") ?? "")


const showCreateDialog = ref(false)
const form = ref<CreateLlmProviderRequest>({
  provider: 'openai',
  display_name: '',

  base_url: null,
  api_key: '',
})
const creating = ref(false)

const showDeleteDialog = ref(false)
const providerToDelete = ref<string | null>(null)
const providerToDeleteName = ref('')
const deleting = ref(false)


const providerTypes: { value: ProviderType; label: string }[] = [
  { value: 'openai', label: 'OpenAI' },
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'ollama', label: 'Ollama' },
  { value: 'custom', label: 'Custom' },
]

// Per-provider onboarding guidance shown in the Add-Provider dialog.
// `keyHref` deep-links to the vendor's API-key console so the user can
// mint a key without leaving the flow; `keyPrefix` is shown in the
// placeholder so they know what shape the key should have; `helpText`
// is a one-liner above the API-key field. Keyed off ProviderType.
// `requiresKey=false` hides API-key inputs entirely (Edit dialog skips
// Rotate-key for these; today only Ollama).
const providerHelp: Record<ProviderType, { keyHref?: string; keyPrefix?: string; helpText: string; requiresKey: boolean }> = {
  openai: {
    keyHref: 'https://platform.openai.com/api-keys',
    keyPrefix: 'sk-...',
    helpText: 'Generate a Secret Key in the OpenAI dashboard and paste it below.',
    requiresKey: true,
  },
  anthropic: {
    keyHref: 'https://console.anthropic.com/settings/keys',
    keyPrefix: 'sk-ant-...',
    helpText: 'Generate an API key in the Anthropic Console and paste it below.',
    requiresKey: true,
  },
  ollama: {
    keyPrefix: 'ollama',
    helpText: 'Ollama runs locally — no API key required, but the field can\'t be empty. Use any placeholder (e.g. "ollama") and set the Base URL to your daemon.',
    requiresKey: false,
  },
  custom: {
    keyPrefix: 'your provider\'s key',
    helpText: 'For any OpenAI-compatible provider (Together, Groq, Fireworks, vLLM, etc.). Set Base URL to the provider\'s /v1 endpoint.',
    requiresKey: true,
  },
}

function modelsForType(type: ProviderType) {
  return PROVIDER_MODELS[type] ?? []
}

function onProviderTypeChange() {
  void modelsForType(form.value.provider)
  
  form.value.base_url = form.value.provider === 'custom' || form.value.provider === 'ollama' ? '' : null
}

const createError = ref('')

async function handleCreate() {
  creating.value = true
  createError.value = ''
  try {
    // Mark the first provider as default automatically — the chat / RAG
    // path 500s ("No active 'X' provider config found") until something
    // is the default, so a fresh org has zero chance of a working chat
    // unless we set this on their behalf.
    const isFirst = store.providers.length === 0
    await store.addProvider(orgId.value, {
      ...form.value,
      ...(isFirst ? { is_default: true } : {}),
    })
    showCreateDialog.value = false
    form.value = { provider: 'openai', display_name: '', base_url: null, api_key: '' }
  } catch (e: unknown) {
    createError.value = e instanceof Error ? e.message : 'Failed to create provider'
  } finally {
    creating.value = false
  }
}

const currentProviderHelp = computed(() => providerHelp[form.value.provider])

// ---------------------------------------------------------------------------
// Edit-credentials dialog (#742)
//
// Reuses the Create dialog markup but:
//   - provider is read-only (changing provider type is a delete-and-create);
//   - api_key starts hidden — clicking "Rotate API key" reveals an empty
//     input, and only then does the PUT carry `api_key`. Skip = key untouched
//     server-side (backend preserves the stored ciphertext on partial PUT);
//   - re-runs Test Connection when EITHER base_url changed OR the rotate
//     input has a non-empty value. If neither changed the gate hits the
//     /test endpoint with {provider_id} so the stored key is used without
//     ever leaving the server;
//   - any keystroke in provider / base_url / rotate-key rolls testStatus
//     back to `idle`, so a passing probe can't be smuggled past a later
//     edit.
// ---------------------------------------------------------------------------
const showEditDialog = ref(false)
const editingProvider = ref<LlmProvider | null>(null)
const editForm = ref<{
  provider: ProviderType
  display_name: string
  base_url: string
  model: string
}>({
  provider: 'openai',
  display_name: '',
  base_url: '',
  model: '',
})
const editInitialBaseUrl = ref<string>('')
const showRotateKey = ref(false)
const rotateApiKey = ref('')
const editTestStatus = ref<'idle' | 'testing' | 'pass' | 'fail'>('idle')
const editTestDetail = ref('')
const editError = ref('')
const editSaving = ref(false)

const editProviderHelp = computed(() => providerHelp[editForm.value.provider])
const editProviderHasKey = computed(() => editProviderHelp.value.requiresKey)
// Edit dialog needs Base URL whenever the provider type does — same
// rule as Create.
const editShowBaseUrl = computed(
  () => editForm.value.provider === 'custom' || editForm.value.provider === 'ollama',
)
// Test-Connection re-gate is REQUIRED when either base_url changed from
// what's persisted OR the user typed a new key into Rotate. Otherwise
// nothing security-sensitive is changing and Save is allowed without
// a probe.
const editGateRequired = computed(() => {
  const baseUrlChanged = editForm.value.base_url !== editInitialBaseUrl.value
  const rotateProvided = showRotateKey.value && rotateApiKey.value.length > 0
  return baseUrlChanged || rotateProvided
})
const editCanSave = computed(() => {
  if (editSaving.value) return false
  if (!editForm.value.display_name) return false
  if (editGateRequired.value && editTestStatus.value !== 'pass') return false
  return true
})

function openEditDialog(provider: LlmProvider) {
  editingProvider.value = provider
  const model =
    typeof provider.config?.model === 'string' ? (provider.config.model as string) : ''
  editForm.value = {
    provider: provider.provider,
    display_name: provider.display_name,
    base_url: provider.base_url ?? '',
    model,
  }
  editInitialBaseUrl.value = provider.base_url ?? ''
  showRotateKey.value = false
  rotateApiKey.value = ''
  editTestStatus.value = 'idle'
  editTestDetail.value = ''
  editError.value = ''
  showEditDialog.value = true
}

function closeEditDialog() {
  showEditDialog.value = false
  editingProvider.value = null
}

// Any keystroke in fields that affect the credential probe (provider,
// base_url, rotate-key input) invalidates a prior pass. The provider
// is read-only in the dialog but we still watch it to stay honest in
// case that ever changes.
watch(
  [
    () => editForm.value.provider,
    () => editForm.value.base_url,
    () => rotateApiKey.value,
    () => showRotateKey.value,
  ],
  () => {
    if (!showEditDialog.value) return
    if (editTestStatus.value !== 'idle') {
      editTestStatus.value = 'idle'
      editTestDetail.value = ''
    }
  },
)

async function runEditTest() {
  if (!editingProvider.value) return
  editTestStatus.value = 'testing'
  editTestDetail.value = ''
  try {
    // Decide which body shape to send. When base_url changed OR the
    // user typed a new key, we have inline values to probe with. When
    // neither changed we don't have a secret in memory — fall back to
    // the {provider_id} shape so the server uses the stored key.
    const baseUrlChanged = editForm.value.base_url !== editInitialBaseUrl.value
    const rotateProvided = showRotateKey.value && rotateApiKey.value.length > 0
    const payload = rotateProvided
      ? {
          provider: editForm.value.provider,
          api_key: rotateApiKey.value,
          ...(editShowBaseUrl.value
            ? { base_url: editForm.value.base_url || null }
            : {}),
        }
      : baseUrlChanged
        ? {
            provider_id: editingProvider.value.id,
            ...(editShowBaseUrl.value
              ? { base_url: editForm.value.base_url || null }
              : {}),
          }
        : { provider_id: editingProvider.value.id }
    const result = await testLlmProviderConnection(orgId.value, payload)
    if (result.ok) {
      editTestStatus.value = 'pass'
      editTestDetail.value = result.detail ?? ''
    } else {
      editTestStatus.value = 'fail'
      editTestDetail.value = result.detail ?? 'Probe rejected the credentials.'
    }
  } catch (e: unknown) {
    editTestStatus.value = 'fail'
    editTestDetail.value = e instanceof Error ? e.message : 'Test connection failed'
  }
}

async function handleEditSave() {
  if (!editingProvider.value) return
  editSaving.value = true
  editError.value = ''
  try {
    const payload: UpdateLlmProviderRequest = {
      display_name: editForm.value.display_name,
    }
    // base_url: include when the dialog renders it. Empty string is
    // explicitly nulled (matches Create behaviour); a value is sent as-is.
    if (editShowBaseUrl.value) {
      payload.base_url = editForm.value.base_url === '' ? null : editForm.value.base_url
    }
    // Rotate semantics: omit api_key entirely when the user did NOT
    // click Rotate — backend (PR #749) preserves the stored ciphertext
    // on a partial PUT.
    if (showRotateKey.value && rotateApiKey.value.length > 0) {
      payload.api_key = rotateApiKey.value
    }
    // Persist the selected model under config.model. Merge into the
    // existing config so unrelated keys aren't dropped.
    if (editForm.value.model) {
      const existingConfig = (editingProvider.value.config ?? {}) as Record<string, unknown>
      payload.config = { ...existingConfig, model: editForm.value.model }
    }
    await store.editProvider(orgId.value, editingProvider.value.id, payload)
    closeEditDialog()
  } catch (e: unknown) {
    editError.value = e instanceof Error ? e.message : 'Failed to save provider'
  } finally {
    editSaving.value = false
  }
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
  <div class="mx-auto max-w-4xl p-6">
    <div class="mb-6 flex items-center justify-between">
      <h1 class="text-2xl font-bold text-gray-900">LLM Providers</h1>
      <button class="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700" @click="showCreateDialog = true">
        Add Provider
      </button>
    </div>

    <div v-if="store.loading" class="py-12 text-center text-gray-500">Loading providers...</div>

    <div v-else-if="store.providers.length === 0" class="rounded-lg border-2 border-dashed border-gray-300 py-12 text-center">
      <p class="text-gray-500">No LLM providers configured yet.</p>
      <button class="mt-2 text-sm text-indigo-600 hover:underline" @click="showCreateDialog = true">Add your first provider</button>
    </div>

    <!-- Desktop: full provider cards -->
    <div v-else-if="!isMobile" class="space-y-4">
      <div v-for="provider in store.providers" :key="provider.id" class="rounded-lg border border-gray-200 bg-white p-4 shadow-sm">
        <div class="flex items-start justify-between">
          <div>
            <div class="flex items-center gap-2">
              <h3 class="font-semibold text-gray-900">{{ provider.display_name }}</h3>
              <span :class="['rounded-full px-2 py-0.5 text-xs font-medium', provider.status === 'active' ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-600']">{{ provider.status }}</span>
            </div>
            <p class="mt-1 text-sm text-gray-500">
              {{ provider.provider.toUpperCase() }} &middot; {{ provider.provider }}
              <span v-if="provider.base_url"> &middot; {{ provider.base_url }}</span>
            </p>
            <p class="mt-1 text-xs text-gray-400">
              API key {{ !!provider.api_key_hint ? 'configured' : 'not set' }}
            </p>
          </div>
          <div class="flex gap-2">
            <button class="rounded border border-gray-300 px-3 py-1 text-xs text-gray-700 hover:bg-gray-50" @click="openEditDialog(provider)">
              Edit credentials
            </button>
            <button class="rounded border border-red-300 px-3 py-1 text-xs text-red-700 hover:bg-red-50" @click="confirmDelete(provider.id, provider.display_name)">
              Delete
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Mobile: compact card list -->
    <div v-else class="space-y-3">
      <div
        v-for="provider in store.providers"
        :key="provider.id"
        class="bg-slate-800 rounded-xl p-3.5"
      >
        <!-- Header: name + status badge -->
        <div class="flex items-start justify-between gap-2">
          <span class="text-white font-semibold text-[15px] truncate">{{ provider.display_name }}</span>
          <span
            class="shrink-0 inline-block rounded-full px-2 py-0.5 text-xs font-medium"
          >
            {{ provider.status }}
          </span>
        </div>

        <p class="text-slate-400 text-xs mt-1">
          {{ provider.provider.toUpperCase() }}
        </p>

        <!-- Action row -->
        <div class="border-t border-slate-700 mt-2.5 pt-2.5 flex items-center justify-end gap-2">
          <button
            class="border border-slate-500 text-slate-200 text-xs px-3 py-1 rounded-lg"
            @click="openEditDialog(provider)"
          >
            Edit credentials
          </button>
          <button
            class="border border-red-500 text-red-400 text-xs px-3 py-1 rounded-lg"
            @click="confirmDelete(provider.id, provider.display_name)"
          >
            Delete
          </button>
        </div>
      </div>
    </div>

    <!-- Create Dialog -->
    <div v-if="showCreateDialog" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div class="w-full max-w-md rounded-lg bg-white p-6 shadow-xl">
        <h2 class="mb-4 text-lg font-semibold">Add LLM Provider</h2>
        <form class="space-y-4" @submit.prevent="handleCreate">
          <div>
            <label class="block text-sm font-medium text-gray-700">Provider Type</label>
            <select v-model="form.provider" class="mt-1 block w-full rounded border-gray-300 shadow-sm" @change="onProviderTypeChange">
              <option v-for="pt in providerTypes" :key="pt.value" :value="pt.value">{{ pt.label }}</option>
            </select>
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700">Display Name</label>
            <input v-model="form.display_name" type="text" required class="mt-1 block w-full rounded border-gray-300 shadow-sm" placeholder="e.g. OpenAI Production" />
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700">Model</label>
            <select  class="mt-1 block w-full rounded border-gray-300 shadow-sm">
              <option v-for="m in modelsForType(form.provider)" :key="m.value" :value="m.value">{{ m.label }}</option>
            </select>
          </div>
          <div v-if="form.provider === 'custom' || form.provider === 'ollama'">
            <label class="block text-sm font-medium text-gray-700">Base URL</label>
            <input v-model="form.base_url" type="url" class="mt-1 block w-full rounded border-gray-300 shadow-sm" placeholder="https://api.example.com/v1" />
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700">API Key</label>
            <p class="mt-1 text-xs text-gray-500">
              {{ currentProviderHelp.helpText }}
              <a
                v-if="currentProviderHelp.keyHref"
                :href="currentProviderHelp.keyHref"
                target="_blank"
                rel="noopener noreferrer"
                class="ml-1 inline-flex items-center text-indigo-600 underline hover:text-indigo-800"
              >
                Get a key
                <svg class="ml-0.5 h-3 w-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/><path d="M15 3h6v6"/><path d="M10 14L21 3"/></svg>
              </a>
            </p>
            <input
              v-model="form.api_key"
              type="password"
              autocomplete="off"
              class="mt-2 block w-full rounded border-gray-300 shadow-sm"
              :placeholder="currentProviderHelp.keyPrefix ?? 'sk-...'"
            />
            <p v-if="store.providers.length === 0" class="mt-1 text-xs text-gray-400">
              This will be set as your default provider since it's the first one.
            </p>
          </div>
          <p v-if="createError" class="text-red-500 text-sm">{{ createError }}</p>
          <div class="flex justify-end gap-2 pt-2">
            <button type="button" class="rounded px-4 py-2 text-sm text-gray-700 hover:bg-gray-100" @click="showCreateDialog = false">Cancel</button>
            <button type="submit" :disabled="creating || !form.display_name || !form.api_key" class="rounded bg-indigo-600 px-4 py-2 text-sm text-white hover:bg-indigo-700 disabled:opacity-50">
              {{ creating ? 'Creating...' : 'Create' }}
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- Edit Credentials Dialog -->
    <div v-if="showEditDialog" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div class="w-full max-w-md rounded-lg bg-white p-6 shadow-xl">
        <h2 class="mb-4 text-lg font-semibold">Edit credentials</h2>
        <form class="space-y-4" @submit.prevent="handleEditSave">
          <div>
            <label class="block text-sm font-medium text-gray-700">Provider Type</label>
            <input
              :value="editForm.provider"
              type="text"
              readonly
              disabled
              class="mt-1 block w-full rounded border-gray-200 bg-gray-50 text-gray-600 shadow-sm"
            />
            <p class="mt-1 text-xs text-gray-400">
              Provider type can't be changed. Delete and re-create to switch.
            </p>
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700">Display Name</label>
            <input
              v-model="editForm.display_name"
              type="text"
              required
              class="mt-1 block w-full rounded border-gray-300 shadow-sm"
            />
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700">Model</label>
            <select v-model="editForm.model" class="mt-1 block w-full rounded border-gray-300 shadow-sm">
              <option v-for="m in modelsForType(editForm.provider)" :key="m.value" :value="m.value">{{ m.label }}</option>
            </select>
          </div>
          <div v-if="editShowBaseUrl">
            <label class="block text-sm font-medium text-gray-700">Base URL</label>
            <input
              v-model="editForm.base_url"
              type="url"
              class="mt-1 block w-full rounded border-gray-300 shadow-sm"
              placeholder="https://api.example.com/v1"
            />
          </div>

          <!-- Rotate API key disclosure: hidden entirely for keyless providers (Ollama). -->
          <div v-if="editProviderHasKey">
            <button
              v-if="!showRotateKey"
              type="button"
              class="rounded border border-gray-300 px-3 py-1 text-xs text-gray-700 hover:bg-gray-50"
              @click="showRotateKey = true"
            >
              Rotate API key
            </button>
            <div v-else>
              <label class="block text-sm font-medium text-gray-700">New API Key</label>
              <p class="mt-1 text-xs text-gray-500">
                {{ editProviderHelp.helpText }}
                <a
                  v-if="editProviderHelp.keyHref"
                  :href="editProviderHelp.keyHref"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="ml-1 inline-flex items-center text-indigo-600 underline hover:text-indigo-800"
                >
                  Get a key
                </a>
              </p>
              <input
                v-model="rotateApiKey"
                type="password"
                autocomplete="off"
                class="mt-2 block w-full rounded border-gray-300 shadow-sm"
                :placeholder="editProviderHelp.keyPrefix ?? 'sk-...'"
              />
              <button
                type="button"
                class="mt-1 text-xs text-gray-500 hover:underline"
                @click="showRotateKey = false; rotateApiKey = ''"
              >
                Cancel rotation
              </button>
            </div>
          </div>

          <!-- Test-Connection gate. Only visible when a credential-affecting
               field changed; result is required to be `pass` before Save. -->
          <div v-if="editGateRequired" class="rounded border border-gray-200 bg-gray-50 p-3">
            <div class="flex items-center justify-between">
              <span class="text-sm font-medium text-gray-700">Test connection</span>
              <button
                type="button"
                :disabled="editTestStatus === 'testing'"
                class="rounded bg-gray-700 px-3 py-1 text-xs text-white hover:bg-gray-800 disabled:opacity-50"
                @click="runEditTest"
              >
                {{ editTestStatus === 'testing' ? 'Testing...' : 'Test now' }}
              </button>
            </div>
            <p
              v-if="editTestStatus === 'pass'"
              class="mt-2 text-xs text-green-700"
            >
              ✓ Connection OK{{ editTestDetail ? ` — ${editTestDetail}` : '' }}
            </p>
            <p
              v-else-if="editTestStatus === 'fail'"
              class="mt-2 text-xs text-red-600"
            >
              ✕ {{ editTestDetail || 'Connection failed' }}
            </p>
            <p
              v-else-if="editTestStatus === 'idle'"
              class="mt-2 text-xs text-gray-500"
            >
              Required: the {{ showRotateKey && rotateApiKey ? 'new key' : 'updated Base URL' }} must pass before Save is enabled.
            </p>
          </div>

          <p v-if="editError" class="text-red-500 text-sm">{{ editError }}</p>
          <div class="flex justify-end gap-2 pt-2">
            <button type="button" class="rounded px-4 py-2 text-sm text-gray-700 hover:bg-gray-100" @click="closeEditDialog">Cancel</button>
            <button
              type="submit"
              :disabled="!editCanSave"
              class="rounded bg-indigo-600 px-4 py-2 text-sm text-white hover:bg-indigo-700 disabled:opacity-50"
            >
              {{ editSaving ? 'Saving...' : 'Save' }}
            </button>
          </div>
        </form>
      </div>
    </div>

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
