# Interaction Layer Approaches for Raven

> Research date: 2026-03-27
> Status: Research/brainstorm only -- no code written

---

## Overview

Raven's interaction layer consists of three client surfaces -- an embeddable chatbot, a voice agent, and a WebRTC/WhatsApp voice channel -- all backed by a shared RAG pipeline. This document evaluates 3 architectural approaches that differ in how they compose these surfaces, which frameworks they lean on, and where they draw the self-hosted vs. managed-API line.

---

## Shared RAG Pipeline (Common to All Approaches)

Regardless of the interaction surface, every query ultimately hits the same backend:

```
User Input (text or transcribed speech)
       |
       v
  Raven RAG Service
    1. Tenant resolution (API key / session -> org + knowledge base)
    2. Query embedding (model per tenant or shared)
    3. Hybrid retrieval: pgvector (semantic) + ParadeDB pg_search (BM25)
    4. Re-ranking (cross-encoder or reciprocal rank fusion)
    5. Context assembly + prompt construction
    6. LLM generation (Claude / GPT / self-hosted)
    7. Response streaming back to caller
       |
       v
  Response (text stream, or text -> TTS for voice)
```

### Key Design Decisions for Shared Pipeline
- **Single gRPC/REST service** that all three surfaces call into
- **Streaming-first**: the RAG service streams tokens so the TTS layer can begin synthesizing before the full response is complete (critical for voice latency)
- **Tenant isolation**: each request carries a tenant context; retrieval is scoped to that tenant's document set
- **Stateless per request** but maintains conversation history via a session store (Redis / Postgres)

---

## 1. Embeddable Chatbot: Three Delivery Mechanisms

### Option A: Iframe Embed

The simplest approach. The host site adds an `<iframe>` pointing to a Raven-hosted page.

**Components:**
- Raven-hosted chat UI (React/Next.js) served at `https://chat.raven.app/{tenant-id}`
- Iframe snippet: `<iframe src="https://chat.raven.app/{tenant-id}" ...>`
- PostMessage API for host-page communication (optional)

**Pros:**
- Complete style isolation (shadow DOM not needed)
- Security: runs in a separate browsing context; no access to host-page DOM
- Simplest to ship -- no SDK to maintain
- Works on any website, including those with strict CSPs

**Cons:**
- Limited customization -- host page cannot style the chat widget
- Cross-origin communication requires PostMessage coordination
- Mobile responsiveness inside iframe can be awkward
- SEO: content inside iframe is invisible to host-page crawlers
- Cookie/auth sharing requires explicit configuration

**When to choose:** MVP, speed to market, customers who want zero integration effort.

---

### Option B: Web Component (Shadow DOM)

A custom element (`<raven-chat>`) that renders inside the host page's DOM using Shadow DOM for style encapsulation.

**Components:**
- `@raven/chatbot` npm package (or CDN script)
- Custom Element (`<raven-chat tenant="..." theme="...">`)
- Shadow DOM for style encapsulation
- Communicates with Raven backend via WebSocket or SSE for streaming responses

**Pros:**
- Style encapsulation via Shadow DOM -- host CSS cannot leak in
- Deeply embeddable: part of the host DOM tree, so it can participate in layout
- Configurable via HTML attributes and JS properties
- Works with any framework (React, Vue, plain HTML)
- Can expose events (`raven-chat:message-sent`, `raven-chat:session-started`) for host-page integration

**Cons:**
- Shadow DOM has quirks: global font loading, form participation, accessibility tree integration
- Slightly larger bundle than a raw JS SDK (includes rendering layer)
- Older browser support for Shadow DOM (Edge 79+, Safari 10+) -- acceptable in 2026
- Still requires a script tag -- some enterprises dislike third-party JS

**When to choose:** Production embed for customers who want moderate customization and clean integration.

---

### Option C: JavaScript SDK (Headless + Pre-built UI)

A JS SDK that provides both a headless API and an optional pre-built UI. This is the most flexible option.

**Components:**
- `@raven/sdk` -- headless TypeScript SDK (API client, session management, streaming)
- `@raven/chat-widget` -- optional pre-built UI built on top of the SDK (React component + vanilla JS wrapper)
- The SDK handles: auth, streaming, conversation state, file uploads, feedback
- The widget is a thin UI layer that can be replaced entirely

