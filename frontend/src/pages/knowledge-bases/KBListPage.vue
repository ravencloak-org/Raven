<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useKnowledgeBasesStore } from '../../stores/knowledge-bases'
import { useMobile } from '../../composables/useMediaQuery'
import BottomSheet from '../../components/BottomSheet.vue'

const route = useRoute()
const router = useRouter()
const store = useKnowledgeBasesStore()
const { isMobile } = useMobile()

const orgId = route.params.orgId as string
const wsId = route.params.wsId as string
const newName = ref('')
const creating = ref(false)

// Archive confirmation state
const showArchiveDialog = ref(false)
const kbToArchiveId = ref<string | null>(null)
const kbToArchiveName = ref('')
const archiving = ref(false)
const archiveError = ref<string | null>(null)

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

function promptArchive(kbId: string, kbName: string) {
  kbToArchiveId.value = kbId
  kbToArchiveName.value = kbName
  showArchiveDialog.value = true
}

async function confirmArchive() {
  if (!kbToArchiveId.value) return
  archiving.value = true
  archiveError.value = null
  try {
    await store.archive(orgId, wsId, kbToArchiveId.value)
    showArchiveDialog.value = false
  } catch (e) {
    archiveError.value = (e as Error).message ?? 'Failed to archive. Please try again.'
  } finally {
    archiving.value = false
  }
}

function statusColor(status: string): string {
  return status === 'active'
    ? 'bg-green-100 text-green-800'
    : 'bg-gray-100 text-gray-600'
}

function mobileStatusClass(status: string): string {
  if (status === 'active') return 'bg-green-900 text-green-300'
  if (status === 'archived') return 'bg-slate-700 text-slate-300'
  return 'bg-amber-900 text-amber-300'
}
</script>

<template>
  <div class="p-6 max-w-4xl mx-auto">
    <h1 class="text-2xl font-bold mb-6">Knowledge Bases</h1>

    <!-- Create KB form -->
    <form
      class="flex gap-3 mb-8"
      :class="isMobile ? 'flex-col' : 'flex-row'"
      @submit.prevent="handleCreate"
    >
      <input
        v-model="newName"
        type="text"
        placeholder="New knowledge base name"
        class="flex-1 rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
        :class="isMobile ? 'min-h-[48px] text-[15px]' : ''"
      />
      <button
        type="submit"
        :disabled="creating || !newName.trim()"
        class="rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-50 disabled:cursor-not-allowed min-h-[44px]"
        :class="isMobile ? 'w-full' : ''"
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

    <!-- Desktop: KB cards grid -->
    <div v-else-if="!isMobile" class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
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
            class="text-red-600 hover:text-red-800 text-xs min-h-[44px] min-w-[44px]"
            @click.stop="promptArchive(kb.id, kb.name)"
          >
            Archive
          </button>
        </div>

        <p class="mt-2 text-xs text-gray-400">
          Updated {{ new Date(kb.updated_at).toLocaleDateString() }}
        </p>
      </div>
    </div>

    <!-- Mobile: card list -->
    <div v-else class="space-y-3">
      <div
        v-for="kb in store.knowledgeBases"
        :key="kb.id"
        class="bg-slate-800 rounded-xl p-3.5 cursor-pointer"
        @click="openKB(kb.id)"
      >
        <!-- Header row: title + status badge -->
        <div class="flex items-start justify-between gap-2">
          <span class="text-white font-semibold text-[15px] truncate">{{ kb.name }}</span>
          <span
            class="shrink-0 inline-block rounded-full px-2 py-0.5 text-xs font-medium"
            :class="mobileStatusClass(kb.status)"
          >
            {{ kb.status }}
          </span>
        </div>

        <!-- Subtitle: doc count -->
        <p class="text-slate-400 text-xs mt-1">
          {{ kb.doc_count }} document{{ kb.doc_count === 1 ? '' : 's' }}
        </p>

        <!-- Metadata -->
        <p class="text-slate-500 text-xs mt-2">
          Updated {{ new Date(kb.updated_at).toLocaleDateString() }}
        </p>

        <!-- Action row -->
        <div
          v-if="kb.status === 'active'"
          class="border-t border-slate-700 mt-2.5 pt-2.5 flex justify-end"
        >
          <button
            class="text-red-400 text-xs min-h-[44px] min-w-[44px]"
            @click.stop="promptArchive(kb.id, kb.name)"
          >
            Archive
          </button>
        </div>
      </div>
    </div>

    <!-- Total count -->
    <p v-if="store.knowledgeBases.length > 0" class="mt-4 text-sm text-gray-400">
      {{ store.total }} knowledge base{{ store.total === 1 ? '' : 's' }} total
    </p>

    <!-- Archive Confirmation Dialog: Desktop -->
    <div
      v-if="!isMobile && showArchiveDialog"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
      @click.self="showArchiveDialog = false"
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="archive-kb-title"
        class="w-full max-w-sm rounded-xl bg-white p-6 shadow-xl"
      >
        <h2 id="archive-kb-title" class="text-lg font-semibold text-gray-900">Archive Knowledge Base</h2>
        <p class="mt-2 text-sm text-gray-600">
          Are you sure you want to archive <strong>{{ kbToArchiveName }}</strong>? It will be removed from active use.
        </p>
        <p v-if="archiveError" class="mt-2 text-sm text-red-600">{{ archiveError }}</p>
        <div class="mt-6 flex justify-end gap-3">
          <button
            type="button"
            class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
            @click="showArchiveDialog = false"
          >
            Cancel
          </button>
          <button
            type="button"
            :disabled="archiving"
            class="rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-red-700 focus:outline-none focus:ring-2 focus:ring-red-500 focus:ring-offset-2 disabled:opacity-50"
            @click="confirmArchive"
          >
            {{ archiving ? 'Archiving...' : 'Archive' }}
          </button>
        </div>
      </div>
    </div>

    <!-- Archive Confirmation: Mobile bottom sheet -->
    <BottomSheet
      v-else-if="isMobile && showArchiveDialog"
      :open="showArchiveDialog"
      @close="showArchiveDialog = false"
    >
      <div class="px-4 pb-6 flex flex-col gap-3">
        <div class="flex flex-col items-center gap-2 py-2">
          <div class="flex h-12 w-12 items-center justify-center rounded-full bg-red-100">
            <svg class="h-6 w-6 text-red-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v2m0 4h.01M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z" />
            </svg>
          </div>
          <h2 class="text-base font-semibold text-white">Archive Knowledge Base</h2>
          <p class="text-center text-sm text-slate-400">
            Are you sure you want to archive <strong class="text-slate-200">{{ kbToArchiveName }}</strong>? It will be removed from active use.
          </p>
          <p v-if="archiveError" class="text-center text-sm text-red-400">{{ archiveError }}</p>
        </div>
        <button
          type="button"
          :disabled="archiving"
          class="w-full min-h-[48px] rounded-xl bg-red-600 text-sm font-semibold text-white disabled:opacity-50"
          @click="confirmArchive"
        >
          {{ archiving ? 'Archiving...' : 'Archive' }}
        </button>
        <button
          type="button"
          class="w-full min-h-[48px] rounded-xl bg-slate-700 text-sm text-slate-200"
          @click="showArchiveDialog = false"
        >
          Cancel
        </button>
      </div>
    </BottomSheet>
  </div>
</template>
