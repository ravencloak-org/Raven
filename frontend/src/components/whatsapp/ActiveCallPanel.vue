<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useWhatsAppStore } from '../../stores/whatsapp'
import BridgeStatus from './BridgeStatus.vue'

const props = defineProps<{
  orgId: string
}>()

const store = useWhatsAppStore()
const ending = ref(false)
let pollTimer: ReturnType<typeof setInterval> | null = null

onMounted(() => {
  if (store.activeCall && store.activeCall.state !== 'ended') {
    pollTimer = setInterval(() => {
      if (store.activeCall) {
        store.fetchCall(props.orgId, store.activeCall.id)
        store.fetchBridgeStatus(props.orgId, store.activeCall.id)
      }
    }, 3000)
  }
})

onUnmounted(() => {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
})

async function handleEnd() {
  if (!store.activeCall) return
  ending.value = true
  try {
    await store.endCall(props.orgId, store.activeCall.id)
    if (pollTimer) {
      clearInterval(pollTimer)
      pollTimer = null
    }
  } catch {
    /* error surfaced via store.error */
  } finally {
    ending.value = false
  }
}

function handleDismiss() {
  store.clearActiveCall()
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

function stateColor(state: string) {
  switch (state) {
    case 'ringing':
      return 'bg-yellow-100 border-yellow-300 text-yellow-900'
    case 'connected':
      return 'bg-green-100 border-green-300 text-green-900'
    case 'ended':
      return 'bg-gray-100 border-gray-300 text-gray-700'
    default:
      return 'bg-gray-100 border-gray-300 text-gray-700'
  }
}
</script>

<template>
  <div
    v-if="store.activeCall"
    :class="[
      'rounded-lg border p-4',
      stateColor(store.activeCall.state),
    ]"
  >
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div>
        <p class="font-medium">
          Active Call: {{ store.activeCall.caller }} &rarr; {{ store.activeCall.callee }}
        </p>
        <p class="text-sm mt-1">
          State:
          <span class="font-semibold capitalize">{{ store.activeCall.state }}</span>
        </p>
        <p v-if="store.activeCall.duration_seconds != null" class="text-sm">
          Duration: {{ store.activeCall.duration_seconds }}s
        </p>
      </div>
      <div class="flex gap-2">
        <button
          v-if="store.activeCall.state !== 'ended'"
          :disabled="ending"
          class="min-h-[44px] min-w-[44px] rounded-md bg-red-600 px-4 py-2 text-sm font-medium text-white hover:bg-red-700 disabled:opacity-50 disabled:cursor-not-allowed"
          @click="handleEnd"
        >
          {{ ending ? 'Ending...' : 'End Call' }}
        </button>
        <button
          v-if="store.activeCall.state === 'ended'"
          class="min-h-[44px] min-w-[44px] rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
          @click="handleDismiss"
        >
          Dismiss
        </button>
      </div>
    </div>

    <!-- Bridge status -->
    <BridgeStatus
      v-if="store.activeCall.state !== 'ended'"
      :org-id="orgId"
      :call-id="store.activeCall.id"
      class="mt-4"
    />
  </div>
</template>
