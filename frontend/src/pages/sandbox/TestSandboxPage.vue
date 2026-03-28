<script setup lang="ts">
import { ref, nextTick, watch, onMounted } from 'vue'
import { useTestSandboxStore } from '../../stores/test-sandbox'
import { useKnowledgeBasesStore } from '../../stores/knowledge-bases'

const sandboxStore = useTestSandboxStore()
const kbStore = useKnowledgeBasesStore()

const messageInput = ref('')
const chatContainer = ref<HTMLElement | null>(null)

// TODO: Replace with real org/workspace IDs from route or auth context
const ORG_ID = 'org-456'
const WS_ID = 'ws-1'

onMounted(() => {
  if (kbStore.knowledgeBases.length === 0) {
    kbStore.fetchKnowledgeBases(ORG_ID, WS_ID)
  }
})

async function onKbChange(event: Event) {
  const kbId = (event.target as HTMLSelectElement).value
  if (kbId) {
    await sandboxStore.selectKb(kbId)
  }
}

async function handleSend() {
  const content = messageInput.value.trim()
  if (!content || !sandboxStore.hasSelectedKb) return
  messageInput.value = ''
  await sandboxStore.sendMessage(content)
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Enter' && !event.shiftKey) {
    event.preventDefault()
    handleSend()
  }
}

function scrollToBottom() {
  nextTick(() => {
    if (chatContainer.value) {
      const lastChild = chatContainer.value.lastElementChild
      if (lastChild) {
        lastChild.scrollIntoView({ behavior: 'smooth', block: 'end' })
      }
    }
  })
}

// Auto-scroll when messages change or during streaming
watch(
  () => sandboxStore.messages.map((m) => m.content).join(''),
  () => scrollToBottom(),
)

function formatTime(timestamp: string): string {
  const date = new Date(timestamp)
  return date.toLocaleTimeString('en-US', {
    hour: 'numeric',
    minute: '2-digit',
    hour12: true,
  })
}

const activeKbs = ref<typeof kbStore.knowledgeBases>([])
watch(
  () => kbStore.knowledgeBases,
  (kbs) => {
    activeKbs.value = kbs.filter((kb) => kb.status === 'active')
  },
  { immediate: true },
)
</script>

