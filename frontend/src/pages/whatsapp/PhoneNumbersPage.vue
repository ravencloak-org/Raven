<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useWhatsAppStore } from '../../stores/whatsapp'
import { useMobile } from '../../composables/useMediaQuery'

const route = useRoute()
const store = useWhatsAppStore()

const { isMobile } = useMobile()

const orgId = route.params.orgId as string

const showForm = ref(false)
const phoneNumber = ref('')
const displayName = ref('')
const wabaId = ref('')
const submitting = ref(false)

onMounted(() => store.fetchPhoneNumbers(orgId))

async function handleAdd() {
  const num = phoneNumber.value.trim()
  const waba = wabaId.value.trim()
  if (!num || !waba) return

  submitting.value = true
  try {
    await store.addPhoneNumber(orgId, {
      phone_number: num,
      display_name: displayName.value.trim() || undefined,
      waba_id: waba,
    })
    phoneNumber.value = ''
    displayName.value = ''
    wabaId.value = ''
    showForm.value = false
  } catch {
    /* error surfaced via store.error */
  } finally {
    submitting.value = false
  }
}

async function handleDelete(phoneId: string) {
  try {
    await store.removePhoneNumber(orgId, phoneId)
  } catch {
    /* error surfaced via store.error */
  }
}
</script>

<template>
  <div class="p-4 sm:p-6 max-w-4xl mx-auto">
    <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between mb-6">
      <h1 class="text-2xl font-bold">Phone Numbers</h1>
      <button
        class="min-h-[44px] min-w-[44px] rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700"
        @click="showForm = !showForm"
      >
        {{ showForm ? 'Cancel' : 'Add Number' }}
      </button>
    </div>

    <!-- Add phone number form -->
    <form
      v-if="showForm"
      class="mb-8 space-y-4 rounded-lg border border-gray-200 p-4"
      @submit.prevent="handleAdd"
    >
      <div>
        <label class="block text-sm font-medium text-gray-700 mb-1" for="phone-number"
          >Phone Number</label
        >
        <input
          id="phone-number"
          v-model="phoneNumber"
          type="text"
          placeholder="+1234567890"
          class="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
          :class="isMobile ? 'min-h-[48px] text-[15px]' : ''"
          required
        />
      </div>
      <div>
        <label class="block text-sm font-medium text-gray-700 mb-1" for="display-name"
          >Display Name</label
        >
        <input
          id="display-name"
          v-model="displayName"
          type="text"
          placeholder="Business Name"
          class="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
          :class="isMobile ? 'min-h-[48px] text-[15px]' : ''"
        />
      </div>
      <div>
        <label class="block text-sm font-medium text-gray-700 mb-1" for="waba-id"
          >WABA ID</label
        >
        <input
          id="waba-id"
          v-model="wabaId"
          type="text"
          placeholder="WhatsApp Business Account ID"
          class="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
          :class="isMobile ? 'min-h-[48px] text-[15px]' : ''"
          required
        />
      </div>
      <button
        type="submit"
        :disabled="submitting || !phoneNumber.trim() || !wabaId.trim()"
        class="min-h-[44px] min-w-[44px] rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-50 disabled:cursor-not-allowed"
      >
        {{ submitting ? 'Adding...' : 'Add Phone Number' }}
      </button>
    </form>

    <!-- Loading -->
    <div v-if="store.loading" class="text-gray-500">Loading...</div>

    <!-- Error -->
    <div v-else-if="store.error" class="text-red-600">{{ store.error }}</div>

    <!-- Empty -->
    <div v-else-if="store.phoneNumbers.length === 0" class="text-gray-500">
      No phone numbers registered yet.
    </div>

    <!-- Phone numbers list -->
    <div v-else class="space-y-3 sm:space-y-0">
      <!-- Mobile cards -->
      <div class="sm:hidden space-y-3">
        <div
          v-for="phone in store.phoneNumbers"
          :key="phone.id"
          class="rounded-lg border border-gray-200 p-4"
        >
          <div class="flex items-center justify-between mb-2">
            <span class="font-medium">{{ phone.phone_number }}</span>
            <span
              v-if="phone.verified"
              class="inline-flex items-center rounded-full bg-green-100 px-2 py-0.5 text-xs font-medium text-green-800"
            >
              Verified
            </span>
            <span
              v-else
              class="inline-flex items-center rounded-full bg-yellow-100 px-2 py-0.5 text-xs font-medium text-yellow-800"
            >
              Unverified
            </span>
          </div>
          <p v-if="phone.display_name" class="text-sm text-gray-500 mb-1">
            {{ phone.display_name }}
          </p>
          <p class="text-xs text-gray-400 mb-3">
            Added {{ new Date(phone.created_at).toLocaleDateString() }}
          </p>
          <button
            class="min-h-[44px] min-w-[44px] text-sm text-red-600 hover:text-red-800"
            @click="handleDelete(phone.id)"
          >
            Delete
          </button>
        </div>
      </div>

      <!-- Desktop table -->
      <table class="hidden sm:table w-full border-collapse">
        <thead>
          <tr class="border-b border-gray-200 text-left text-sm font-medium text-gray-500">
            <th class="pb-3 pr-4">Number</th>
            <th class="pb-3 pr-4">Display Name</th>
            <th class="pb-3 pr-4">Status</th>
            <th class="pb-3 pr-4">Added</th>
            <th class="pb-3">Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="phone in store.phoneNumbers"
            :key="phone.id"
            class="border-b border-gray-100 hover:bg-gray-50"
          >
            <td class="py-3 pr-4 font-medium">{{ phone.phone_number }}</td>
            <td class="py-3 pr-4 text-sm text-gray-500">
              {{ phone.display_name || '-' }}
            </td>
            <td class="py-3 pr-4">
              <span
                v-if="phone.verified"
                class="inline-flex items-center rounded-full bg-green-100 px-2 py-0.5 text-xs font-medium text-green-800"
              >
                Verified
              </span>
              <span
                v-else
                class="inline-flex items-center rounded-full bg-yellow-100 px-2 py-0.5 text-xs font-medium text-yellow-800"
              >
                Unverified
              </span>
            </td>
            <td class="py-3 pr-4 text-sm text-gray-500">
              {{ new Date(phone.created_at).toLocaleDateString() }}
            </td>
            <td class="py-3">
              <button
                class="min-h-[44px] min-w-[44px] text-sm text-red-600 hover:text-red-800"
                @click="handleDelete(phone.id)"
              >
                Delete
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <p v-if="store.phoneNumbers.length > 0" class="mt-4 text-sm text-gray-400">
      {{ store.phoneTotal }} number{{ store.phoneTotal === 1 ? '' : 's' }} total
    </p>
  </div>
</template>
