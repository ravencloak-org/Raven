<script setup lang="ts">
import { ref } from 'vue'
import { useWhatsAppStore } from '../../stores/whatsapp'
import type { WhatsAppPhoneNumber } from '../../api/whatsapp'
import ResponsiveModal from '../ResponsiveModal.vue'

const props = defineProps<{
  orgId: string
  phoneNumbers: WhatsAppPhoneNumber[]
  open?: boolean
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
  <ResponsiveModal :open="props.open ?? true" title="New Call" @close="emit('close')">
    <div class="p-6">
      <div class="hidden sm:flex items-center justify-between mb-4">
        <h2 class="text-lg font-bold text-white">New Call</h2>
        <button
          class="flex h-11 w-11 items-center justify-center rounded-lg text-slate-400 hover:bg-slate-700 hover:text-slate-200"
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
          <label class="block text-sm font-medium text-slate-300 mb-1" for="from-phone"
            >From Number</label
          >
          <select
            id="from-phone"
            v-model="selectedPhoneId"
            class="min-h-[44px] w-full rounded-md border border-slate-600 bg-slate-700 px-3 py-2 text-sm text-white focus:outline-none focus:ring-2 focus:ring-indigo-500"
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
          <label class="block text-sm font-medium text-slate-300 mb-1" for="callee-number"
            >Callee Number</label
          >
          <input
            id="callee-number"
            v-model="callee"
            type="text"
            placeholder="+1234567890"
            class="min-h-[44px] w-full rounded-md border border-slate-600 bg-slate-700 px-3 py-2 text-sm text-white placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-indigo-500"
            required
          />
        </div>

        <div v-if="submitError" class="text-sm text-red-400">{{ submitError }}</div>

        <div class="hidden sm:flex gap-3 pt-2">
          <button
            type="button"
            class="min-h-[44px] flex-1 rounded-md border border-slate-600 px-4 py-2 text-sm font-medium text-slate-300 hover:bg-slate-700"
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

    <template #actions>
      <div class="flex flex-col gap-2 sm:hidden">
        <button
          type="submit"
          class="w-full min-h-[48px] rounded-xl bg-indigo-600 text-sm font-semibold text-white disabled:opacity-50 disabled:cursor-not-allowed"
          :disabled="submitting || !selectedPhoneId || !callee.trim()"
        >
          {{ submitting ? 'Calling...' : 'Call' }}
        </button>
        <button
          type="button"
          class="w-full min-h-[48px] rounded-xl bg-slate-700 text-sm text-slate-200"
          @click="emit('close')"
        >
          Cancel
        </button>
      </div>
    </template>
  </ResponsiveModal>
</template>
