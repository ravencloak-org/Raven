# Voice Agent & WebRTC Stack Research

> Research date: 2026-03-27

---

## 1. Speech-to-Text (STT)

### OpenAI Whisper & Variants
| Model | Notes | Latency | License |
|-------|-------|---------|---------|
| **Whisper Large V3** | Baseline; 5.4x speedup over V2 via reduced decoder layers | Batch only | MIT |
| **faster-whisper** | CTranslate2-based reimplementation; ~4x faster than original | Near-real-time with chunking | MIT |
| **whisper.cpp** | C/C++ port; runs on CPU, mobile, edge | Low (CPU-friendly) | MIT |
| **Canary-1B-Flash** | NVIDIA; 1000+ RTFx, 32 encoder / 4 decoder layers | Very low | Apache 2.0 |
| **NVIDIA Parakeet TDT** | RTFx ~2000+; fastest open-source option | Ultra-low | Apache 2.0 |
| **Voxtral (Mistral)** | Built-in language intelligence; successor-class to Whisper | Competitive | Apache 2.0 |
| **Vosk** | Lightweight, fully offline, works on edge/embedded | Low | Apache 2.0 |
| **Deepgram Nova-3** | Commercial API; best-in-class streaming accuracy | <300ms streaming | Proprietary |

### Recommendation for Real-Time Voice Agent
For a real-time pipeline, **faster-whisper** or **Deepgram Nova-3** (API) are the pragmatic choices. For fully self-hosted, **NVIDIA Parakeet TDT** or **Canary-1B-Flash** offer the best speed-accuracy tradeoff. Whisper.cpp is ideal if you need CPU-only or edge deployment.

---

## 2. WebRTC Integration Options

### Browser-Based Voice Calls
WebRTC is the standard for browser-based real-time audio/video. Key options:

| Approach | Description |
|----------|-------------|
| **LiveKit (OSS SFU)** | Open-source Go-based SFU. Agents join a "Room" as headless participants. Full WebRTC stack with TURN/STUN. Self-hostable or use LiveKit Cloud. |
| **Daily.co** | Managed WebRTC infrastructure. Pipecat's default transport. Simple REST API for room creation. |
| **Raw WebRTC (via browser APIs)** | Direct `RTCPeerConnection` usage. Requires your own signaling server. Maximum control, maximum effort. |
| **Twilio** | WebRTC + PSTN bridging. Good for telephony integration. |

### WhatsApp Call Integration (see Section 5)
WhatsApp Business Calling API (launched July 2025) uses WebRTC for media transport with Graph API webhooks for signaling.

---

## 3. Text-to-Speech (TTS)

| Model/Service | Type | Quality | Latency | License | Notes |
|---------------|------|---------|---------|---------|-------|
| **Coqui XTTS v2.5** | OSS | High (voice cloning with 6s sample) | <200ms streaming | MPL 2.0 | Coqui AI shut down Dec 2025; code remains on GitHub (idiap fork) |
| **Piper** | OSS | Good | Very low | MIT | Best for edge/real-time; runs on Raspberry Pi |
| **StyleTTS2** | OSS | Studio-quality prosody | Medium | MIT | Best for narration/long-form |
| **Bark** | OSS | Expressive (laughter, hesitations) | Higher | MIT | Non-verbal sound support |
| **VibeVoice (Microsoft)** | OSS | High | Medium | MIT | Multi-speaker, up to 90min |
| **Cartesia Sonic** | API | Very high | <100ms streaming | Proprietary | Purpose-built for voice agents |
| **ElevenLabs** | API | Very high | <150ms | Proprietary | Best commercial option |
| **Deepgram Aura** | API | High | <100ms | Proprietary | Optimized for conversational AI |

### Recommendation
For production voice agents: **Cartesia Sonic** (API, lowest latency) or **ElevenLabs**. For self-hosted OSS: **Piper** (lightweight, fast) or **Coqui XTTS v2.5** (voice cloning). The idiap/coqui-ai-TTS fork on GitHub is the maintained successor.

---

## 4. Voice Activity Detection (VAD)

| Library | Approach | Latency | Platform | License |
|---------|----------|---------|----------|---------|
| **Silero VAD** | Deep learning (ONNX) | <1ms per 30ms chunk | Python, JS (ONNX Runtime), cross-platform | MIT |
| **WebRTC VAD** | GMM-based signal processing | Very low | C, Python bindings | BSD |
| **TEN VAD** | Deep learning | <1ms | C, Python, WASM (Web), iOS, Android | Apache 2.0 |
| **Cobra VAD (Picovoice)** | Deep learning | Low | Cross-platform | Proprietary (free tier) |
| **@ricky0123/vad** | Silero VAD wrapper for JS/browser | <1ms | Browser, Node.js | MIT |

