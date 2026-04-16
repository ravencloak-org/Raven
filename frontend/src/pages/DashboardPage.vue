<template>
  <div>
    <div class="mb-8">
      <h1 class="text-2xl font-bold text-gray-900">Welcome to Raven</h1>
      <p class="mt-1 text-sm text-gray-500">Your knowledge management platform.</p>
    </div>

    <!-- No org state -->
    <div v-if="!orgId" class="rounded-xl border border-amber-200 bg-amber-50 p-8 text-center">
      <p class="mb-4 text-gray-700">You don't have an organization set up yet.</p>
      <router-link
        to="/onboarding"
        class="inline-block rounded-lg bg-amber-500 px-5 py-2.5 text-sm font-semibold text-white hover:bg-amber-600"
      >
        Complete Onboarding
      </router-link>
    </div>

    <!-- Loading -->
    <div v-else-if="loading" class="space-y-4">
      <div v-for="i in 2" :key="i" class="h-32 animate-pulse rounded-xl border border-gray-100 bg-gray-50" />
    </div>

    <!-- Error -->
    <div v-else-if="fetchError" class="rounded-xl border border-red-200 bg-red-50 p-6 text-center">
      <p class="text-red-700">{{ fetchError }}</p>
      <button class="mt-3 text-sm text-red-500 underline" @click="loadDashboard">Retry</button>
    </div>

    <!-- Empty state -->
    <div v-else-if="workspaces.length === 0" class="rounded-xl border border-gray-200 bg-gray-50 p-8 text-center">
      <p class="text-lg font-medium text-gray-700 mb-2">No workspaces yet</p>
      <p class="text-sm text-gray-500 mb-4">Complete onboarding to set up your first workspace and knowledge base.</p>
      <router-link to="/onboarding" class="inline-block rounded-lg bg-amber-500 px-5 py-2.5 text-sm font-semibold text-white hover:bg-amber-600">
        Go to Onboarding
      </router-link>
    </div>

    <!-- Workspaces + KBs -->
    <div v-else class="space-y-6">
      <div v-for="ws in workspaces" :key="ws.id" class="rounded-xl border border-gray-200 bg-white shadow-sm">
        <div class="flex items-center justify-between border-b border-gray-100 px-6 py-4">
          <div class="flex items-center gap-3">
            <div class="flex h-9 w-9 items-center justify-center rounded-lg bg-amber-100 text-amber-700 font-bold text-sm">
              {{ ws.name.charAt(0).toUpperCase() }}
            </div>
            <div>
              <p class="font-semibold text-gray-900">{{ ws.name }}</p>
              <p class="text-xs text-gray-400">Workspace</p>
            </div>
          </div>
          <router-link
            :to="`/orgs/${orgId}/workspaces/${ws.id}/knowledge-bases`"
            class="text-xs text-amber-600 hover:underline"
          >
            + New KB
          </router-link>
        </div>

        <!-- KB loading -->
        <div v-if="kbsByWorkspace[ws.id]?.loading" class="px-6 py-6">
          <div class="h-8 animate-pulse rounded bg-gray-100" />
        </div>

        <!-- No KBs -->
        <div v-else-if="!kbsByWorkspace[ws.id]?.items?.length" class="px-6 py-6 text-center text-sm text-gray-400">
          No knowledge bases yet
        </div>

        <!-- KB list -->
        <div v-else class="divide-y divide-gray-50">
          <router-link
            v-for="kb in kbsByWorkspace[ws.id].items"
            :key="kb.id"
            :to="`/orgs/${orgId}/workspaces/${ws.id}/knowledge-bases/${kb.id}`"
            class="flex items-center justify-between px-6 py-3.5 hover:bg-gray-50"
          >
            <div class="flex items-center gap-3">
              <div class="flex h-7 w-7 items-center justify-center rounded-md bg-amber-50">
                <svg class="h-4 w-4 text-amber-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M12 6.042A8.967 8.967 0 006 3.75c-1.052 0-2.062.18-3 .512v14.25A8.987 8.987 0 016 18c2.305 0 4.408.867 6 2.292m0-14.25a8.966 8.966 0 016-2.292c1.052 0 2.062.18 3 .512v14.25A8.987 8.987 0 0018 18a8.967 8.967 0 00-6 2.292m0-14.25v14.25" />
                </svg>
              </div>
              <p class="text-sm font-medium text-gray-900">{{ kb.name }}</p>
            </div>
          </router-link>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const apiUrl = import.meta.env.VITE_API_BASE_URL ?? '/api/v1'

// Reactive orgId — updates when auth store changes
const orgId = computed(() => auth.orgId ?? sessionStorage.getItem('raven_org_id'))

const loading = ref(false)
const fetchError = ref<string | null>(null)

interface Workspace { id: string; name: string; slug: string }
interface KB { id: string; name: string }
interface KBState { loading: boolean; items: KB[] }

const workspaces = ref<Workspace[]>([])
const kbsByWorkspace = reactive<Record<string, KBState>>({})

async function authGet<T>(path: string): Promise<T> {
  const res = await fetch(`${apiUrl}${path}`, { credentials: 'include' })
  if (!res.ok) {
    const data = await res.json().catch(() => ({}))
    throw new Error(data.message || data.error || `Request failed (${res.status})`)
  }
  return res.json()
}

async function fetchKBsForWorkspace(wsId: string) {
  kbsByWorkspace[wsId] = { loading: true, items: [] }
  try {
    const res = await authGet<KB[] | { items: KB[] }>(
      `/orgs/${orgId.value}/workspaces/${wsId}/knowledge-bases`,
    )
    // API may return array or { items: [...] }
    const items = Array.isArray(res) ? res : (res.items ?? [])
    kbsByWorkspace[wsId] = { loading: false, items }
  } catch {
    kbsByWorkspace[wsId] = { loading: false, items: [] }
  }
}

async function loadDashboard() {
  if (!orgId.value) return
  loading.value = true
  fetchError.value = null
  try {
    const res = await authGet<Workspace[] | { items: Workspace[] }>(
      `/orgs/${orgId.value}/workspaces`,
    )
    // API may return array or { items: [...] }
    workspaces.value = Array.isArray(res) ? res : (res.items ?? [])
    await Promise.all(workspaces.value.map((ws) => fetchKBsForWorkspace(ws.id)))
  } catch (e: unknown) {
    fetchError.value = e instanceof Error ? e.message : 'Failed to load dashboard data'
  } finally {
    loading.value = false
  }
}

onMounted(loadDashboard)
</script>
