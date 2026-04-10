import { useAuthStore } from '../stores/auth'

export type VoiceSessionState = 'created' | 'active' | 'ended'
export type VoiceSpeaker = 'agent' | 'user'

export interface VoiceSession {
  id: string
  org_id: string
  user_id?: string
  stranger_id?: string
  livekit_room: string
  state: VoiceSessionState
  started_at?: string
  ended_at?: string
  call_duration_seconds?: number
  created_at: string
  updated_at: string
}

export interface VoiceTurn {
  id: string
  session_id: string
  org_id: string
  speaker: VoiceSpeaker
  transcript: string
  started_at: string
  ended_at?: string
  created_at: string
}

export interface VoiceSessionListResponse {
  sessions: VoiceSession[]
  total: number
  limit: number
  offset: number
}

export interface VoiceTurnListResponse {
  session_id: string
  turns: VoiceTurn[]
}

export interface VoiceTokenResponse {
  token: string
  url: string
}

export interface CreateVoiceSessionRequest {
  livekit_room?: string
  user_id?: string
  stranger_id?: string
}

export interface AppendVoiceTurnRequest {
  speaker: VoiceSpeaker
  transcript: string
  started_at: string
  ended_at?: string
}

async function authFetch(path: string, init?: RequestInit): Promise<Response> {
  const auth = useAuthStore()
  if (!auth.accessToken) {
    throw new Error('Not authenticated')
  }
  const base = import.meta.env.VITE_API_BASE_URL ?? '/api/v1'
  return fetch(base + path, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...init?.headers,
      Authorization: `Bearer ${auth.accessToken}`,
    },
  })
}

export async function listVoiceSessions(
  orgId: string,
  offset = 0,
  limit = 20,
): Promise<VoiceSessionListResponse> {
  const res = await authFetch(
    `/orgs/${encodeURIComponent(orgId)}/voice-sessions?offset=${offset}&limit=${limit}`,
  )
  if (!res.ok) throw new Error(`listVoiceSessions failed: ${res.status}`)
  return res.json()
}

export async function getVoiceSession(
  orgId: string,
  sessionId: string,
): Promise<VoiceSession> {
  const res = await authFetch(`/orgs/${encodeURIComponent(orgId)}/voice-sessions/${encodeURIComponent(sessionId)}`)
  if (!res.ok) throw new Error(`getVoiceSession failed: ${res.status}`)
  return res.json()
}

export async function createVoiceSession(
  orgId: string,
  req: CreateVoiceSessionRequest,
): Promise<VoiceSession> {
  const res = await authFetch(`/orgs/${encodeURIComponent(orgId)}/voice-sessions`, {
    method: 'POST',
    body: JSON.stringify(req),
  })
  if (!res.ok) throw new Error(`createVoiceSession failed: ${res.status}`)
  return res.json()
}

export async function updateVoiceSessionState(
  orgId: string,
  sessionId: string,
  state: VoiceSessionState,
): Promise<VoiceSession> {
  const res = await authFetch(`/orgs/${encodeURIComponent(orgId)}/voice-sessions/${encodeURIComponent(sessionId)}`, {
    method: 'PATCH',
    body: JSON.stringify({ state }),
  })
  if (!res.ok) throw new Error(`updateVoiceSessionState failed: ${res.status}`)
  return res.json()
}

export async function generateVoiceToken(
  orgId: string,
  sessionId: string,
): Promise<VoiceTokenResponse> {
  const res = await authFetch(
    `/orgs/${encodeURIComponent(orgId)}/voice-sessions/${encodeURIComponent(sessionId)}/token`,
    { method: 'POST' },
  )
  if (!res.ok) throw new Error(`generateVoiceToken failed: ${res.status}`)
  return res.json()
}

export async function listVoiceTurns(
  orgId: string,
  sessionId: string,
): Promise<VoiceTurnListResponse> {
  const res = await authFetch(
    `/orgs/${encodeURIComponent(orgId)}/voice-sessions/${encodeURIComponent(sessionId)}/turns`,
  )
  if (!res.ok) throw new Error(`listVoiceTurns failed: ${res.status}`)
  return res.json()
}

export async function appendVoiceTurn(
  orgId: string,
  sessionId: string,
  req: AppendVoiceTurnRequest,
): Promise<VoiceTurn> {
  const res = await authFetch(
    `/orgs/${encodeURIComponent(orgId)}/voice-sessions/${encodeURIComponent(sessionId)}/turns`,
    {
      method: 'POST',
      body: JSON.stringify(req),
    },
  )
  if (!res.ok) throw new Error(`appendVoiceTurn failed: ${res.status}`)
  return res.json()
}
