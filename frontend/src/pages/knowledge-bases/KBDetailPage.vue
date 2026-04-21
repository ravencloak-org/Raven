<script setup lang="ts">
import { nextTick, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useKnowledgeBasesStore } from '../../stores/knowledge-bases'
import RecentConversationsCard from '../../components/conversations/RecentConversationsCard.vue'

const route = useRoute()
const router = useRouter()
const store = useKnowledgeBasesStore()

const orgId = route.params.orgId as string
const wsId = route.params.wsId as string
const kbId = route.params.kbId as string

// Edit state
const editing = ref(false)
const editName = ref('')
const saving = ref(false)

// Document upload state
const uploading = ref(false)
const dragOver = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)

// URL source state
const newSourceUrl = ref('')
const addingSource = ref(false)

onMounted(async () => {
  await store.fetchKnowledgeBase(orgId, wsId, kbId)
  store.fetchDocuments(orgId, wsId, kbId)
  store.fetchSources(orgId, wsId, kbId)
})

function startEdit() {
  if (!store.currentKB) return
  editName.value = store.currentKB.name
  editing.value = true
}

function cancelEdit() {
  editing.value = false
  editName.value = ''
}

async function handleSave() {
  const name = editName.value.trim()
  if (!name) return
  saving.value = true
  try {
    // TODO: add store.update when backend supports PATCH
    void name
    editing.value = false
  } catch {
    /* error surfaced via store.error */
  } finally {
    saving.value = false
  }
}

async function handleArchive() {
  try {
    await store.archive(orgId, wsId, kbId)
  } catch {
    /* error surfaced via store.error */
  }
}

function goBack() {
  router.push(`/orgs/${orgId}/workspaces/${wsId}/knowledge-bases`)
}

// --- Document upload ---

function triggerFileInput() {
  fileInput.value?.click()
}

async function handleFileSelect(event: Event) {
  const target = event.target as HTMLInputElement
  const files = target.files
  if (!files || files.length === 0) return
  await uploadFiles(Array.from(files))
  target.value = ''
}

function handleDragOver(event: DragEvent) {
  event.preventDefault()
  dragOver.value = true
}

function handleDragLeave() {
  dragOver.value = false
}

async function handleDrop(event: DragEvent) {
  event.preventDefault()
  dragOver.value = false
  const files = event.dataTransfer?.files
  if (!files || files.length === 0) return
  await uploadFiles(Array.from(files))
}

async function uploadFiles(files: File[]) {
  uploading.value = true
  try {
    for (const file of files) {
      await store.uploadDocument(orgId, wsId, kbId, file)
    }
  } catch {
    /* error surfaced via store.error */
  } finally {
    uploading.value = false
  }
}

// --- URL source ---

async function handleAddSource() {
  const url = newSourceUrl.value.trim()
  if (!url) return
  addingSource.value = true
  try {
    await store.createSource(orgId, wsId, kbId, url)
    newSourceUrl.value = ''
  } catch {
    /* error surfaced via store.error */
  } finally {
    addingSource.value = false
  }
}

// --- Chat ---

interface ChatMessage {
  role: 'user' | 'assistant'
  content: string
  streaming?: boolean
}

const chatMessages = ref<ChatMessage[]>([])
const chatInput = ref('')
const chatSessionId = ref<string | undefined>(undefined)
const chatStreaming = ref(false)
const chatError = ref('')
const chatContainer = ref<HTMLElement | null>(null)

async function scrollChatToBottom() {
  await nextTick()
  if (chatContainer.value) {
    chatContainer.value.scrollTop = chatContainer.value.scrollHeight
  }
}

