<script setup lang="ts">
import { computed } from 'vue'
import type { WhatsAppCall } from '../../api/whatsapp'

const props = defineProps<{
  calls: WhatsAppCall[]
  phoneNumberId?: string
}>()

interface LogEntry {
  id: string
  event: string
  callId: string
  timestamp: string
  direction: string
  state: string
}

const events = computed<LogEntry[]>(() => {
  const filtered = props.phoneNumberId
    ? props.calls.filter((c) => c.phone_number_id === props.phoneNumberId)
    : props.calls

  return filtered.flatMap((call) => {
    const entries: LogEntry[] = []

    if (call.started_at) {
      entries.push({
        id: `${call.id}-started`,
        event: 'call_started',
        callId: call.call_id,
        timestamp: call.started_at,
        direction: call.direction,
        state: 'ringing',
      })
    }

    if (call.state === 'connected') {
      entries.push({
        id: `${call.id}-connected`,
        event: 'call_connected',
        callId: call.call_id,
        timestamp: call.updated_at,
        direction: call.direction,
        state: 'connected',
      })
    }

    if (call.ended_at) {
      entries.push({
        id: `${call.id}-ended`,
        event: 'call_ended',
        callId: call.call_id,
        timestamp: call.ended_at,
        direction: call.direction,
        state: 'ended',
      })
    }

    return entries
  })
    .sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime())
    .slice(0, 50)
})

function eventBadgeColor(event: string): string {
  switch (event) {
    case 'call_started':
      return 'bg-amber-100 text-amber-800'
    case 'call_connected':
      return 'bg-green-100 text-green-800'
    case 'call_ended':
      return 'bg-gray-100 text-gray-700'
    default:
      return 'bg-gray-100 text-gray-700'
  }
}

function formatTime(ts: string): string {
  return new Date(ts).toLocaleTimeString(undefined, {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}
</script>

<template>
  <div class="rounded-xl border border-gray-200 bg-white">
    <div class="border-b border-gray-100 px-4 py-3">
      <h3 class="text-sm font-semibold text-gray-900">Webhook Event Log</h3>
      <p class="mt-0.5 text-xs text-gray-500">
        Recent call events{{ phoneNumberId ? ' for this number' : '' }} (derived from call records)
      </p>
    </div>

    <div v-if="events.length === 0" class="px-4 py-6 text-center text-sm text-gray-500">
      No events yet
    </div>

    <ul v-else class="divide-y divide-gray-50">
      <li
        v-for="entry in events"
        :key="entry.id"
        class="flex items-start gap-3 px-4 py-3 text-xs"
      >
        <span
          class="mt-0.5 shrink-0 rounded-full px-2 py-0.5 font-medium"
          :class="eventBadgeColor(entry.event)"
        >
          {{ entry.event }}
        </span>
        <div class="min-w-0 flex-1">
          <p class="truncate font-mono text-gray-700">{{ entry.callId }}</p>
          <p class="mt-0.5 capitalize text-gray-500">{{ entry.direction }}</p>
        </div>
        <time :datetime="new Date(entry.timestamp).toISOString()" class="shrink-0 text-gray-400">{{ formatTime(entry.timestamp) }}</time>
      </li>
    </ul>
  </div>
</template>
