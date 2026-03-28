import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { sendMessage, parseSSELines, type SendMessageOptions } from '../chat-api'

describe('chat-api', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  describe('sendMessage', () => {
    function createOptions(
      overrides: Partial<SendMessageOptions> = {},
    ): SendMessageOptions {
      return {
        apiUrl: 'https://api.test.com',
        apiKey: 'test-key-123',
        message: 'Hello',
        onChunk: vi.fn(),
        onDone: vi.fn(),
        onError: vi.fn(),
        ...overrides,
      }
    }

    it('calls onChunk with streamed word chunks', async () => {
      const options = createOptions()
      sendMessage(options)

      // Advance past the initial 300ms delay
      await vi.advanceTimersByTimeAsync(300)

      // First chunk should have been emitted (no leading space for first word)
      expect(options.onChunk).toHaveBeenCalled()
      const firstCall = (options.onChunk as ReturnType<typeof vi.fn>).mock
        .calls[0][0] as string
      // First word should not have a leading space
      expect(firstCall).not.toMatch(/^ /)
    })

    it('calls onDone after all chunks are emitted', async () => {
      const options = createOptions()
      sendMessage(options)

      // Advance enough time for all words to stream
      // Mock responses are ~20-30 words, each taking up to 80ms + 300ms initial
      await vi.advanceTimersByTimeAsync(10_000)

      expect(options.onDone).toHaveBeenCalledTimes(1)
    })

    it('emits multiple chunks that reconstruct the full message', async () => {
      const options = createOptions()
      sendMessage(options)

      await vi.advanceTimersByTimeAsync(10_000)

      const chunks = (options.onChunk as ReturnType<typeof vi.fn>).mock.calls.map(
        (call) => call[0] as string,
      )
      expect(chunks.length).toBeGreaterThan(1)

      // Reconstruct and verify it forms a coherent sentence
      const full = chunks.join('')
      expect(full.length).toBeGreaterThan(10)
      // The mock response includes the user's message
      expect(full).toContain('Hello')
    })

    it('stops streaming when abort signal is triggered', async () => {
      const controller = new AbortController()
      const options = createOptions({ signal: controller.signal })
      sendMessage(options)

      // Let initial delay + a few words through
      await vi.advanceTimersByTimeAsync(500)
      const chunksBeforeAbort = (options.onChunk as ReturnType<typeof vi.fn>)
        .mock.calls.length

      // Abort
      controller.abort()

      // Try to advance more -- no new chunks should arrive
      await vi.advanceTimersByTimeAsync(10_000)
      const chunksAfterAbort = (options.onChunk as ReturnType<typeof vi.fn>)
        .mock.calls.length

      expect(chunksAfterAbort).toBe(chunksBeforeAbort)
      // onDone should NOT have been called
      expect(options.onDone).not.toHaveBeenCalled()
    })

    it('logs a warning when apiUrl or apiKey is missing', async () => {
      const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
      const options = createOptions({ apiUrl: '', apiKey: '' })
      sendMessage(options)

      await vi.advanceTimersByTimeAsync(10_000)

      expect(warnSpy).toHaveBeenCalledWith(
        expect.stringContaining('Missing apiUrl or apiKey'),
      )
      warnSpy.mockRestore()
    })

    it('includes user message in the mock response', async () => {
      const options = createOptions({ message: 'Tell me about TypeScript' })
      sendMessage(options)

      await vi.advanceTimersByTimeAsync(10_000)

      const chunks = (options.onChunk as ReturnType<typeof vi.fn>).mock.calls.map(
        (call) => call[0] as string,
      )
      const full = chunks.join('')
      expect(full).toContain('Tell me about TypeScript')
    })

    it('subsequent words have leading spaces', async () => {
      const options = createOptions()
      sendMessage(options)

      await vi.advanceTimersByTimeAsync(10_000)

      const chunks = (options.onChunk as ReturnType<typeof vi.fn>).mock.calls.map(
        (call) => call[0] as string,
      )

      // Every chunk after the first should start with a space
      for (let i = 1; i < chunks.length; i++) {
        expect(chunks[i]).toMatch(/^ /)
      }
    })
  })

  describe('parseSSELines', () => {
    it('extracts data payloads from SSE lines', () => {
      const raw = [
        'data: {"content":"Hello"}',
        'data: {"content":" world"}',
        '',
        'data: [DONE]',
      ].join('\n')

      const result = parseSSELines(raw)
      expect(result).toEqual(['{"content":"Hello"}', '{"content":" world"}'])
    })

    it('returns empty array for empty input', () => {
      expect(parseSSELines('')).toEqual([])
    })

    it('ignores comment lines', () => {
      const raw = [
        ': this is a comment',
        'data: {"content":"real"}',
        ': another comment',
      ].join('\n')

      const result = parseSSELines(raw)
      expect(result).toEqual(['{"content":"real"}'])
    })

    it('ignores non-data SSE fields', () => {
      const raw = [
        'event: message',
        'id: 42',
        'retry: 3000',
        'data: {"content":"token"}',
      ].join('\n')

      const result = parseSSELines(raw)
      expect(result).toEqual(['{"content":"token"}'])
    })

    it('handles data lines with extra whitespace', () => {
      const raw = '  data: {"content":"padded"}  '
      const result = parseSSELines(raw)
      expect(result).toEqual(['{"content":"padded"}'])
    })

    it('filters out [DONE] sentinel', () => {
      const raw = 'data: [DONE]'
      const result = parseSSELines(raw)
      expect(result).toEqual([])
    })

    it('handles multiple consecutive empty lines', () => {
      const raw = [
        'data: {"content":"a"}',
        '',
        '',
        '',
        'data: {"content":"b"}',
      ].join('\n')

      const result = parseSSELines(raw)
      expect(result).toEqual(['{"content":"a"}', '{"content":"b"}'])
    })
  })
})
