# Browser WebRTC Voice UI Design

**Date:** 2026-04-02
**Status:** Approved
**Issue:** #68

## Problem

The raven-chat web component is text-only. Users need the ability to have voice conversations with the AI assistant directly from the browser widget.

## Goal

Add a voice call button to the chat widget header that starts a browser-based voice session via LiveKit. Built against mocks initially — wired to real LiveKit when #57/#58 land.

## Design

### Voice Call Button

Phone/mic icon in the chat widget header bar (next to title/close button). Only visible when `voice-enabled="true"` attribute is set on the widget.

### Voice Session View

When the call button is clicked, the message area transitions to a voice session view:
- Connection status indicator ("Connecting...", "Connected", "Disconnected")
- Pulsing animation indicating agent is listening/speaking
- Mute toggle button
- Red "End call" button
- Duration timer (MM:SS)

Widget stays the same size — voice view replaces the message area. Ending the call returns to text chat.

### New Widget Attributes

- `voice-enabled` — boolean, defaults to false. When true, shows the call button.
- `livekit-url` — LiveKit WebSocket URL (e.g., `wss://raven.example.com/livekit`)

### Technical Approach

**New dependency:** `livekit-client` (LiveKit browser SDK)

**New file — `voice-session.ts`:**
- `VoiceSession` class wrapping LiveKit room connect/disconnect
- `getUserMedia` for microphone access
- Audio track publish/subscribe
- Event callbacks: `onStateChange`, `onError`
- Mock mode: when `livekit-url` is empty or connection fails, simulate connection states with timeouts for development/testing

**Modified — `RavenChat.ce.ts`:**
- Add call button to header rendering
- Add voice session state (`idle`, `connecting`, `connected`, `disconnected`, `error`)
- Voice view render method (replaces message area when in call)
- Mute/unmute toggle
- Duration timer (setInterval, cleared on disconnect)
- Microphone permission error handling

**Modified — `chat-styles.ts`:**
- Voice session container styles
- Pulsing animation keyframes
- Mute/end call button styles
- Connection status indicator styles

### State Machine

```
idle → connecting → connected → disconnected → idle
                  → error → idle
```

### Mock Mode

When `livekit-url` is not set:
- Click call → "Connecting..." (1s) → "Connected" (stays connected)
- Audio visualizer pulses on a timer
- End call returns to idle
- This allows full UI development without a LiveKit server

## Files Changed

| File | Change |
|------|--------|
| `frontend/src/components/chat-widget/voice-session.ts` | New — LiveKit client wrapper + mock mode |
| `frontend/src/components/chat-widget/RavenChat.ce.ts` | Add call button, voice view, state management |
| `frontend/src/components/chat-widget/chat-styles.ts` | Voice UI styles + animations |
| `frontend/package.json` | Add `livekit-client` dependency |

## Out of Scope

- Backend room token endpoint (separate issue, needs #57/#58)
- Speech-to-text / text-to-speech (issues #59/#60)
- Voice session persistence/history (#61)
- LiveKit Agents integration (#58)