async function sendChatMessage() {
  const query = chatInput.value.trim()
  if (!query || chatStreaming.value) return

  chatError.value = ''
  chatInput.value = ''
  chatStreaming.value = true

  chatMessages.value.push({ role: 'user', content: query })
  const assistantMsg: ChatMessage = { role: 'assistant', content: '', streaming: true }
  chatMessages.value.push(assistantMsg)
  const assistantIndex = chatMessages.value.length - 1

  await scrollChatToBottom()

  const apiBase = import.meta.env.VITE_API_BASE_URL ?? '/api/v1'
  const url = `${apiBase}/orgs/${orgId}/workspaces/${wsId}/knowledge-bases/${kbId}/completions`

  try {
    const response = await fetch(url, {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        query,
        session_id: chatSessionId.value,
      }),
    })

    if (!response.ok || !response.body) {
      throw new Error(`Request failed: ${response.status}`)
    }

    const reader = response.body.getReader()
    const decoder = new TextDecoder()
    let buffer = ''

    while (true) {
      const { value, done } = await reader.read()
      if (done) break

      buffer += decoder.decode(value, { stream: true })

      // SSE lines are separated by double newlines
      const parts = buffer.split('\n\n')
      buffer = parts.pop() ?? ''

      for (const part of parts) {
        if (!part.trim()) continue

        let eventType = ''
        let dataLine = ''

        for (const line of part.split('\n')) {
          if (line.startsWith('event: ')) {
            eventType = line.slice('event: '.length).trim()
          } else if (line.startsWith('data: ')) {
            dataLine = line.slice('data: '.length).trim()
          }
        }

        if (!dataLine) continue

        try {
          const parsed = JSON.parse(dataLine)

          if (eventType === 'token') {
            chatMessages.value[assistantIndex].content += parsed.text ?? ''
            await scrollChatToBottom()
          } else if (eventType === 'done') {
            chatMessages.value[assistantIndex].streaming = false
          } else if (eventType === 'error') {
            chatError.value = parsed.error ?? 'Streaming error'
            chatMessages.value[assistantIndex].streaming = false
          } else if (eventType === 'sources') {
            // Sources are informational; ignore for now
          }

          // Persist session_id from any event that provides it
          if (parsed.session_id) {
            chatSessionId.value = parsed.session_id
          }
        } catch {
          // Malformed JSON chunk, skip
        }
      }
    }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Unknown error'
    chatError.value = msg
  } finally {
    chatMessages.value[assistantIndex].streaming = false
    chatStreaming.value = false
    await scrollChatToBottom()
  }
}

function handleChatKeydown(event: KeyboardEvent) {
  if (event.key === 'Enter' && !event.shiftKey) {
    event.preventDefault()
    void sendChatMessage()
  }
}

// --- Status badge helpers ---

function statusBadgeClass(status: string): string {
  switch (status) {
    case 'pending':
      return 'bg-yellow-100 text-yellow-800'
    case 'processing':
      return 'bg-blue-100 text-blue-800'
    case 'completed':
      return 'bg-green-100 text-green-800'
    case 'failed':
      return 'bg-red-100 text-red-800'
    case 'active':
      return 'bg-green-100 text-green-800'
    case 'archived':
      return 'bg-gray-100 text-gray-600'
    default:
      return 'bg-gray-100 text-gray-800'
  }
}
</script>

