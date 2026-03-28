<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useWorkspacesStore } from '../../stores/workspaces'

const route = useRoute()
const router = useRouter()
const store = useWorkspacesStore()

const orgId = route.params.orgId as string
const newName = ref('')
const creating = ref(false)

onMounted(() => store.fetchWorkspaces(orgId))

async function handleCreate() {
  const name = newName.value.trim()
  if (!name) return
  creating.value = true
  try {
    const ws = await store.create(orgId, name)
    newName.value = ''
    router.push(`/orgs/${orgId}/workspaces/${ws.id}`)
  } catch {
    /* error surfaced via store.error */
  } finally {
    creating.value = false
  }
}

function openWorkspace(wsId: string) {
  router.push(`/orgs/${orgId}/workspaces/${wsId}`)
}

async function handleDelete(wsId: string) {
  try {
    await store.remove(orgId, wsId)
  } catch {
    /* error surfaced via store.error */
  }
}
</script>

<template>
  <div class="p-6 max-w-4xl mx-auto">
    <h1 class="text-2xl font-bold mb-6">Workspaces</h1>

    <!-- Create workspace form -->
    <form class="flex gap-3 mb-8" @submit.prevent="handleCreate">
      <input
        v-model="newName"
        type="text"
        placeholder="New workspace name"
        class="flex-1 rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
      />
      <button
        type="submit"
        :disabled="creating || !newName.trim()"
        class="rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-50 disabled:cursor-not-allowed"
      >
        {{ creating ? 'Creating...' : 'Create Workspace' }}
      </button>
    </form>

    <!-- Loading state -->
    <div v-if="store.loading" class="text-gray-500">Loading...</div>

    <!-- Error state -->
    <div v-else-if="store.error" class="text-red-600">{{ store.error }}</div>

    <!-- Empty state -->
    <div v-else-if="store.workspaces.length === 0" class="text-gray-500">
      No workspaces yet. Create one above.
    </div>

    <!-- Workspace table -->
    <table v-else class="w-full border-collapse">
      <thead>
        <tr class="border-b border-gray-200 text-left text-sm font-medium text-gray-500">
          <th class="pb-3 pr-4">Name</th>
          <th class="pb-3 pr-4">Slug</th>
          <th class="pb-3 pr-4">Created</th>
          <th class="pb-3">Actions</th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="ws in store.workspaces"
          :key="ws.id"
          class="border-b border-gray-100 hover:bg-gray-50 cursor-pointer"
          @click="openWorkspace(ws.id)"
        >
          <td class="py-3 pr-4 font-medium">{{ ws.name }}</td>
          <td class="py-3 pr-4 text-sm text-gray-500">{{ ws.slug }}</td>
          <td class="py-3 pr-4 text-sm text-gray-500">
            {{ new Date(ws.created_at).toLocaleDateString() }}
          </td>
          <td class="py-3">
            <button
              class="text-sm text-red-600 hover:text-red-800"
              @click.stop="handleDelete(ws.id)"
            >
              Delete
            </button>
          </td>
        </tr>
      </tbody>
    </table>

    <!-- Total count -->
    <p v-if="store.workspaces.length > 0" class="mt-4 text-sm text-gray-400">
      {{ store.total }} workspace{{ store.total === 1 ? '' : 's' }} total
    </p>
  </div>
</template>
