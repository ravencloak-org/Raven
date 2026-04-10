<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { useRoute } from 'vue-router'
import { useWhatsAppStore } from '../../stores/whatsapp'
import InitiateCallModal from '../../components/whatsapp/InitiateCallModal.vue'
import ActiveCallPanel from '../../components/whatsapp/ActiveCallPanel.vue'

const route = useRoute()
const store = useWhatsAppStore()

const orgId = route.params.orgId as string

const showCallModal = ref(false)
const dateFrom = ref('')
const dateTo = ref('')

onMounted(async () => {
  await Promise.all([store.fetchCalls(orgId), store.fetchPhoneNumbers(orgId)])
})

const filteredCalls = computed(() => {
  if (!dateFrom.value && !dateTo.value) return store.calls

  return store.calls.filter((call) => {
    const created = new Date(call.created_at)
    if (dateFrom.value && created < new Date(dateFrom.value)) return false
    if (dateTo.value) {
      const end = new Date(dateTo.value)
      end.setDate(end.getDate() + 1)
      if (created >= end) return false
    }
    return true
  })
})

function directionLabel(dir: string) {
  return dir === 'inbound' ? 'Inbound' : 'Outbound'
}

function directionClasses(dir: string) {
  return dir === 'inbound'
    ? 'bg-blue-100 text-blue-800'
    : 'bg-purple-100 text-purple-800'
}

function stateClasses(state: string) {
  switch (state) {
    case 'ringing':
      return 'bg-yellow-100 text-yellow-800'
    case 'connected':
      return 'bg-green-100 text-green-800'
    case 'ended':
      return 'bg-gray-100 text-gray-800'
    default:
      return 'bg-gray-100 text-gray-600'
  }
}

function handleCallStarted() {
  showCallModal.value = false
}

function selectCall(callId: string) {
  store.fetchCall(orgId, callId)
  store.fetchBridgeStatus(orgId, callId)
}
</script>

<template>
  <div class="p-4 sm:p-6 max-w-5xl mx-auto">
    <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between mb-6">
      <h1 class="text-2xl font-bold">WhatsApp Calls</h1>
      <button
        class="min-h-[44px] min-w-[44px] rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700"
        @click="showCallModal = true"
      >
        New Call
      </button>
    </div>

    <!-- Active call panel -->
    <ActiveCallPanel
      v-if="store.activeCall"
      :org-id="orgId"
      class="mb-6"
    />

    <!-- Date filter -->
    <div class="flex flex-col gap-3 sm:flex-row sm:items-end mb-6">
      <div>
        <label class="block text-sm font-medium text-gray-700 mb-1" for="date-from">From</label>
        <input
          id="date-from"
          v-model="dateFrom"
          type="date"
          class="min-h-[44px] rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
        />
      </div>
      <div>
        <label class="block text-sm font-medium text-gray-700 mb-1" for="date-to">To</label>
        <input
          id="date-to"
          v-model="dateTo"
          type="date"
          class="min-h-[44px] rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
        />
      </div>
      <button
        v-if="dateFrom || dateTo"
        class="min-h-[44px] min-w-[44px] rounded-md border border-gray-300 px-3 py-2 text-sm text-gray-600 hover:bg-gray-50"
        @click="dateFrom = ''; dateTo = ''"
      >
        Clear
      </button>
    </div>

    <!-- Loading -->
    <div v-if="store.loading" class="text-gray-500">Loading...</div>

    <!-- Error -->
    <div v-else-if="store.error" class="text-red-600">{{ store.error }}</div>

    <!-- Empty -->
    <div v-else-if="filteredCalls.length === 0" class="text-gray-500">
      No calls found.
    </div>

    <!-- Calls list -->
    <div v-else>
      <!-- Mobile cards -->
      <div class="sm:hidden space-y-3">
        <div
          v-for="call in filteredCalls"
          :key="call.id"
          class="rounded-lg border border-gray-200 p-4 cursor-pointer hover:bg-gray-50"
          @click="selectCall(call.id)"
        >
          <div class="flex items-center gap-2 mb-2">
            <span
              :class="[
                'inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium',
                directionClasses(call.direction),
              ]"
            >
              {{ directionLabel(call.direction) }}
            </span>
            <span
              :class="[
                'inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium capitalize',
                stateClasses(call.state),
              ]"
            >
              {{ call.state }}
            </span>
          </div>
          <p class="text-sm font-medium">
            {{ call.caller }} &rarr; {{ call.callee }}
          </p>
          <p class="text-xs text-gray-400 mt-1">
            {{ new Date(call.created_at).toLocaleString() }}
          </p>
          <p v-if="call.duration_seconds != null" class="text-xs text-gray-400">
            Duration: {{ call.duration_seconds }}s
          </p>
        </div>
      </div>

      <!-- Desktop table -->
      <table class="hidden sm:table w-full border-collapse">
        <thead>
          <tr class="border-b border-gray-200 text-left text-sm font-medium text-gray-500">
            <th class="pb-3 pr-4">Direction</th>
            <th class="pb-3 pr-4">State</th>
            <th class="pb-3 pr-4">Caller</th>
            <th class="pb-3 pr-4">Callee</th>
            <th class="pb-3 pr-4">Date</th>
            <th class="pb-3">Duration</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="call in filteredCalls"
            :key="call.id"
            class="border-b border-gray-100 hover:bg-gray-50 cursor-pointer"
            @click="selectCall(call.id)"
          >
            <td class="py-3 pr-4">
              <span
                :class="[
                  'inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium',
                  directionClasses(call.direction),
                ]"
              >
                {{ directionLabel(call.direction) }}
              </span>
            </td>
            <td class="py-3 pr-4">
              <span
                :class="[
                  'inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium capitalize',
                  stateClasses(call.state),
                ]"
              >
                {{ call.state }}
              </span>
            </td>
            <td class="py-3 pr-4 text-sm">{{ call.caller }}</td>
            <td class="py-3 pr-4 text-sm">{{ call.callee }}</td>
            <td class="py-3 pr-4 text-sm text-gray-500">
              {{ new Date(call.created_at).toLocaleString() }}
            </td>
            <td class="py-3 text-sm text-gray-500">
              {{ call.duration_seconds != null ? `${call.duration_seconds}s` : '-' }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <p v-if="filteredCalls.length > 0" class="mt-4 text-sm text-gray-400">
      {{ store.callTotal }} call{{ store.callTotal === 1 ? '' : 's' }} total
    </p>

    <!-- Initiate call modal -->
    <InitiateCallModal
      v-if="showCallModal"
      :org-id="orgId"
      :phone-numbers="store.phoneNumbers"
      @close="showCallModal = false"
      @started="handleCallStarted"
    />
  </div>
</template>
