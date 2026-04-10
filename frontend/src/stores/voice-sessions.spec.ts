import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useVoiceSessionsStore } from './voice-sessions'
import type {
  VoiceSession,
  VoiceSessionListResponse,
  VoiceTurnListResponse,
  VoiceTurn,
  VoiceTokenResponse,
} from '../api/voice-sessions'

vi.mock('../api/voice-sessions', () => ({
  listVoiceSessions: vi.fn(),
  getVoiceSession: vi.fn(),
  createVoiceSession: vi.fn(),
  updateVoiceSessionState: vi.fn(),
  generateVoiceToken: vi.fn(),
  listVoiceTurns: vi.fn(),
  appendVoiceTurn: vi.fn(),
}))

import {
  listVoiceSessions,
  getVoiceSession,
  createVoiceSession,
  updateVoiceSessionState,
  generateVoiceToken,
  listVoiceTurns,
  appendVoiceTurn,
} from '../api/voice-sessions'

const mockedListVoiceSessions = vi.mocked(listVoiceSessions)
const mockedGetVoiceSession = vi.mocked(getVoiceSession)
const mockedCreateVoiceSession = vi.mocked(createVoiceSession)
const mockedUpdateVoiceSessionState = vi.mocked(updateVoiceSessionState)
const mockedGenerateVoiceToken = vi.mocked(generateVoiceToken)
const mockedListVoiceTurns = vi.mocked(listVoiceTurns)
const mockedAppendVoiceTurn = vi.mocked(appendVoiceTurn)

const ORG_ID = 'org-1'

