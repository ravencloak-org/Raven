/**
 * <raven-chat> — Embeddable AI chat web component.
 *
 * Attributes:
 *   api-key       — API key for authenticating with the Raven backend
 *   api-url       — Base URL of the Raven API (e.g. https://api.example.com)
 *   theme-color   — Primary brand colour (default: #6366f1)
 *   avatar-url    — URL for the assistant avatar image in the header
 *   welcome-text  — Greeting shown before the first message
 *   position      — "bottom-right" (default) or "bottom-left"
 *   voice-enabled — Show call button in header (default: false)
 *   livekit-url   — LiveKit WebSocket URL for voice sessions
 */

import { chatStyles } from './chat-styles'
import { sendMessage, type ChatMessage } from './chat-api'
import { VoiceSession, formatDuration, type VoiceState } from './voice-session'

const DEFAULT_THEME_COLOR = '#6366f1'
const DEFAULT_WELCOME_TEXT = 'Hi there! How can I help you today?'
const DEFAULT_POSITION = 'bottom-right'

// SVG icons used in the widget
const ICON_CHAT = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M20 2H4c-1.1 0-2 .9-2 2v18l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm0 14H5.17L4 17.17V4h16v12z"/><path d="M7 9h2v2H7zm4 0h2v2h-2zm4 0h2v2h-2z"/></svg>`
const ICON_CLOSE = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"/></svg>`
const ICON_SEND = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M2.01 21L23 12 2.01 3 2 10l15 2-15 2z"/></svg>`
const ICON_PHONE = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M6.62 10.79c1.44 2.83 3.76 5.14 6.59 6.59l2.2-2.2c.27-.27.67-.36 1.02-.24 1.12.37 2.33.57 3.57.57.55 0 1 .45 1 1V20c0 .55-.45 1-1 1-9.39 0-17-7.61-17-17 0-.55.45-1 1-1h3.5c.55 0 1 .45 1 1 0 1.25.2 2.45.57 3.57.11.35.03.74-.25 1.02l-2.2 2.2z"/></svg>`
const ICON_MIC = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M12 14c1.66 0 3-1.34 3-3V5c0-1.66-1.34-3-3-3S9 3.34 9 5v6c0 1.66 1.34 3 3 3z"/><path d="M17 11c0 2.76-2.24 5-5 5s-5-2.24-5-5H5c0 3.53 2.61 6.43 6 6.92V21h2v-3.08c3.39-.49 6-3.39 6-6.92h-2z"/></svg>`
const ICON_MIC_OFF = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M19 11h-1.7c0 .74-.16 1.43-.43 2.05l1.23 1.23c.56-.98.9-2.09.9-3.28zm-4.02.17c0-.06.02-.11.02-.17V5c0-1.66-1.34-3-3-3S9 3.34 9 5v.18l5.98 5.99zM4.27 3L3 4.27l6.01 6.01V11c0 1.66 1.33 3 2.99 3 .22 0 .44-.03.65-.08l1.66 1.66c-.71.33-1.5.52-2.31.52-2.76 0-5.3-2.1-5.3-5.1H5c0 3.41 2.72 6.23 6 6.72V21h2v-3.28c.91-.13 1.77-.45 2.55-.9l4.17 4.18L21 19.73 4.27 3z"/></svg>`
const ICON_END_CALL = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M12 9c-1.6 0-3.15.25-4.6.72v3.1c0 .39-.23.74-.56.9-.98.49-1.87 1.12-2.66 1.85-.18.18-.43.28-.7.28-.28 0-.53-.11-.71-.29L.29 13.08c-.18-.17-.29-.42-.29-.7 0-.28.11-.53.29-.71C3.34 8.78 7.46 7 12 7s8.66 1.78 11.71 4.67c.18.18.29.43.29.71 0 .28-.11.53-.29.71l-2.48 2.48c-.18.18-.43.29-.71.29-.27 0-.52-.11-.7-.28-.79-.74-1.69-1.36-2.67-1.85-.33-.16-.56-.5-.56-.9v-3.1C15.15 9.25 13.6 9 12 9z"/></svg>`

export class RavenChat extends HTMLElement {
  static get observedAttributes(): string[] {
    return [
      'api-key',
      'api-url',
      'theme-color',
      'avatar-url',
      'welcome-text',
      'position',
      'voice-enabled',
      'livekit-url',
    ]
  }

  // ---- Internal state ----
  private _isOpen = false
  private _messages: ChatMessage[] = []
  private _isStreaming = false
  private _abortController: AbortController | null = null
  private _voiceSession: VoiceSession | null = null
  private _voiceState: VoiceState = 'idle'

