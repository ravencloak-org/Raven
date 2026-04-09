<script setup lang="ts">
import type { VoiceTurn } from '../../api/voice-sessions'

defineProps<{
  turns: VoiceTurn[]
}>()

function formatTime(iso: string): string {
  return new Date(iso).toLocaleTimeString()
}

function speakerLabel(speaker: string): string {
  return speaker === 'agent' ? 'Agent' : 'User'
}
</script>

<template>
  <div class="space-y-3">
    <div v-if="turns.length === 0" class="text-sm text-gray-500">
      No turns recorded yet.
    </div>
    <div
      v-for="turn in turns"
      :key="turn.id"
      :class="[
        'rounded-lg border p-3',
        turn.speaker === 'agent'
          ? 'border-blue-200 bg-blue-50'
          : 'border-gray-200 bg-white',
      ]"
    >
      <div class="mb-1 flex items-center justify-between">
        <span
          :class="[
            'text-xs font-semibold uppercase',
            turn.speaker === 'agent' ? 'text-blue-700' : 'text-gray-700',
          ]"
        >
          {{ speakerLabel(turn.speaker) }}
        </span>
        <span class="text-xs text-gray-400">
          {{ formatTime(turn.started_at) }}
          <template v-if="turn.ended_at">
            &ndash; {{ formatTime(turn.ended_at) }}
          </template>
        </span>
      </div>
      <p class="text-sm text-gray-800">{{ turn.transcript }}</p>
    </div>
  </div>
</template>
