<script setup lang="ts">
import { computed } from 'vue'
import type { WhatsAppCall } from '../../api/whatsapp'

const props = defineProps<{
  calls: WhatsAppCall[]
  phoneNumberId?: string
}>()

const filteredEvents = computed(() => {
  if (!props.phoneNumberId) return props.calls
  return props.calls.filter((c) => c.phone_number_id === props.phoneNumberId)
})

function eventIcon(state: string) {
  switch (state) {
    case 'ringing':
      return 'ring'
    case 'connected':
      return 'connected'
    case 'ended':
      return 'ended'
    default:
      return 'unknown'
  }
}

function eventColor(state: string) {
  switch (state) {
    case 'ringing':
      return 'text-yellow-600'
    case 'connected':
      return 'text-green-600'
    case 'ended':
      return 'text-gray-500'
    default:
      return 'text-gray-400'
  }
}
</script>

<template>
  <div>
    <h3 class="text-sm font-semibold mb-3">Recent Webhook Events</h3>
    <div v-if="filteredEvents.length === 0" class="text-sm text-gray-400">
      No events recorded.
    </div>
    <ul v-else class="space-y-2 max-h-64 overflow-y-auto">
      <li
        v-for="call in filteredEvents"
        :key="call.id"
        class="flex items-start gap-3 rounded-md border border-gray-100 p-3 text-sm"
      >
        <span
          :class="['mt-0.5 font-medium uppercase text-xs', eventColor(call.state)]"
        >
          {{ eventIcon(call.state) }}
        </span>
        <div class="flex-1 min-w-0">
          <p class="font-medium truncate">
            {{ call.direction === 'inbound' ? call.caller : call.callee }}
          </p>
          <p class="text-xs text-gray-400">
            {{ call.direction }} &middot;
            {{ new Date(call.created_at).toLocaleString() }}
          </p>
        </div>
        <span
          :class="[
            'shrink-0 inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium capitalize',
            call.state === 'ringing' ? 'bg-yellow-100 text-yellow-800' : '',
            call.state === 'connected' ? 'bg-green-100 text-green-800' : '',
            call.state === 'ended' ? 'bg-gray-100 text-gray-800' : '',
          ]"
        >
          {{ call.state }}
        </span>
      </li>
    </ul>
  </div>
</template>