**Pros:**
- Maximum flexibility: customers can build their own UI using the headless SDK
- Pre-built widget available for quick starts
- Tree-shakeable: customers who only need the API client get a tiny bundle
- Full programmatic control: trigger messages, listen to events, inject context
- Can be used in React Native / Electron / Node.js (not just browsers)

**Cons:**
- More surface area to maintain (SDK + widget + docs)
- Headless mode requires customers to build UI -- higher integration effort
- Version compatibility: SDK updates need to stay backward-compatible
- Security: runs in the host page's context (XSS in host page could compromise SDK)

**When to choose:** Platform play -- when you want developers to deeply integrate Raven into their products.

---

### Chatbot Recommendation

**Ship in phases:**
1. **Phase 1 (MVP):** Web Component (`<raven-chat>`). It balances ease of embedding with customization. A single `<script>` + `<raven-chat>` tag gets customers running.
2. **Phase 2:** Extract the headless SDK (`@raven/sdk`) from the web component internals and publish separately. This gives power users programmatic access.
3. **Phase 3 (if needed):** Iframe option for enterprise customers with extreme CSP requirements.

Do NOT start with iframe -- it limits the platform long-term. Do NOT start with a full headless SDK -- it is over-engineering for initial adoption.

---

## 2. Voice Agent Architecture

### Approach A: LiveKit Agents (WebRTC-Native, Self-Hosted Core)

LiveKit provides both the WebRTC transport (SFU) and the agent framework.

**Components:**
- **Transport:** LiveKit SFU (self-hosted or LiveKit Cloud)
- **Agent Framework:** LiveKit Agents SDK (Python)
- **VAD:** Silero VAD (built into LiveKit Agents)
- **STT:** faster-whisper (self-hosted) or Deepgram Nova-3 (API) via LiveKit plugin
- **LLM:** Raven RAG Service (called as a custom LLM plugin, streams tokens)
- **TTS:** Cartesia Sonic (API, lowest latency) or Piper (self-hosted, MIT)
- **Turn Detection:** LiveKit's open-weights turn detection model

**Architecture:**
```
Browser/Mobile
    |
    | WebRTC (audio)
    v
LiveKit SFU (Room)
    |
    | Audio frames
    v
LiveKit Agent (Python process, joins Room as participant)
    |
    +---> Silero VAD (detect speech start/end)
    +---> STT: faster-whisper or Deepgram
    +---> Raven RAG Service (gRPC stream)
    +---> TTS: Cartesia or Piper
    +---> Audio out -> LiveKit Room -> User
```

**Pros:**
- Single framework handles WebRTC + agent orchestration
- LiveKit SFU is battle-tested (used by Slack, Spotify)
- Agent joins a "Room" -- same Room can have browser users, WhatsApp bridges, etc.
- Built-in interruption handling and turn detection
- 15+ STT/TTS provider plugins already available
- Self-hostable end-to-end (SFU + agent + STT + TTS)

**Cons:**
- Tied to LiveKit ecosystem (though SFU is standard WebRTC)
- Agent SDK is Python-first (JS/Go SDKs are less mature for voice)
- Self-hosting LiveKit SFU requires ops investment (TURN servers, scaling)
- LiveKit Cloud pricing can add up at scale ($0.005-$0.01/min)

**Latency budget (optimistic):**
| Stage | Target | Notes |
|-------|--------|-------|
| VAD detection | ~30ms | Silero, 30ms chunks |
| STT | 200-400ms | faster-whisper streaming; Deepgram <300ms |
| RAG + LLM (first token) | 300-600ms | Depends on retrieval + LLM TTFT |
| TTS (first audio chunk) | 50-150ms | Cartesia streaming; Piper ~50ms |
| WebRTC transport | 50-100ms | Jitter buffer + network |
| **Total (speech-to-speech)** | **630-1280ms** | Target: <1s for 80th percentile |

---

### Approach B: Pipecat (Transport-Agnostic, Maximum Flexibility)

Pipecat is a frame-based pipeline framework that is transport-agnostic.

