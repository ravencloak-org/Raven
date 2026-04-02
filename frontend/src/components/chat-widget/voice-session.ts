/**
 * Voice session manager for <raven-chat>.
 *
 * Wraps LiveKit client SDK for browser-based voice calls.
 * Falls back to mock mode when no LiveKit URL is configured,
 * allowing UI development without a running LiveKit server.
 */

export type VoiceState = 'idle' | 'connecting' | 'connected' | 'disconnected' | 'error'

export interface VoiceSessionCallbacks {
  onStateChange: (state: VoiceState) => void
  onError: (error: Error) => void
}

export class VoiceSession {
  private _state: VoiceState = 'idle'
  private _isMuted = false
  private _startTime: number | null = null
  private _durationTimer: ReturnType<typeof setInterval> | null = null
  private _mockMode: boolean
  private _callbacks: VoiceSessionCallbacks

  constructor(
    private _livekitUrl: string,
    callbacks: VoiceSessionCallbacks,
  ) {
    this._mockMode = !_livekitUrl
    this._callbacks = callbacks
  }

  get state(): VoiceState {
    return this._state
  }

  get isMuted(): boolean {
    return this._isMuted
  }

  get duration(): number {
    if (!this._startTime) return 0
    return Math.floor((Date.now() - this._startTime) / 1000)
  }

  async connect(): Promise<void> {
    if (this._state === 'connecting' || this._state === 'connected') return

    this._setState('connecting')

    if (this._mockMode) {
      // Simulate connection delay
      await this._delay(1000)
      this._setState('connected')
      this._startTimer()
      return
    }

    // Real LiveKit connection — dynamic import to avoid bundling
    // livekit-client when voice is not used
    try {
      const { Room, RoomEvent } = await import('livekit-client')
      const room = new Room()

      room.on(RoomEvent.Disconnected, () => {
        this._setState('disconnected')
        this._stopTimer()
      })

      // TODO: Fetch room token from backend endpoint
      // const tokenResponse = await fetch(`${apiUrl}/v1/voice/token`, { ... })
      // const { token } = await tokenResponse.json()
      // await room.connect(this._livekitUrl, token)

      // For now, fall back to mock since token endpoint doesn't exist yet
      console.warn('[raven-chat] LiveKit URL set but no token endpoint available — using mock mode')
      await this._delay(1000)
      this._setState('connected')
      this._startTimer()
    } catch (err) {
      this._setState('error')
      this._callbacks.onError(
        err instanceof Error ? err : new Error(String(err)),
      )
    }
  }

  disconnect(): void {
    this._stopTimer()
    this._setState('disconnected')
    this._isMuted = false

    // Brief delay then reset to idle
    setTimeout(() => {
      this._setState('idle')
    }, 500)
  }

  toggleMute(): boolean {
    this._isMuted = !this._isMuted
    return this._isMuted
  }

  private _setState(state: VoiceState): void {
    this._state = state
    this._callbacks.onStateChange(state)
  }

  private _startTimer(): void {
    this._startTime = Date.now()
    this._durationTimer = setInterval(() => {
      // Trigger re-render via state change callback
      this._callbacks.onStateChange(this._state)
    }, 1000)
  }

  private _stopTimer(): void {
    if (this._durationTimer) {
      clearInterval(this._durationTimer)
      this._durationTimer = null
    }
    this._startTime = null
  }

  private _delay(ms: number): Promise<void> {
    return new Promise((resolve) => setTimeout(resolve, ms))
  }

  destroy(): void {
    this._stopTimer()
    this._state = 'idle'
    this._isMuted = false
  }
}

/**
 * Format seconds into MM:SS display string.
 */
export function formatDuration(seconds: number): string {
  const m = Math.floor(seconds / 60)
  const s = seconds % 60
  return `${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`
}
