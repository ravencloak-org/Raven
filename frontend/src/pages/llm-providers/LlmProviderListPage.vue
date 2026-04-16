<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useAuthStore } from "../../stores/auth"
import { useLlmProvidersStore } from '../../stores/llm-providers'
import { useMobile } from '../../composables/useMediaQuery'
import { PROVIDER_MODELS, type ProviderType, type CreateLlmProviderRequest } from '../../api/llm-providers'

const store = useLlmProvidersStore()
const { isMobile } = useMobile()
const authStore = useAuthStore()
const orgId = authStore.orgId ?? sessionStorage.getItem("raven_org_id") ?? ""


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

const testingId = ref<string | null>(null)
const testResult = ref<Record<string, { success: boolean; message: string; latency_ms?: number }>>({})

const providerTypes: { value: ProviderType; label: string }[] = [
  { value: 'openai', label: 'OpenAI' },
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'ollama', label: 'Ollama' },
  { value: 'custom', label: 'Custom' },
]

function modelsForType(type: ProviderType) {
  return PROVIDER_MODELS[type] ?? []
}

function onProviderTypeChange() {
  void modelsForType(form.value.provider)
  
  form.value.base_url = form.value.provider === 'custom' || form.value.provider === 'ollama' ? '' : null
}

async function handleCreate() {
  creating.value = true
  try {
    await store.addProvider(orgId, { ...form.value })
    showCreateDialog.value = false
    form.value = { provider: 'openai', display_name: '', base_url: null, api_key: '' }
  } finally {
    creating.value = false
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
    await store.removeProvider(orgId, providerToDelete.value)
    showDeleteDialog.value = false
  } finally {
    deleting.value = false
  }
}

async function handleTest(id: string) {
  testingId.value = id
  const result = await store.testProviderConnection(orgId, id)
  testResult.value[id] = result
  testingId.value = null
}

function statusColor(status: string) {
  return status === 'active' ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-600'
}

function mobileStatusClass(status: string): string {
  if (status === 'active') return 'bg-green-900 text-green-300'
  return 'bg-slate-700 text-slate-300'
}

function modelCountForProvider(providerType: ProviderType): number {
  return (PROVIDER_MODELS[providerType] ?? []).length
}

onMounted(() => store.fetchProviders(orgId))
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
              <span :class="['rounded-full px-2 py-0.5 text-xs font-medium', statusColor(provider.status)]">{{ provider.status }}</span>
            </div>
            <p class="mt-1 text-sm text-gray-500">
              {{ provider.provider.toUpperCase() }} &middot; {{ provider.model }}
              <span v-if="provider.base_url"> &middot; {{ provider.base_url }}</span>
            </p>
            <p class="mt-1 text-xs text-gray-400">
              {{ provider.workspace_id ? `Workspace: ${provider.workspace_id}` : 'Org-wide' }}
              &middot; API key {{ provider.api_key_set ? 'configured' : 'not set' }}
            </p>
          </div>
          <div class="flex gap-2">
            <button
              class="rounded border border-gray-300 px-3 py-1 text-xs text-gray-700 hover:bg-gray-50 disabled:opacity-50"
              :disabled="testingId === provider.id"
              @click="handleTest(provider.id)"
            >
              {{ testingId === provider.id ? 'Testing...' : 'Test' }}
            </button>
            <button class="rounded border border-red-300 px-3 py-1 text-xs text-red-700 hover:bg-red-50" @click="confirmDelete(provider.id, provider.display_name)">
              Delete
            </button>
          </div>
        </div>
        <div v-if="testResult[provider.id]" class="mt-2 rounded px-3 py-2 text-sm" :class="testResult[provider.id].success ? 'bg-green-50 text-green-800' : 'bg-red-50 text-red-800'">
          {{ testResult[provider.id].message }}
          <span v-if="testResult[provider.id].latency_ms"> ({{ testResult[provider.id].latency_ms }}ms)</span>
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
            :class="mobileStatusClass(provider.status)"
          >
            {{ provider.status }}
          </span>
        </div>

        <!-- Subtitle: model count -->
        <p class="text-slate-400 text-xs mt-1">
          {{ modelCountForProvider(provider.provider) }} model{{ modelCountForProvider(provider.provider) === 1 ? '' : 's' }}
          &bull; {{ provider.provider.toUpperCase() }}
        </p>

        <!-- Test result (if present) -->
        <div v-if="testResult[provider.id]" class="mt-2 rounded px-3 py-2 text-xs" :class="testResult[provider.id].success ? 'bg-green-900 text-green-300' : 'bg-red-900 text-red-300'">
          {{ testResult[provider.id].message }}
          <span v-if="testResult[provider.id].latency_ms"> ({{ testResult[provider.id].latency_ms }}ms)</span>
        </div>

        <!-- Action row -->
        <div class="border-t border-slate-700 mt-2.5 pt-2.5 flex items-center justify-end gap-2">
          <button
            class="border border-slate-600 text-slate-300 text-xs px-3 py-1 rounded-lg disabled:opacity-50"
            :disabled="testingId === provider.id"
            @click="handleTest(provider.id)"
          >
            {{ testingId === provider.id ? 'Testing...' : 'Test' }}
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
            <select v-model="selectedModel" class="mt-1 block w-full rounded border-gray-300 shadow-sm">
              <option v-for="m in modelsForType(form.provider)" :key="m.value" :value="m.value">{{ m.label }}</option>
            </select>
          </div>
          <div v-if="form.provider === 'custom' || form.provider === 'ollama'">
            <label class="block text-sm font-medium text-gray-700">Base URL</label>
            <input v-model="form.base_url" type="url" class="mt-1 block w-full rounded border-gray-300 shadow-sm" placeholder="https://api.example.com/v1" />
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700">API Key</label>
            <input v-model="form.api_key" type="password" class="mt-1 block w-full rounded border-gray-300 shadow-sm" placeholder="sk-..." />
          </div>
          <div class="flex justify-end gap-2 pt-2">
            <button type="button" class="rounded px-4 py-2 text-sm text-gray-700 hover:bg-gray-100" @click="showCreateDialog = false">Cancel</button>
            <button type="submit" :disabled="creating || !form.display_name" class="rounded bg-indigo-600 px-4 py-2 text-sm text-white hover:bg-indigo-700 disabled:opacity-50">
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