**Components:**
- **Transport:** Daily.co (default) or LiveKit or raw WebSocket
- **Framework:** Pipecat (Python)
- **VAD:** Silero VAD (via Pipecat's VAD processor)
- **STT:** Deepgram Nova-3 (Pipecat's best-supported STT) or faster-whisper
- **LLM:** Raven RAG Service (custom Pipecat processor wrapping gRPC call)
- **TTS:** ElevenLabs or Coqui XTTS (Pipecat has strong ElevenLabs integration) or Cartesia

**Architecture:**
```
Browser/Mobile
    |
    | WebRTC (via Daily.co or LiveKit transport)
    v
Pipecat Pipeline:
    InputTransport (audio frames)
    -> SileroVADProcessor (detect speech)
    -> STTProcessor (Deepgram/faster-whisper)
    -> RavenRAGProcessor (custom: calls Raven RAG Service)
    -> TTSProcessor (ElevenLabs/Cartesia/Coqui)
    -> OutputTransport (audio frames back to user)
```

**Pros:**
- Transport-agnostic: swap Daily <-> LiveKit <-> Twilio without rewriting pipeline
- Frame-based architecture makes it easy to insert custom processing (e.g., profanity filter, language detection)
- Most extensive provider integrations (20+ STT/TTS/LLM connectors)
- NVIDIA partnership (conversational AI blueprint)
- Can bridge to telephony (Twilio) for future PSTN support
- Strong community (8k+ GitHub stars)

**Cons:**
- Requires a separate WebRTC transport layer (Daily.co or LiveKit SFU)
- Daily.co is managed-only (cannot self-host the transport)
- Slightly higher latency than LiveKit Agents due to frame-processing overhead (~500-800ms typical)
- Less opinionated about turn detection (requires manual tuning)

**Latency budget (optimistic):**
| Stage | Target | Notes |
|-------|--------|-------|
| VAD detection | ~30ms | Silero |
| Frame processing overhead | 20-50ms | Pipecat frame pipeline |
| STT | 200-400ms | Deepgram streaming |
| RAG + LLM (first token) | 300-600ms | Same as Approach A |
| TTS (first audio chunk) | 50-200ms | ElevenLabs ~150ms; Cartesia ~100ms |
| WebRTC transport | 50-100ms | Same as Approach A |
| **Total (speech-to-speech)** | **650-1380ms** | Slightly higher ceiling than LiveKit |

---

### Approach C: Hybrid -- LiveKit SFU + Pipecat Pipeline

Use LiveKit as the WebRTC transport but Pipecat as the agent orchestration framework. This gives you LiveKit's SFU quality with Pipecat's pipeline flexibility.

**Components:**
- **Transport:** LiveKit SFU (self-hosted) with Pipecat's LiveKit transport adapter
- **Framework:** Pipecat (pipeline orchestration)
- **VAD/STT/TTS:** Same choices as Approach B
- **LLM:** Raven RAG Service

**Architecture:**
```
Browser/Mobile/WhatsApp Bridge
    |
    | WebRTC
    v
LiveKit SFU (self-hosted)
    |
    | Audio frames via LiveKit transport adapter
    v
Pipecat Pipeline (same as Approach B)
```

**Pros:**
- Self-hostable WebRTC transport (LiveKit SFU) + flexible pipeline (Pipecat)
- Can switch STT/TTS providers without touching transport layer
- Pipecat's LiveKit transport is officially supported
- Best of both worlds: LiveKit's Room model for multi-participant + Pipecat's processor chain

**Cons:**
- Two frameworks to learn and maintain (LiveKit + Pipecat)
- Potential version compatibility issues between Pipecat and LiveKit SDKs
- Smaller community using this specific combination (most use one or the other)

---

### Voice Agent Recommendation

**Approach A (LiveKit Agents)** for Raven. Rationale:
1. Raven needs WebRTC for both the voice agent and WhatsApp bridging -- LiveKit's Room model is the natural fit
2. LiveKit Agents' built-in turn detection and interruption handling reduce custom work
3. Self-hostable end-to-end (important for multi-tenant SaaS with data residency requirements)
4. The STT/TTS provider plugins mean we can start with APIs (Deepgram + Cartesia) and migrate to self-hosted (faster-whisper + Piper) as volume grows

If vendor flexibility becomes critical later, migrate to Approach C (LiveKit SFU + Pipecat pipeline) -- the transport layer stays the same.

**Recommended STT/TTS choices by phase:**
| Phase | STT | TTS | Rationale |
|-------|-----|-----|-----------|
| MVP | Deepgram Nova-3 (API) | Cartesia Sonic (API) | Fastest to production, lowest latency |
| Scale | faster-whisper (self-hosted) | Piper (self-hosted) | Cost control, data residency |
| Premium tier | Deepgram Nova-3 (API) | ElevenLabs or Cartesia (API) | Best quality for paying customers |

---

## 3. WebRTC Integration & WhatsApp Connectivity

### WebRTC Architecture

With LiveKit as the chosen SFU, the WebRTC layer is straightforward:

```
Raven Frontend (React)
    |
    | livekit-client-sdk-js
    v
LiveKit SFU
    |
    | LiveKit Agents SDK
    v
Voice Agent (STT -> RAG -> TTS)
```

**Room Model:**
- Each voice session creates a LiveKit Room
- The user joins as a `Participant` (browser or mobile)
- The Raven voice agent joins as a headless `Participant`
- Audio flows bidirectionally via WebRTC tracks
- Room metadata carries tenant context (knowledge base ID, session ID)

**Scaling:**
- LiveKit SFU scales horizontally (multiple SFU instances behind a load balancer)
- Agents scale independently (multiple agent workers subscribe to LiveKit's dispatch system)
- LiveKit's built-in room service handles room creation/destruction

---

### WhatsApp Connectivity

#### Architecture
```
WhatsApp User (mobile app)
    |
    | WhatsApp internal protocol
    v
Meta Cloud API (Graph API)
    |
    | Webhook: incoming call notification + SDP offer
    v
Raven WhatsApp Bridge Service
    |
    | 1. Receives SDP offer from Meta webhook
    | 2. Creates RTCPeerConnection (or LiveKit Room ingress)
    | 3. Sends SDP answer back to Meta via Graph API
    | 4. WebRTC media channel established with Meta
    v
LiveKit Room (via LiveKit Ingress or custom bridge)
    |
    | Audio frames routed to agent participant
    v
Voice Agent (same pipeline as browser-based voice)
```

#### Three Integration Strategies

**Strategy 1: Direct WebRTC Bridge (Recommended)**

The Raven WhatsApp Bridge service handles the SDP exchange with Meta's Graph API and establishes a direct WebRTC peer connection. Audio is then forwarded into a LiveKit Room via LiveKit's SIP/WebRTC ingress.

| Aspect | Detail |
|--------|--------|
| Complexity | Medium |
| Components | Webhook server, WebRTC peer, LiveKit ingress |
| Latency | Lowest (one hop from Meta to LiveKit) |
| Self-hostable | Yes |
| Dependency | Meta Cloud API, LiveKit |

**Strategy 2: Third-Party BSP (Fastest to Market)**

Use a Business Solution Provider (Route Mobile, 2Factor, Vonage) that handles the Meta webhook + SDP exchange and provides a SIP trunk or WebRTC endpoint to connect to.

| Aspect | Detail |
|--------|--------|
| Complexity | Low |
| Components | BSP account, SIP/WebRTC endpoint, LiveKit SIP ingress |
| Latency | Slightly higher (extra hop through BSP) |
| Self-hostable | Partially (BSP is managed) |
| Dependency | BSP + Meta Cloud API + LiveKit |

**Strategy 3: Pipecat + Twilio Bridge (Telephony-First)**

If telephony (PSTN) support is also needed, use Pipecat with Twilio as the transport. Twilio can bridge WhatsApp voice to a SIP trunk.

| Aspect | Detail |
|--------|--------|
| Complexity | Medium-High |
| Components | Twilio account, Pipecat, SIP trunk |
| Latency | Higher (Twilio + SIP transcoding) |
| Self-hostable | No (Twilio is fully managed) |
| Dependency | Twilio + Meta partnership |

#### WhatsApp Connectivity Recommendation

**Strategy 1 (Direct WebRTC Bridge)** for Raven. The WhatsApp Business Calling API already speaks WebRTC, and LiveKit already speaks WebRTC -- bridging them directly avoids unnecessary hops and keeps latency minimal.

**Implementation order:**
1. Build the webhook receiver for Meta's Graph API call events
2. Implement SDP offer/answer exchange
3. Bridge the WebRTC media stream into a LiveKit Room (via LiveKit's ingress API or a custom WebRTC-to-Room bridge)
4. The voice agent in the Room handles the call exactly as it would a browser call -- no special logic needed

---

## 4. How All Three Surfaces Share the RAG Pipeline

### Unified Architecture Diagram

```
+-------------------+     +------------------+     +----------------------------+
|  Embeddable Chat  |     |   Voice Agent    |     | WhatsApp / WebRTC Client   |
|  (Web Component)  |     | (LiveKit Agent)  |     | (WhatsApp Bridge Service)  |
+--------+----------+     +--------+---------+     +-----------+----------------+
         |                         |                            |
         | WebSocket/SSE           | gRPC (streaming)           | LiveKit Room
         | (text stream)           | (text from STT)            | (audio -> STT -> text)
         |                         |                            |
         v                         v                            v
+--------+-------------------------+----------------------------+---------+
|                                                                         |
|                        Raven RAG Service (gRPC + REST)                  |
|                                                                         |
|   1. Tenant Resolution                                                  |
|   2. Conversation History (Redis/Postgres session store)                |
|   3. Query Embedding                                                    |
|   4. Hybrid Retrieval (pgvector + ParadeDB)                             |
|   5. Re-ranking                                                         |
|   6. LLM Generation (streaming)                                         |
|                                                                         |
+--------+-------------------------+--------------------------------------+
         |                         |
         v                         v
+--------+---------+     +---------+----------+
|  PostgreSQL      |     |  LLM Provider      |
|  (pgvector +     |     |  (Claude / GPT /   |
|   ParadeDB)      |     |   self-hosted)     |
+-----------------+      +--------------------+
```

### Interface Contract

All three surfaces call the same RAG service via a unified interface:

```protobuf
service RavenRAG {
  // Streaming query -- used by all three surfaces
  rpc Query(QueryRequest) returns (stream QueryResponse);

  // Session management
  rpc CreateSession(CreateSessionRequest) returns (Session);
  rpc GetSessionHistory(SessionHistoryRequest) returns (SessionHistory);
}

message QueryRequest {
  string tenant_id = 1;
  string session_id = 2;
  string query_text = 3;          // For chatbot: user's typed message
                                   // For voice: STT transcription
  map<string, string> metadata = 4; // Source channel, user info, etc.
}

message QueryResponse {
  string token = 1;                // Streamed token
  repeated Source sources = 2;     // Retrieved sources (sent with final chunk)
  bool is_final = 3;
}
```

### Surface-Specific Adaptations

| Aspect | Chatbot | Voice Agent | WhatsApp |
|--------|---------|-------------|----------|
| Input | User-typed text | STT transcription | STT transcription |
| Output | Streamed text (SSE/WebSocket) | Streamed text -> TTS -> audio | Streamed text -> TTS -> audio |
| Session mgmt | Cookie/JWT + session ID | LiveKit Room metadata | WhatsApp call_id -> session |
| Interruption | N/A (user can just type) | VAD detects new speech; cancel current TTS | Same as voice agent |
| Sources/citations | Rendered as clickable links | Optionally read aloud or sent as follow-up text | Sent as follow-up WhatsApp message |
| Latency tolerance | 1-3s acceptable | <1s target (speech-to-speech) | <1s target (speech-to-speech) |
| Streaming granularity | Token-level | Token-level (TTS needs sentence boundaries) | Token-level (same as voice) |

---

## 5. Latency Considerations for Real-Time Voice

### End-to-End Latency Budget

For natural conversation, total speech-to-speech latency must be **under 1 second** (80th percentile). Human conversational turn-taking gaps average ~200ms, so anything over 1.5s feels unresponsive.

```
User speaks -> [VAD] -> [STT] -> [RAG+LLM] -> [TTS] -> User hears response
              30ms    200-400ms  300-600ms    50-150ms   50-100ms (transport)
                                                         ___________________
                                                         Total: 630-1280ms
```

### Optimization Strategies

#### 1. Streaming at Every Stage
- **STT streaming**: Send partial transcripts to RAG service before utterance is complete (speculative execution)
- **LLM streaming**: Stream tokens as they are generated
- **TTS streaming**: Begin synthesizing audio from the first sentence while LLM is still generating the rest
- **Result**: Each stage's latency partially overlaps with the next

#### 2. Sentence-Boundary TTS (Critical Optimization)
Instead of waiting for the full LLM response, buffer tokens until a sentence boundary (period, question mark, exclamation) and send each sentence to TTS immediately:

```
LLM Token Stream:  "The | answer | is | in | section | 3. | It | explains | ..."
                                                         ^
                                          Sentence boundary detected
                                          -> Send "The answer is in section 3." to TTS
                                          -> Begin audio playback
                                          -> Continue buffering next sentence
```

This alone can reduce perceived latency by 40-60%.

#### 3. Endpointing Optimization
- **Aggressive endpointing**: Silero VAD can detect end-of-speech in ~300ms of silence. Tune this per use case.
- **Speculative STT**: Start RAG retrieval on partial STT transcript (before user finishes speaking). If the final transcript differs, cancel and re-query. Works well for short queries.

#### 4. RAG Latency Reduction
- **Pre-computed embeddings**: All knowledge base documents are pre-embedded at ingestion time
- **Approximate nearest neighbor (HNSW)**: pgvector's HNSW index for sub-100ms retrieval
- **Cache frequent queries**: LRU cache on embedding + retrieval results per tenant
- **Limit context**: Retrieve top-3 to top-5 chunks instead of top-10 to reduce LLM input tokens

#### 5. LLM Latency Reduction
- **Smaller models for voice**: Use a faster model (Claude Haiku / GPT-4o-mini) for voice interactions where brevity matters
- **Shorter max tokens**: Voice responses should be 1-3 sentences; set `max_tokens` accordingly
- **Prompt optimization**: Shorter system prompts reduce time-to-first-token

#### 6. Infrastructure
- **Co-locate services**: STT, RAG, LLM, and TTS services should be in the same region/datacenter
- **GPU inference for STT/TTS**: faster-whisper and Piper benefit enormously from GPU
- **Connection pooling**: Keep persistent gRPC connections between agent and RAG service
- **Edge deployment**: For global users, deploy LiveKit SFU at edge (multiple regions)

### Latency Monitoring

Track these metrics per voice session:
| Metric | Target | Alert Threshold |
|--------|--------|-----------------|
| VAD-to-STT-complete | <500ms | >800ms |
| STT-complete-to-first-LLM-token | <400ms | >700ms |
| First-LLM-token-to-first-TTS-audio | <200ms | >400ms |
| Total speech-to-speech | <1000ms | >1500ms |
| Interruption detection | <200ms | >500ms |

---

## Summary Comparison

| Dimension | Approach A (LiveKit-Centric) | Approach B (Pipecat-Centric) | Approach C (Hybrid) |
|-----------|------------------------------|-------------------------------|----------------------|
| **Chatbot** | Web Component (all approaches) | Same | Same |
| **Voice transport** | LiveKit SFU | Daily.co (or LiveKit) | LiveKit SFU |
| **Voice framework** | LiveKit Agents | Pipecat | Pipecat on LiveKit |
| **WhatsApp bridge** | Direct WebRTC -> LiveKit Room | Direct WebRTC -> Daily/Pipecat | Direct WebRTC -> LiveKit Room |
| **Self-hostable** | Fully | Partially (Daily.co is managed) | Fully |
| **Vendor flexibility** | Medium (LiveKit ecosystem) | High (transport-agnostic) | High |
| **Operational complexity** | Low-Medium | Medium | Medium-High |
| **Latency** | Best (tightest integration) | Good (frame overhead) | Good |
| **Community/maturity** | Strong (10k+ stars) | Strong (8k+ stars) | Smaller |

---

## Final Recommendation

**Go with Approach A (LiveKit-Centric) for the initial build:**

1. **Chatbot**: Web Component (`<raven-chat>`) backed by WebSocket/SSE to the Raven RAG Service. Extract headless SDK in Phase 2.

2. **Voice Agent**: LiveKit Agents with Silero VAD + Deepgram Nova-3 (STT) + Cartesia Sonic (TTS). The agent calls the same Raven RAG Service via gRPC streaming. Migrate STT/TTS to self-hosted (faster-whisper / Piper) as volume grows.

3. **WhatsApp**: Direct WebRTC bridge from Meta's Calling API into a LiveKit Room. The voice agent handles WhatsApp calls identically to browser calls.

4. **Shared RAG**: Single gRPC streaming service with tenant isolation. All three surfaces converge at the `RavenRAG.Query()` endpoint.

This approach minimizes the number of moving parts while keeping the option to adopt Pipecat later if vendor flexibility becomes a priority. The critical path to sub-1-second voice latency is: streaming at every stage + sentence-boundary TTS + co-located infrastructure.
