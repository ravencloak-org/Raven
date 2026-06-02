<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useAuthStore } from "../../stores/auth"
import { useLlmProvidersStore } from '../../stores/llm-providers'
import { useMobile } from '../../composables/useMediaQuery'
import { PROVIDER_MODELS, type ProviderType, type CreateLlmProviderRequest } from '../../api/llm-providers'

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

// In-memory connection-test status keyed by provider id. The Create dialog
// (and a future Edit flow) writes here when a "Test connection" call comes
// back; the list page reads it to warn before switching default to a
// provider whose last test failed. Not persisted — session-only.
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
const providerHelp: Record<ProviderType, { keyHref?: string; keyPrefix?: string; helpText: string }> = {
  openai: {
    keyHref: 'https://platform.openai.com/api-keys',
    keyPrefix: 'sk-...',
    helpText: 'Generate a Secret Key in the OpenAI dashboard and paste it below.',
  },
  anthropic: {
    keyHref: 'https://console.anthropic.com/settings/keys',
    keyPrefix: 'sk-ant-...',
    helpText: 'Generate an API key in the Anthropic Console and paste it below.',
  },
  ollama: {
    keyPrefix: 'ollama',
    helpText: 'Ollama runs locally — no API key required, but the field can\'t be empty. Use any placeholder (e.g. "ollama") and set the Base URL to your daemon.',
  },
  custom: {
    keyPrefix: 'your provider\'s key',
    helpText: 'For any OpenAI-compatible provider (Together, Groq, Fireworks, vLLM, etc.). Set Base URL to the provider\'s /v1 endpoint.',
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
              <span
                v-if="provider.is_default"
                data-testid="default-pill"
                class="rounded-full bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-800 transition duration-200 ease-out"
              >
                Default
              </span>
            </div>
            <p class="mt-1 text-sm text-gray-500">
              {{ provider.provider.toUpperCase() }} &middot; {{ provider.provider }}
              <span v-if="provider.base_url"> &middot; {{ provider.base_url }}</span>
            </p>
            <p class="mt-1 text-xs text-gray-400">
              API key {{ !!provider.api_key_hint ? 'configured' : 'not set' }}
            </p>
            <p
              v-if="!provider.is_default && testStatus[provider.id] === 'fail'"
              class="mt-2 rounded border border-amber-300 bg-amber-50 px-2 py-1 text-xs text-amber-800"
            >
              Last connection test failed — switching default may break chat.
            </p>
          </div>
          <div class="flex gap-2">
            <button
              v-if="!provider.is_default"
              data-testid="make-default-btn"
              :disabled="settingDefaultId === provider.id"
              class="rounded border border-amber-300 px-3 py-1 text-xs font-medium text-amber-800 hover:bg-amber-50 disabled:opacity-50"
              @click="handleMakeDefault(provider.id)"
            >
              {{ settingDefaultId === provider.id ? 'Setting...' : 'Make default' }}
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
        <!-- Header: name + status badge + default pill -->
        <div class="flex items-start justify-between gap-2">
          <span class="text-white font-semibold text-[15px] truncate">{{ provider.display_name }}</span>
          <div class="flex shrink-0 items-center gap-1.5">
            <span class="inline-block rounded-full px-2 py-0.5 text-xs font-medium">
              {{ provider.status }}
            </span>
            <span
              v-if="provider.is_default"
              data-testid="default-pill-mobile"
              class="inline-block rounded-full bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-800 transition duration-200 ease-out"
            >
              Default
            </span>
          </div>
        </div>

        <p class="text-slate-400 text-xs mt-1">
          {{ provider.provider.toUpperCase() }}
        </p>

        <p
          v-if="!provider.is_default && testStatus[provider.id] === 'fail'"
          class="mt-2 rounded border border-amber-400/40 bg-amber-500/10 px-2 py-1 text-[11px] text-amber-300"
        >
          Last connection test failed — switching default may break chat.
        </p>

        <!-- Action row -->
        <div class="border-t border-slate-700 mt-2.5 pt-2.5 flex items-center justify-end gap-2">
          <button
            v-if="!provider.is_default"
            data-testid="make-default-btn-mobile"
            :disabled="settingDefaultId === provider.id"
            class="border border-amber-400 text-amber-300 text-xs px-3 py-1 rounded-lg disabled:opacity-50"
            @click="handleMakeDefault(provider.id)"
          >
            {{ settingDefaultId === provider.id ? 'Setting...' : 'Make default' }}
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
