/**
 * Entry point for the <raven-chat> web component.
 *
 * Usage (CDN / script tag):
 *   <script src="https://cdn.example.com/raven-chat.js"></script>
 *   <raven-chat
 *     api-key="your-key"
 *     api-url="https://api.example.com"
 *     theme-color="#6366f1"
 *     welcome-text="Hi! How can I help?"
 *     position="bottom-right"
 *   ></raven-chat>
 */

import { RavenChat } from './RavenChat.ce'

// Guard against double-registration (e.g. script loaded twice)
if (!customElements.get('raven-chat')) {
  customElements.define('raven-chat', RavenChat)
}

export { RavenChat }
export type { ChatMessage } from './chat-api'
