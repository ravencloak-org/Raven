export interface TestMessage {
  id: string
  role: 'user' | 'assistant'
  content: string
  timestamp: string
}

export interface TestConversation {
  kbId: string
  messages: TestMessage[]
}

async function authFetch(path: string, init?: RequestInit): Promise<Response> {
  const base = import.meta.env.VITE_API_BASE_URL ?? '/api/v1'
  return fetch(base + path, {
    ...init,
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...init?.headers,
    },
  })
}

// --- Mock data ---

const MOCK_RESPONSES: Record<string, string[]> = {
  default: [
    'Based on the knowledge base documents, I can help you with that. The information suggests that the system supports multiple file formats including PDF, Markdown, and plain text. Each document is processed through our ingestion pipeline which extracts text, generates embeddings, and indexes the content for semantic search.',
    'That is a great question! According to the documentation in this knowledge base, you can configure the chatbot behavior through the settings panel. Key options include response length, creativity level, and source citation preferences.',
    'Let me look through the available sources. The knowledge base contains several relevant documents that address your query. The recommended approach is to start with the getting-started guide and then explore the API reference for more advanced usage.',
    'I found some relevant information in the indexed documents. The platform supports real-time streaming responses, webhook integrations, and customizable prompts. You can also set up fallback responses for topics not covered in the knowledge base.',
    'According to the sources available, the best practice is to organize your documents into logical categories, use descriptive file names, and keep content up to date. The system automatically re-indexes when documents are updated.',
  ],
}

let _nextMsgId = 1

function generateMockId(): string {
  return `msg-${_nextMsgId++}`
}

function pickMockResponse(message: string): string {
  const responses = MOCK_RESPONSES['default']
  const idx = Math.abs(hashCode(message)) % responses.length
  return responses[idx]
}

function hashCode(str: string): number {
  let hash = 0
  for (let i = 0; i < str.length; i++) {
    const char = str.charCodeAt(i)
    hash = (hash << 5) - hash + char
    hash |= 0
  }
  return hash
}

const mockDelay = (ms = 200) => new Promise((r) => setTimeout(r, ms))

/**
 * Sends a test message to a knowledge base and returns a streamed response.
 * The response is delivered word-by-word via an async generator to simulate streaming.
 *
 * TODO: Replace with real API call:
 *   POST /orgs/{orgId}/workspaces/{wsId}/knowledge-bases/{kbId}/test
 *   Body: { message, conversation_id? }
 *   Response: Server-Sent Events stream
 */
export async function* sendTestMessage(
  _kbId: string,
  message: string,
): AsyncGenerator<string, void, unknown> {
  // TODO: const res = await authFetch(`/knowledge-bases/${kbId}/test`, {
  //   method: 'POST',
  //   body: JSON.stringify({ message }),
  // })
  // Then read SSE stream from response.body
  void authFetch

  await mockDelay(300)

  const fullResponse = pickMockResponse(message)
  const words = fullResponse.split(' ')

  for (const word of words) {
    await mockDelay(50)
    yield word + ' '
  }
}

/**
 * Loads previous test conversation history for a knowledge base.
 *
 * TODO: Replace with real API call:
 *   GET /orgs/{orgId}/workspaces/{wsId}/knowledge-bases/{kbId}/test/history
 */
// eslint-disable-next-line @typescript-eslint/no-unused-vars
export async function getTestHistory(_kbId: string): Promise<TestMessage[]> {
  // TODO: const res = await authFetch(`/knowledge-bases/${kbId}/test/history`)
  // return res.json()
  await mockDelay(150)

  // Return empty history by default (fresh sandbox session)
  return []
}

export { generateMockId }
