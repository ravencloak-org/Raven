<script setup lang="ts">
import type { VoiceSessionState } from '../../api/voice-sessions'

defineProps<{
  state: VoiceSessionState
}>()

const stateStyles: Record<VoiceSessionState, string> = {
  created: 'bg-yellow-100 text-yellow-800',
  active: 'bg-green-100 text-green-800',
  ended: 'bg-gray-100 text-gray-600',
}
</script>

<template>
  <span
    :class="[
      'inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium capitalize',
      stateStyles[state],
    ]"
  >
    {{ state }}
  </span>
</template>
