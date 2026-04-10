import { defineStore } from 'pinia'
import { ref } from 'vue'
import { findIndex } from 'remeda'
import {
  listVoiceSessions,
  getVoiceSession,
  createVoiceSession,
  updateVoiceSessionState,
  generateVoiceToken,
  listVoiceTurns,
  appendVoiceTurn,
  type VoiceSession,
  type VoiceTurn,
  type CreateVoiceSessionRequest,
  type AppendVoiceTurnRequest,
  type VoiceSessionState,
  type VoiceTokenResponse,
} from '../api/voice'

export const useVoiceStore = defineStore('voice', () => {
  // ---- List state ----
  const sessions = ref<VoiceSession[]>([])
  const total = ref(0)
  const limit = ref(20)
  const offset = ref(0)
  const loading = ref(false)
  const error = ref<string | null>(null)

  // ---- Current session detail ----
  const currentSession = ref<VoiceSession | null>(null)
  const turns = ref<VoiceTurn[]>([])
  const sessionLoading = ref(false)
  const turnsLoading = ref(false)

  // ---- Token ----
  const lastToken = ref<VoiceTokenResponse | null>(null)

  async function fetchSessions(orgId: string, pg_limit = 20, pg_offset = 0): Promise<void> {
    loading.value = true
    error.value = null
    try {
      const resp = await listVoiceSessions(orgId, pg_limit, pg_offset)
      sessions.value = resp.sessions
      total.value = resp.total
      limit.value = resp.limit
      offset.value = resp.offset
    } catch (e) {
      error.value = (e as Error).message
    } finally {
      loading.value = false
    }
  }

  async function fetchSession(orgId: string, sessionId: string): Promise<void> {
    sessionLoading.value = true
    error.value = null
    try {
      currentSession.value = await getVoiceSession(orgId, sessionId)
    } catch (e) {
      error.value = (e as Error).message
    } finally {
      sessionLoading.value = false
    }
  }

  async function refreshSession(orgId: string, sessionId: string): Promise<void> {
    try {
      currentSession.value = await getVoiceSession(orgId, sessionId)
    } catch {
      // Silently fail during polling
    }
  }

  async function createSession(orgId: string, req: CreateVoiceSessionRequest): Promise<VoiceSession> {
    error.value = null
    try {
      const session = await createVoiceSession(orgId, req)
      sessions.value.unshift(session)
      total.value += 1
      return session
    } catch (e) {
      error.value = (e as Error).message
      throw e
    }
  }

  async function activateSession(orgId: string, sessionId: string): Promise<void> {
    error.value = null
    try {
      const updated = await updateVoiceSessionState(orgId, sessionId, 'active')
      _patchSession(updated)
    } catch (e) {
      error.value = (e as Error).message
      throw e
    }
  }

  async function endSession(orgId: string, sessionId: string): Promise<void> {
    error.value = null
    try {
      const updated = await updateVoiceSessionState(orgId, sessionId, 'ended')
      _patchSession(updated)
    } catch (e) {
      error.value = (e as Error).message
      throw e
    }
  }

  async function fetchToken(orgId: string, sessionId: string): Promise<VoiceTokenResponse> {
    error.value = null
    try {
      const tokenResp = await generateVoiceToken(orgId, sessionId)
      lastToken.value = tokenResp
      return tokenResp
    } catch (e) {
      error.value = (e as Error).message
      throw e
    }
  }

  async function fetchTurns(orgId: string, sessionId: string): Promise<void> {
    turnsLoading.value = true
    error.value = null
    try {
      const resp = await listVoiceTurns(orgId, sessionId)
      turns.value = resp.turns
    } catch (e) {
      error.value = (e as Error).message
    } finally {
      turnsLoading.value = false
    }
  }

  async function addTurn(orgId: string, sessionId: string, req: AppendVoiceTurnRequest): Promise<VoiceTurn> {
    error.value = null
    try {
      const turn = await appendVoiceTurn(orgId, sessionId, req)
      turns.value.push(turn)
      return turn
    } catch (e) {
      error.value = (e as Error).message
      throw e
    }
  }

  function _patchSession(updated: VoiceSession): void {
    if (currentSession.value?.id === updated.id) {
      currentSession.value = updated
    }
    const idx = findIndex(sessions.value, (s) => s.id === updated.id)
    if (idx !== -1) sessions.value[idx] = updated
  }

  function $reset(): void {
    sessions.value = []
    total.value = 0
    limit.value = 20
    offset.value = 0
    loading.value = false
    error.value = null
    currentSession.value = null
    turns.value = []
    sessionLoading.value = false
    turnsLoading.value = false
    lastToken.value = null
  }

  return {
    sessions,
    total,
    limit,
    offset,
    loading,
    error,
    currentSession,
    turns,
    sessionLoading,
    turnsLoading,
    lastToken,
    fetchSessions,
    fetchSession,
    refreshSession,
    createSession,
    activateSession,
    endSession,
    fetchToken,
    fetchTurns,
    addTurn,
    $reset,
  }
})
