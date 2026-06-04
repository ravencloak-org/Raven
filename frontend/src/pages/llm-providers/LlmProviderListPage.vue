<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useAuthStore } from "../../stores/auth"
import { useLlmProviderHealthStore } from '../../stores/llm-provider-health'
import { useLlmProvidersStore } from '../../stores/llm-providers'
import {
  PROVIDER_MODELS,
  testLlmProviderConnection,
  type CreateLlmProviderRequest,
  type LlmProvider,
  type ProviderType,
  type UpdateLlmProviderRequest,
} from '../../api/llm-providers'

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
const orgId = computed(() => authStore.orgId ?? sessionStorage.getItem("raven_org_id") ?? "")


const showCreateDialog = ref(false)
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


const providerTypes: { value: ProviderType; label: string }[] = [
  { value: 'openai', label: 'OpenAI' },
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'ollama', label: 'Ollama' },
  { value: 'custom', label: 'Custom' },
]

// Pretty-print provider type for the card body. Keep in lockstep with
// providerTypes above — single source of truth for label text.
const providerTypeLabel: Record<ProviderType, string> = {
  openai: 'OpenAI',
  anthropic: 'Anthropic',
  ollama: 'Ollama',
  custom: 'Custom',
}

// Per-provider onboarding guidance shown in the Add-Provider dialog.
// `keyHref` deep-links to the vendor's API-key console so the user can
// mint a key without leaving the flow; `keyPrefix` is shown in the
// placeholder so they know what shape the key should have; `helpText`
// is a one-liner above the API-key field. Keyed off ProviderType.
// `requiresKey=false` hides API-key inputs entirely (Edit dialog skips
// Rotate-key for these; today only Ollama). `defaultBaseUrl` auto-fills
// the Base URL input when the provider type flips; `baseUrlHelp` is a
// one-liner shown below the input.
const providerHelp: Record<ProviderType, {
  keyHref?: string
  keyPrefix?: string
  helpText: string
  requiresKey: boolean
  defaultBaseUrl?: string
  baseUrlHelp?: string
}> = {
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
    helpText: 'Ollama runs locally and needs no API key. Point Base URL at your Ollama daemon — http://localhost:11434 by default. The hosted demo can\'t reach a private LAN address, so this works for self-hosted / Tauri-desktop Raven, or expose your Ollama via a public tunnel.',
    requiresKey: false,
    defaultBaseUrl: 'http://localhost:11434',
    baseUrlHelp: 'Address of the Ollama daemon (default port 11434). Must be reachable from wherever the Raven server runs.',
  },
  custom: {
    keyPrefix: 'your provider\'s key',
    helpText: 'For any OpenAI-compatible provider (Together, Groq, Fireworks, vLLM, etc.). Set Base URL to the provider root (Raven appends /v1/models, /v1/chat/completions, etc).',
    requiresKey: true,
  },
}

function modelsForType(type: ProviderType) {
  return PROVIDER_MODELS[type] ?? []
}

function providerModel(provider: LlmProvider): string {
  const m = provider.config?.model
  return typeof m === 'string' ? m : ''
}

function onProviderTypeChange() {
  const help = providerHelp[form.value.provider]
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
    const help = providerHelp[form.value.provider]
    // The Go model declares api_key as binding:"required,min=1", so
    // keyless providers (Ollama) need a stub value to satisfy validation.
    // Backend ignores the value for these providers — it's purely a
    // schema artefact, not an actual credential. Note this in the comment
    // so a future cleanup doesn't get rid of it without also relaxing
    // the binding.
    const api_key = help.requiresKey ? form.value.api_key : 'not-required'
    await store.addProvider(orgId.value, {
      ...form.value,
      api_key,
      ...(selectedModel.value ? { config: { ...(form.value.config ?? {}), model: selectedModel.value } } : {}),
      ...(isFirst ? { is_default: true } : {}),
    })
    showCreateDialog.value = false
    form.value = { provider: 'openai', display_name: '', base_url: null, api_key: '' }
    selectedModel.value = undefined
  } catch (e: unknown) {
    createError.value = e instanceof Error ? e.message : 'Failed to create provider'
  } finally {
    creating.value = false
  }
}

