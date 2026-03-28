import { defineStore } from 'pinia'
import { ref } from 'vue'
import {
  getChatbotConfig,
  updateChatbotConfig,
  type ChatbotConfig,
  type UpdateChatbotConfigRequest,
} from '../api/chatbot-config'

export const useChatbotConfigStore = defineStore('chatbotConfig', () => {
  const config = ref<ChatbotConfig | null>(null)
  const loading = ref(false)
  const saving = ref(false)
  const error = ref<string | null>(null)

  async function fetchConfig(): Promise<void> {
    loading.value = true
    error.value = null
    try {
      config.value = await getChatbotConfig()
    } catch (e) {
      error.value = (e as Error).message
    } finally {
      loading.value = false
    }
  }

  async function saveConfig(updates: UpdateChatbotConfigRequest): Promise<void> {
    saving.value = true
    error.value = null
    try {
      config.value = await updateChatbotConfig(updates)
    } catch (e) {
      error.value = (e as Error).message
      throw e
    } finally {
      saving.value = false
    }
  }

  function $reset(): void {
    config.value = null
    loading.value = false
    saving.value = false
    error.value = null
  }

  return { config, loading, saving, error, fetchConfig, saveConfig, $reset }
})
