<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useAuthStore } from '../../stores/auth'
import { useServerConfigStore } from '../../stores/server-config'
import { useRouter } from 'vue-router'
import PasskeysSection from './PasskeysSection.vue'

type Tab = 'general' | 'models' | 'api-keys' | 'authentication' | 'privacy'

const apiUrl = import.meta.env.VITE_API_BASE_URL ?? ''
const auth = useAuthStore()
const serverConfig = useServerConfigStore()
const router = useRouter()

const activeTab = ref<Tab>('general')

// ─── General tab: workspace name ──────────────────────────────────────────
const workspaceName = ref('')
const workspaceId = ref<string | null>(null)
const generalLoading = ref(false)
const generalError = ref<string | null>(null)
const generalToast = ref<string | null>(null)

async function loadWorkspace() {
  if (!auth.orgId) return
  try {
    const res = await fetch(`${apiUrl}/api/v1/orgs/${auth.orgId}/workspaces`, {
      credentials: 'include',
    })
    if (!res.ok) return
    const data = await res.json()
    const list = Array.isArray(data) ? data : (data.items ?? [])
    const ws = list[0]
    if (ws) {
      workspaceId.value = ws.id
      workspaceName.value = ws.name
    }
  } catch {
    // best-effort; tab still renders
  }
}

async function saveWorkspaceName() {
  if (!workspaceId.value || !auth.orgId || workspaceName.value.trim().length < 1) return
  generalLoading.value = true
  generalError.value = null
  generalToast.value = null
  try {
    const res = await fetch(
      `${apiUrl}/api/v1/orgs/${auth.orgId}/workspaces/${workspaceId.value}`,
      {
        method: 'PUT',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: workspaceName.value.trim() }),
      },
    )
    if (!res.ok) {
      const data = await res.json().catch(() => ({}))
      throw new Error(data.message || data.error || `Save failed (${res.status})`)
    }
    generalToast.value = 'Saved'
    setTimeout(() => (generalToast.value = null), 2500)
  } catch (e) {
    generalError.value = e instanceof Error ? e.message : String(e)
  } finally {
    generalLoading.value = false
  }
}

// ─── Models tab: list Ollama models + trigger pull ────────────────────────
interface OllamaModel {
  name: string
  size: number
  digest: string
}

const ollamaModels = ref<OllamaModel[]>([])
const ollamaError = ref<string | null>(null)
const ollamaLoading = ref(false)
const newModelName = ref('')
const pullProgress = ref<{ model: string; percent: number } | null>(null)

async function refreshOllamaModels() {
  ollamaLoading.value = true
  ollamaError.value = null
  try {
    const { invoke } = await import('@tauri-apps/api/core')
    ollamaModels.value = await invoke<OllamaModel[]>('ollama_list_models')
  } catch (e) {
    ollamaError.value =
      e instanceof Error ? e.message : 'Tauri not available — Ollama listing only works in the desktop app.'
    ollamaModels.value = []
  } finally {
    ollamaLoading.value = false
  }
}

async function pullModel() {
  const model = newModelName.value.trim()
  if (!model) return
  pullProgress.value = { model, percent: 0 }
  ollamaError.value = null
  try {
    const { invoke } = await import('@tauri-apps/api/core')
    const { listen } = await import('@tauri-apps/api/event')
    const unlisten = await listen<{ model: string; percent: number }>(
      'ollama:pull-progress',
      (e) => {
        if (e.payload.model === model && pullProgress.value) {
          pullProgress.value.percent = e.payload.percent
        }
      },
    )
    try {
      await invoke('ollama_pull_model', { model })
      newModelName.value = ''
      await refreshOllamaModels()
    } finally {
      unlisten()
    }
  } catch (e) {
    ollamaError.value = e instanceof Error ? e.message : String(e)
  } finally {
    pullProgress.value = null
  }
}

function formatGb(bytes: number): string {
  return (bytes / (1024 * 1024 * 1024)).toFixed(1) + ' GB'
}

