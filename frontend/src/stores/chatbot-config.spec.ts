import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useChatbotConfigStore } from './chatbot-config'
import * as chatbotApi from '../api/chatbot-config'
import type { ChatbotConfig } from '../api/chatbot-config'

vi.mock('../api/chatbot-config')

const mockConfig: ChatbotConfig = {
  theme_color: '#4f46e5',
  avatar_url: 'https://cdn.raven.example/avatar.svg',
  welcome_text: 'Hello! How can I help?',
  suggested_questions: ['What is your return policy?', 'Track my order'],
  position: 'bottom-right',
  widget_title: 'Raven Chat',
}

describe('useChatbotConfigStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  describe('fetchConfig', () => {
    it('populates config on success', async () => {
      vi.mocked(chatbotApi.getChatbotConfig).mockResolvedValue({ ...mockConfig })

      const store = useChatbotConfigStore()
      await store.fetchConfig()

      expect(store.config).toEqual(mockConfig)
      expect(store.loading).toBe(false)
      expect(store.error).toBeNull()
    })

    it('sets loading state during fetch', async () => {
      let resolvePromise: (value: ChatbotConfig) => void
      vi.mocked(chatbotApi.getChatbotConfig).mockImplementation(
        () => new Promise((resolve) => { resolvePromise = resolve }),
      )

      const store = useChatbotConfigStore()
      const promise = store.fetchConfig()

      expect(store.loading).toBe(true)

      resolvePromise!({ ...mockConfig })
      await promise

      expect(store.loading).toBe(false)
    })

    it('sets error on failure', async () => {
      vi.mocked(chatbotApi.getChatbotConfig).mockRejectedValue(new Error('Network error'))

      const store = useChatbotConfigStore()
      await store.fetchConfig()

      expect(store.error).toBe('Network error')
      expect(store.config).toBeNull()
    })
  })

  describe('saveConfig', () => {
    it('updates config on success', async () => {
      const updated: ChatbotConfig = { ...mockConfig, theme_color: '#ff0000' }
      vi.mocked(chatbotApi.updateChatbotConfig).mockResolvedValue(updated)

      const store = useChatbotConfigStore()
      await store.saveConfig({ theme_color: '#ff0000' })

      expect(store.config).toEqual(updated)
      expect(store.saving).toBe(false)
      expect(store.error).toBeNull()
    })

    it('sets saving state during save', async () => {
      let resolvePromise: (value: ChatbotConfig) => void
      vi.mocked(chatbotApi.updateChatbotConfig).mockImplementation(
        () => new Promise((resolve) => { resolvePromise = resolve }),
      )

      const store = useChatbotConfigStore()
      const promise = store.saveConfig({ theme_color: '#ff0000' })

      expect(store.saving).toBe(true)

      resolvePromise!({ ...mockConfig, theme_color: '#ff0000' })
      await promise

      expect(store.saving).toBe(false)
    })

    it('sets error and rethrows on failure', async () => {
      vi.mocked(chatbotApi.updateChatbotConfig).mockRejectedValue(new Error('Save failed'))

      const store = useChatbotConfigStore()

      await expect(store.saveConfig({ theme_color: '#ff0000' })).rejects.toThrow('Save failed')
      expect(store.error).toBe('Save failed')
    })

    it('passes partial updates to the API', async () => {
      vi.mocked(chatbotApi.updateChatbotConfig).mockResolvedValue({ ...mockConfig })

      const store = useChatbotConfigStore()
      await store.saveConfig({ welcome_text: 'New welcome!' })

      expect(chatbotApi.updateChatbotConfig).toHaveBeenCalledWith({
        welcome_text: 'New welcome!',
      })
    })
  })

  describe('$reset', () => {
    it('resets all state to initial values', async () => {
      vi.mocked(chatbotApi.getChatbotConfig).mockResolvedValue({ ...mockConfig })

      const store = useChatbotConfigStore()
      await store.fetchConfig()

      expect(store.config).not.toBeNull()

      store.$reset()

      expect(store.config).toBeNull()
      expect(store.loading).toBe(false)
      expect(store.saving).toBe(false)
      expect(store.error).toBeNull()
    })
  })
})