  // ---- Shadow DOM element refs ----
  private _panel!: HTMLElement
  private _bubble!: HTMLButtonElement
  private _messagesContainer!: HTMLElement
  private _typingIndicator!: HTMLElement
  private _input!: HTMLTextAreaElement
  private _sendBtn!: HTMLButtonElement
  private _welcomeEl!: HTMLElement
  private _voiceView!: HTMLElement
  private _voiceStatusEl!: HTMLElement
  private _voicePulse!: HTMLElement
  private _voiceTimerEl!: HTMLElement

  constructor() {
    super()
    this.attachShadow({ mode: 'open' })
  }

  // ---- Lifecycle ----

  connectedCallback(): void {
    this._render()
    this._bindEvents()
    // Ensure position attribute has a default
    if (!this.getAttribute('position')) {
      this.setAttribute('position', DEFAULT_POSITION)
    }
  }

  disconnectedCallback(): void {
    this._abortController?.abort()
    this._voiceSession?.destroy()
  }

  attributeChangedCallback(
    name: string,
    oldVal: string | null,
    newVal: string | null,
  ): void {
    if (oldVal === newVal) return

    // Re-render only when theme changes (requires full CSS rebuild)
    if (name === 'theme-color') {
      this._render()
      this._bindEvents()
      return
    }

    // Update specific parts without full re-render
    if (name === 'welcome-text' && this._welcomeEl) {
      this._welcomeEl.textContent = newVal ?? DEFAULT_WELCOME_TEXT
    }

    if (name === 'avatar-url') {
      const avatar = this.shadowRoot?.querySelector(
        '.rc-header-avatar',
      ) as HTMLImageElement | null
      if (avatar && newVal) {
        avatar.src = newVal
      }
    }
  }

  // ---- Getters for attributes ----

  private get _apiKey(): string {
    return this.getAttribute('api-key') ?? ''
  }

  private get _apiUrl(): string {
    return this.getAttribute('api-url') ?? ''
  }

  private get _themeColor(): string {
    return this.getAttribute('theme-color') ?? DEFAULT_THEME_COLOR
  }

  private get _avatarUrl(): string {
    return this.getAttribute('avatar-url') ?? ''
  }

  private get _welcomeText(): string {
    return this.getAttribute('welcome-text') ?? DEFAULT_WELCOME_TEXT
  }

  private get _voiceEnabled(): boolean {
    return this.getAttribute('voice-enabled') === 'true'
  }

  private get _livekitUrl(): string {
    return this.getAttribute('livekit-url') ?? ''
  }

  // ---- Rendering ----

  private _render(): void {
    const shadow = this.shadowRoot!

    const avatarHtml = this._avatarUrl
      ? `<img class="rc-header-avatar" src="${this._escapeHtml(this._avatarUrl)}" alt="Assistant avatar" />`
      : `<div class="rc-header-avatar"></div>`

    shadow.innerHTML = `
      <style>${chatStyles(this._themeColor)}</style>

      <div class="rc-panel" role="dialog" aria-label="Chat with assistant">
        <div class="rc-header">
          ${avatarHtml}
          <div class="rc-header-info">
            <div class="rc-header-title">Raven Assistant</div>
            <div class="rc-header-subtitle">Typically replies instantly</div>
          </div>
          ${this._voiceEnabled ? `<button class="rc-call-btn${this._voiceState !== 'idle' ? ' active' : ''}" aria-label="Start voice call" type="button">${ICON_PHONE}</button>` : ''}
          <button class="rc-close-btn" aria-label="Close chat" type="button">
            ${ICON_CLOSE}
          </button>
        </div>

        <div class="rc-messages" aria-live="polite" aria-relevant="additions">
          <div class="rc-welcome">${this._escapeHtml(this._welcomeText)}</div>
          <div class="rc-typing" aria-label="Assistant is typing">
            <span class="rc-typing-dot"></span>
            <span class="rc-typing-dot"></span>
            <span class="rc-typing-dot"></span>
          </div>
        </div>

        <div class="rc-voice" style="display: none">
          <div class="rc-voice-status">Ready</div>
          <div class="rc-voice-pulse">
            ${ICON_MIC}
          </div>
          <div class="rc-voice-timer">00:00</div>
          <div class="rc-voice-controls">
            <button class="rc-voice-mute" aria-label="Mute microphone" type="button">
              ${ICON_MIC}
            </button>
            <button class="rc-voice-end" aria-label="End call" type="button">
              ${ICON_END_CALL}
            </button>
          </div>
        </div>

        <div class="rc-input-area">
          <textarea
            class="rc-input"
            placeholder="Type a message..."
            rows="1"
            aria-label="Message input"
          ></textarea>
          <button class="rc-send-btn" aria-label="Send message" type="button">
            ${ICON_SEND}
          </button>
        </div>

        <div class="rc-powered">Powered by <a href="https://ravenapp.dev" target="_blank" rel="noopener">Raven</a></div>
      </div>

      <button class="rc-bubble" aria-label="Open chat" aria-expanded="false" type="button">
        ${ICON_CHAT}
      </button>
    `

    // Cache element refs
    this._panel = shadow.querySelector('.rc-panel') as HTMLElement
    this._bubble = shadow.querySelector('.rc-bubble') as HTMLButtonElement
    this._messagesContainer = shadow.querySelector(
      '.rc-messages',
    ) as HTMLElement
    this._typingIndicator = shadow.querySelector('.rc-typing') as HTMLElement
    this._input = shadow.querySelector('.rc-input') as HTMLTextAreaElement
    this._sendBtn = shadow.querySelector('.rc-send-btn') as HTMLButtonElement
    this._welcomeEl = shadow.querySelector('.rc-welcome') as HTMLElement

    // Voice UI refs
    if (this._voiceEnabled) {
      this._voiceView = shadow.querySelector('.rc-voice') as HTMLElement
      this._voiceStatusEl = shadow.querySelector('.rc-voice-status') as HTMLElement
      this._voicePulse = shadow.querySelector('.rc-voice-pulse') as HTMLElement
      this._voiceTimerEl = shadow.querySelector('.rc-voice-timer') as HTMLElement
    }

    // Restore existing messages after re-render
    this._restoreMessages()
  }

