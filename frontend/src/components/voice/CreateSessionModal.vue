<script setup lang="ts">
import { ref } from 'vue'
import ResponsiveModal from '../ResponsiveModal.vue'

const props = defineProps<{
  open?: boolean
}>()

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
  <ResponsiveModal :open="props.open ?? true" title="Create Voice Session" @close="$emit('close')">
    <form class="p-6" @submit.prevent="handleSubmit">
      <h2 class="mb-4 text-lg font-semibold text-white">Create Voice Session</h2>
      <label
        for="room-name"
        class="mb-1 block text-sm font-medium text-slate-300"
      >
        LiveKit Room Name
        <span class="text-slate-400">(optional)</span>
      </label>
      <input
        id="room-name"
        v-model="roomName"
        type="text"
        placeholder="Auto-generated if blank"
        class="mb-4 w-full rounded-md border border-slate-600 bg-slate-700 px-3 py-2 text-sm text-white placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-indigo-500"
      />
      <div class="hidden sm:flex justify-end gap-3">
        <button
          type="button"
          class="min-h-[44px] min-w-[44px] rounded-md border border-slate-600 px-4 py-2 text-sm font-medium text-slate-300 hover:bg-slate-700"
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
    <template #actions>
      <div class="flex flex-col gap-2 sm:hidden">
        <button
          type="button"
          class="w-full min-h-[48px] rounded-xl bg-indigo-600 text-sm font-semibold text-white"
          :disabled="submitting"
          @click="handleSubmit"
        >
          {{ submitting ? 'Creating...' : 'Create Session' }}
        </button>
        <button
          type="button"
          class="w-full min-h-[48px] rounded-xl bg-slate-700 text-sm text-slate-200"
          @click="$emit('close')"
        >
          Cancel
        </button>
      </div>
    </template>
  </ResponsiveModal>
</template>