<template>
  <div class="flex h-full flex-col">
    <!-- Header -->
    <div class="border-b border-gray-200 bg-white px-6 py-4">
      <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 class="text-2xl font-bold text-gray-900">Test Sandbox</h1>
          <p class="mt-1 text-sm text-gray-500">
            Test your chatbot against a knowledge base before going live
          </p>
        </div>
        <div class="flex items-center gap-3">
          <button
            v-if="sandboxStore.hasMessages"
            class="rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
            @click="sandboxStore.clearConversation()"
          >
            Clear conversation
          </button>
        </div>
      </div>

      <!-- KB Selector -->
      <div class="mt-4">
        <label for="kb-select" class="block text-sm font-medium text-gray-700">
          Knowledge Base
        </label>
        <select
          id="kb-select"
          :value="sandboxStore.selectedKbId ?? ''"
          class="mt-1 block w-full max-w-md rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 shadow-sm focus:border-indigo-500 focus:ring-2 focus:ring-indigo-500 focus:outline-none"
          @change="onKbChange"
        >
          <option value="" disabled>Select a knowledge base...</option>
          <option v-for="kb in activeKbs" :key="kb.id" :value="kb.id">
            {{ kb.name }} ({{ kb.doc_count }} docs)
          </option>
        </select>
      </div>
    </div>

    <!-- Chat area -->
    <div class="flex flex-1 flex-col overflow-hidden bg-gray-50">
      <!-- Empty state: no KB selected -->
      <div
        v-if="!sandboxStore.hasSelectedKb"
        class="flex flex-1 items-center justify-center"
      >
        <div class="text-center">
          <div class="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-gray-200">
            <svg class="h-8 w-8 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
              <path stroke-linecap="round" stroke-linejoin="round" d="M20.25 8.511c.884.284 1.5 1.128 1.5 2.097v4.286c0 1.136-.847 2.1-1.98 2.193-.34.027-.68.052-1.02.072v3.091l-3-3c-1.354 0-2.694-.055-4.02-.163a2.115 2.115 0 01-.825-.242m9.345-8.334a2.126 2.126 0 00-.476-.095 48.64 48.64 0 00-8.048 0c-1.131.094-1.976 1.057-1.976 2.192v4.286c0 .837.46 1.58 1.155 1.951m9.345-8.334V6.637c0-1.621-1.152-3.026-2.76-3.235A48.455 48.455 0 0011.25 3c-2.115 0-4.198.137-6.24.402-1.608.209-2.76 1.614-2.76 3.235v6.226c0 1.621 1.152 3.026 2.76 3.235.577.075 1.157.14 1.74.194V21l4.155-4.155" />
            </svg>
          </div>
          <h3 class="text-lg font-medium text-gray-900">Select a Knowledge Base</h3>
          <p class="mt-1 text-sm text-gray-500">
            Choose a knowledge base from the dropdown above to start testing your chatbot.
          </p>
        </div>
      </div>

      <!-- Loading history -->
      <div
        v-else-if="sandboxStore.loading"
        class="flex flex-1 items-center justify-center"
      >
        <div class="flex items-center gap-3">
          <div class="h-6 w-6 animate-spin rounded-full border-2 border-indigo-200 border-t-indigo-600"></div>
          <span class="text-sm text-gray-500">Loading conversation...</span>
        </div>
      </div>

      <!-- Chat messages -->
      <template v-else>
        <!-- Empty conversation state -->
        <div
          v-if="!sandboxStore.hasMessages && !sandboxStore.streaming"
          class="flex flex-1 items-center justify-center"
        >
          <div class="text-center">
            <div class="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-indigo-100">
              <svg class="h-8 w-8 text-indigo-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                <path stroke-linecap="round" stroke-linejoin="round" d="M7.5 8.25h9m-9 3H12m-9.75 1.51c0 1.6 1.123 2.994 2.707 3.227 1.129.166 2.27.293 3.423.379.35.026.67.21.865.501L12 21l2.755-4.133a1.14 1.14 0 01.865-.501 48.172 48.172 0 003.423-.379c1.584-.233 2.707-1.626 2.707-3.228V6.741c0-1.602-1.123-2.995-2.707-3.228A48.394 48.394 0 0012 3c-2.392 0-4.744.175-7.043.513C3.373 3.746 2.25 5.14 2.25 6.741v6.018z" />
              </svg>
            </div>
            <h3 class="text-lg font-medium text-gray-900">Start a Conversation</h3>
            <p class="mt-1 text-sm text-gray-500">
              Send a message below to test your chatbot's responses.
            </p>
          </div>
        </div>

        <!-- Message list -->
        <div
          v-else
          ref="chatContainer"
          class="flex-1 space-y-4 overflow-y-auto px-6 py-4"
        >
          <div
            v-for="msg in sandboxStore.messages"
            :key="msg.id"
            class="flex"
            :class="msg.role === 'user' ? 'justify-end' : 'justify-start'"
          >
            <div
              class="max-w-[75%] rounded-2xl px-4 py-3"
              :class="
                msg.role === 'user'
                  ? 'bg-indigo-600 text-white'
                  : 'border border-gray-200 bg-white text-gray-900'
              "
            >
              <p class="whitespace-pre-wrap text-sm leading-relaxed">{{ msg.content }}<span
                v-if="msg.role === 'assistant' && sandboxStore.streaming && msg === sandboxStore.messages[sandboxStore.messages.length - 1]"
                class="ml-0.5 inline-block h-4 w-1 animate-pulse bg-current align-middle"
              ></span></p>
              <p
                class="mt-1 text-xs"
                :class="msg.role === 'user' ? 'text-indigo-200' : 'text-gray-400'"
              >
                {{ formatTime(msg.timestamp) }}
              </p>
            </div>
          </div>

          <!-- Typing indicator when streaming hasn't produced content yet -->
          <div
            v-if="sandboxStore.streaming && sandboxStore.messages.length > 0 && sandboxStore.messages[sandboxStore.messages.length - 1].content === ''"
            class="flex justify-start"
          >
            <div class="rounded-2xl border border-gray-200 bg-white px-4 py-3">
              <div class="flex items-center gap-1">
                <span class="h-2 w-2 animate-bounce rounded-full bg-gray-400" style="animation-delay: 0ms"></span>
                <span class="h-2 w-2 animate-bounce rounded-full bg-gray-400" style="animation-delay: 150ms"></span>
                <span class="h-2 w-2 animate-bounce rounded-full bg-gray-400" style="animation-delay: 300ms"></span>
              </div>
            </div>
          </div>
        </div>

        <!-- Error banner -->
        <div
          v-if="sandboxStore.error"
          class="mx-6 mb-2 rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
        >
          {{ sandboxStore.error }}
        </div>

        <!-- Message input -->
        <div class="border-t border-gray-200 bg-white px-6 py-4">
          <form class="flex items-end gap-3" @submit.prevent="handleSend">
            <textarea
              v-model="messageInput"
              rows="1"
              placeholder="Type a message to test..."
              :disabled="!sandboxStore.hasSelectedKb || sandboxStore.streaming"
              class="flex-1 resize-none rounded-lg border border-gray-300 px-4 py-2.5 text-sm focus:border-indigo-500 focus:ring-2 focus:ring-indigo-500 focus:outline-none disabled:cursor-not-allowed disabled:bg-gray-100"
              @keydown="handleKeydown"
            ></textarea>
            <button
              type="submit"
              :disabled="!messageInput.trim() || !sandboxStore.hasSelectedKb || sandboxStore.streaming"
              class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-indigo-600 text-white hover:bg-indigo-700 disabled:cursor-not-allowed disabled:opacity-50"
            >
              <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M6 12L3.269 3.126A59.768 59.768 0 0121.485 12 59.77 59.77 0 013.27 20.876L5.999 12zm0 0h7.5" />
              </svg>
            </button>
          </form>
          <p class="mt-2 text-xs text-gray-400">
            Press Enter to send, Shift+Enter for a new line
          </p>
        </div>
      </template>
    </div>
  </div>
</template>
