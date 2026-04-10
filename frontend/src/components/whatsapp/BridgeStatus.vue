<script setup lang="ts">
import { useWhatsAppStore } from '../../stores/whatsapp'

defineProps<{
  orgId: string
  callId: string
}>()

const store = useWhatsAppStore()

function bridgeStateClasses(state: string) {
  switch (state) {
    case 'initializing':
      return 'bg-yellow-100 text-yellow-800'
    case 'active':
      return 'bg-green-100 text-green-800'
    case 'failed':
      return 'bg-red-100 text-red-800'
    case 'closed':
      return 'bg-gray-100 text-gray-800'
    default:
      return 'bg-gray-100 text-gray-600'
  }
}
</script>

<template>
  <div v-if="store.currentBridge" class="rounded-md border border-gray-200 bg-white p-3">
    <h3 class="text-sm font-semibold mb-2">LiveKit Bridge</h3>
    <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-4">
      <div class="flex items-center gap-2">
        <span class="text-sm text-gray-600">State:</span>
        <span
          :class="[
            'inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium capitalize',
            bridgeStateClasses(store.currentBridge.bridge_state),
          ]"
        >
          {{ store.currentBridge.bridge_state }}
        </span>
      </div>
      <div class="flex items-center gap-2">
        <span class="text-sm text-gray-600">Room:</span>
        <code class="rounded bg-gray-100 px-2 py-0.5 text-xs">
          {{ store.currentBridge.livekit_room }}
        </code>
      </div>
    </div>
  </div>
  <div v-else class="text-sm text-gray-400">
    No LiveKit bridge active for this call.
  </div>
</template>
