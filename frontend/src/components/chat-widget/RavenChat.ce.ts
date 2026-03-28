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
 */

import { chatStyles } from './chat-styles'
import { sendMessage, type ChatMessage } from './chat-api'

const DEFAULT_THEME_COLOR = '#6366f1'
const DEFAULT_WELCOME_TEXT = 'Hi there! How can I help you today?'
const DEFAULT_POSITION = 'bottom-right'

// SVG icons used in the widget
const ICON_CHAT = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M20 2H4c-1.1 0-2 .9-2 2v18l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm0 14H5.17L4 17.17V4h16v12z"/><path d="M7 9h2v2H7zm4 0h2v2h-2zm4 0h2v2h-2z"/></svg>`
const ICON_CLOSE = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"/></svg>`
const ICON_SEND = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M2.01 21L23 12 2.01 3 2 10l15 2-15 2z"/></svg>`

export class RavenChat extends HTMLElement {
  static get observedAttributes(): string[] {
    return [
      'api-key',
      'api-url',
      'theme-color',
      'avatar-url',
      'welcome-text',
      'position',
    ]
  }

  // ---- Internal state ----
  private _isOpen = false
  private _messages: ChatMessage[] = []
  private _isStreaming = false
  private _abortController: AbortController | null = null

  // ---- Shadow DOM element refs ----
  private _panel!: HTMLElement
  private _bubble!: HTMLButtonElement
  private _messagesContainer!: HTMLElement
  private _typingIndicator!: HTMLElement
  private _input!: HTMLTextAreaElement
  private _sendBtn!: HTMLButtonElement
  private _welcomeEl!: HTMLElement

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