### Recommendation
**Silero VAD** is the industry standard for voice agent pipelines -- used by LiveKit, Pipecat, and most frameworks. For browser-only, use **@ricky0123/vad** (which wraps Silero). **TEN VAD** is a strong emerging alternative with broader platform support.

---

## 5. WhatsApp Call Integration via WebRTC

### Feasibility: PROVEN AND PRODUCTION-READY

Meta launched the **WhatsApp Business Calling API** in July 2025. It is globally available (except sanctioned countries).

### Architecture
```
WhatsApp User (mobile app)
       |
       | (WhatsApp internal protocol)
       v
  Meta Cloud API
       |
       | (Graph API webhooks for signaling, WebRTC for media)
       v
  Your Server (webhook receiver + WebRTC bridge)
       |
       | (WebRTC or SIP)
       v
  Your Voice Agent / Browser Client
```

### Signaling Flow
1. WhatsApp user initiates call to your business number
2. Meta sends POST webhook to your server with `call_id` + **SDP offer**
3. Your server creates an `RTCPeerConnection`, generates **SDP answer**
4. SDP answer sent back to Meta via Graph API (`/calls` endpoint) with `pre-accept` then `accept` actions
5. WebRTC media channel established -- bidirectional audio flows
6. Your server bridges audio to voice agent pipeline or browser client

### Requirements
- WhatsApp Business Account with verified phone number
- Messaging limit of at least 2,000 conversations per rolling 24-hour period
- Webhook endpoint (HTTPS)
- WebRTC or SIP infrastructure for the business leg

### Options
| Approach | Complexity | Notes |
|----------|-----------|-------|
| **WhatsApp Business Calling API (official)** | Medium | Recommended. Direct WebRTC/SIP integration. |
| **Third-party BSPs** (Route Mobile, 2Factor, etc.) | Low | Managed integration; handle webhook + media bridging for you. |
| **green-api** (unofficial) | Low | Third-party WhatsApp API; not officially sanctioned by Meta. |

### Limitations
- Cannot bridge to PSTN (traditional phone networks)
- Business-initiated calls require user opt-in via prior message thread
- Call duration limits may apply depending on account tier

---

## 6. Real-Time Voice Agent Frameworks

### Framework Comparison Matrix

| Framework | Language | Transport | Architecture | STT/TTS Plugins | Turn Detection | License | Best For |
|-----------|----------|-----------|--------------|-----------------|----------------|---------|----------|
| **LiveKit Agents** | Python, JS, Go | WebRTC (native SFU) | Session + Room model | 15+ providers (Deepgram, OpenAI, Cartesia, etc.) | Custom open-weights model | Apache 2.0 | WebRTC-first, low-latency production |
| **Pipecat** (by Daily) | Python | WebRTC, WebSocket, Telephony | Frame-based pipeline | 20+ providers | Custom VAD + heuristics | BSD 2-Clause | Flexible orchestration, vendor-neutral |
| **TEN Framework** | Python, Go, C++ | WebRTC, custom | Graph-based extensions | Extensible | Custom solution | Apache 2.0 | Maximum flexibility, multi-language |
| **Vapi** | API-first | WebRTC, Telephony | Managed service | Managed | Managed | Proprietary | No-code / low-code, fast to market |
| **Vocode** | Python | WebRTC, Telephony | Agent-based | Multiple | Built-in | MIT | Telephony-focused voice agents |
| **Retell AI** | API-first | WebRTC, Telephony | Managed service | Managed | Managed | Proprietary | Enterprise telephony |
| **RoomKit** | Multi | WebRTC | Room-based | Extensible | Built-in | Varies | Video + voice combined |

### Detailed Framework Notes

#### LiveKit Agents (Recommended for WebRTC-first)
- **GitHub**: https://github.com/livekit/agents (10k+ stars)
- Open-source Go SFU + Python/JS agent SDK
- Agent joins a LiveKit Room as a headless WebRTC participant
- Pipeline: Audio In -> VAD -> STT -> LLM -> TTS -> Audio Out
- Each stage independently testable and swappable
- Self-hostable or use LiveKit Cloud ($0.005-$0.01/min)
- Strong turn detection with custom open-weights model
- Native support for interruption handling

#### Pipecat (Recommended for flexibility)
- **GitHub**: https://github.com/pipecat-ai/pipecat (8k+ stars)
- Created by Daily.co, transport-agnostic
- Frame-based streaming: data flows as typed frames through processor chain
- Supports Daily, LiveKit, Twilio, raw WebSocket transports
- 500-800ms typical round-trip latency
- NVIDIA partnership for conversational AI blueprint
- Most extensive service integrations (20+ STT/TTS/LLM providers)

