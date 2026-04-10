<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useApiKeysStore } from '../../stores/apikeys'
import { pipe, split, map, filter, isTruthy } from 'remeda'
import { useMobile } from '../../composables/useMediaQuery'

const store = useApiKeysStore()
const { isMobile } = useMobile()

// --- Create dialog state ---
const showCreateDialog = ref(false)
const newKeyName = ref('')
const newKeyDomains = ref('')
const newKeyRateLimit = ref(1000)
const creating = ref(false)

// --- Revoke confirmation state ---
const showRevokeDialog = ref(false)
const keyToRevoke = ref<string | null>(null)
const keyToRevokeName = ref('')
const revoking = ref(false)

// --- Newly created key display ---
const showNewKeyBanner = ref(false)
const newRawKey = ref('')
const copiedKey = ref(false)
const copiedEmbed = ref(false)

// --- Embed code ---
// TODO: Update embed URL when the widget script is deployed
const WIDGET_SCRIPT_URL = import.meta.env.VITE_WIDGET_URL ?? 'https://cdn.raven.example/widget.js'

const embedCode = computed(() => {
  if (!newRawKey.value) return ''
  return `<script src="${WIDGET_SCRIPT_URL}" data-api-key="${newRawKey.value}" async><` + '/script>'
})

onMounted(() => store.fetchKeys())

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  })
}

function openCreateDialog() {
  newKeyName.value = ''
  newKeyDomains.value = ''
  newKeyRateLimit.value = 1000
  showCreateDialog.value = true
}

async function handleCreate() {
  if (!newKeyName.value.trim()) return
  creating.value = true
  try {
    const domains = pipe(
      newKeyDomains.value,
      split(','),
      map((d) => d.trim()),
      filter(isTruthy),
    )
    const result = await store.create({
      name: newKeyName.value.trim(),
      allowed_domains: domains,
      rate_limit: newKeyRateLimit.value,
    })
    newRawKey.value = result.raw_key
    showCreateDialog.value = false
    showNewKeyBanner.value = true
    copiedKey.value = false
    copiedEmbed.value = false
  } finally {
    creating.value = false
  }
}

function promptRevoke(keyId: string, keyName: string) {
  keyToRevoke.value = keyId
  keyToRevokeName.value = keyName
  showRevokeDialog.value = true
}

async function confirmRevoke() {
  if (!keyToRevoke.value) return
  revoking.value = true
  try {
    await store.revoke(keyToRevoke.value)
    showRevokeDialog.value = false
  } finally {
    revoking.value = false
  }
}

async function copyToClipboard(text: string, type: 'key' | 'embed') {
  await navigator.clipboard.writeText(text)
  if (type === 'key') {
    copiedKey.value = true
    setTimeout(() => (copiedKey.value = false), 2000)
  } else {
    copiedEmbed.value = true
    setTimeout(() => (copiedEmbed.value = false), 2000)
  }
}

function dismissNewKeyBanner() {
  showNewKeyBanner.value = false
  newRawKey.value = ''
  store.clearLastCreatedKey()
}
</script>

