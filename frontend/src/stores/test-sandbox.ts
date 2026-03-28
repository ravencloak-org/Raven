import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import {
  sendTestMessage,
  getTestHistory,
  generateMockId,
  type TestMessage,
} from '../api/test-sandbox'

export const useTestSandboxStore = defineStore('testSandbox', () => {
  const messages = ref<TestMessage[]>([])
  const selectedKbId = ref<string | null>(null)
  const loading = ref(false)
  const streaming = ref(false)
  const error = ref<string | null>(null)

  const hasMessages = computed(() => messages.value.length > 0)
  const hasSelectedKb = computed(() => selectedKbId.value !== null)

  async function selectKb(kbId: string): Promise<void> {
    if (selectedKbId.value === kbId) return
    selectedKbId.value = kbId
    messages.value = []
    error.value = null
    await loadHistory(kbId)
  }

  async function loadHistory(kbId: string): Promise<void> {
    loading.value = true
    error.value = null
    try {
      messages.value = await getTestHistory(kbId)
    } catch (e) {
      error.value = (e as Error).message
    } finally {
      loading.value = false
    }
  }

  async function sendMessage(content: string): Promise<void> {
    if (!selectedKbId.value || !content.trim()) return

    const userMessage: TestMessage = {
      id: generateMockId(),
      role: 'user',
      content: content.trim(),
      timestamp: new Date().toISOString(),
    }
    messages.value.push(userMessage)

    const assistantMessage: TestMessage = {
      id: generateMockId(),
      role: 'assistant',
      content: '',
      timestamp: new Date().toISOString(),
    }
    messages.value.push(assistantMessage)

    streaming.value = true
    error.value = null

    try {
      const stream = sendTestMessage(selectedKbId.value, content.trim())
      for await (const chunk of stream) {
        // Update the last message (assistant) with streaming content
        const lastMsg = messages.value[messages.value.length - 1]
        if (lastMsg && lastMsg.role === 'assistant') {
          lastMsg.content += chunk
        }
      }
    } catch (e) {
      error.value = (e as Error).message
      // Remove the empty assistant message on error
      const lastMsg = messages.value[messages.value.length - 1]
      if (lastMsg && lastMsg.role === 'assistant' && !lastMsg.content) {
        messages.value.pop()
      }
    } finally {
      streaming.value = false
    }
  }

  function clearConversation(): void {
    messages.value = []
    error.value = null
  }

  function $reset(): void {
    messages.value = []
    selectedKbId.value = null
    loading.value = false
    streaming.value = false
    error.value = null
  }

  return {
    messages,
    selectedKbId,
    loading,
    streaming,
    error,
    hasMessages,
    hasSelectedKb,
    selectKb,
    loadHistory,
    sendMessage,
    clearConversation,
    $reset,
  }
})