const currentProviderHelp = computed(() => providerHelp[form.value.provider])

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
const editingNameId = ref<string | null>(null)
const editingNameDraft = ref('')
const savedTickId = ref<string | null>(null)
let savedTickTimer: ReturnType<typeof setTimeout> | null = null
const nameInputRef = ref<HTMLInputElement | null>(null)

function flashSavedTick(providerId: string) {
  savedTickId.value = providerId
  if (savedTickTimer) clearTimeout(savedTickTimer)
  savedTickTimer = setTimeout(() => {
    savedTickId.value = null
    savedTickTimer = null
  }, 1500)
}

async function startEditName(provider: LlmProvider) {
  editingNameId.value = provider.id
  editingNameDraft.value = provider.display_name
  await nextTick()
  nameInputRef.value?.focus()
  nameInputRef.value?.select()
}

function cancelEditName() {
  editingNameId.value = null
  editingNameDraft.value = ''
}

async function commitEditName(provider: LlmProvider) {
  const target = editingNameDraft.value.trim()
  // No-op if unchanged or empty — just close the inline editor.
  if (!target || target === provider.display_name) {
    cancelEditName()
    return
  }
  const previousName = provider.display_name
  // Optimistic local mutation: paint the new name immediately.
  const idx = store.providers.findIndex((p) => p.id === provider.id)
  if (idx !== -1) store.providers[idx] = { ...store.providers[idx], display_name: target }
  editingNameId.value = null
  editingNameDraft.value = ''
  try {
    await store.editProvider(orgId.value, provider.id, { display_name: target })
    flashSavedTick(provider.id)
  } catch (e: unknown) {
    // Roll back the optimistic name.
    const rollbackIdx = store.providers.findIndex((p) => p.id === provider.id)
    if (rollbackIdx !== -1) {
      store.providers[rollbackIdx] = { ...store.providers[rollbackIdx], display_name: previousName }
    }
    const msg = e instanceof Error ? e.message : 'Failed to save name'
    showDefaultError(`Could not save name: ${msg}`)
  }
}