<template>
  <div class="p-6 max-w-5xl">
    <!-- Header -->
    <div class="mb-6 flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold text-gray-900">API Keys</h1>
        <p class="mt-1 text-sm text-gray-500">
          Manage API keys for embedding the Raven widget on your sites.
        </p>
      </div>
      <button
        class="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 min-h-[44px]"
        @click="openCreateDialog"
      >
        Create API Key
      </button>
    </div>

    <!-- Newly created key banner (shown once) -->
    <div
      v-if="showNewKeyBanner"
      class="mb-6 rounded-lg border border-green-200 bg-green-50 p-4"
    >
      <div class="flex items-start justify-between">
        <div class="flex-1">
          <h3 class="text-sm font-semibold text-green-800">API Key Created Successfully</h3>
          <p class="mt-1 text-xs text-green-700">
            Copy this key now. You will not be able to see it again.
          </p>

          <!-- Raw key -->
          <div class="mt-3 flex items-center gap-2">
            <code class="flex-1 rounded bg-white px-3 py-2 text-sm font-mono text-gray-800 border border-green-200 select-all">
              {{ newRawKey }}
            </code>
            <button
              class="rounded-md px-3 py-2 text-sm font-medium"
              :class="copiedKey ? 'bg-green-600 text-white' : 'bg-white text-gray-700 border border-gray-300 hover:bg-gray-50'"
              @click="copyToClipboard(newRawKey, 'key')"
            >
              {{ copiedKey ? 'Copied!' : 'Copy Key' }}
            </button>
          </div>

          <!-- Embed code snippet -->
          <div class="mt-3">
            <p class="text-xs font-medium text-green-700 mb-1">Embed code:</p>
            <div class="flex items-center gap-2">
              <code class="flex-1 rounded bg-white px-3 py-2 text-xs font-mono text-gray-700 border border-green-200 break-all select-all">
                {{ embedCode }}
              </code>
              <button
                class="rounded-md px-3 py-2 text-sm font-medium whitespace-nowrap"
                :class="copiedEmbed ? 'bg-green-600 text-white' : 'bg-white text-gray-700 border border-gray-300 hover:bg-gray-50'"
                @click="copyToClipboard(embedCode, 'embed')"
              >
                {{ copiedEmbed ? 'Copied!' : 'Copy Embed' }}
              </button>
            </div>
          </div>
        </div>
        <button
          class="ml-4 text-green-400 hover:text-green-600 min-h-[44px] min-w-[44px] flex items-center justify-center"
          aria-label="Dismiss"
          @click="dismissNewKeyBanner"
        >
          <svg class="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
            <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd" />
          </svg>
        </button>
      </div>
    </div>

    <!-- Loading -->
    <div v-if="store.loading" class="text-gray-500">Loading API keys...</div>

    <!-- Error -->
    <div v-else-if="store.error" class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700">
      {{ store.error }}
    </div>

    <!-- Empty state -->
    <div
      v-else-if="store.keys.length === 0"
      class="rounded-xl border border-dashed border-gray-300 bg-white p-12 text-center"
    >
      <p class="text-gray-500">No API keys yet. Create one to get started.</p>
    </div>

    <!-- Keys table -->
    <div v-else class="overflow-hidden rounded-xl border border-gray-200 bg-white shadow-sm">
      <table class="min-w-full divide-y divide-gray-200">
        <thead class="bg-gray-50">
          <tr>
            <th class="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Name</th>
            <th class="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Key Prefix</th>
            <th class="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Domains</th>
            <th class="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Rate Limit</th>
            <th class="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Created</th>
            <th class="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Status</th>
            <th class="px-6 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500">Actions</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-200">
          <tr v-for="key in store.keys" :key="key.id">
            <td class="whitespace-nowrap px-6 py-4 text-sm font-medium text-gray-900">
              {{ key.name }}
            </td>
            <td class="whitespace-nowrap px-6 py-4 text-sm text-gray-500">
              <code class="rounded bg-gray-100 px-2 py-0.5 font-mono text-xs">{{ key.key_prefix }}...</code>
            </td>
            <td class="px-6 py-4 text-sm text-gray-500">
              <span v-if="key.allowed_domains.length === 0" class="text-gray-400 italic">Any</span>
              <span v-else>{{ key.allowed_domains.join(', ') }}</span>
            </td>
            <td class="whitespace-nowrap px-6 py-4 text-sm text-gray-500">
              {{ key.rate_limit.toLocaleString() }} req/hr
            </td>
            <td class="whitespace-nowrap px-6 py-4 text-sm text-gray-500">
              {{ formatDate(key.created_at) }}
            </td>
            <td class="whitespace-nowrap px-6 py-4">
              <span
                class="inline-flex rounded-full px-2 py-0.5 text-xs font-semibold"
                :class="key.status === 'active' ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'"
              >
                {{ key.status }}
              </span>
            </td>
            <td class="whitespace-nowrap px-6 py-4 text-right text-sm">
              <button
                v-if="key.status === 'active'"
                class="text-red-600 hover:text-red-800 font-medium min-h-[44px] min-w-[44px]"
                @click="promptRevoke(key.id, key.name)"
              >
                Revoke
              </button>
              <span v-else class="text-gray-400">--</span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Create API Key Dialog (modal overlay) -->
    <div
      v-if="showCreateDialog"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
      @click.self="showCreateDialog = false"
    >
      <div class="w-full max-w-md rounded-xl bg-white p-6 shadow-xl">
        <h2 class="text-lg font-semibold text-gray-900">Create API Key</h2>
        <p class="mt-1 text-sm text-gray-500">The key value will be shown only once after creation.</p>

        <form class="mt-4 space-y-4" @submit.prevent="handleCreate">
          <!-- Name -->
          <div>
            <label for="key-name" class="block text-sm font-medium text-gray-700">Name</label>
            <input
              id="key-name"
              v-model="newKeyName"
              type="text"
              required
              placeholder="e.g. Production Widget"
              class="mt-1 block w-full rounded-lg border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
              :class="isMobile ? 'min-h-[48px] text-[15px]' : ''"
            />
          </div>

          <!-- Allowed Domains -->
          <div>
            <label for="key-domains" class="block text-sm font-medium text-gray-700">
              Allowed Domains
            </label>
            <input
              id="key-domains"
              v-model="newKeyDomains"
              type="text"
              placeholder="example.com, app.example.com"
              class="mt-1 block w-full rounded-lg border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
              :class="isMobile ? 'min-h-[48px] text-[15px]' : ''"
            />
            <p class="mt-1 text-xs text-gray-400">Comma-separated. Leave blank to allow any domain.</p>
          </div>

          <!-- Rate Limit -->
          <div>
            <label for="key-rate-limit" class="block text-sm font-medium text-gray-700">
              Rate Limit (requests/hour)
            </label>
            <input
              id="key-rate-limit"
              v-model.number="newKeyRateLimit"
              type="number"
              min="1"
              required
              class="mt-1 block w-full rounded-lg border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
              :class="isMobile ? 'min-h-[48px] text-[15px]' : ''"
            />
          </div>

          <!-- Actions -->
          <div
            class="flex gap-3 pt-2"
            :class="isMobile ? 'flex-col' : 'flex-row justify-end'"
          >
            <button
              type="button"
              class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 min-h-[44px]"
              :class="isMobile ? 'w-full' : ''"
              @click="showCreateDialog = false"
            >
              Cancel
            </button>
            <button
              type="submit"
              :disabled="creating || !newKeyName.trim()"
              class="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed min-h-[44px]"
              :class="isMobile ? 'w-full' : ''"
            >
              {{ creating ? 'Creating...' : 'Create Key' }}
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- Revoke Confirmation Dialog -->
    <div
      v-if="showRevokeDialog"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
      @click.self="showRevokeDialog = false"
    >
      <div class="w-full max-w-sm rounded-xl bg-white p-6 shadow-xl">
        <h2 class="text-lg font-semibold text-gray-900">Revoke API Key</h2>
        <p class="mt-2 text-sm text-gray-600">
          Are you sure you want to revoke <strong>{{ keyToRevokeName }}</strong>? This action cannot
          be undone. Any integrations using this key will stop working immediately.
        </p>
        <div
          class="mt-6 flex gap-3"
          :class="isMobile ? 'flex-col' : 'flex-row justify-end'"
        >
          <button
            type="button"
            class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 min-h-[44px]"
            :class="isMobile ? 'w-full' : ''"
            @click="showRevokeDialog = false"
          >
            Cancel
          </button>
          <button
            type="button"
            :disabled="revoking"
            class="rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-red-700 focus:outline-none focus:ring-2 focus:ring-red-500 focus:ring-offset-2 disabled:opacity-50 min-h-[44px]"
            :class="isMobile ? 'w-full' : ''"
            @click="confirmRevoke"
          >
            {{ revoking ? 'Revoking...' : 'Revoke Key' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