<template>
  <div class="p-6 max-w-4xl mx-auto">
    <!-- Loading state -->
    <div v-if="store.loading" class="text-gray-500">Loading...</div>

    <!-- Error state -->
    <div v-else-if="store.error && !store.currentKB" class="text-red-600">{{ store.error }}</div>

    <!-- KB detail -->
    <div v-else-if="store.currentKB">
      <!-- Navigation -->
      <button class="text-sm text-indigo-600 hover:text-indigo-800 mb-4" @click="goBack">
        &larr; Back to Knowledge Bases
      </button>

      <!-- Error banner (non-blocking) -->
      <div v-if="store.error" class="mb-4 rounded-md bg-red-50 border border-red-200 px-4 py-3 text-sm text-red-700">
        {{ store.error }}
      </div>

      <!-- KB header -->
      <div class="flex items-start justify-between mb-6">
        <div v-if="!editing" class="flex-1 min-w-0">
          <div class="flex items-center gap-3">
            <h1 class="text-2xl font-bold truncate">{{ store.currentKB.name }}</h1>
            <span
              class="shrink-0 inline-block rounded-full px-2 py-0.5 text-xs font-medium"
              :class="statusBadgeClass(store.currentKB.status)"
            >
              {{ store.currentKB.status }}
            </span>
          </div>
          <p class="text-sm text-gray-500 mt-1">{{ store.currentKB.slug }}</p>
        </div>

        <!-- Edit name form -->
        <form v-else class="flex-1 flex gap-3" @submit.prevent="handleSave">
          <input
            v-model="editName"
            type="text"
            class="flex-1 rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
          />
          <button
            type="submit"
            :disabled="saving || !editName.trim()"
            class="rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {{ saving ? 'Saving...' : 'Save' }}
          </button>
          <button
            type="button"
            class="rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
            @click="cancelEdit"
          >
            Cancel
          </button>
        </form>

        <div v-if="!editing" class="flex gap-2 ml-4">
          <button
            class="rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
            @click="startEdit"
          >
            Edit
          </button>
          <button
            v-if="store.currentKB.status === 'active'"
            class="rounded-md bg-red-600 px-4 py-2 text-sm font-medium text-white hover:bg-red-700"
            @click="handleArchive"
          >
            Archive
          </button>
        </div>
      </div>

      <!-- KB info -->
      <dl class="grid grid-cols-2 sm:grid-cols-4 gap-4 mb-8 text-sm">
        <div>
          <dt class="text-gray-500">Documents</dt>
          <dd class="font-medium">{{ store.currentKB.doc_count }}</dd>
        </div>
        <div>
          <dt class="text-gray-500">Status</dt>
          <dd>
            <span
              class="inline-block rounded-full px-2 py-0.5 text-xs font-medium"
              :class="statusBadgeClass(store.currentKB.status)"
            >
              {{ store.currentKB.status }}
            </span>
          </dd>
        </div>
        <div>
          <dt class="text-gray-500">Created</dt>
          <dd>{{ new Date(store.currentKB.created_at).toLocaleDateString() }}</dd>
        </div>
        <div>
          <dt class="text-gray-500">Updated</dt>
          <dd>{{ new Date(store.currentKB.updated_at).toLocaleDateString() }}</dd>
        </div>
      </dl>

      <!-- Documents section -->
      <section class="mb-8">
        <h2 class="text-lg font-semibold mb-4">Documents</h2>

        <!-- Drag-and-drop upload zone -->
        <div
          class="mb-4 rounded-lg border-2 border-dashed p-8 text-center transition-colors"
          :class="dragOver ? 'border-indigo-500 bg-indigo-50' : 'border-gray-300 bg-gray-50'"
          @dragover="handleDragOver"
          @dragleave="handleDragLeave"
          @drop="handleDrop"
        >
          <input
            ref="fileInput"
            type="file"
            multiple
            class="hidden"
            @change="handleFileSelect"
          />
          <div v-if="uploading" class="text-gray-500">
            Uploading...
          </div>
          <div v-else>
            <p class="text-gray-600 mb-2">Drag and drop files here, or</p>
            <button
              type="button"
              class="rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700"
              @click="triggerFileInput"
            >
              Browse Files
            </button>
          </div>
        </div>

        <!-- Document list -->
        <div v-if="store.documents.length === 0" class="text-sm text-gray-500">
          No documents uploaded yet.
        </div>
        <table v-else class="w-full border-collapse">
          <thead>
            <tr class="border-b border-gray-200 text-left text-sm font-medium text-gray-500">
              <th class="pb-3 pr-4">File Name</th>
              <th class="pb-3 pr-4">Type</th>
              <th class="pb-3 pr-4">Uploaded</th>
              <th class="pb-3">Status</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="doc in store.documents"
              :key="doc.id"
              class="border-b border-gray-100"
            >
              <td class="py-3 pr-4 text-sm font-medium">{{ doc.file_name || doc.name }}</td>
              <td class="py-3 pr-4 text-sm text-gray-500">{{ doc.file_type || doc.type }}</td>
              <td class="py-3 pr-4 text-sm text-gray-500">{{ doc.created_at ? new Date(doc.created_at).toLocaleDateString() : '—' }}</td>
              <td class="py-3">
                <span
                  class="inline-block rounded-full px-2 py-0.5 text-xs font-medium"
                  :class="statusBadgeClass(doc.status)"
                >
                  {{ doc.status }}
                </span>
                <span
                  v-if="doc.status === 'failed'"
                  class="ml-2 text-xs text-red-600"
                >
                  Processing failed
                </span>
              </td>
            </tr>
          </tbody>
        </table>
      </section>

      <!-- URL Sources section -->
      <section class="mb-8">
        <h2 class="text-lg font-semibold mb-4">URL Sources</h2>

        <!-- Add source form -->
        <form class="flex gap-3 mb-4" @submit.prevent="handleAddSource">
          <input
            v-model="newSourceUrl"
            type="url"
            placeholder="https://example.com/docs"
            class="flex-1 rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
          />
          <button
            type="submit"
            :disabled="addingSource || !newSourceUrl.trim()"
            class="rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {{ addingSource ? 'Adding...' : 'Add Source' }}
          </button>
        </form>

        <!-- Source list -->
        <div v-if="store.sources.length === 0" class="text-sm text-gray-500">
          No URL sources added yet.
        </div>
        <table v-else class="w-full border-collapse">
          <thead>
            <tr class="border-b border-gray-200 text-left text-sm font-medium text-gray-500">
              <th class="pb-3 pr-4">URL</th>
              <th class="pb-3 pr-4">Added</th>
              <th class="pb-3">Status</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="source in store.sources"
              :key="source.id"
              class="border-b border-gray-100"
            >
              <td class="py-3 pr-4 text-sm">
                <a
                  :href="source.url"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="text-indigo-600 hover:text-indigo-800 underline break-all"
                  @click.stop
                >
                  {{ source.url }}
                </a>
              </td>
              <td class="py-3 pr-4 text-sm text-gray-500">
                {{ new Date(source.created_at).toLocaleDateString() }}
              </td>
              <td class="py-3">
                <span
                  class="inline-block rounded-full px-2 py-0.5 text-xs font-medium"
                  :class="statusBadgeClass(source.status)"
                >
                  {{ source.status }}
                </span>
                <span
                  v-if="source.status === 'failed'"
                  class="ml-2 text-xs text-red-600"
                >
                  Processing failed
                </span>
              </td>
            </tr>
          </tbody>
        </table>
      </section>
      <!-- Chat section -->
      <section class="mt-10">
        <h2 class="text-lg font-semibold mb-1">Chat with this KB</h2>
        <p class="text-sm text-gray-500 mb-4">Ask questions about the documents in this knowledge base.</p>

        <!-- Message list -->
        <div
          ref="chatContainer"
          class="border border-gray-200 rounded-lg bg-white h-96 overflow-y-auto p-4 flex flex-col gap-3 mb-3"
        >
          <!-- Empty state -->
          <div
            v-if="chatMessages.length === 0"
            class="flex-1 flex items-center justify-center text-sm text-gray-400"
          >
            No messages yet. Send a question to get started.
          </div>

          <!-- Messages -->
          <template v-for="(msg, idx) in chatMessages" :key="idx">
            <!-- User bubble -->
            <div v-if="msg.role === 'user'" class="flex justify-end">
              <div class="max-w-[75%] rounded-2xl rounded-tr-sm bg-amber-400 text-black px-4 py-2 text-sm leading-relaxed shadow-sm">
                {{ msg.content }}
              </div>
            </div>

            <!-- Assistant bubble -->
            <div v-else class="flex justify-start">
              <div class="max-w-[75%] rounded-2xl rounded-tl-sm bg-gray-100 text-gray-900 px-4 py-2 text-sm leading-relaxed shadow-sm">
                <span v-if="msg.content">{{ msg.content }}</span>
                <!-- Typing indicator -->
                <span v-else-if="msg.streaming" class="inline-flex items-center gap-1">
                  <span class="inline-block w-1.5 h-1.5 rounded-full bg-gray-500 animate-bounce" style="animation-delay:0ms" />
                  <span class="inline-block w-1.5 h-1.5 rounded-full bg-gray-500 animate-bounce" style="animation-delay:150ms" />
                  <span class="inline-block w-1.5 h-1.5 rounded-full bg-gray-500 animate-bounce" style="animation-delay:300ms" />
                </span>
                <!-- Streaming cursor -->
                <span
                  v-if="msg.streaming && msg.content"
                  class="inline-block w-0.5 h-4 bg-gray-600 ml-0.5 align-middle animate-pulse"
                />
              </div>
            </div>
          </template>
        </div>

        <!-- Error banner -->
        <div
          v-if="chatError"
          class="mb-3 rounded-md bg-red-50 border border-red-200 px-4 py-2 text-sm text-red-700"
        >
          {{ chatError }}
        </div>

        <!-- Input row -->
        <div class="flex gap-2">
          <textarea
            v-model="chatInput"
            rows="2"
            placeholder="Ask a question…"
            :disabled="chatStreaming"
            class="flex-1 resize-none rounded-lg border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-amber-400 disabled:opacity-50 disabled:cursor-not-allowed"
            @keydown="handleChatKeydown"
          />
          <button
            type="button"
            :disabled="chatStreaming || !chatInput.trim()"
            class="self-end rounded-lg bg-black px-5 py-2 text-sm font-medium text-white hover:bg-gray-800 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            @click="sendChatMessage"
          >
            {{ chatStreaming ? 'Sending…' : 'Send' }}
          </button>
        </div>
        <p class="mt-1.5 text-xs text-gray-400">Press Enter to send · Shift+Enter for new line</p>
      </section>

      <section class="mt-6">
        <RecentConversationsCard :org-id="orgId" :kb-id="kbId" />
      </section>
    </div>
  </div>
</template>
