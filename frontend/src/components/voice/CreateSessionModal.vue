<script setup lang="ts">
import { ref } from 'vue'

const emit = defineEmits<{
  create: [roomName: string]
  close: []
}>()

const roomName = ref('')
const submitting = ref(false)

async function handleSubmit() {
  submitting.value = true
  try {
    emit('create', roomName.value.trim())
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="fixed inset-0 z-50 flex items-center justify-center">
    <div
      class="fixed inset-0 bg-black/50"
      data-testid="modal-backdrop"
      @click="$emit('close')"
    />
    <div
      class="relative z-10 w-full max-w-md rounded-lg bg-white p-6 shadow-xl"
    >
      <h2 class="mb-4 text-lg font-semibold">Create Voice Session</h2>
      <form @submit.prevent="handleSubmit">
        <label
          for="room-name"
          class="mb-1 block text-sm font-medium text-gray-700"
        >
          LiveKit Room Name
          <span class="text-gray-400">(optional)</span>
        </label>
        <input
          id="room-name"
          v-model="roomName"
          type="text"
          placeholder="Auto-generated if blank"
          class="mb-4 w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
        />
        <div class="flex justify-end gap-3">
          <button
            type="button"
            class="min-h-[44px] min-w-[44px] rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
            @click="$emit('close')"
          >
            Cancel
          </button>
          <button
            type="submit"
            :disabled="submitting"
            class="min-h-[44px] min-w-[44px] rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {{ submitting ? 'Creating...' : 'Create Session' }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>
