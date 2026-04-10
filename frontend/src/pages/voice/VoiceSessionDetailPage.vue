<script setup lang="ts">
import { onMounted, onUnmounted, ref, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useVoiceSessionsStore } from '../../stores/voice-sessions'
import VoiceStatusBadge from '../../components/voice/VoiceStatusBadge.vue'
import VoiceTurnList from '../../components/voice/VoiceTurnList.vue'

const route = useRoute()
const router = useRouter()
const store = useVoiceSessionsStore()

const orgId = route.params.orgId as string
const sessionId = route.params.sessionId as string

const joining = ref(false)
const tokenError = ref<string | null>(null)

const isActive = computed(() => store.currentSession?.state === 'active')
const isCreated = computed(() => store.currentSession?.state === 'created')
const isEnded = computed(() => store.currentSession?.state === 'ended')

onMounted(async () => {
  await Promise.all([
    store.fetchSession(orgId, sessionId),
    store.fetchTurns(orgId, sessionId),
  ])
  // Start polling if the session is not ended
  if (store.currentSession && store.currentSession.state !== 'ended') {
    store.startPolling(orgId, sessionId)
  }
})

onUnmounted(() => {
  store.stopPolling()
})

async function handleActivate() {
  try {
    await store.updateState(orgId, sessionId, 'active')
    store.startPolling(orgId, sessionId)
  } catch {
    /* error surfaced via store.error */
  }
}

async function handleEnd() {
  try {
    await store.updateState(orgId, sessionId, 'ended')
  } catch {
    /* error surfaced via store.error */
  }
}

async function handleJoinRoom() {
  joining.value = true
  tokenError.value = null
  try {
    const resp = await store.getToken(orgId, sessionId)
    // Open LiveKit room in a new tab via the LiveKit meet URL
    const meetUrl = `${resp.url}?token=${encodeURIComponent(resp.token)}`
    window.open(meetUrl, '_blank', 'noopener,noreferrer')
  } catch (e) {
    tokenError.value = (e as Error).message
  } finally {
    joining.value = false
  }
}

function goBack() {
  router.push(`/orgs/${orgId}/voice`)
}

function formatDateTime(iso?: string): string {
  if (!iso) return '--'
  return new Date(iso).toLocaleString()
}

function formatDuration(seconds?: number): string {
  if (seconds == null) return '--'
  const m = Math.floor(seconds / 60)
  const s = seconds % 60
  return `${m}m ${s}s`
}
</script>

<template>
  <div class="mx-auto max-w-4xl p-4 sm:p-6">
    <!-- Back link -->
    <button
      class="mb-4 inline-flex min-h-[44px] min-w-[44px] items-center gap-1 text-sm text-indigo-600 hover:text-indigo-800"
      @click="goBack"
    >
      <svg
        xmlns="http://www.w3.org/2000/svg"
        class="h-4 w-4"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
        stroke-width="2"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          d="M15 19l-7-7 7-7"
        />
      </svg>
      Back to sessions
    </button>

    <!-- Loading state -->
    <div v-if="store.loading && !store.currentSession" class="text-gray-500">
      Loading...
    </div>

    <!-- Error state -->
    <div v-else-if="store.error && !store.currentSession" class="text-red-600">
      {{ store.error }}
    </div>

    <!-- Session detail -->
    <template v-else-if="store.currentSession">
      <div class="mb-6">
        <div class="flex flex-wrap items-center gap-3">
          <h1 class="text-2xl font-bold">
            {{ store.currentSession.livekit_room }}
          </h1>
          <VoiceStatusBadge :state="store.currentSession.state" />
        </div>
        <p class="mt-1 text-sm text-gray-500">
          Session {{ store.currentSession.id }}
        </p>
      </div>

      <!-- Session info -->
      <div class="mb-6 grid grid-cols-1 gap-4 sm:grid-cols-3">
        <div class="rounded-lg border border-gray-200 p-4">
          <p class="text-xs font-medium uppercase text-gray-500">Created</p>
          <p class="mt-1 text-sm font-semibold">
            {{ formatDateTime(store.currentSession.created_at) }}
          </p>
        </div>
        <div class="rounded-lg border border-gray-200 p-4">
          <p class="text-xs font-medium uppercase text-gray-500">Started</p>
          <p class="mt-1 text-sm font-semibold">
            {{ formatDateTime(store.currentSession.started_at) }}
          </p>
        </div>
        <div class="rounded-lg border border-gray-200 p-4">
          <p class="text-xs font-medium uppercase text-gray-500">Duration</p>
          <p class="mt-1 text-sm font-semibold">
            {{
              formatDuration(store.currentSession.call_duration_seconds)
            }}
          </p>
        </div>
      </div>

      <!-- Controls -->
      <div class="mb-6 flex flex-wrap gap-3">
        <button
          v-if="isCreated"
          class="min-h-[44px] min-w-[44px] rounded-md bg-green-600 px-4 py-2 text-sm font-medium text-white hover:bg-green-700"
          @click="handleActivate"
        >
          Start Session
        </button>
        <button
          v-if="isActive"
          class="min-h-[44px] min-w-[44px] rounded-md bg-red-600 px-4 py-2 text-sm font-medium text-white hover:bg-red-700"
          @click="handleEnd"
        >
          End Session
        </button>
        <button
          v-if="isActive || isCreated"
          :disabled="joining"
          class="min-h-[44px] min-w-[44px] rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:cursor-not-allowed disabled:opacity-50"
          @click="handleJoinRoom"
        >
          {{ joining ? 'Joining...' : 'Join LiveKit Room' }}
        </button>
      </div>

      <!-- Token error -->
      <div v-if="tokenError" class="mb-4 text-sm text-red-600">
        {{ tokenError }}
      </div>

      <!-- Turns -->
      <div>
        <h2 class="mb-3 text-lg font-semibold">Conversation Turns</h2>
        <VoiceTurnList :turns="store.turns" />
      </div>

      <!-- Ended indicator -->
      <div
        v-if="isEnded"
        class="mt-6 rounded-lg border border-gray-200 bg-gray-50 p-4 text-center text-sm text-gray-500"
      >
        This session has ended.
      </div>
    </template>
  </div>
</template>
