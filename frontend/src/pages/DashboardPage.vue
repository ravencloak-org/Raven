<template>
  <div>
    <div class="mb-8 flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold text-gray-900">Welcome to Raven</h1>
        <p class="mt-1 text-sm text-gray-500">Your knowledge management platform.</p>
      </div>
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
      <div
        v-for="i in 2"
        :key="i"
        class="h-32 animate-pulse rounded-xl border border-gray-100 bg-gray-50"
      />
    </div>

    <!-- Error -->
    <div v-else-if="fetchError" class="rounded-xl border border-red-200 bg-red-50 p-6 text-sm text-red-700">
      {{ fetchError }}
    </div>

    <!-- Empty state — no workspaces -->
    <div
      v-else-if="workspaces.length === 0"
      class="rounded-xl border border-dashed border-gray-300 bg-white p-12 text-center"
    >
      <div class="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-amber-50">
        <svg class="h-6 w-6 text-amber-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
          <path stroke-linecap="round" stroke-linejoin="round" d="M2.25 12.75V12A2.25 2.25 0 014.5 9.75h15A2.25 2.25 0 0121.75 12v.75m-8.69-6.44-2.12-2.12a1.5 1.5 0 00-1.061-.44H4.5A2.25 2.25 0 002.25 6v12a2.25 2.25 0 002.25 2.25h15A2.25 2.25 0 0021.75 18V9a2.25 2.25 0 00-2.25-2.25h-5.379a1.5 1.5 0 01-1.06-.44z" />
        </svg>
      </div>
      <h3 class="mb-1 text-base font-semibold text-gray-900">No workspaces yet</h3>
      <p class="mb-6 text-sm text-gray-500">Complete onboarding to set up your first workspace and knowledge base.</p>
      <router-link
        to="/onboarding"
        class="inline-block rounded-lg bg-amber-500 px-5 py-2.5 text-sm font-semibold text-white hover:bg-amber-600"
      >
        Go to Onboarding
      </router-link>
    </div>

    <!-- Workspaces and KBs -->
    <div v-else class="space-y-6">
      <div
        v-for="ws in workspaces"
        :key="ws.id"
        class="rounded-xl border border-gray-200 bg-white shadow-sm"
      >
        <!-- Workspace header -->
        <div class="flex items-center justify-between border-b border-gray-100 px-6 py-4">
          <div class="flex items-center gap-3">
            <div class="flex h-8 w-8 items-center justify-center rounded-lg bg-black text-sm font-bold text-white">
              {{ ws.name.charAt(0).toUpperCase() }}
            </div>
            <div>
              <router-link
                :to="`/orgs/${orgId}/workspaces/${ws.id}`"
                class="font-semibold text-gray-900 hover:text-amber-600"
              >
                {{ ws.name }}
              </router-link>
              <p class="text-xs text-gray-400">{{ ws.slug }}</p>
            </div>
          </div>
          <router-link
            :to="`/orgs/${orgId}/workspaces/${ws.id}/knowledge-bases`"
            class="rounded-lg border border-gray-200 px-3 py-1.5 text-xs font-medium text-gray-600 hover:border-amber-400 hover:text-amber-600"
          >
            Create Knowledge Base
          </router-link>
        </div>

        <!-- KB list -->
        <div v-if="kbsByWorkspace[ws.id]?.loading" class="px-6 py-4 text-sm text-gray-400">
          Loading knowledge bases...
        </div>
        <div
          v-else-if="!kbsByWorkspace[ws.id]?.items.length"
          class="px-6 py-6 text-center text-sm text-gray-400"
        >
          No knowledge bases yet —
          <router-link
            :to="`/orgs/${orgId}/workspaces/${ws.id}/knowledge-bases`"
            class="text-amber-500 underline-offset-2 hover:underline"
          >
            create one
          </router-link>
        </div>
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
              <div>
                <p class="text-sm font-medium text-gray-900">{{ kb.name }}</p>
                <p class="text-xs text-gray-400">{{ kb.doc_count }} document{{ kb.doc_count === 1 ? '' : 's' }}</p>
              </div>
            </div>
            <div class="flex items-center gap-3">
              <span
                class="inline-block rounded-full px-2 py-0.5 text-xs font-medium"
                :class="kb.status === 'active' ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-500'"
              >
                {{ kb.status }}
              </span>
              <svg class="h-4 w-4 text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M8.25 4.5l7.5 7.5-7.5 7.5" />
              </svg>
            </div>
          </router-link>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useAuthStore } from '../stores/auth'
import type { Workspace } from '../api/workspaces'
import type { KnowledgeBase } from '../api/knowledge-bases'

const auth = useAuthStore()
const apiUrl = import.meta.env.VITE_API_BASE_URL ?? '/api/v1'

const orgId = auth.orgId ?? sessionStorage.getItem('raven_org_id')
const loading = ref(false)
const fetchError = ref<string | null>(null)
const workspaces = ref<Workspace[]>([])

interface KBState {
  loading: boolean
  items: KnowledgeBase[]
}
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
    const res = await authGet<{ items: KnowledgeBase[]; total: number }>(
      `/orgs/${orgId}/workspaces/${wsId}/knowledge-bases`,
    )
    kbsByWorkspace[wsId] = { loading: false, items: res.items ?? [] }
  } catch {
    kbsByWorkspace[wsId] = { loading: false, items: [] }
  }
}

onMounted(async () => {
  if (!orgId) return
  loading.value = true
  fetchError.value = null
  try {
    const res = await authGet<{ items: Workspace[]; total: number }>(
      `/orgs/${orgId}/workspaces`,
    )
    workspaces.value = res.items ?? []
    // Fetch KBs for all workspaces in parallel
    await Promise.all(workspaces.value.map((ws) => fetchKBsForWorkspace(ws.id)))
  } catch (e: unknown) {
    fetchError.value = e instanceof Error ? e.message : 'Failed to load dashboard data'
  } finally {
    loading.value = false
  }
})
</script>
