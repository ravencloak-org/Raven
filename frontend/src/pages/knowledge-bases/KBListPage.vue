<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useKnowledgeBasesStore } from '../../stores/knowledge-bases'

const route = useRoute()
const router = useRouter()
const store = useKnowledgeBasesStore()

const orgId = route.params.orgId as string
const wsId = route.params.wsId as string
const newName = ref('')
const creating = ref(false)

onMounted(() => store.fetchKnowledgeBases(orgId, wsId))

async function handleCreate() {
  const name = newName.value.trim()
  if (!name) return
  creating.value = true
  try {
    const kb = await store.create(orgId, wsId, { name })
    newName.value = ''
    router.push(`/orgs/${orgId}/workspaces/${wsId}/knowledge-bases/${kb.id}`)
  } catch {
    /* error surfaced via store.error */
  } finally {
    creating.value = false
  }
}

function openKB(kbId: string) {
  router.push(`/orgs/${orgId}/workspaces/${wsId}/knowledge-bases/${kbId}`)
}

async function handleArchive(kbId: string) {
  try {
    await store.archive(orgId, wsId, kbId)
  } catch {
    /* error surfaced via store.error */
  }
}

function statusColor(status: string): string {
  return status === 'active'
    ? 'bg-green-100 text-green-800'
    : 'bg-gray-100 text-gray-600'
}
</script>

<template>
  <div class="p-6 max-w-4xl mx-auto">
    <h1 class="text-2xl font-bold mb-6">Knowledge Bases</h1>

    <!-- Create KB form -->
    <form class="flex gap-3 mb-8" @submit.prevent="handleCreate">
      <input
        v-model="newName"
        type="text"
        placeholder="New knowledge base name"
        class="flex-1 rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
      />
      <button
        type="submit"
        :disabled="creating || !newName.trim()"
        class="rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-50 disabled:cursor-not-allowed"
      >
        {{ creating ? 'Creating...' : 'Create Knowledge Base' }}
      </button>
    </form>

    <!-- Loading state -->
    <div v-if="store.loading" class="text-gray-500">Loading...</div>

    <!-- Error state -->
    <div v-else-if="store.error" class="text-red-600">{{ store.error }}</div>

    <!-- Empty state -->
    <div v-else-if="store.knowledgeBases.length === 0" class="text-gray-500">
      No knowledge bases yet. Create one above.
    </div>

    <!-- KB cards grid -->
    <div v-else class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
      <div
        v-for="kb in store.knowledgeBases"
        :key="kb.id"
        class="rounded-lg border border-gray-200 p-4 hover:shadow-md transition-shadow cursor-pointer"
        @click="openKB(kb.id)"
      >
        <div class="flex items-start justify-between mb-2">
          <h3 class="font-semibold text-lg truncate" :title="kb.name">{{ kb.name }}</h3>
          <span
            class="ml-2 shrink-0 inline-block rounded-full px-2 py-0.5 text-xs font-medium"
            :class="statusColor(kb.status)"
          >
            {{ kb.status }}
          </span>
        </div>

        <p class="text-sm text-gray-500 mb-3">{{ kb.slug }}</p>

        <div class="flex items-center justify-between text-sm">
          <span class="text-gray-600">
            {{ kb.doc_count }} document{{ kb.doc_count === 1 ? '' : 's' }}
          </span>
          <button
            v-if="kb.status === 'active'"
            class="text-red-600 hover:text-red-800 text-xs"
            @click.stop="handleArchive(kb.id)"
          >
            Archive
          </button>
        </div>

        <p class="mt-2 text-xs text-gray-400">
          Updated {{ new Date(kb.updated_at).toLocaleDateString() }}
        </p>
      </div>
    </div>

    <!-- Total count -->
    <p v-if="store.knowledgeBases.length > 0" class="mt-4 text-sm text-gray-400">
      {{ store.total }} knowledge base{{ store.total === 1 ? '' : 's' }} total
    </p>
  </div>
</template>
