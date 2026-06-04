<script setup lang="ts">
import { computed, ref } from 'vue'
import {
  PROVIDER_MODELS,
  type CreateLlmProviderRequest,
  type ProviderType,
  type TestProviderRequest,
} from '../../../api/llm-providers'
import { useTestConnectionGate } from '../../../composables/useTestConnectionGate'
import { PROVIDER_HELP, PROVIDER_TYPES } from '../providerHelp'
import TunnelHint from './TunnelHint.vue'

const props = defineProps<{
  orgId: string
  isFirstProvider: boolean
  onSubmit: (payload: CreateLlmProviderRequest) => Promise<void>
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const form = ref<CreateLlmProviderRequest>({
  provider: 'openai',
  display_name: '',
  base_url: null,
  api_key: '',
})
// The model lives in CreateLlmProviderRequest.config rather than as a
// top-level field — keep it as a separate component-local ref and merge
// at submit time so the v-model on the dropdown actually round-trips.
const selectedModel = ref<string | undefined>(undefined)
const creating = ref(false)
const createError = ref('')

function modelsForType(type: ProviderType) {
  return PROVIDER_MODELS[type] ?? []
}

const currentProviderHelp = computed(() => PROVIDER_HELP[form.value.provider])

function onProviderTypeChange() {
  const help = PROVIDER_HELP[form.value.provider]
  // Apply provider-specific Base URL default (Ollama → localhost:11434);
  // null for cloud providers that don't need it; empty string for custom
  // so the input is shown but unset.
  if (help.defaultBaseUrl !== undefined) {
    form.value.base_url = help.defaultBaseUrl
  } else if (form.value.provider === 'custom') {
    form.value.base_url = ''
  } else {
    form.value.base_url = null
  }
  // Reset the per-provider model selection to the first model in the
  // list, so the previously-selected (and possibly unrelated) model
  // doesn't leak when switching providers.
  const models = modelsForType(form.value.provider)
  selectedModel.value = models[0]?.value
  // Ollama doesn't require an API key — clear any stale value the user
  // typed before switching providers.
  if (!help.requiresKey) {
    form.value.api_key = ''
  }
}

// Create-dialog test-connection gate. The Create button is locked
// until the probe returns 'pass' so a bad key / unreachable
// endpoint can't slip into the DB. Any edit to provider / api_key /
// base_url rolls the state back to 'idle' so the user re-tests.
const gate = useTestConnectionGate({
  buildPayload: (): TestProviderRequest => {
    const help = PROVIDER_HELP[form.value.provider]
    const api_key = help.requiresKey ? form.value.api_key : 'not-required'
    return {
      provider: form.value.provider,
      api_key,
      base_url: form.value.base_url,
    }
  },
  invalidateOn: [
    () => form.value.provider,
    () => form.value.api_key,
    () => form.value.base_url,
  ],
  orgId: () => props.orgId,
})

async function handleCreate() {
  creating.value = true
  createError.value = ''
  try {
    // Mark the first provider as default automatically — the chat / RAG
    // path 500s ("No active 'X' provider config found") until something
    // is the default, so a fresh org has zero chance of a working chat
    // unless we set this on their behalf.
    const help = PROVIDER_HELP[form.value.provider]
    // The Go model declares api_key as binding:"required,min=1", so
    // keyless providers (Ollama) need a stub value to satisfy validation.
    // Backend ignores the value for these providers — it's purely a
    // schema artefact, not an actual credential. Note this in the comment
    // so a future cleanup doesn't get rid of it without also relaxing
    // the binding.
    const api_key = help.requiresKey ? form.value.api_key : 'not-required'
    const payload: CreateLlmProviderRequest = {
      ...form.value,
      api_key,
      ...(selectedModel.value
        ? { config: { ...(form.value.config ?? {}), model: selectedModel.value } }
        : {}),
      ...(props.isFirstProvider ? { is_default: true } : {}),
    }
    await props.onSubmit(payload)
    form.value = { provider: 'openai', display_name: '', base_url: null, api_key: '' }
    selectedModel.value = undefined
    emit('close')
  } catch (e: unknown) {
    createError.value = e instanceof Error ? e.message : 'Failed to create provider'
  } finally {
    creating.value = false
  }
}

// True only when a loopback Base URL is paired with a non-loopback
// browser origin — i.e., the user typed `http://localhost:...` while
// hitting Raven on a real public host (demo.ravencloak.org, etc.).
// On Raven-running-locally (desktop / self-host) the loopback address
// works as-is and the tunnel hint would be noise.
const isLoopbackBaseURL = computed(() => {
  const raw = (form.value.base_url ?? '').trim().toLowerCase()
  if (!raw) return false
  return (
    raw.startsWith('http://localhost') ||
    raw.startsWith('https://localhost') ||
    raw.startsWith('http://127.') ||
    raw.startsWith('https://127.') ||
    raw.startsWith('http://[::1]') ||
    raw.startsWith('https://[::1]') ||
    raw.startsWith('http://0.0.0.0') ||
    raw.startsWith('https://0.0.0.0')
  )
})

const ravenHost = computed(() => window.location.hostname)
const ravenIsRemote = computed(() => {
  const host = ravenHost.value
  return host !== 'localhost' && host !== '127.0.0.1' && host !== '[::1]' && host !== '0.0.0.0'
})

const showTunnelHint = computed(
  () => form.value.provider === 'ollama' && isLoopbackBaseURL.value && ravenIsRemote.value,
)
</script>

<template>
  <div
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
    role="dialog"
    aria-modal="true"
    aria-labelledby="add-llm-provider-title"
  >
    <div class="w-full max-w-2xl max-h-[90vh] overflow-y-auto rounded-lg bg-white p-6 shadow-xl">
      <h2 id="add-llm-provider-title" class="mb-4 text-lg font-semibold">Add LLM Provider</h2>
      <form class="space-y-4" @submit.prevent="handleCreate">
        <div>
          <label class="block text-sm font-medium text-gray-700">Provider Type</label>
          <select v-model="form.provider" class="mt-1 block w-full rounded border-gray-300 shadow-sm" @change="onProviderTypeChange">
            <option v-for="pt in PROVIDER_TYPES" :key="pt.value" :value="pt.value">{{ pt.label }}</option>
          </select>
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-700">Display Name</label>
          <input v-model="form.display_name" type="text" required class="mt-1 block w-full rounded border-gray-300 shadow-sm" placeholder="e.g. OpenAI Production" />
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-700">Model</label>
          <select v-model="selectedModel" class="mt-1 block w-full rounded border-gray-300 shadow-sm">
            <option v-for="m in modelsForType(form.provider)" :key="m.value" :value="m.value">{{ m.label }}</option>
          </select>
        </div>
        <div v-if="form.provider === 'custom' || form.provider === 'ollama'">
          <label class="block text-sm font-medium text-gray-700">Base URL</label>
          <input
            v-model="form.base_url"
            type="url"
            class="mt-1 block w-full rounded border-gray-300 shadow-sm"
            :placeholder="currentProviderHelp.defaultBaseUrl ?? 'https://api.example.com/v1'"
          />
          <p v-if="currentProviderHelp.baseUrlHelp" class="mt-1 text-xs text-gray-500">
            {{ currentProviderHelp.baseUrlHelp }}
          </p>
          <!-- Tunnel hints: only when the user has typed a loopback
               Base URL while sitting on a remote Raven (e.g. the
               hosted demo). For self-hosted / Tauri Raven the
               loopback address works as-is and the hint is noise. -->
          <TunnelHint
            v-if="showTunnelHint"
            :raven-host="ravenHost"
            :base-url="form.base_url ?? ''"
          />
        </div>
        <div v-if="currentProviderHelp.requiresKey">
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
        </div>
        <p v-else class="text-xs text-gray-500">
          {{ currentProviderHelp.helpText }}
        </p>
        <p v-if="isFirstProvider" class="text-xs text-gray-400">
          This will be set as your default provider since it's the first one.
        </p>
        <p v-if="createError" class="text-red-500 text-sm">{{ createError }}</p>
        <!-- Test-connection gate. The Create button is locked until
             the probe returns 'pass' so a bad key / unreachable
             endpoint can't slip into the DB. -->
        <div
class="rounded-md border p-3 text-xs" :class="{
          'border-gray-200 bg-gray-50': gate.status.value === 'idle',
          'border-blue-200 bg-blue-50': gate.status.value === 'testing',
          'border-green-200 bg-green-50': gate.status.value === 'pass',
          'border-red-200 bg-red-50': gate.status.value === 'fail',
        }">
          <div class="flex items-center justify-between gap-2">
            <span
