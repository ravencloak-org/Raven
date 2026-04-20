// Cross-channel conversation memory API — issue #258.
// Depends on the conversation_sessions table shared with #257.

export type ConversationChannel = 'chat' | 'voice' | 'webrtc'

export interface ConversationTurn {
  role: string
  content: string
  ts: string
}

export interface ConversationSession {
  id: string
  org_id: string
  kb_id: string
  user_id: string
  channel: ConversationChannel
  messages: ConversationTurn[]
  started_at: string
  ended_at?: string
  summary?: string
}

export interface ConversationSessionSummary {
  id: string
  org_id: string
  kb_id: string
  user_id: string
  channel: ConversationChannel
  started_at: string
  ended_at?: string
  message_count: number
  summary?: string
}

export interface ConversationListResponse {
  sessions: ConversationSessionSummary[]
  total: number
  limit: number
  offset: number
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

export async function listConversations(
  orgId: string,
  kbId: string,
  limit = 5,
  offset = 0,
): Promise<ConversationListResponse> {
  const res = await authFetch(
    `/orgs/${encodeURIComponent(orgId)}/kbs/${encodeURIComponent(kbId)}/conversations?limit=${limit}&offset=${offset}`,
  )
  if (!res.ok) throw new Error(`listConversations failed: ${res.status}`)
  return res.json()
}

export async function getConversation(
  orgId: string,
  kbId: string,
  sessionId: string,
): Promise<ConversationSession> {
  const res = await authFetch(
    `/orgs/${encodeURIComponent(orgId)}/kbs/${encodeURIComponent(kbId)}/conversations/${encodeURIComponent(sessionId)}`,
  )
  if (!res.ok) throw new Error(`getConversation failed: ${res.status}`)
  return res.json()
}