function fakeSession(overrides: Partial<VoiceSession> = {}): VoiceSession {
  return {
    id: 'vs-1',
    org_id: ORG_ID,
    livekit_room: 'room-abc',
    state: 'created',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

function fakeTurn(overrides: Partial<VoiceTurn> = {}): VoiceTurn {
  return {
    id: 'vt-1',
    session_id: 'vs-1',
    org_id: ORG_ID,
    speaker: 'user',
    transcript: 'Hello world',
    started_at: '2026-01-01T00:00:10Z',
    created_at: '2026-01-01T00:00:10Z',
    ...overrides,
  }
}

describe('useVoiceSessionsStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    vi.useFakeTimers()
  })

  afterEach(() => {
    const store = useVoiceSessionsStore()
    store.stopPolling()
    vi.useRealTimers()
  })

  describe('fetchSessions', () => {
    it('populates sessions and total on success', async () => {
      const session = fakeSession()
      const response: VoiceSessionListResponse = {
        sessions: [session],
        total: 1,
        limit: 20,
        offset: 0,
      }
      mockedListVoiceSessions.mockResolvedValue(response)

      const store = useVoiceSessionsStore()
      await store.fetchSessions(ORG_ID)

      expect(mockedListVoiceSessions).toHaveBeenCalledWith(ORG_ID, 0, 20)
      expect(store.sessions).toEqual([session])
      expect(store.total).toBe(1)
      expect(store.loading).toBe(false)
      expect(store.error).toBeNull()
    })

    it('sets error on failure', async () => {
      mockedListVoiceSessions.mockRejectedValue(
        new Error('listVoiceSessions failed: 500'),
      )

      const store = useVoiceSessionsStore()
      await store.fetchSessions(ORG_ID)

      expect(store.sessions).toEqual([])
      expect(store.error).toBe('listVoiceSessions failed: 500')
      expect(store.loading).toBe(false)
    })

    it('passes offset and limit through', async () => {
      mockedListVoiceSessions.mockResolvedValue({
        sessions: [],
        total: 0,
        offset: 10,
        limit: 5,
      })

      const store = useVoiceSessionsStore()
      await store.fetchSessions(ORG_ID, 10, 5)

      expect(mockedListVoiceSessions).toHaveBeenCalledWith(ORG_ID, 10, 5)
    })
  })

  describe('fetchSession', () => {
    it('sets currentSession on success', async () => {
      const session = fakeSession()
      mockedGetVoiceSession.mockResolvedValue(session)

      const store = useVoiceSessionsStore()
      await store.fetchSession(ORG_ID, 'vs-1')

      expect(mockedGetVoiceSession).toHaveBeenCalledWith(ORG_ID, 'vs-1')
      expect(store.currentSession).toEqual(session)
      expect(store.loading).toBe(false)
    })

    it('sets error on failure', async () => {
      mockedGetVoiceSession.mockRejectedValue(
        new Error('getVoiceSession failed: 404'),
      )

      const store = useVoiceSessionsStore()
      await store.fetchSession(ORG_ID, 'vs-1')

      expect(store.currentSession).toBeNull()
      expect(store.error).toBe('getVoiceSession failed: 404')
    })
  })

  describe('create', () => {
    it('appends new session and increments total', async () => {
      const session = fakeSession({ id: 'vs-new' })
      mockedCreateVoiceSession.mockResolvedValue(session)

      const store = useVoiceSessionsStore()
      store.total = 3

      const result = await store.create(ORG_ID, { livekit_room: 'room-1' })

      expect(mockedCreateVoiceSession).toHaveBeenCalledWith(ORG_ID, {
        livekit_room: 'room-1',
      })
      expect(result).toEqual(session)
      expect(store.sessions).toContainEqual(session)
      expect(store.total).toBe(4)
    })

    it('propagates API errors', async () => {
      mockedCreateVoiceSession.mockRejectedValue(
        new Error('createVoiceSession failed: 400'),
      )

      const store = useVoiceSessionsStore()

      await expect(store.create(ORG_ID, {})).rejects.toThrow(
        'createVoiceSession failed: 400',
      )
    })
  })

  describe('updateState', () => {
    it('updates currentSession and the matching session in list', async () => {
      const original = fakeSession({ state: 'created' })
      const updated = fakeSession({ state: 'active' })
      mockedUpdateVoiceSessionState.mockResolvedValue(updated)

      const store = useVoiceSessionsStore()
      store.sessions = [original]
      store.currentSession = original

      const result = await store.updateState(ORG_ID, 'vs-1', 'active')

      expect(mockedUpdateVoiceSessionState).toHaveBeenCalledWith(
        ORG_ID,
        'vs-1',
        'active',
      )
      expect(result).toEqual(updated)
      expect(store.currentSession).toEqual(updated)
      expect(store.sessions[0]).toEqual(updated)
    })

    it('propagates API errors', async () => {
      mockedUpdateVoiceSessionState.mockRejectedValue(
        new Error('updateVoiceSessionState failed: 400'),
      )

      const store = useVoiceSessionsStore()

      await expect(
        store.updateState(ORG_ID, 'vs-1', 'active'),
      ).rejects.toThrow('updateVoiceSessionState failed: 400')
    })
  })

  describe('getToken', () => {
    it('returns token response', async () => {
      const tokenResp: VoiceTokenResponse = {
        token: 'jwt-token',
        url: 'wss://livekit.example.com',
      }
      mockedGenerateVoiceToken.mockResolvedValue(tokenResp)

      const store = useVoiceSessionsStore()
      const result = await store.getToken(ORG_ID, 'vs-1')

      expect(mockedGenerateVoiceToken).toHaveBeenCalledWith(ORG_ID, 'vs-1')
      expect(result).toEqual(tokenResp)
    })
  })

  describe('fetchTurns', () => {
    it('populates turns on success', async () => {
      const turn = fakeTurn()
      const response: VoiceTurnListResponse = {
        session_id: 'vs-1',
        turns: [turn],
      }
      mockedListVoiceTurns.mockResolvedValue(response)

      const store = useVoiceSessionsStore()
      await store.fetchTurns(ORG_ID, 'vs-1')

      expect(mockedListVoiceTurns).toHaveBeenCalledWith(ORG_ID, 'vs-1')
      expect(store.turns).toEqual([turn])
    })

    it('sets error on failure', async () => {
      mockedListVoiceTurns.mockRejectedValue(
        new Error('listVoiceTurns failed: 404'),
      )

      const store = useVoiceSessionsStore()
      await store.fetchTurns(ORG_ID, 'vs-1')

      expect(store.error).toBe('listVoiceTurns failed: 404')
    })
  })

  describe('addTurn', () => {
    it('appends turn to the turns list', async () => {
      const turn = fakeTurn({ id: 'vt-new' })
      mockedAppendVoiceTurn.mockResolvedValue(turn)

      const store = useVoiceSessionsStore()
      const result = await store.addTurn(ORG_ID, 'vs-1', {
        speaker: 'user',
        transcript: 'Hello',
        started_at: '2026-01-01T00:00:10Z',
      })

      expect(mockedAppendVoiceTurn).toHaveBeenCalledWith(ORG_ID, 'vs-1', {
        speaker: 'user',
        transcript: 'Hello',
        started_at: '2026-01-01T00:00:10Z',
      })
      expect(result).toEqual(turn)
      expect(store.turns).toContainEqual(turn)
    })

    it('propagates API errors', async () => {
      mockedAppendVoiceTurn.mockRejectedValue(
        new Error('appendVoiceTurn failed: 400'),
      )

      const store = useVoiceSessionsStore()

      await expect(
        store.addTurn(ORG_ID, 'vs-1', {
          speaker: 'user',
          transcript: '',
          started_at: '2026-01-01T00:00:10Z',
        }),
      ).rejects.toThrow('appendVoiceTurn failed: 400')
    })
  })

  describe('removeSession', () => {
    it('removes session from list and decrements total', () => {
      const store = useVoiceSessionsStore()
      store.sessions = [fakeSession()]
      store.total = 1

      store.removeSession('vs-1')

      expect(store.sessions).toEqual([])
      expect(store.total).toBe(0)
    })

    it('clears currentSession if it was the removed one', () => {
      const store = useVoiceSessionsStore()
      const session = fakeSession()
      store.currentSession = session
      store.sessions = [session]
      store.total = 1

      store.removeSession('vs-1')

      expect(store.currentSession).toBeNull()
    })
  })

  describe('polling', () => {
    it('startPolling sets up an interval that fetches session and turns', async () => {
      const session = fakeSession({ state: 'active' })
      const turnResponse: VoiceTurnListResponse = {
        session_id: 'vs-1',
        turns: [fakeTurn()],
      }
      mockedGetVoiceSession.mockResolvedValue(session)
      mockedListVoiceTurns.mockResolvedValue(turnResponse)

      const store = useVoiceSessionsStore()
      store.sessions = [fakeSession()]
      store.startPolling(ORG_ID, 'vs-1')

      expect(store.pollingHandle).not.toBeNull()

      // Advance timer to trigger the interval
      await vi.advanceTimersByTimeAsync(5000)

      expect(mockedGetVoiceSession).toHaveBeenCalledWith(ORG_ID, 'vs-1')
      expect(mockedListVoiceTurns).toHaveBeenCalledWith(ORG_ID, 'vs-1')
      expect(store.currentSession).toEqual(session)
      expect(store.turns).toEqual(turnResponse.turns)

      store.stopPolling()
    })

    it('stopPolling clears the interval', () => {
      const store = useVoiceSessionsStore()
      store.startPolling(ORG_ID, 'vs-1')
      expect(store.pollingHandle).not.toBeNull()

      store.stopPolling()
      expect(store.pollingHandle).toBeNull()
    })

    it('auto-stops polling when session ends', async () => {
      const endedSession = fakeSession({ state: 'ended' })
      mockedGetVoiceSession.mockResolvedValue(endedSession)
      mockedListVoiceTurns.mockResolvedValue({
        session_id: 'vs-1',
        turns: [],
      })

      const store = useVoiceSessionsStore()
      store.sessions = [fakeSession()]
      store.startPolling(ORG_ID, 'vs-1')

      await vi.advanceTimersByTimeAsync(5000)

      expect(store.pollingHandle).toBeNull()
      expect(store.currentSession?.state).toBe('ended')
    })
  })

  describe('clearSession', () => {
    it('resets currentSession, turns, and stops polling', () => {
      const store = useVoiceSessionsStore()
      store.currentSession = fakeSession()
      store.turns = [fakeTurn()]
      store.startPolling(ORG_ID, 'vs-1')

      store.clearSession()

      expect(store.currentSession).toBeNull()
      expect(store.turns).toEqual([])
      expect(store.pollingHandle).toBeNull()
    })
  })
})
