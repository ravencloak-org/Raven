<script setup lang="ts">
import { ref } from 'vue'
import { useWhatsAppStore } from '../../stores/whatsapp'
import type { WhatsAppPhoneNumber } from '../../api/whatsapp'

const props = defineProps<{
  orgId: string
  phoneNumbers: WhatsAppPhoneNumber[]
}>()

const emit = defineEmits<{
  close: []
  started: []
}>()

const store = useWhatsAppStore()

const selectedPhoneId = ref(props.phoneNumbers.length > 0 ? props.phoneNumbers[0].id : '')
const callee = ref('')
const submitting = ref(false)
const submitError = ref<string | null>(null)

async function handleSubmit() {
  const num = callee.value.trim()
  if (!selectedPhoneId.value || !num) return

  submitting.value = true
  submitError.value = null
  try {
    await store.startCall(props.orgId, {
      phone_number_id: selectedPhoneId.value,
      callee: num,
    })
    emit('started')
  } catch (e) {
    submitError.value = (e as Error).message
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="fixed inset-0 z-50 flex items-center justify-center">
    <!-- Backdrop -->
    <div class="absolute inset-0 bg-black/50" @click="emit('close')" />

    <!-- Modal -->
    <div class="relative z-10 w-full max-w-md mx-4 rounded-lg bg-white p-6 shadow-xl">
      <div class="flex items-center justify-between mb-4">
        <h2 class="text-lg font-bold">New Call</h2>
        <button
          class="flex h-11 w-11 items-center justify-center rounded-lg text-gray-400 hover:bg-gray-100 hover:text-gray-600"
          @click="emit('close')"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-5 w-5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            stroke-width="2"
          >
            <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>

      <form class="space-y-4" @submit.prevent="handleSubmit">
        <div>
          <label class="block text-sm font-medium text-gray-700 mb-1" for="from-phone"
            >From Number</label
          >
          <select
            id="from-phone"
            v-model="selectedPhoneId"
            class="min-h-[44px] w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
            required
          >
            <option v-if="phoneNumbers.length === 0" value="" disabled>
              No phone numbers available
            </option>
            <option v-for="phone in phoneNumbers" :key="phone.id" :value="phone.id">
              {{ phone.phone_number }}
              {{ phone.display_name ? `(${phone.display_name})` : '' }}
            </option>
          </select>
        </div>

        <div>
          <label class="block text-sm font-medium text-gray-700 mb-1" for="callee-number"
            >Callee Number</label
          >
          <input
            id="callee-number"
            v-model="callee"
            type="text"
            placeholder="+1234567890"
            class="min-h-[44px] w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
            required
          />
        </div>

        <div v-if="submitError" class="text-sm text-red-600">{{ submitError }}</div>

        <div class="flex gap-3 pt-2">
          <button
            type="button"
            class="min-h-[44px] flex-1 rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
            @click="emit('close')"
          >
            Cancel
          </button>
          <button
            type="submit"
            :disabled="submitting || !selectedPhoneId || !callee.trim()"
            class="min-h-[44px] flex-1 rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {{ submitting ? 'Calling...' : 'Call' }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>
