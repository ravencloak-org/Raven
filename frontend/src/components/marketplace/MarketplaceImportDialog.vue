<script setup lang="ts">
// MarketplaceImportDialog — workspace picker + POST to import endpoint.
// On success emits `imported` with the new KB id so the parent route can
// navigate. We require an authStore.orgId (no marketplace flow without an
// Org); the dialog defensively shows an empty-state when workspaces are
// missing rather than rendering an empty <select>.
import { onMounted, ref, watch } from 'vue'
import { listWorkspaces, type Workspace } from '../../api/workspaces'
import { importMarketplaceKb, MarketplaceApiError } from '../../api/marketplace'
import { useAuthStore } from '../../stores/auth'

const props = defineProps<{
  open: boolean
  publicKbId: string
}>()

const emit = defineEmits<{
  close: []
  imported: [payload: { orgId: string; workspaceId: string; kbId: string }]
}>()

const auth = useAuthStore()

const workspaces = ref<Workspace[]>([])
const selected = ref<string>('')
const loadingWorkspaces = ref(false)
const submitting = ref(false)
const errorMessage = ref('')

async function loadWorkspaces() {
  if (!props.open) return
  errorMessage.value = ''
  if (!auth.orgId) {
    errorMessage.value = 'You must belong to an organization to import a KB.'
    return
  }
  loadingWorkspaces.value = true
  try {
    const res = await listWorkspaces(auth.orgId, 0, 100)
    workspaces.value = res.items
    if (!selected.value && workspaces.value.length > 0) {
      selected.value = workspaces.value[0].id
    }
  } catch (err) {
    errorMessage.value = err instanceof Error ? err.message : 'Failed to load workspaces.'
  } finally {
    loadingWorkspaces.value = false
  }
}

onMounted(loadWorkspaces)
watch(() => props.open, (open) => { if (open) loadWorkspaces() })

async function submit() {
  if (!selected.value || !auth.orgId) return
  submitting.value = true
  errorMessage.value = ''
  try {
    const result = await importMarketplaceKb(props.publicKbId, selected.value)
    emit('imported', {
      orgId: auth.orgId,
      workspaceId: selected.value,
      kbId: result.kb_id,
    })
  } catch (err) {
    if (err instanceof MarketplaceApiError) {
      if (err.status === 403) {
        errorMessage.value = 'You do not have permission to import into this workspace.'
      } else if (err.status === 422) {
        errorMessage.value = 'Your plan does not allow this import.'
      } else {
        errorMessage.value = err.message
      }
    } else {
      errorMessage.value = err instanceof Error ? err.message : 'Import failed.'
    }
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div
    v-if="open"
    class="fixed inset-0 z-40 flex items-center justify-center bg-black/40 p-4"
    role="dialog"
    aria-modal="true"
    aria-labelledby="import-title"
    data-test="marketplace-import-dialog"
    @click.self="emit('close')"
  >
    <div class="w-full max-w-md rounded-xl bg-white p-6 shadow-xl">
      <header class="mb-4 flex items-start justify-between">
        <div>
          <h2 id="import-title" class="text-lg font-semibold text-gray-900">
            Import to workspace
          </h2>
          <p class="text-xs text-gray-500">
            A new Knowledge Base will be created as a fork of the source.
          </p>
        </div>
        <button
          type="button"
          class="rounded-md p-1 text-gray-500 hover:bg-gray-100"
          aria-label="Close import dialog"
          @click="emit('close')"
        >
          ×
        </button>
      </header>

      <div v-if="loadingWorkspaces" class="py-6 text-center text-sm text-gray-500">
        Loading workspaces…
      </div>
      <div
        v-else-if="workspaces.length === 0 && !errorMessage"
        class="rounded-md bg-yellow-50 border border-yellow-200 px-3 py-2 text-sm text-yellow-800"
      >
        You don't have any workspaces yet. Create one first.
      </div>
      <form v-else class="space-y-3" @submit.prevent="submit">
        <label class="block text-sm font-medium text-gray-700" for="import-workspace">
          Destination workspace
        </label>
        <select
          id="import-workspace"
          v-model="selected"
          required
          class="w-full rounded-md border border-gray-300 bg-white px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
          data-test="marketplace-import-workspace"
        >
          <option v-for="ws in workspaces" :key="ws.id" :value="ws.id">
            {{ ws.name }}
          </option>
        </select>
        <p v-if="errorMessage" class="text-xs text-red-600" data-test="marketplace-import-error">
          {{ errorMessage }}
        </p>
        <div class="flex justify-end gap-2 pt-2">
          <button
            type="button"
            class="rounded-md border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
            @click="emit('close')"
          >
            Cancel
          </button>
          <button
            type="submit"
            :disabled="submitting || !selected"
            class="rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:cursor-not-allowed disabled:opacity-50"
            data-test="marketplace-import-confirm"
          >
            {{ submitting ? 'Importing…' : 'Import' }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>