async function commitModelChange(provider: LlmProvider, nextModel: string) {
  const previousConfig = provider.config ?? {}
  const previousModel = providerModel(provider)
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

// Tunnel suggestions shown in the Ollama branch so a user on the
// hosted demo (whose Vultr box can't reach `localhost`) has a one-liner
// to expose their local daemon publicly. Cloudflare's `trycloudflare`
// mode is the recommended first option — no account, no install of
// anything Raven-side, free, auto-TLS. ngrok is the fallback for users
// who already have it set up.
//
// `install` carries platform-specific install one-liners + a download
// docs URL for users who don't already have the binary. Detecting the
// host OS via navigator.userAgent picks the matching command; the docs
// link is always shown as the universal fallback.
const ollamaTunnels: {
  label: string
  command: string
  note?: string
  install: { docsHref: string; macos: string; windows: string; linux: string }
}[] = [
  {
    label: 'Cloudflare Tunnel (recommended)',
    // --http-host-header localhost is critical: Ollama's HTTP server rejects
    // any request whose Host header isn't 127.0.0.1 / localhost (security
    // hardening). cloudflared's default forwards the public tunnel hostname
    // as Host, which Ollama returns 403 to with an empty body. The flag
    // tells cloudflared to rewrite Host to "localhost" on the upstream hop
    // so Ollama treats it as a local call.
    command: 'cloudflared tunnel --url http://localhost:11434 --http-host-header localhost',
    note: 'Prints an https://<random>.trycloudflare.com URL. No account needed. The --http-host-header flag is mandatory — Ollama 403s any other Host. Paste the printed URL into Base URL above.',
    install: {
      docsHref: 'https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/',
      macos: 'brew install cloudflared',
      windows: 'winget install --id Cloudflare.cloudflared',
      linux: 'See the download page for your distro\'s package or static binary.',
    },
  },
  {
    label: 'ngrok',
    // --host-header=rewrite serves the same purpose as cloudflared's
    // --http-host-header: rewrites the Host on the upstream hop to
    // 127.0.0.1:11434 so Ollama's Host-allowlist check accepts the call.
    // Without it, ngrok forwards Host: <subdomain>.ngrok.app and Ollama 403s.
    command: 'ngrok http 11434 --host-header=rewrite',
    note: 'Free account required for stable URLs. The --host-header=rewrite flag is mandatory — Ollama 403s any other Host. Use the https Forwarding address.',
    install: {
      docsHref: 'https://ngrok.com/download',
      macos: 'brew install ngrok',
      windows: 'winget install ngrok.ngrok',
      linux: 'curl -sSL https://ngrok-agent.s3.amazonaws.com/ngrok.asc | sudo tee /etc/apt/trusted.gpg.d/ngrok.asc >/dev/null && echo "deb https://ngrok-agent.s3.amazonaws.com buster main" | sudo tee /etc/apt/sources.list.d/ngrok.list && sudo apt update && sudo apt install ngrok',
    },
  },
]

// Best-effort platform detection — userAgent is fine here because we
// only use it to pick a default install one-liner, not for anything
// security-sensitive. Falls back to the docs URL if unsure.
function detectOS(): 'macos' | 'windows' | 'linux' | 'unknown' {
  const ua = navigator.userAgent.toLowerCase()
  if (ua.includes('mac')) return 'macos'
  if (ua.includes('win')) return 'windows'
  if (ua.includes('linux')) return 'linux'
  return 'unknown'
}
const userOS = detectOS()
function installCommandForOS(t: (typeof ollamaTunnels)[number]) {
  if (userOS === 'macos') return t.install.macos
  if (userOS === 'windows') return t.install.windows
  if (userOS === 'linux') return t.install.linux
  return null
}
function installLabelForOS(): string {
  if (userOS === 'macos') return 'Install on macOS'
  if (userOS === 'windows') return 'Install on Windows'
  if (userOS === 'linux') return 'Install on Linux'
  return 'Install'
}

// Create-dialog test-connection state machine. createTestStatus drives
// the icon/colour shown on the Test button and the Create button's
// disabled state:
//   - 'idle'  : nothing tested yet (Create is disabled if the user
//               hasn't confirmed the creds work).
//   - 'testing'
//   - 'pass'  : last probe succeeded → Create unlocked.
//   - 'fail'  : last probe failed → Create stays disabled, error
//               surfaced in createTestDetail.
// Any edit to the form fields rolls the state back to 'idle' so the
// user re-tests after changing the key / endpoint.
// NB: distinct from the page-level `testStatus` map (keyed by provider
// id, used for the post-create "last test failed" warning on each card).
type TestStatus = 'idle' | 'testing' | 'pass' | 'fail'
const createTestStatus = ref<TestStatus>('idle')
const createTestDetail = ref<string>('')
async function runTestConnection() {
  createTestStatus.value = 'testing'
  createTestDetail.value = ''
  try {
    const help = providerHelp[form.value.provider]
    const api_key = help.requiresKey ? form.value.api_key : 'not-required'
    const result = await testLlmProviderConnection(orgId.value, {
      ...form.value,
      api_key,
    })
    createTestStatus.value = result.ok ? 'pass' : 'fail'
    createTestDetail.value = result.detail ?? ''
  } catch (e: unknown) {
    createTestStatus.value = 'fail'
    createTestDetail.value = e instanceof Error ? e.message : 'Test failed'
  }
}

// Invalidate the test result whenever a form field that affects the
// probe changes — so the user can't pass a test on one key, swap it,
// and slip a bad config past the gate.
watch(
  () => [form.value.provider, form.value.api_key, form.value.base_url],
  () => {
    if (createTestStatus.value !== 'idle') {
      createTestStatus.value = 'idle'
      createTestDetail.value = ''
    }
  },
)

const copied = ref<string | null>(null)
async function copyTunnel(command: string) {
  try {
    await navigator.clipboard.writeText(command)
    copied.value = command
    setTimeout(() => (copied.value === command ? (copied.value = null) : null), 1500)
  } catch {
    // Clipboard access can fail under restrictive permissions / older
    // browsers — surface the command in the UI and let the user copy
    // it manually instead of throwing.
    copied.value = null
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
      <article
        v-for="provider in store.providers"
        :key="provider.id"
        class="rounded-lg border border-gray-200 bg-white p-4 shadow-sm"
        data-testid="provider-card"
      >
        <!-- Header row: icon + (editable name | status | default pill) -->
        <div class="flex items-start gap-3">
          <!-- Provider-type icon (chip glyph for visual consistency with the M13 menu). -->
          <span
            aria-hidden="true"
            class="mt-0.5 inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-indigo-50 text-indigo-600"
          >
            <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <rect x="4" y="4" width="16" height="16" rx="2" />
              <rect x="9" y="9" width="6" height="6" />
              <path d="M9 1v3M15 1v3M9 20v3M15 20v3M1 9h3M1 15h3M20 9h3M20 15h3" />
            </svg>
          </span>

          <div class="min-w-0 flex-1">
            <!-- Editable display_name + status badge + default pill -->
            <div class="flex flex-wrap items-center gap-2">
              <input
                v-if="editingNameId === provider.id"
                ref="nameInputRef"
                v-model="editingNameDraft"
                type="text"
                data-testid="display-name-input"
                class="min-w-0 flex-1 rounded border border-indigo-300 px-2 py-1 text-base font-semibold text-gray-900 shadow-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                @keydown.enter.prevent="commitEditName(provider)"
                @keydown.esc.prevent="cancelEditName"
                @blur="commitEditName(provider)"
              />
              <button
                v-else
                type="button"
                data-testid="display-name-label"
                class="truncate rounded px-1 py-0.5 text-left text-base font-semibold text-gray-900 hover:bg-gray-50 focus:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-indigo-300"
                :title="`Click to rename — ${provider.display_name}`"
                @click="startEditName(provider)"
              >
                {{ provider.display_name }}
              </button>
              <span
                :class="[
                  'shrink-0 rounded-full px-2 py-0.5 text-xs font-medium',
                  provider.status === 'active'
                    ? 'bg-green-100 text-green-700'
                    : 'bg-gray-100 text-gray-600',
                ]"
              >
                {{ provider.status }}
              </span>
              <span
                v-if="provider.is_default"
                data-testid="default-pill"
                class="shrink-0 rounded-full bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-800 transition duration-200 ease-out"
              >
                Default
              </span>
              <span
                v-if="provider.is_default && !healthStore.isHealthy()"
                data-testid="default-connection-error"
                class="shrink-0 rounded-full bg-red-100 px-2 py-0.5 text-xs font-medium text-red-700"
                :title="healthStore.failureReason()"
              >
                Connection error
              </span>
              <span
                v-if="savedTickId === provider.id"
                data-testid="saved-tick"
                class="shrink-0 text-xs font-medium text-green-600"
                role="status"
                aria-live="polite"
              >
                Saved ✓
              </span>
            </div>

            <!-- Type + base_url meta -->
            <p class="mt-1 text-sm text-gray-500">
              {{ providerTypeLabel[provider.provider] }}
              <span v-if="provider.base_url" class="break-all"> &middot; {{ provider.base_url }}</span>
            </p>

            <!-- Inline model selector + api-key hint -->
            <div class="mt-2 flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-3">
              <label class="flex items-center gap-2 text-xs text-gray-600">
                <span class="shrink-0">Model</span>
                <select
                  data-testid="model-select"
                  class="rounded border-gray-300 px-2 py-1 text-xs shadow-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                  :value="providerModel(provider)"
                  @change="commitModelChange(provider, ($event.target as HTMLSelectElement).value)"
                >
                  <option v-if="!providerModel(provider)" value="">Select model…</option>
                  <option
                    v-for="m in modelsForType(provider.provider)"
                    :key="m.value"
                    :value="m.value"
                  >
                    {{ m.label }}
                  </option>
                </select>
              </label>
              <span class="text-xs text-gray-400">
                API key {{ provider.api_key_hint ? 'configured' : 'not set' }}
              </span>
            </div>

            <p
              v-if="!provider.is_default && testStatus[provider.id] === 'fail'"
              class="mt-2 rounded border border-amber-300 bg-amber-50 px-2 py-1 text-xs text-amber-800"
            >
              Last connection test failed — switching default may break chat.
            </p>
          </div>
        </div>

        <!-- Action row: wraps under content on narrow viewports. -->
        <div class="mt-3 flex flex-wrap justify-end gap-2 border-t border-gray-100 pt-3">
          <button
            v-if="!provider.is_default"
            data-testid="make-default-btn"
            :disabled="settingDefaultId === provider.id"
            class="rounded border border-amber-300 px-3 py-1 text-xs font-medium text-amber-800 hover:bg-amber-50 disabled:opacity-50"
            @click="handleMakeDefault(provider.id)"
          >
            {{ settingDefaultId === provider.id ? 'Setting...' : 'Make default' }}
          </button>
          <button
            data-testid="edit-credentials-btn"
            class="rounded border border-gray-300 px-3 py-1 text-xs text-gray-700 hover:bg-gray-50"
            @click="openEditDialog(provider)"
          >
            Edit credentials
          </button>
          <button
            data-testid="delete-btn"
            class="rounded border border-red-300 px-3 py-1 text-xs text-red-700 hover:bg-red-50"
            @click="confirmDelete(provider.id, provider.display_name)"
          >
            Delete
          </button>
        </div>
      </article>
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
    <div
      v-if="showCreateDialog"
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
            <div v-if="showTunnelHint" class="mt-3 rounded-md border border-gray-200 bg-gray-50 p-3">
              <p class="text-xs font-medium text-gray-700">
                {{ ravenHost }} can't reach <code class="font-mono">{{ form.base_url }}</code> on your machine. Expose Ollama temporarily:
              </p>
              <ul class="mt-2 space-y-3">
                <li v-for="t in ollamaTunnels" :key="t.label" class="text-xs">
                  <div class="font-medium text-gray-600">{{ t.label }}</div>
                  <!-- Install one-liner for the user's detected OS, with a
                       fallback link to the upstream download docs that
                       cover Linux distros + manual installs. -->
                  <div v-if="installCommandForOS(t)" class="mt-0.5">
                    <div class="text-[11px] text-gray-500">{{ installLabelForOS() }}:</div>
                    <div class="mt-0.5 flex items-center gap-1.5">
                      <code class="flex-1 truncate rounded bg-gray-800 px-2 py-1 text-[11px] text-gray-100">{{ installCommandForOS(t) }}</code>
                      <button
                        type="button"
                        class="shrink-0 rounded border border-gray-300 bg-white px-2 py-1 text-[11px] text-gray-700 hover:bg-gray-100"
                        @click="copyTunnel(installCommandForOS(t) ?? '')"
                      >
                        {{ copied === installCommandForOS(t) ? 'Copied' : 'Copy' }}
                      </button>
                    </div>
                  </div>
                  <a
                    :href="t.install.docsHref"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="mt-1 inline-flex items-center text-[11px] text-indigo-600 underline hover:text-indigo-800"
                  >
                    Other install options
                    <svg class="ml-0.5 h-3 w-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/><path d="M15 3h6v6"/><path d="M10 14L21 3"/></svg>
                  </a>
                  <div class="mt-1.5">
                    <div class="text-[11px] text-gray-500">Then run:</div>
                    <div class="mt-0.5 flex items-center gap-1.5">
                      <code class="flex-1 truncate rounded bg-gray-900 px-2 py-1 text-[11px] text-gray-100">{{ t.command }}</code>
                      <button
                        type="button"
                        class="shrink-0 rounded border border-gray-300 bg-white px-2 py-1 text-[11px] text-gray-700 hover:bg-gray-100"
                        @click="copyTunnel(t.command)"
                      >
                        {{ copied === t.command ? 'Copied' : 'Copy' }}
                      </button>
                    </div>
                  </div>
                  <p v-if="t.note" class="mt-0.5 text-gray-500">{{ t.note }}</p>
                </li>
              </ul>
              <p class="mt-2 text-[11px] text-amber-700">
                <strong>Heads up:</strong> a public tunnel exposes your local Ollama to anyone with the URL. Close the tunnel when you're done, or restrict by IP / auth.
              </p>
            </div>
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
          <p v-if="store.providers.length === 0" class="text-xs text-gray-400">
            This will be set as your default provider since it's the first one.
          </p>
          <p v-if="createError" class="text-red-500 text-sm">{{ createError }}</p>
          <!-- Test-connection gate. The Create button is locked until
               the probe returns 'pass' so a bad key / unreachable
               endpoint can't slip into the DB. -->
          <div class="rounded-md border p-3 text-xs" :class="{
            'border-gray-200 bg-gray-50': createTestStatus === 'idle',
            'border-blue-200 bg-blue-50': createTestStatus === 'testing',
            'border-green-200 bg-green-50': createTestStatus === 'pass',
            'border-red-200 bg-red-50': createTestStatus === 'fail',
          }">
            <div class="flex items-center justify-between gap-2">
              <span class="font-medium" :class="{
                'text-gray-700': createTestStatus === 'idle',
                'text-blue-700': createTestStatus === 'testing',
                'text-green-700': createTestStatus === 'pass',
                'text-red-700': createTestStatus === 'fail',
              }">
                <template v-if="createTestStatus === 'idle'">Test the connection before saving</template>
                <template v-else-if="createTestStatus === 'testing'">Testing…</template>
                <template v-else-if="createTestStatus === 'pass'">✓ Connection looks good</template>
                <template v-else>✕ Connection failed</template>
              </span>
              <button
                type="button"
                class="rounded border border-gray-300 bg-white px-3 py-1 text-xs text-gray-700 hover:bg-gray-100 disabled:opacity-50"
                :disabled="createTestStatus === 'testing' || !form.display_name || (currentProviderHelp.requiresKey && !form.api_key)"
                @click="runTestConnection"
              >
                {{ createTestStatus === 'pass' ? 'Re-test' : createTestStatus === 'testing' ? 'Testing…' : 'Test connection' }}
              </button>
            </div>
            <p v-if="createTestDetail" class="mt-1 text-xs" :class="{
              'text-green-700': createTestStatus === 'pass',
              'text-red-700': createTestStatus === 'fail',
              'text-gray-600': createTestStatus !== 'pass' && createTestStatus !== 'fail',
            }">
              {{ createTestDetail }}
            </p>
          </div>
          <div class="flex justify-end gap-2 pt-2">
            <button type="button" class="rounded px-4 py-2 text-sm text-gray-700 hover:bg-gray-100" @click="showCreateDialog = false">Cancel</button>
            <button type="submit" :disabled="creating || createTestStatus !== 'pass' || !form.display_name || (currentProviderHelp.requiresKey && !form.api_key)" class="rounded bg-indigo-600 px-4 py-2 text-sm text-white hover:bg-indigo-700 disabled:opacity-50">
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