  private _restoreMessages(): void {
    if (this._messages.length > 0) {
      this._welcomeEl.style.display = 'none'
    }

    for (const msg of this._messages) {
      this._appendMessageBubble(msg.role, msg.content)
    }

    if (this._isOpen) {
      this._panel.classList.add('open')
      this._bubble.setAttribute('aria-expanded', 'true')
    }
  }

  // ---- Event binding ----

  private _bindEvents(): void {
    // Toggle chat panel
    this._bubble.addEventListener('click', () => this._togglePanel())

    // Close button
    const closeBtn = this.shadowRoot!.querySelector(
      '.rc-close-btn',
    ) as HTMLButtonElement
    closeBtn.addEventListener('click', () => this._togglePanel(false))

    // Send on button click
    this._sendBtn.addEventListener('click', () => this._handleSend())

    // Send on Enter (Shift+Enter for newline)
    this._input.addEventListener('keydown', (e: KeyboardEvent) => {
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault()
        this._handleSend()
      }
    })

    // Auto-resize textarea
    this._input.addEventListener('input', () => {
      this._input.style.height = 'auto'
      this._input.style.height =
        Math.min(this._input.scrollHeight, 100) + 'px'
    })

    // Voice call button
    if (this._voiceEnabled) {
      const callBtn = this.shadowRoot!.querySelector('.rc-call-btn') as HTMLButtonElement | null
      callBtn?.addEventListener('click', () => this._handleCallToggle())

      const muteBtn = this.shadowRoot!.querySelector('.rc-voice-mute') as HTMLButtonElement | null
      muteBtn?.addEventListener('click', () => this._handleMuteToggle())

      const endBtn = this.shadowRoot!.querySelector('.rc-voice-end') as HTMLButtonElement | null
      endBtn?.addEventListener('click', () => this._handleEndCall())
    }
  }

  // ---- Panel toggle ----

  private _togglePanel(forceState?: boolean): void {
    this._isOpen = forceState !== undefined ? forceState : !this._isOpen

    if (this._isOpen) {
      this._panel.classList.add('open')
      this._bubble.setAttribute('aria-expanded', 'true')
      // Focus input after panel animation
      setTimeout(() => this._input.focus(), 260)
    } else {
      this._panel.classList.remove('open')
      this._bubble.setAttribute('aria-expanded', 'false')
    }
  }

  // ---- Message handling ----

  private _handleSend(): void {
    const text = this._input.value.trim()
    if (!text || this._isStreaming) return

    // Hide welcome text on first message
    if (this._messages.length === 0) {
      this._welcomeEl.style.display = 'none'
    }

    // Add user message
    this._messages.push({ role: 'user', content: text })
    this._appendMessageBubble('user', text)

    // Clear input and reset height
    this._input.value = ''
    this._input.style.height = 'auto'

    // Show typing indicator
    this._setTyping(true)
    this._isStreaming = true
    this._sendBtn.disabled = true

    // Create abort controller for this request
    this._abortController = new AbortController()

    // Prepare a mutable assistant message
    let assistantContent = ''

    sendMessage({
      apiUrl: this._apiUrl,
      apiKey: this._apiKey,
      message: text,
      history: [...this._messages],
      signal: this._abortController.signal,
      onChunk: (chunk: string) => {
        // Hide typing indicator on first chunk
        if (assistantContent === '') {
          this._setTyping(false)
          this._appendMessageBubble('assistant', '')
        }
        assistantContent += chunk
        this._updateLastAssistantMessage(assistantContent)
        this._scrollToBottom()
      },
      onDone: () => {
        this._messages.push({ role: 'assistant', content: assistantContent })
        this._isStreaming = false
        this._sendBtn.disabled = false
        this._setTyping(false)
        this._scrollToBottom()
      },
      onError: (error: Error) => {
        console.error('[raven-chat]', error)
        this._setTyping(false)
        this._isStreaming = false
        this._sendBtn.disabled = false

        const errorText = 'Sorry, something went wrong. Please try again.'
        this._messages.push({ role: 'assistant', content: errorText })
        this._appendMessageBubble('assistant', errorText)
        this._scrollToBottom()
      },
    })

    this._scrollToBottom()
  }

  // ---- Voice call handling ----

  private _handleCallToggle(): void {
    if (this._voiceState === 'idle' || this._voiceState === 'disconnected') {
      this._startVoiceCall()
    } else if (this._voiceState === 'connected') {
      this._handleEndCall()
    }
  }

  private _startVoiceCall(): void {
    this._voiceSession = new VoiceSession(this._livekitUrl, {
      onStateChange: (state: VoiceState) => {
        this._voiceState = state
        this._updateVoiceUI()
      },
      onError: (error: Error) => {
        console.error('[raven-chat] Voice error:', error)
      },
    })

    this._voiceSession.connect()
    this._showVoiceView(true)
  }

  private _handleEndCall(): void {
    this._voiceSession?.disconnect()
    // Wait for disconnect → idle transition, then hide
    setTimeout(() => {
      this._showVoiceView(false)
      this._voiceSession?.destroy()
      this._voiceSession = null
    }, 600)
  }

  private _handleMuteToggle(): void {
    if (!this._voiceSession) return
    const muted = this._voiceSession.toggleMute()
    const muteBtn = this.shadowRoot!.querySelector('.rc-voice-mute') as HTMLElement | null
    if (muteBtn) {
      muteBtn.classList.toggle('muted', muted)
      muteBtn.innerHTML = muted ? ICON_MIC_OFF : ICON_MIC
      muteBtn.setAttribute('aria-label', muted ? 'Unmute microphone' : 'Mute microphone')
    }
  }

  private _showVoiceView(show: boolean): void {
    if (!this._voiceView) return
    const messagesEl = this._messagesContainer
    const inputArea = this.shadowRoot!.querySelector('.rc-input-area') as HTMLElement

    if (show) {
      messagesEl.style.display = 'none'
      inputArea.style.display = 'none'
      this._voiceView.style.display = 'flex'
    } else {
      messagesEl.style.display = 'flex'
      inputArea.style.display = 'flex'
      this._voiceView.style.display = 'none'
    }

    // Update call button state
    const callBtn = this.shadowRoot!.querySelector('.rc-call-btn') as HTMLElement | null
    callBtn?.classList.toggle('active', show)
  }

  private _updateVoiceUI(): void {
    if (!this._voiceStatusEl || !this._voiceSession) return

    const state = this._voiceState
    this._voiceStatusEl.textContent =
      state === 'connecting' ? 'Connecting...' :
      state === 'connected' ? 'Connected' :
      state === 'disconnected' ? 'Disconnected' :
      state === 'error' ? 'Connection failed' : 'Ready'

    this._voiceStatusEl.className = `rc-voice-status${state === 'connected' ? ' connected' : ''}${state === 'error' ? ' error' : ''}`

    // Pulse animation when connected
    this._voicePulse?.classList.toggle('active', state === 'connected')

    // Timer
    if (this._voiceTimerEl && this._voiceSession) {
      this._voiceTimerEl.textContent = formatDuration(this._voiceSession.duration)
    }
  }

  // ---- DOM helpers ----

  private _appendMessageBubble(role: 'user' | 'assistant', text: string): void {
    const el = document.createElement('div')
    el.className = `rc-msg rc-msg--${role}`
    el.textContent = text

    // Insert before the typing indicator
    this._messagesContainer.insertBefore(el, this._typingIndicator)
    this._scrollToBottom()
  }

  private _updateLastAssistantMessage(content: string): void {
    const allMsgs = this._messagesContainer.querySelectorAll('.rc-msg--assistant')
    const last = allMsgs[allMsgs.length - 1]
    if (last) {
      last.textContent = content
    }
  }

  private _setTyping(visible: boolean): void {
    if (visible) {
      this._typingIndicator.classList.add('visible')
    } else {
      this._typingIndicator.classList.remove('visible')
    }
    this._scrollToBottom()
  }

  private _scrollToBottom(): void {
    requestAnimationFrame(() => {
      this._messagesContainer.scrollTop =
        this._messagesContainer.scrollHeight
    })
  }

  private _escapeHtml(str: string): string {
    const div = document.createElement('div')
    div.textContent = str
    return div.innerHTML
  }
}