// ─── Privacy tab: telemetry toggle (frontend opt-in) ──────────────────────
const TELEMETRY_KEY = 'raven.local.telemetry'
const telemetryEnabled = ref(localStorage.getItem(TELEMETRY_KEY) === 'true')
const privacyToast = ref<string | null>(null)

function toggleTelemetry(value: boolean) {
  telemetryEnabled.value = value
  localStorage.setItem(TELEMETRY_KEY, value ? 'true' : 'false')
  privacyToast.value = value ? 'Telemetry enabled' : 'Telemetry disabled'
  setTimeout(() => (privacyToast.value = null), 2500)
}

// ─── lifecycle ────────────────────────────────────────────────────────────
const showModelsTab = computed(() => serverConfig.singleUser)

onMounted(async () => {
  await loadWorkspace()
  if (showModelsTab.value) {
    await refreshOllamaModels()
  }
})
</script>

<template>
  <div class="settings-page p-6 max-w-3xl mx-auto">
    <h1 class="text-2xl font-bold text-neutral-900 dark:text-white mb-6">Settings</h1>

    <!-- Tab strip -->
    <div class="border-b border-neutral-200 dark:border-neutral-800 mb-6">
      <nav class="flex gap-1" aria-label="Settings tabs">
        <button
          v-for="t in ['general', 'models', 'api-keys', 'authentication', 'privacy'] as Tab[]"
          v-show="t !== 'models' || showModelsTab"
          :key="t"
          class="px-4 py-2 text-sm font-medium border-b-2 transition-colors"
          :class="
            activeTab === t
              ? 'border-amber-500 text-amber-600 dark:text-amber-400'
              : 'border-transparent text-neutral-500 hover:text-neutral-900 dark:hover:text-white'
          "
          :data-test="`tab-${t}`"
          @click="activeTab = t"
        >
          {{
            t === 'general' ? 'General'
            : t === 'models' ? 'Models'
            : t === 'api-keys' ? 'API Keys'
            : t === 'authentication' ? 'Authentication'
            : 'Privacy'
          }}
        </button>
      </nav>
    </div>

    <!-- General -->
    <section v-if="activeTab === 'general'" data-test="panel-general">
      <h2 class="text-lg font-semibold mb-4 text-neutral-900 dark:text-white">Workspace</h2>
      <label class="block mb-4">
        <span class="text-sm text-neutral-700 dark:text-neutral-300">Workspace name</span>
        <input
          v-model="workspaceName"
          type="text"
          class="w-full mt-1 rounded-lg border border-neutral-300 dark:border-neutral-700 bg-white dark:bg-neutral-900 text-neutral-900 dark:text-white px-3 py-2"
          :disabled="generalLoading || !workspaceId"
        />
      </label>
      <button
        :disabled="generalLoading || !workspaceId || workspaceName.trim().length < 1"
        class="bg-amber-500 hover:bg-amber-600 disabled:opacity-50 text-white font-semibold px-4 py-2 rounded-lg transition-colors"
        @click="saveWorkspaceName"
      >
        {{ generalLoading ? 'Saving…' : 'Save' }}
      </button>
      <p v-if="generalError" class="text-red-500 text-sm mt-3">{{ generalError }}</p>
      <p v-if="generalToast" class="text-emerald-600 dark:text-emerald-400 text-sm mt-3">{{ generalToast }}</p>

      <p class="text-xs text-neutral-500 mt-6">
        Data directory changes (moving the database, models, and uploads to another disk)
        are deferred to a follow-up issue. For now, the data dir is fixed at the docker
        compose volume defaults.
      </p>
    </section>

    <!-- Models (desktop only) -->
    <section v-else-if="activeTab === 'models' && showModelsTab" data-test="panel-models">
      <h2 class="text-lg font-semibold mb-4 text-neutral-900 dark:text-white">Local Ollama models</h2>

      <div v-if="ollamaLoading" class="text-sm text-neutral-500">Loading…</div>
      <p v-else-if="ollamaError" class="text-red-500 text-sm mb-4">{{ ollamaError }}</p>
      <ul v-else-if="ollamaModels.length > 0" class="space-y-2 mb-6">
        <li
          v-for="m in ollamaModels"
          :key="m.digest"
          class="rounded-lg border border-neutral-300 dark:border-neutral-700 p-3 flex justify-between items-center"
        >
          <div>
            <div class="font-semibold text-neutral-900 dark:text-white">{{ m.name }}</div>
            <div class="text-xs text-neutral-500">{{ formatGb(m.size) }}</div>
          </div>
        </li>
      </ul>
      <p v-else class="text-sm text-neutral-500 mb-6">No models pulled yet.</p>

      <h3 class="text-sm font-semibold mb-2 text-neutral-700 dark:text-neutral-300">Pull a model</h3>
      <div class="flex gap-2">
        <input
          v-model="newModelName"
          type="text"
          placeholder="e.g. llama3.1:8b"
          class="flex-1 rounded-lg border border-neutral-300 dark:border-neutral-700 bg-white dark:bg-neutral-900 text-neutral-900 dark:text-white px-3 py-2"
          :disabled="!!pullProgress"
        />
        <button
          :disabled="!!pullProgress || newModelName.trim().length < 1"
          class="bg-amber-500 hover:bg-amber-600 disabled:opacity-50 text-white font-semibold px-4 py-2 rounded-lg transition-colors"
          @click="pullModel"
        >
          {{ pullProgress ? `Pulling ${pullProgress.percent}%` : 'Pull' }}
        </button>
      </div>
      <progress
        v-if="pullProgress"
        :value="pullProgress.percent"
        max="100"
        class="w-full h-2 mt-3"
      />
    </section>

    <!-- API Keys: link to existing page (don't duplicate the CRUD) -->
    <section v-else-if="activeTab === 'api-keys'" data-test="panel-api-keys">
      <h2 class="text-lg font-semibold mb-4 text-neutral-900 dark:text-white">API Keys</h2>
      <p class="text-sm text-neutral-700 dark:text-neutral-300 mb-4">
        BYOK provider keys (OpenAI, Anthropic, Cohere, custom) are managed on the
        dedicated providers page. Each provider lives at the org level and is
        encrypted at rest.
      </p>
      <button
        class="bg-amber-500 hover:bg-amber-600 text-white font-semibold px-4 py-2 rounded-lg transition-colors"
        @click="router.push('/llm-providers')"
      >
        Manage providers
      </button>
    </section>

    <!-- Authentication: passkeys (#773) -->
    <PasskeysSection v-else-if="activeTab === 'authentication'" />

    <!-- Privacy -->
    <section v-else-if="activeTab === 'privacy'" data-test="panel-privacy">
      <h2 class="text-lg font-semibold mb-4 text-neutral-900 dark:text-white">Privacy</h2>

      <label class="flex items-start gap-3 mb-4 cursor-pointer">
        <input
          type="checkbox"
          :checked="telemetryEnabled"
          class="mt-1"
          data-test="telemetry-toggle"
          @change="toggleTelemetry(($event.target as HTMLInputElement).checked)"
        />
        <div>
          <div class="font-semibold text-neutral-900 dark:text-white">Share anonymous usage data</div>
          <div class="text-xs text-neutral-500">
            Off by default. Only sends counters like "Ollama model pulled" — never prompts,
            content, or PII. You can change this anytime.
          </div>
        </div>
      </label>
      <p v-if="privacyToast" class="text-emerald-600 dark:text-emerald-400 text-sm">{{ privacyToast }}</p>

      <hr class="my-6 border-neutral-200 dark:border-neutral-800" />

      <h3 class="text-sm font-semibold mb-2 text-neutral-700 dark:text-neutral-300">Data export &amp; factory reset</h3>
      <p class="text-xs text-neutral-500">
        Export and factory-reset workflows are deferred to a follow-up issue.
        For now, you can manually back up the docker compose volumes:
        <code class="font-mono">~/Library/Containers/io.ravencloak.ai/Data/docker/volumes/</code>
        on macOS, or the equivalent docker volumes on Windows/Linux.
      </p>
    </section>
  </div>
</template>
