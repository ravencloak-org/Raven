import { defineStore } from 'pinia'
import { ref } from 'vue'
import { filter } from 'remeda'
import {
  listVoiceSessions,
  getVoiceSession,
  createVoiceSession,
  updateVoiceSessionState,
  generateVoiceToken,
  listVoiceTurns,
  appendVoiceTurn,
  type VoiceSession,
  type VoiceSessionState,
  type VoiceTurn,
  type VoiceTokenResponse,
  type CreateVoiceSessionRequest,
  type AppendVoiceTurnRequest,
} from '../api/voice-sessions'

export const useVoiceSessionsStore = defineStore('voice-sessions', () => {
  const sessions = ref<VoiceSession[]>([])
  const currentSession = ref<VoiceSession | null>(null)
  const turns = ref<VoiceTurn[]>([])
  const total = ref(0)
  const loading = ref(false)
  const error = ref<string | null>(null)
  const pollingHandle = ref<ReturnType<typeof setInterval> | null>(null)

  async function fetchSessions(orgId: string, offset = 0, limit = 20) {
    loading.value = true
    error.value = null
    try {
      const res = await listVoiceSessions(orgId, offset, limit)
      sessions.value = res.sessions
      total.value = res.total
    } catch (e) {
      error.value = (e as Error).message
    } finally {
      loading.value = false
    }
  }

  async function fetchSession(orgId: string, sessionId: string) {
    loading.value = true
    error.value = null
    try {
      currentSession.value = await getVoiceSession(orgId, sessionId)
    } catch (e) {
      error.value = (e as Error).message
    } finally {
      loading.value = false
    }
  }

  async function create(
    orgId: string,
    req: CreateVoiceSessionRequest,
  ): Promise<VoiceSession> {
    const session = await createVoiceSession(orgId, req)
    sessions.value.push(session)
    total.value += 1
    return session
  }

  async function updateState(
    orgId: string,
    sessionId: string,
    state: VoiceSessionState,
  ): Promise<VoiceSession> {
    const session = await updateVoiceSessionState(orgId, sessionId, state)
    currentSession.value = session
    sessions.value = sessions.value.map((s) =>
      s.id === sessionId ? session : s,
    )
    return session
  }

  async function getToken(
    orgId: string,
    sessionId: string,
  ): Promise<VoiceTokenResponse> {
    return generateVoiceToken(orgId, sessionId)
  }

  async function fetchTurns(orgId: string, sessionId: string) {
    error.value = null
    try {
      const res = await listVoiceTurns(orgId, sessionId)
      turns.value = res.turns
    } catch (e) {
      error.value = (e as Error).message
    }
  }

  async function addTurn(
    orgId: string,
    sessionId: string,
    req: AppendVoiceTurnRequest,
  ): Promise<VoiceTurn> {
    const turn = await appendVoiceTurn(orgId, sessionId, req)
    turns.value.push(turn)
    return turn
  }

  function startPolling(orgId: string, sessionId: string) {
    stopPolling()
    pollingHandle.value = setInterval(async () => {
      try {
        const [session, turnRes] = await Promise.all([
          getVoiceSession(orgId, sessionId),
          listVoiceTurns(orgId, sessionId),
        ])
        currentSession.value = session
        turns.value = turnRes.turns
        sessions.value = sessions.value.map((s) =>
          s.id === sessionId ? session : s,
        )
        if (session.state === 'ended') {
          stopPolling()
        }
      } catch {
        /* polling errors are non-fatal */
      }
    }, 5000)
  }

  function stopPolling() {
    if (pollingHandle.value) {
      clearInterval(pollingHandle.value)
      pollingHandle.value = null
    }
  }

  function clearSession() {
    currentSession.value = null
    turns.value = []
    stopPolling()
  }

  function removeSession(sessionId: string) {
    sessions.value = filter(sessions.value, (s) => s.id !== sessionId)
    total.value = Math.max(0, total.value - 1)
    if (currentSession.value?.id === sessionId) {
      clearSession()
    }
  }

  return {
    sessions,
    currentSession,
    turns,
    total,
    loading,
    error,
    pollingHandle,
    fetchSessions,
    fetchSession,
    create,
    updateState,
    getToken,
    fetchTurns,
    addTurn,
    startPolling,
    stopPolling,
    clearSession,
    removeSession,
  }
})