class="font-medium" :class="{
              'text-gray-700': gate.status.value === 'idle',
              'text-blue-700': gate.status.value === 'testing',
              'text-green-700': gate.status.value === 'pass',
              'text-red-700': gate.status.value === 'fail',
            }">
              <template v-if="gate.status.value === 'idle'">Test the connection before saving</template>
              <template v-else-if="gate.status.value === 'testing'">Testing…</template>
              <template v-else-if="gate.status.value === 'pass'">✓ Connection looks good</template>
              <template v-else>✕ Connection failed</template>
            </span>
            <button
              type="button"
              class="rounded border border-gray-300 bg-white px-3 py-1 text-xs text-gray-700 hover:bg-gray-100 disabled:opacity-50"
              :disabled="gate.status.value === 'testing' || !form.display_name || (currentProviderHelp.requiresKey && !form.api_key)"
              @click="gate.runTest"
            >
              {{ gate.status.value === 'pass' ? 'Re-test' : gate.status.value === 'testing' ? 'Testing…' : 'Test connection' }}
            </button>
          </div>
          <p
v-if="gate.detail.value" class="mt-1 text-xs" :class="{
            'text-green-700': gate.status.value === 'pass',
            'text-red-700': gate.status.value === 'fail',
            'text-gray-600': gate.status.value !== 'pass' && gate.status.value !== 'fail',
          }">
            {{ gate.detail.value }}
          </p>
        </div>
        <div class="flex justify-end gap-2 pt-2">
          <button type="button" class="rounded px-4 py-2 text-sm text-gray-700 hover:bg-gray-100" @click="emit('close')">Cancel</button>
          <button type="submit" :disabled="creating || gate.status.value !== 'pass' || !form.display_name || (currentProviderHelp.requiresKey && !form.api_key)" class="rounded bg-indigo-600 px-4 py-2 text-sm text-white hover:bg-indigo-700 disabled:opacity-50">
            {{ creating ? 'Creating...' : 'Create' }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>
