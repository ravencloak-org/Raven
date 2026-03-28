import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useTestSandboxStore } from './test-sandbox'
import type { TestMessage } from '../api/test-sandbox'

vi.mock('../api/test-sandbox', () => ({
  sendTestMessage: vi.fn(),
  getTestHistory: vi.fn(),
  generateMockId: vi.fn(),
}))

import {
  sendTestMessage,
  getTestHistory,
  generateMockId,
} from '../api/test-sandbox'

const mockedSendTestMessage = vi.mocked(sendTestMessage)
const mockedGetTestHistory = vi.mocked(getTestHistory)
const mockedGenerateMockId = vi.mocked(generateMockId)

let idCounter = 0

function fakeMessage(overrides: Partial<TestMessage> = {}): TestMessage {
  return {
    id: `msg-${++idCounter}`,
    role: 'user',
    content: 'Hello',
    timestamp: '2026-03-28T10:00:00Z',
    ...overrides,
  }
}

/** Helper to create a mock async generator that yields words. */
async function* fakeStream(words: string[]): AsyncGenerator<string, void, unknown> {
  for (const word of words) {
    yield word + ' '
  }
}

describe('useTestSandboxStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    idCounter = 0
    mockedGenerateMockId.mockImplementation(() => `msg-${++idCounter}`)
  })

  describe('initial state', () => {
    it('has empty default state', () => {
      const store = useTestSandboxStore()

      expect(store.messages).toEqual([])
      expect(store.selectedKbId).toBeNull()
      expect(store.loading).toBe(false)
      expect(store.streaming).toBe(false)
      expect(store.error).toBeNull()
      expect(store.hasMessages).toBe(false)
      expect(store.hasSelectedKb).toBe(false)
    })
  })

  describe('selectKb', () => {
    it('sets selectedKbId and loads history', async () => {
      const history: TestMessage[] = [
        fakeMessage({ role: 'user', content: 'Old question' }),
        fakeMessage({ role: 'assistant', content: 'Old answer' }),
      ]
      mockedGetTestHistory.mockResolvedValue(history)

      const store = useTestSandboxStore()
      await store.selectKb('kb-1')

      expect(store.selectedKbId).toBe('kb-1')
      expect(mockedGetTestHistory).toHaveBeenCalledWith('kb-1')
      expect(store.messages).toEqual(history)
      expect(store.hasSelectedKb).toBe(true)
      expect(store.loading).toBe(false)
    })

    it('clears messages when switching KB', async () => {
      mockedGetTestHistory.mockResolvedValue([])

      const store = useTestSandboxStore()
      store.messages = [fakeMessage()]
      await store.selectKb('kb-2')

      expect(store.selectedKbId).toBe('kb-2')
      expect(store.messages).toEqual([])
    })

    it('does nothing if same KB is selected', async () => {
      mockedGetTestHistory.mockResolvedValue([])

      const store = useTestSandboxStore()
      await store.selectKb('kb-1')
      vi.clearAllMocks()

      await store.selectKb('kb-1')
      expect(mockedGetTestHistory).not.toHaveBeenCalled()
    })

    it('sets error on history load failure', async () => {
      mockedGetTestHistory.mockRejectedValue(new Error('Failed to load history'))

      const store = useTestSandboxStore()
      await store.selectKb('kb-1')

      expect(store.error).toBe('Failed to load history')
      expect(store.loading).toBe(false)
    })
  })

  describe('sendMessage', () => {
    it('adds user message and streams assistant response', async () => {
      mockedGetTestHistory.mockResolvedValue([])
      mockedSendTestMessage.mockReturnValue(fakeStream(['Hello', 'world']))

      const store = useTestSandboxStore()
      await store.selectKb('kb-1')
      await store.sendMessage('Test question')

      expect(store.messages).toHaveLength(2)
      expect(store.messages[0].role).toBe('user')
      expect(store.messages[0].content).toBe('Test question')
      expect(store.messages[1].role).toBe('assistant')
      expect(store.messages[1].content).toBe('Hello world ')
      expect(store.streaming).toBe(false)
    })

    it('trims whitespace from message content', async () => {
      mockedGetTestHistory.mockResolvedValue([])
      mockedSendTestMessage.mockReturnValue(fakeStream(['OK']))

      const store = useTestSandboxStore()
      await store.selectKb('kb-1')
      await store.sendMessage('  padded message  ')

      expect(store.messages[0].content).toBe('padded message')
      expect(mockedSendTestMessage).toHaveBeenCalledWith('kb-1', 'padded message')
    })

    it('does nothing if no KB selected', async () => {
      const store = useTestSandboxStore()
      await store.sendMessage('Test')

      expect(store.messages).toHaveLength(0)
      expect(mockedSendTestMessage).not.toHaveBeenCalled()
    })

    it('does nothing for empty message', async () => {
      mockedGetTestHistory.mockResolvedValue([])

      const store = useTestSandboxStore()
      await store.selectKb('kb-1')
      await store.sendMessage('   ')

      expect(store.messages).toHaveLength(0)
      expect(mockedSendTestMessage).not.toHaveBeenCalled()
    })

    it('sets error and removes empty assistant message on stream failure', async () => {
      mockedGetTestHistory.mockResolvedValue([])
      // eslint-disable-next-line require-yield
      mockedSendTestMessage.mockImplementation(async function* () {
        throw new Error('Stream failed')
      })

      const store = useTestSandboxStore()
      await store.selectKb('kb-1')
      await store.sendMessage('Test')

      expect(store.error).toBe('Stream failed')
      // User message remains, empty assistant message is removed
      expect(store.messages).toHaveLength(1)
      expect(store.messages[0].role).toBe('user')
      expect(store.streaming).toBe(false)
    })

    it('keeps partial assistant message on mid-stream failure', async () => {
      mockedGetTestHistory.mockResolvedValue([])
      mockedSendTestMessage.mockImplementation(async function* () {
        yield 'Partial '
        yield 'response '
        throw new Error('Stream interrupted')
      })

      const store = useTestSandboxStore()
      await store.selectKb('kb-1')
      await store.sendMessage('Test')

      expect(store.error).toBe('Stream interrupted')
      // Both messages remain since assistant has content
      expect(store.messages).toHaveLength(2)
      expect(store.messages[1].role).toBe('assistant')
      expect(store.messages[1].content).toBe('Partial response ')
    })

    it('assigns timestamps to new messages', async () => {
      mockedGetTestHistory.mockResolvedValue([])
      mockedSendTestMessage.mockReturnValue(fakeStream(['Reply']))

      const store = useTestSandboxStore()
      await store.selectKb('kb-1')
      await store.sendMessage('Test')

      expect(store.messages[0].timestamp).toBeTruthy()
      expect(store.messages[1].timestamp).toBeTruthy()
      // Timestamps should be valid ISO strings
      expect(() => new Date(store.messages[0].timestamp)).not.toThrow()
      expect(() => new Date(store.messages[1].timestamp)).not.toThrow()
    })
  })

  describe('clearConversation', () => {
    it('clears messages and error', async () => {
      mockedGetTestHistory.mockResolvedValue([])

      const store = useTestSandboxStore()
      await store.selectKb('kb-1')
      store.messages = [fakeMessage(), fakeMessage({ role: 'assistant', content: 'Response' })]
      store.error = 'some error'

      store.clearConversation()

      expect(store.messages).toEqual([])
      expect(store.error).toBeNull()
      // KB selection should remain
      expect(store.selectedKbId).toBe('kb-1')
    })
  })

  describe('$reset', () => {
    it('resets all state', async () => {
      mockedGetTestHistory.mockResolvedValue([])

      const store = useTestSandboxStore()
      await store.selectKb('kb-1')
      store.messages = [fakeMessage()]
      store.error = 'some error'

      store.$reset()

      expect(store.messages).toEqual([])
      expect(store.selectedKbId).toBeNull()
      expect(store.loading).toBe(false)
      expect(store.streaming).toBe(false)
      expect(store.error).toBeNull()
    })
  })
})
