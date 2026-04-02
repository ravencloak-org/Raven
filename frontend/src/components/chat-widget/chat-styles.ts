/**
 * Self-contained CSS for the <raven-chat> web component.
 * All styles are scoped to the shadow root -- no Tailwind dependency.
 */

export function chatStyles(themeColor: string): string {
  return /* css */ `
    :host {
      --rc-primary: ${themeColor};
      --rc-primary-hover: color-mix(in srgb, ${themeColor} 85%, black);
      --rc-bg: #ffffff;
      --rc-bg-secondary: #f3f4f6;
      --rc-text: #111827;
      --rc-text-muted: #6b7280;
      --rc-border: #e5e7eb;
      --rc-shadow: 0 4px 24px rgba(0, 0, 0, 0.15);
      --rc-radius: 12px;
      --rc-font: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto,
        Oxygen, Ubuntu, Cantarell, 'Open Sans', 'Helvetica Neue', sans-serif;

      position: fixed;
      z-index: 2147483647;
      font-family: var(--rc-font);
      font-size: 14px;
      line-height: 1.5;
      color: var(--rc-text);
      box-sizing: border-box;
    }

    :host([position="bottom-right"]) {
      bottom: 20px;
      right: 20px;
    }

    :host([position="bottom-left"]) {
      bottom: 20px;
      left: 20px;
    }

    /* Default to bottom-right when no position attribute */
    :host(:not([position])) {
      bottom: 20px;
      right: 20px;
    }

    *, *::before, *::after {
      box-sizing: border-box;
      margin: 0;
      padding: 0;
    }

    /* ---- Chat Bubble (FAB) ---- */
    .rc-bubble {
      width: 56px;
      height: 56px;
      border-radius: 50%;
      background: var(--rc-primary);
      color: #ffffff;
      border: none;
      cursor: pointer;
      display: flex;
      align-items: center;
      justify-content: center;
      box-shadow: var(--rc-shadow);
      transition: transform 0.2s ease, background 0.2s ease;
      position: relative;
    }

    .rc-bubble:hover {
      transform: scale(1.08);
      background: var(--rc-primary-hover);
    }

    .rc-bubble:focus-visible {
      outline: 2px solid var(--rc-primary);
      outline-offset: 3px;
    }

    .rc-bubble svg {
      width: 26px;
      height: 26px;
      fill: currentColor;
      transition: transform 0.25s ease;
    }

    .rc-bubble[aria-expanded="true"] svg {
      transform: rotate(90deg);
    }

    /* ---- Chat Panel ---- */
    .rc-panel {
      display: none;
      flex-direction: column;
      width: 370px;
      max-width: calc(100vw - 40px);
      height: 520px;
      max-height: calc(100vh - 100px);
      background: var(--rc-bg);
      border-radius: var(--rc-radius);
      box-shadow: var(--rc-shadow);
      overflow: hidden;
      margin-bottom: 12px;
      border: 1px solid var(--rc-border);
      animation: rc-slide-up 0.25s ease forwards;
    }

    .rc-panel.open {
      display: flex;
    }

    @keyframes rc-slide-up {
      from {
        opacity: 0;
        transform: translateY(12px);
      }
      to {
        opacity: 1;
        transform: translateY(0);
      }
    }

    /* ---- Header ---- */
    .rc-header {
      display: flex;
      align-items: center;
      gap: 10px;
      padding: 14px 16px;
      background: var(--rc-primary);
      color: #ffffff;
      flex-shrink: 0;
    }

    .rc-header-avatar {
      width: 34px;
      height: 34px;
      border-radius: 50%;
      object-fit: cover;
      background: rgba(255, 255, 255, 0.2);
      flex-shrink: 0;
    }

    .rc-header-info {
      flex: 1;
      min-width: 0;
    }

    .rc-header-title {
      font-size: 15px;
      font-weight: 600;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .rc-header-subtitle {
      font-size: 12px;
      opacity: 0.85;
    }

    .rc-close-btn {
      background: none;
      border: none;
      color: #ffffff;
      cursor: pointer;
      padding: 4px;
      border-radius: 4px;
      display: flex;
      align-items: center;
      justify-content: center;
    }

    .rc-close-btn:hover {
      background: rgba(255, 255, 255, 0.2);
    }

    .rc-close-btn svg {
      width: 18px;
      height: 18px;
      fill: currentColor;
    }

    /* ---- Messages ---- */
    .rc-messages {
      flex: 1;
      overflow-y: auto;
      padding: 16px;
      display: flex;
      flex-direction: column;
      gap: 12px;
      scroll-behavior: smooth;
    }

    .rc-messages::-webkit-scrollbar {
      width: 5px;
    }

    .rc-messages::-webkit-scrollbar-thumb {
      background: var(--rc-border);
      border-radius: 3px;
    }

    .rc-welcome {
      text-align: center;
      color: var(--rc-text-muted);
      font-size: 13px;
      padding: 24px 12px;
      line-height: 1.6;
    }

    .rc-msg {
      max-width: 82%;
      padding: 10px 14px;
      border-radius: 16px;
      word-wrap: break-word;
      white-space: pre-wrap;
      font-size: 14px;
      line-height: 1.5;
    }

    .rc-msg--user {
      align-self: flex-end;
      background: var(--rc-primary);
      color: #ffffff;
      border-bottom-right-radius: 4px;
    }

    .rc-msg--assistant {
      align-self: flex-start;
      background: var(--rc-bg-secondary);
      color: var(--rc-text);
      border-bottom-left-radius: 4px;
    }

    /* ---- Typing Indicator ---- */
    .rc-typing {
      align-self: flex-start;
      display: none;
      align-items: center;
      gap: 4px;
      padding: 10px 14px;
      background: var(--rc-bg-secondary);
      border-radius: 16px;
      border-bottom-left-radius: 4px;
    }

    .rc-typing.visible {
      display: flex;
    }

    .rc-typing-dot {
      width: 7px;
      height: 7px;
      border-radius: 50%;
      background: var(--rc-text-muted);
      animation: rc-bounce 1.4s infinite ease-in-out both;
    }

    .rc-typing-dot:nth-child(1) { animation-delay: 0s; }
    .rc-typing-dot:nth-child(2) { animation-delay: 0.16s; }
    .rc-typing-dot:nth-child(3) { animation-delay: 0.32s; }

    @keyframes rc-bounce {
      0%, 80%, 100% { transform: scale(0.6); opacity: 0.4; }
      40% { transform: scale(1); opacity: 1; }
    }

    /* ---- Input Area ---- */
    .rc-input-area {
      display: flex;
      align-items: flex-end;
      gap: 8px;
      padding: 12px 16px;
      border-top: 1px solid var(--rc-border);
      background: var(--rc-bg);
      flex-shrink: 0;
    }

    .rc-input {
      flex: 1;
      resize: none;
      border: 1px solid var(--rc-border);
      border-radius: 20px;
      padding: 8px 14px;
      font-family: var(--rc-font);
      font-size: 14px;
      line-height: 1.4;
      color: var(--rc-text);
      background: var(--rc-bg);
      outline: none;
      max-height: 100px;
      overflow-y: auto;
      transition: border-color 0.2s ease;
    }

    .rc-input::placeholder {
      color: var(--rc-text-muted);
    }

    .rc-input:focus {
      border-color: var(--rc-primary);
    }

    .rc-send-btn {
      width: 36px;
      height: 36px;
      border-radius: 50%;
      background: var(--rc-primary);
      color: #ffffff;
      border: none;
      cursor: pointer;
      display: flex;
      align-items: center;
      justify-content: center;
      flex-shrink: 0;
      transition: background 0.2s ease, opacity 0.2s ease;
    }

    .rc-send-btn:hover {
      background: var(--rc-primary-hover);
    }

    .rc-send-btn:disabled {
      opacity: 0.5;
      cursor: not-allowed;
    }

    .rc-send-btn svg {
      width: 18px;
      height: 18px;
      fill: currentColor;
    }

    /* ---- Powered By ---- */
    .rc-powered {
      text-align: center;
      font-size: 11px;
      color: var(--rc-text-muted);
      padding: 6px;
      background: var(--rc-bg);
      border-top: 1px solid var(--rc-border);
      flex-shrink: 0;
    }

    .rc-powered a {
      color: var(--rc-primary);
      text-decoration: none;
      font-weight: 500;
    }

    .rc-powered a:hover {
      text-decoration: underline;
    }

    /* ---- Responsive ---- */
    @media (max-width: 420px) {
      .rc-panel {
        width: calc(100vw - 20px);
        height: calc(100vh - 80px);
        border-radius: 8px;
      }
    }

    /* ---- Voice Call Button (Header) ---- */
    .rc-call-btn {
      background: none;
      border: none;
      color: #ffffff;
      cursor: pointer;
      padding: 4px;
      border-radius: 4px;
      display: flex;
      align-items: center;
      justify-content: center;
    }

    .rc-call-btn:hover {
      background: rgba(255, 255, 255, 0.2);
    }

    .rc-call-btn svg {
      width: 18px;
      height: 18px;
      fill: currentColor;
    }

    .rc-call-btn.active {
      background: rgba(255, 255, 255, 0.3);
    }

    /* ---- Voice Session View ---- */
    .rc-voice {
      flex: 1;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      gap: 24px;
      padding: 32px 16px;
      background: var(--rc-bg);
    }

    .rc-voice-status {
      font-size: 14px;
      color: var(--rc-text-muted);
      text-transform: uppercase;
      letter-spacing: 0.05em;
    }

    .rc-voice-status.connected {
      color: #10b981;
    }

    .rc-voice-status.error {
      color: #ef4444;
    }

    .rc-voice-timer {
      font-size: 32px;
      font-weight: 300;
      color: var(--rc-text);
      font-variant-numeric: tabular-nums;
    }

    /* ---- Pulse Animation ---- */
    .rc-voice-pulse {
      width: 80px;
      height: 80px;
      border-radius: 50%;
      background: var(--rc-primary);
      position: relative;
      display: flex;
      align-items: center;
      justify-content: center;
    }

    .rc-voice-pulse::before,
    .rc-voice-pulse::after {
      content: '';
      position: absolute;
      inset: 0;
      border-radius: 50%;
      background: var(--rc-primary);
      opacity: 0;
    }

    .rc-voice-pulse.active::before {
      animation: rc-pulse-ring 2s ease-out infinite;
    }

    .rc-voice-pulse.active::after {
      animation: rc-pulse-ring 2s ease-out 0.5s infinite;
    }

    .rc-voice-pulse svg {
      width: 32px;
      height: 32px;
      fill: #ffffff;
      z-index: 1;
    }

    @keyframes rc-pulse-ring {
      0% {
        transform: scale(1);
        opacity: 0.4;
      }
      100% {
        transform: scale(1.8);
        opacity: 0;
      }
    }

    /* ---- Voice Controls ---- */
    .rc-voice-controls {
      display: flex;
      align-items: center;
      gap: 16px;
    }

    .rc-voice-mute {
      width: 48px;
      height: 48px;
      border-radius: 50%;
      background: var(--rc-bg-secondary);
      border: 1px solid var(--rc-border);
      color: var(--rc-text);
      cursor: pointer;
      display: flex;
      align-items: center;
      justify-content: center;
      transition: background 0.2s ease;
    }

    .rc-voice-mute:hover {
      background: var(--rc-border);
    }

    .rc-voice-mute.muted {
      background: #fef2f2;
      border-color: #fecaca;
      color: #ef4444;
    }

    .rc-voice-mute svg {
      width: 20px;
      height: 20px;
      fill: currentColor;
    }

    .rc-voice-end {
      width: 56px;
      height: 56px;
      border-radius: 50%;
      background: #ef4444;
      border: none;
      color: #ffffff;
      cursor: pointer;
      display: flex;
      align-items: center;
      justify-content: center;
      transition: background 0.2s ease, transform 0.2s ease;
    }

    .rc-voice-end:hover {
      background: #dc2626;
      transform: scale(1.05);
    }

    .rc-voice-end svg {
      width: 24px;
      height: 24px;
      fill: currentColor;
    }
  `
}
