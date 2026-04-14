<template>
  <div class="min-h-screen flex items-center justify-center bg-white dark:bg-black p-4">
    <div class="w-full max-w-md">
      <!-- Progress dots -->
      <div class="flex justify-center gap-2 mb-8">
        <div
          v-for="step in 2"
          :key="step"
          class="w-2.5 h-2.5 rounded-full transition-colors duration-200"
          :class="step <= currentStep ? 'bg-amber-500' : 'bg-neutral-300 dark:bg-neutral-700'"
        />
      </div>

      <!-- Step 1: Name your organization -->
      <div v-if="currentStep === 1">
        <h2 class="text-2xl font-bold text-neutral-900 dark:text-white mb-2">Name your organization</h2>
        <p class="text-neutral-500 text-sm mb-6">This will be your team's workspace on Raven.</p>
        <label for="org-name" class="sr-only">Organization name</label>
        <input
          id="org-name"
          v-model="orgName"
          type="text"
          placeholder="e.g. Acme Corp"
          class="w-full rounded-lg border border-neutral-300 dark:border-neutral-700 bg-white dark:bg-neutral-900 text-neutral-900 dark:text-white placeholder-neutral-400 px-4 py-3 text-sm focus:outline-none focus:ring-2 focus:ring-amber-500 mb-6"
          aria-describedby="org-error"
          @keyup.enter="createOrg"
        />
        <button
          :disabled="orgName.length < 3 || loading"
          class="w-full bg-amber-500 hover:bg-amber-600 disabled:opacity-50 text-white font-semibold py-3 rounded-lg transition-colors"
          @click="createOrg"
        >
          {{ loading ? 'Creating...' : 'Continue' }}
        </button>
        <p v-if="error" id="org-error" class="text-red-500 text-sm mt-3">{{ error }}</p>
      </div>

      <!-- Step 2: Create first knowledge base -->
      <div v-else-if="currentStep === 2">
        <h2 class="text-2xl font-bold text-neutral-900 dark:text-white mb-2">Create your first knowledge base</h2>
        <p class="text-neutral-500 text-sm mb-6">A knowledge base holds the documents your AI will learn from.</p>
        <label for="kb-name" class="sr-only">Knowledge base name</label>
        <input
          id="kb-name"
          v-model="kbName"
          type="text"
          placeholder="e.g. Product Docs"
          class="w-full rounded-lg border border-neutral-300 dark:border-neutral-700 bg-white dark:bg-neutral-900 text-neutral-900 dark:text-white placeholder-neutral-400 px-4 py-3 text-sm focus:outline-none focus:ring-2 focus:ring-amber-500 mb-4"
          aria-describedby="kb-error"
          @keyup.enter="createKB"
        />

        <!-- Drop zone -->
        <div
          class="border-2 border-dashed rounded-lg p-6 text-center mb-6 transition-colors"
          :class="isDragging ? 'border-amber-500 bg-amber-50 dark:bg-amber-900/10' : 'border-neutral-300 dark:border-neutral-700'"
          @dragover.prevent="isDragging = true"
          @dragleave.prevent="isDragging = false"
          @drop.prevent="handleDrop"
        >
          <input ref="fileInput" type="file" multiple accept=".pdf,.docx,.txt,.md,.csv,.html" class="hidden" @change="handleFileSelect" />
          <p v-if="files.length === 0" class="text-neutral-400 text-sm">
            Drag &amp; drop files here or <button class="text-amber-500 underline" @click="($refs.fileInput as HTMLInputElement).click()">browse</button>
          </p>
          <p v-if="files.length === 0" class="text-neutral-400 text-xs mt-1">PDF, DOCX, TXT, MD — up to 50 MB</p>
          <div v-if="files.length > 0">
            <p class="text-neutral-900 dark:text-white text-sm font-medium mb-2">{{ files.length }} file{{ files.length > 1 ? 's' : '' }} selected</p>
            <ul class="text-neutral-500 text-xs space-y-1">
              <li v-for="f in files" :key="f.name">{{ f.name }} ({{ (f.size / 1024).toFixed(0) }} KB)</li>
            </ul>
          </div>
        </div>

        <button
          :disabled="kbName.length < 1 || loading"
          class="w-full bg-amber-500 hover:bg-amber-600 disabled:opacity-50 text-white font-semibold py-3 rounded-lg transition-colors"
          @click="createKB"
        >
          {{ loading ? 'Setting up...' : 'Get Started' }}
        </button>
        <p v-if="error" id="kb-error" class="text-red-500 text-sm mt-3">{{ error }}</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../../stores/auth'

