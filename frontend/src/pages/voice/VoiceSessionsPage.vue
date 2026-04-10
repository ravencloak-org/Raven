<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useVoiceStore } from '../../stores/voice'
import CreateSessionModal from '../../components/voice/CreateSessionModal.vue'

const route = useRoute()
const router = useRouter()
const store = useVoiceStore()

const orgId = route.params.orgId as string
const showCreateModal = ref(false)

const PAGE_SIZE = 20

onMounted(() => {
  store.fetchSessions(orgId, PAGE_SIZE, 0)
})

function formatDate(iso: string): string {
  return new Date(iso).toLocaleString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function formatDuration(seconds?: number): string {
  if (seconds == null) return '--'
  const m = Math.floor(seconds / 60)
  const s = seconds % 60
  return `${m}m ${s}s`
}

function stateClass(state: string): string {
  switch (state) {
    case 'active':
      return 'bg-green-100 text-green-800'
    case 'ended':
      return 'bg-gray-100 text-gray-600'
    default:
      return 'bg-yellow-100 text-yellow-800'
  }
}

function openSession(sessionId: string) {
  void router.push({ name: 'voice-session-detail', params: { orgId, sessionId } })
}

function onSessionCreated(sessionId: string) {
  void router.push({ name: 'voice-session-detail', params: { orgId, sessionId } })
}

function prevPage() {
  const newOffset = Math.max(0, store.offset - PAGE_SIZE)
  store.fetchSessions(orgId, PAGE_SIZE, newOffset)
}

function nextPage() {
  const newOffset = store.offset + PAGE_SIZE
  if (newOffset < store.total) {
    store.fetchSessions(orgId, PAGE_SIZE, newOffset)
  }
}

const hasPrev = computed(() => store.offset > 0)
const hasNext = computed(() => store.offset + PAGE_SIZE < store.total)
const currentPage = computed(() => Math.floor(store.offset / PAGE_SIZE) + 1)
const totalPages = computed(() => Math.max(1, Math.ceil(store.total / PAGE_SIZE)))
</script>

<template>
  <div class="p-4 md:p-6 max-w-6xl">
    <!-- Header -->
    <div class="mb-6 flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-2xl font-bold text-gray-900">Voice Sessions</h1>
        <p class="mt-1 text-sm text-gray-500">
          Manage LiveKit-backed voice sessions for your organisation.
        </p>
      </div>
      <button
        class="min-h-[44px] rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2"
        @click="showCreateModal = true"
      >
        New Session
      </button>
    </div>

    <!-- Loading -->
    <div v-if="store.loading" class="text-gray-500">Loading sessions...</div>

    <!-- Error -->
    <div
      v-else-if="store.error"
      class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700"
    >
      {{ store.error }}
    </div>

    <!-- Empty state -->
    <div
      v-else-if="store.sessions.length === 0"
      class="rounded-xl border border-dashed border-gray-300 bg-white p-12 text-center"
    >
      <p class="text-gray-500">No voice sessions yet. Create one to get started.</p>
    </div>

    <!-- Sessions table -->
    <div v-else class="overflow-hidden rounded-xl border border-gray-200 bg-white shadow-sm">
      <div class="overflow-x-auto">
        <table class="min-w-full divide-y divide-gray-200">
          <thead class="bg-gray-50">
            <tr>
              <th
                class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500"
              >
                State
              </th>
              <th
                class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500"
              >
                LiveKit Room
              </th>
              <th
                class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500"
              >
                Duration
              </th>
              <th
                class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500"
              >
                Created
              </th>
              <th
                class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500"
              >
                Actions
              </th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-200">
            <tr
              v-for="session in store.sessions"
              :key="session.id"
              class="cursor-pointer hover:bg-gray-50"
              @click="openSession(session.id)"
            >
              <td class="whitespace-nowrap px-4 py-4">
                <span
                  class="inline-flex rounded-full px-2 py-0.5 text-xs font-semibold"
                  :class="stateClass(session.state)"
                >
                  {{ session.state }}
                </span>
              </td>
              <td class="px-4 py-4 text-sm text-gray-700">
                <code class="rounded bg-gray-100 px-2 py-0.5 font-mono text-xs">
                  {{ session.livekit_room }}
                </code>
              </td>
              <td class="whitespace-nowrap px-4 py-4 text-sm text-gray-500">
                {{ formatDuration(session.call_duration_seconds) }}
              </td>
              <td class="whitespace-nowrap px-4 py-4 text-sm text-gray-500">
                {{ formatDate(session.created_at) }}
              </td>
              <td class="whitespace-nowrap px-4 py-4 text-right text-sm">
                <button
                  class="font-medium text-indigo-600 hover:text-indigo-800"
                  @click.stop="openSession(session.id)"
                >
                  View
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Pagination -->
      <div
        v-if="store.total > PAGE_SIZE"
        class="flex items-center justify-between border-t border-gray-200 px-4 py-3"
      >
        <p class="text-sm text-gray-500">
          Page {{ currentPage }} of {{ totalPages }} &mdash; {{ store.total }} total
        </p>
        <div class="flex gap-2">
          <button
            :disabled="!hasPrev"
            class="min-h-[44px] rounded-lg border border-gray-300 bg-white px-3 py-1 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-40"
            @click="prevPage"
          >
            Previous
          </button>
          <button
            :disabled="!hasNext"
            class="min-h-[44px] rounded-lg border border-gray-300 bg-white px-3 py-1 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-40"
            @click="nextPage"
          >
            Next
          </button>
        </div>
      </div>
    </div>

    <!-- Create Modal -->
    <CreateSessionModal
      v-if="showCreateModal"
      :org-id="orgId"
      @close="showCreateModal = false"
      @created="onSessionCreated"
    />
  </div>
</template>
