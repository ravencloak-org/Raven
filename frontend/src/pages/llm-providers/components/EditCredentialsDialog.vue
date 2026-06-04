<script setup lang="ts">
import { computed, ref } from 'vue'
import {
  PROVIDER_MODELS,
  type LlmProvider,
  type ProviderType,
  type TestProviderRequest,
  type UpdateLlmProviderRequest,
} from '../../../api/llm-providers'
import { useTestConnectionGate } from '../../../composables/useTestConnectionGate'
import { PROVIDER_HELP } from '../providerHelp'

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
//   - any keystroke in provider / base_url / rotate-key rolls the gate
//     status back to `idle`, so a passing probe can't be smuggled past a
//     later edit.
// ---------------------------------------------------------------------------

const props = defineProps<{
  provider: LlmProvider
  orgId: string
  onSubmit: (payload: UpdateLlmProviderRequest) => Promise<void>
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const editForm = ref<{
  provider: ProviderType
  display_name: string
  base_url: string
  model: string
}>({
  provider: props.provider.provider,
  display_name: props.provider.display_name,
  base_url: props.provider.base_url ?? '',
  model:
    typeof props.provider.config?.model === 'string'
      ? (props.provider.config.model as string)
      : '',
})
const editInitialBaseUrl = ref<string>(props.provider.base_url ?? '')
const showRotateKey = ref(false)
const rotateApiKey = ref('')
const editError = ref('')
const editSaving = ref(false)

function modelsForType(type: ProviderType) {
  return PROVIDER_MODELS[type] ?? []
}

const editProviderHelp = computed(() => PROVIDER_HELP[editForm.value.provider])
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

// Any keystroke in fields that affect the credential probe (provider,
// base_url, rotate-key input) invalidates a prior pass. The provider
// is read-only in the dialog but we still watch it to stay honest in
// case that ever changes.
const gate = useTestConnectionGate({
  buildPayload: (): TestProviderRequest => {
    // Decide which body shape to send. When base_url changed OR the
    // user typed a new key, we have inline values to probe with. When
    // neither changed we don't have a secret in memory — fall back to
    // the {provider_id} shape so the server uses the stored key.
    const baseUrlChanged = editForm.value.base_url !== editInitialBaseUrl.value
    const rotateProvided = showRotateKey.value && rotateApiKey.value.length > 0
    if (rotateProvided) {
      return {
        provider: editForm.value.provider,
        api_key: rotateApiKey.value,
        ...(editShowBaseUrl.value
          ? { base_url: editForm.value.base_url || null }
          : {}),
      }
    }
    if (baseUrlChanged) {
      return {
        provider_id: props.provider.id,
        ...(editShowBaseUrl.value
          ? { base_url: editForm.value.base_url || null }
          : {}),
      }
    }
    return { provider_id: props.provider.id }
  },
  invalidateOn: [
    () => editForm.value.provider,
    () => editForm.value.base_url,
    () => rotateApiKey.value,
    () => showRotateKey.value,
  ],
  orgId: () => props.orgId,
})

const editCanSave = computed(() => {
  if (editSaving.value) return false
  if (!editForm.value.display_name) return false
  if (editGateRequired.value && gate.status.value !== 'pass') return false
  return true
})

async function handleEditSave() {
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
      const existingConfig = (props.provider.config ?? {}) as Record<string, unknown>
      payload.config = { ...existingConfig, model: editForm.value.model }
    }
    await props.onSubmit(payload)
    emit('close')
  } catch (e: unknown) {
    editError.value = e instanceof Error ? e.message : 'Failed to save provider'
  } finally {
    editSaving.value = false
  }
}
</script>

<template>
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
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
              :disabled="gate.status.value === 'testing'"
              class="rounded bg-gray-700 px-3 py-1 text-xs text-white hover:bg-gray-800 disabled:opacity-50"
              @click="gate.runTest"
            >
              {{ gate.status.value === 'testing' ? 'Testing...' : 'Test now' }}
            </button>
          </div>
          <p
            v-if="gate.status.value === 'pass'"
            class="mt-2 text-xs text-green-700"
          >
            ✓ Connection OK{{ gate.detail.value ? ` — ${gate.detail.value}` : '' }}
          </p>
          <p
            v-else-if="gate.status.value === 'fail'"
            class="mt-2 text-xs text-red-600"
          >
            ✕ {{ gate.detail.value || 'Connection failed' }}
          </p>
          <p
            v-else-if="gate.status.value === 'idle'"
            class="mt-2 text-xs text-gray-500"
          >
            Required: the {{ showRotateKey && rotateApiKey ? 'new key' : 'updated Base URL' }} must pass before Save is enabled.
          </p>
        </div>

        <p v-if="editError" class="text-red-500 text-sm">{{ editError }}</p>
        <div class="flex justify-end gap-2 pt-2">
          <button type="button" class="rounded px-4 py-2 text-sm text-gray-700 hover:bg-gray-100" @click="emit('close')">Cancel</button>
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
</template>
