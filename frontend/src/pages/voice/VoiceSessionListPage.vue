<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useVoiceSessionsStore } from '../../stores/voice-sessions'
import { useMobile } from '../../composables/useMediaQuery'
import VoiceStatusBadge from '../../components/voice/VoiceStatusBadge.vue'
import CreateSessionModal from '../../components/voice/CreateSessionModal.vue'

const route = useRoute()
const router = useRouter()
const store = useVoiceSessionsStore()
const { isMobile } = useMobile()

const orgId = route.params.orgId as string
const showCreateModal = ref(false)
const currentPage = ref(0)
const pageSize = 20

const totalPages = computed(() => Math.max(1, Math.ceil(store.total / pageSize)))

onMounted(() => store.fetchSessions(orgId, 0, pageSize))

async function handleCreate(roomName: string) {
  try {
    const session = await store.create(orgId, {
      livekit_room: roomName || undefined,
    })
    showCreateModal.value = false
    router.push(`/orgs/${orgId}/voice/${session.id}`)
  } catch {
    /* error surfaced via store.error */
  }
}

function openSession(sessionId: string) {
  router.push(`/orgs/${orgId}/voice/${sessionId}`)
}

function goToPage(page: number) {
  currentPage.value = page
  store.fetchSessions(orgId, page * pageSize, pageSize)
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString()
}

function formatDuration(seconds?: number): string {
  if (seconds == null) return '--'
  const m = Math.floor(seconds / 60)
  const s = seconds % 60
  return `${m}m ${s}s`
}

function mobileStateClass(state: string): string {
  if (state === 'active') return 'bg-green-900 text-green-300'
  if (state === 'created') return 'bg-amber-900 text-amber-300'
  return 'bg-slate-700 text-slate-300'
}
</script>

<template>
  <div class="mx-auto max-w-4xl p-4 sm:p-6">
    <div class="mb-6 flex items-center justify-between">
      <h1 class="text-2xl font-bold">Voice Sessions</h1>
      <button
        class="min-h-[44px] min-w-[44px] rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700"
        @click="showCreateModal = true"
      >
        New Session
      </button>
    </div>

    <!-- Loading state -->
    <div v-if="store.loading" class="text-gray-500">Loading...</div>

    <!-- Error state -->
    <div v-else-if="store.error" class="text-red-600">{{ store.error }}</div>

    <!-- Empty state -->
    <div
      v-else-if="store.sessions.length === 0"
      class="text-gray-500"
    >
      No voice sessions yet. Create one to get started.
    </div>

    <!-- Desktop: Sessions table -->
    <div v-else-if="!isMobile" class="overflow-x-auto">
      <table class="w-full border-collapse">
        <thead>
          <tr
            class="border-b border-gray-200 text-left text-sm font-medium text-gray-500"
          >
            <th class="pb-3 pr-4">Room</th>
            <th class="pb-3 pr-4">Status</th>
            <th class="hidden pb-3 pr-4 sm:table-cell">Duration</th>
            <th class="hidden pb-3 pr-4 sm:table-cell">Created</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="session in store.sessions"
            :key="session.id"
            class="min-h-[44px] cursor-pointer border-b border-gray-100 hover:bg-gray-50"
            @click="openSession(session.id)"
          >
            <td class="py-3 pr-4 font-medium">
              {{ session.livekit_room }}
            </td>
            <td class="py-3 pr-4">
              <VoiceStatusBadge :state="session.state" />
            </td>
            <td class="hidden py-3 pr-4 text-sm text-gray-500 sm:table-cell">
              {{ formatDuration(session.call_duration_seconds) }}
            </td>
            <td class="hidden py-3 pr-4 text-sm text-gray-500 sm:table-cell">
              {{ formatDate(session.created_at) }}
            </td>
          </tr>
        </tbody>
      </table>

      <!-- Pagination -->
      <div class="mt-4 flex items-center justify-between">
        <p class="text-sm text-gray-400">
          {{ store.total }} session{{ store.total === 1 ? '' : 's' }} total
        </p>
        <div v-if="totalPages > 1" class="flex gap-1">
          <button
            v-for="page in totalPages"
            :key="page"
            :disabled="page - 1 === currentPage"
            :class="[
              'min-h-[44px] min-w-[44px] rounded-md px-3 py-1 text-sm',
              page - 1 === currentPage
                ? 'bg-indigo-600 text-white'
                : 'bg-gray-100 text-gray-700 hover:bg-gray-200',
            ]"
            @click="goToPage(page - 1)"
          >
            {{ page }}
          </button>
        </div>
      </div>
    </div>

    <!-- Mobile: card list -->
    <div v-else class="space-y-3">
      <div
        v-for="session in store.sessions"
        :key="session.id"
        class="bg-slate-800 rounded-xl p-3.5 cursor-pointer"
        @click="openSession(session.id)"
      >
        <!-- Header: session name + state badge -->
        <div class="flex items-start justify-between gap-2">
          <span class="text-white font-semibold text-[15px] truncate">{{ session.livekit_room }}</span>
          <span
            class="shrink-0 inline-block rounded-full px-2 py-0.5 text-xs font-medium capitalize"
            :class="mobileStateClass(session.state)"
          >
            {{ session.state }}
          </span>
        </div>

        <!-- Metadata -->
        <p class="text-slate-500 text-xs mt-2">
          {{ formatDuration(session.call_duration_seconds) }} &bull; {{ formatDate(session.created_at) }}
        </p>
      </div>

      <!-- Pagination (mobile) -->
      <div class="mt-4 flex items-center justify-between">
        <p class="text-sm text-gray-400">
          {{ store.total }} session{{ store.total === 1 ? '' : 's' }} total
        </p>
        <div v-if="totalPages > 1" class="flex gap-1">
          <button
            v-for="page in totalPages"
            :key="page"
            :disabled="page - 1 === currentPage"
            :class="[
              'min-h-[44px] min-w-[44px] rounded-md px-3 py-1 text-sm',
              page - 1 === currentPage
                ? 'bg-indigo-600 text-white'
                : 'bg-gray-100 text-gray-700 hover:bg-gray-200',
            ]"
            @click="goToPage(page - 1)"
          >
            {{ page }}
          </button>
        </div>
      </div>
    </div>

    <!-- Create session modal -->
    <CreateSessionModal
      :open="showCreateModal"
      @create="handleCreate"
      @close="showCreateModal = false"
    />
  </div>
</template>
