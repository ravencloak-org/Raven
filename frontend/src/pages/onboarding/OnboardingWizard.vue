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

        <!-- Drop zone (visual placeholder) -->
        <div class="border-2 border-dashed border-neutral-300 dark:border-neutral-700 rounded-lg p-6 text-center mb-6">
          <p class="text-neutral-400 text-sm">Drag &amp; drop files here (optional)</p>
          <p class="text-neutral-400 text-xs mt-1">PDF, DOCX, TXT, MD — up to 50 MB</p>
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

async function apiFetch(path: string, body: Record<string, unknown>) {
  const res = await fetch(`${apiUrl}${path}`, {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${auth.accessToken}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(body),
  })
  if (!res.ok) {
    const data = await res.json().catch(() => ({}))
    throw new Error(data.message || data.error || `Request failed (${res.status})`)
  }
  return res.json()
}

async function createOrg() {
  if (orgName.value.length < 3) return
  loading.value = true
  error.value = ''
  try {
    const data = await apiFetch('/orgs', { name: orgName.value })
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
    // Create default workspace
    const ws = await apiFetch(`/orgs/${orgId}/workspaces`, { name: 'Default' })
    // Create knowledge base
    await apiFetch(`/orgs/${orgId}/workspaces/${ws.id}/knowledge-bases`, { name: kbName.value })
    // Set org in auth store and navigate to dashboard
    auth.setOrgId(orgId)
    router.push('/dashboard')
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : 'Something went wrong'
  } finally {
    loading.value = false
  }
}
</script>
