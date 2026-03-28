/**
 * Chat API client for <raven-chat>.
 *
 * Handles SSE streaming communication with the Raven backend.
 * Currently ships with a mock implementation that simulates
 * word-by-word streaming via setTimeout.
 */

export interface ChatMessage {
  role: 'user' | 'assistant'
  content: string
}

export interface SendMessageOptions {
  apiUrl: string
  apiKey: string
  message: string
  history?: ChatMessage[]
  onChunk: (chunk: string) => void
  onDone: () => void
  onError: (error: Error) => void
  signal?: AbortSignal
}

/**
 * Sends a chat message and streams the response back chunk-by-chunk.
 *
 * TODO: Replace mock implementation with real SSE fetch once the
 *       backend streaming endpoint is available.
 *
 * Real implementation will:
 *   1. POST to `${apiUrl}/v1/chat/stream`
 *   2. Read the response body as a ReadableStream
 *   3. Parse SSE `data:` frames and call `onChunk` for each token
 *   4. Call `onDone` when the stream ends (receives `[DONE]`)
 */
export function sendMessage(options: SendMessageOptions): void {
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  const { message, onChunk, onDone, onError: _onError, signal } = options

  // --- Mock streaming implementation ---
  const mockResponses = [
    `That's a great question! Let me help you with that. Based on what you've asked about "${truncate(message, 40)}", here's what I can tell you.`,
    `I'd be happy to assist! Regarding "${truncate(message, 40)}", there are a few important things to consider. Let me walk you through the key points.`,
    `Thanks for reaching out! Here's some helpful information about "${truncate(message, 40)}". I'll break it down step by step for you.`,
  ]

  const responseText =
    mockResponses[Math.floor(Math.random() * mockResponses.length)]
  const words = responseText.split(' ')
  let index = 0
  let cancelled = false

  const onAbort = () => {
    cancelled = true
  }
  signal?.addEventListener('abort', onAbort, { once: true })

  function emitNextWord() {
    if (cancelled) {
      signal?.removeEventListener('abort', onAbort)
      return
    }

    if (index < words.length) {
      const word = (index === 0 ? '' : ' ') + words[index]
      onChunk(word)
      index++
      // Simulate variable network latency (30-80ms per word)
      const delay = 30 + Math.random() * 50
      setTimeout(emitNextWord, delay)
    } else {
      signal?.removeEventListener('abort', onAbort)
      onDone()
    }
  }

  // Small initial delay to simulate network round-trip
  setTimeout(emitNextWord, 300)

  // TODO: Real SSE implementation outline:
  //
  // try {
  //   const response = await fetch(`${apiUrl}/v1/chat/stream`, {
  //     method: 'POST',
  //     headers: {
  //       'Content-Type': 'application/json',
  //       'Authorization': `Bearer ${apiKey}`,
  //     },
  //     body: JSON.stringify({ message, history }),
  //     signal,
  //   })
  //
  //   if (!response.ok) {
  //     throw new Error(`Chat API error: ${response.status}`)
  //   }
  //
  //   const reader = response.body!.getReader()
  //   const decoder = new TextDecoder()
  //   let buffer = ''
  //
  //   while (true) {
  //     const { done, value } = await reader.read()
  //     if (done) break
  //
  //     buffer += decoder.decode(value, { stream: true })
  //     const lines = buffer.split('\n')
  //     buffer = lines.pop() ?? ''
  //
  //     for (const line of lines) {
  //       if (line.startsWith('data: ')) {
  //         const data = line.slice(6).trim()
  //         if (data === '[DONE]') {
  //           onDone()
  //           return
  //         }
  //         const parsed = JSON.parse(data)
  //         if (parsed.content) {
  //           onChunk(parsed.content)
  //         }
  //       }
  //     }
  //   }
  //
  //   onDone()
  // } catch (err) {
  //   if ((err as DOMException).name !== 'AbortError') {
  //     onError(err instanceof Error ? err : new Error(String(err)))
  //   }
  // }

  // For the mock, simulate potential errors on bad config
  if (!options.apiUrl || !options.apiKey) {
    // In mock mode we still stream, but log a warning
    // In production this would call onError immediately
    console.warn(
      '[raven-chat] Missing apiUrl or apiKey — using mock responses.',
    )
  }
}

/**
 * Parse an SSE line buffer into individual data payloads.
 * Exported for unit-testing.
 */
export function parseSSELines(raw: string): string[] {
  const results: string[] = []
  const lines = raw.split('\n')

  for (const line of lines) {
    const trimmed = line.trim()
    if (trimmed.startsWith('data: ')) {
      const payload = trimmed.slice(6)
      if (payload !== '[DONE]') {
        results.push(payload)
      }
    }
    // Ignore comment lines (`:`) and event/id/retry fields
  }

  return results
}

function truncate(str: string, maxLen: number): string {
  if (str.length <= maxLen) return str
  return str.slice(0, maxLen) + '...'
}