const router = useRouter()
const auth = useAuthStore()
const apiUrl = import.meta.env.VITE_API_BASE_URL

const currentStep = ref(1)
const loading = ref(false)
const error = ref('')

// Step 1
const orgName = ref('')
let orgId = ''

// Step 2
const kbName = ref('')
const files = ref<File[]>([])
const isDragging = ref(false)
// Template uses $refs.fileInput directly

async function apiFetch(path: string, options: { method?: string; body?: Record<string, unknown> | FormData; headers?: Record<string, string> } = {}) {
  const method = options.method || 'POST'
  const headers: Record<string, string> = {
    ...options.headers,
  }
  let body: string | FormData | undefined
  if (options.body instanceof FormData) {
    body = options.body
  } else if (options.body) {
    headers['Content-Type'] = 'application/json'
    body = JSON.stringify(options.body)
  }
  const res = await fetch(`${apiUrl}${path}`, { method, credentials: 'include', headers, body })
  if (!res.ok) {
    const data = await res.json().catch(() => ({}))
    throw new Error(data.message || data.error || `Request failed (${res.status})`)
  }
  return res.json()
}

function handleDrop(e: DragEvent) {
  isDragging.value = false
  if (e.dataTransfer?.files) {
    files.value = [...files.value, ...Array.from(e.dataTransfer.files)]
  }
}

function handleFileSelect(e: Event) {
  const input = e.target as HTMLInputElement
  if (input.files) {
    files.value = [...files.value, ...Array.from(input.files)]
  }
}

async function createOrg() {
  if (orgName.value.length < 3) return
  loading.value = true
  error.value = ''
  try {
    const data = await apiFetch('/orgs', { body: { name: orgName.value } })
    orgId = data.id
    currentStep.value = 2
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : 'Something went wrong'
  } finally {
    loading.value = false
  }
}

async function createKB() {
  if (!kbName.value) return
  loading.value = true
  error.value = ''
  try {
    // Create default workspace (ignore "already exists" errors)
    let wsId: string
    try {
      const ws = await apiFetch(`/orgs/${orgId}/workspaces`, { body: { name: 'Default' } })
      wsId = ws.id
    } catch (wsErr) {
      // Workspace may already exist — fetch the list (returns array directly)
      const wsList = await apiFetch(`/orgs/${orgId}/workspaces`, { method: 'GET' })
      const arr = Array.isArray(wsList) ? wsList : (wsList.items || [])
      const existing = arr.find((w: { slug: string }) => w.slug === 'default') || arr[0]
      if (!existing) throw new Error('Failed to create workspace', { cause: wsErr })
      wsId = existing.id
    }

    // Create knowledge base (ignore "already exists" — just proceed to dashboard)
    try {
      const kb = await apiFetch(`/orgs/${orgId}/workspaces/${wsId}/knowledge-bases`, { body: { name: kbName.value } })

      // Upload files if any
      for (const file of files.value) {
        const formData = new FormData()
        formData.append('file', file)
        await apiFetch(`/orgs/${orgId}/workspaces/${wsId}/knowledge-bases/${kb.id}/documents/upload`, {
          body: formData,
        }).catch(() => {
          // Non-fatal — files can be uploaded later
        })
      }
    } catch {
      // KB may already exist from a previous attempt — that's fine
    }

    // Always set org and go to dashboard regardless of KB creation result
    auth.setOrgId(orgId)
    router.push('/dashboard')
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : 'Something went wrong'
  } finally {
    loading.value = false
  }
}
</script>