#### TEN Framework (For advanced use cases)
- Graph-based architecture with extensions as nodes
- JSON-configured directed graph for data flow
- Supports Python, Go, C++
- More complex but maximum flexibility
- Includes its own TEN VAD

---

## Recommended Architecture for Raven

Based on this research, a practical voice agent stack would be:

### Option A: LiveKit-Centric (Simplest WebRTC path)
```
Browser/WhatsApp -> LiveKit SFU (WebRTC) -> LiveKit Agent
  Agent pipeline:
    Silero VAD -> faster-whisper/Deepgram (STT)
    -> Claude/GPT (LLM)
    -> Cartesia/Piper (TTS)
    -> Audio out via LiveKit
```

### Option B: Pipecat-Centric (Most flexible)
```
Browser -> Daily.co/LiveKit (WebRTC transport) -> Pipecat Pipeline
WhatsApp -> WhatsApp Business Calling API -> WebRTC Bridge -> Pipecat Pipeline
  Pipeline:
    Silero VAD -> Deepgram/faster-whisper (STT)
    -> Claude/GPT (LLM)
    -> ElevenLabs/Coqui XTTS (TTS)
    -> Audio out via transport
```

### WhatsApp Integration Path
```
WhatsApp User -> Meta Cloud API -> Webhook Server
  -> SDP exchange via Graph API
  -> WebRTC media bridge
  -> Route audio into LiveKit Room or Pipecat transport
  -> Voice agent processes and responds
```

---

## Key Sources

### STT
- [Top Open Source STT Models (Modal)](https://modal.com/blog/open-source-stt)
- [Best OSS STT 2026 Benchmarks (Northflank)](https://northflank.com/blog/best-open-source-speech-to-text-stt-model-in-2026-benchmarks)
- [Voxtral vs Whisper](https://apidog.com/blog/voxtral-open-source-whisper-alternative/)

### Voice Agent Frameworks
- [LiveKit Agents GitHub](https://github.com/livekit/agents)
- [Pipecat GitHub](https://github.com/pipecat-ai/pipecat)
- [Voice Agent Frameworks Comparison (Arun Baby)](https://www.arunbaby.com/ai-agents/0018-voice-agent-frameworks/)
- [LiveKit vs Pipecat vs Bedrock vs Vertex (WebRTC.ventures)](https://webrtc.ventures/2026/03/choosing-a-voice-ai-agent-production-framework/)
- [LiveKit vs Vapi (Modal)](https://modal.com/blog/livekit-vs-vapi-article)
- [Framework Comparison: LiveKit, Pipecat, TEN (Medium)](https://medium.com/@ggarciabernardo/realtime-ai-agents-frameworks-bb466ccb2a09)
- [LiveKit Voice Agent Architecture](https://livekit.com/blog/voice-agent-architecture-stt-llm-tts-pipelines-explained)

### TTS
- [OSS TTS Models 2026 (BentoML)](https://www.bentoml.com/blog/exploring-the-world-of-open-source-text-to-speech-models)
- [OSS TTS Beyond ElevenLabs (Apatero)](https://apatero.com/blog/open-source-text-to-speech-models-beyond-elevenlabs-2026)
- [Coqui AI TTS GitHub (idiap fork)](https://github.com/idiap/coqui-ai-TTS)

### VAD
- [Silero VAD GitHub](https://github.com/snakers4/silero-vad)
- [TEN VAD GitHub](https://github.com/TEN-framework/ten-vad)
- [VAD Complete Guide 2026 (Picovoice)](https://picovoice.ai/blog/complete-guide-voice-activity-detection-vad/)
- [@ricky0123/vad (JS)](https://github.com/ricky0123/vad)

### WhatsApp
- [WhatsApp Business Calling API + WebRTC (WebRTC.ventures)](https://webrtc.ventures/2025/11/how-to-integrate-the-whatsapp-business-calling-api-with-webrtc-to-enable-customer-voice-calls/)
- [WhatsApp Calling API WebRTC Integration (Medium)](https://medium.com/@arslan.ali1396/how-to-integrate-whatsapp-calling-api-in-your-web-app-using-webrtc-5c073041e819)
- [WhatsApp Calling API SIP & Limits 2026 (wuseller)](https://www.wuseller.com/whatsapp-business-knowledge-hub/whatsapp-business-calling-api-integration-sip-limits-2026/)
- [whatsapp-calling GitHub Demo](https://github.com/arslan1317/whatsapp-calling)
