# TEN Framework vs LiveKit Agents + Custom RAG Pipeline

**Date:** 2026-03-27
**Context:** Evaluating TEN Framework as a potential replacement for LiveKit Agents + custom Python RAG pipeline for Raven -- a knowledge-base platform with embeddable chatbot, voice agent, and WebRTC/WhatsApp voice call modes.

---

## 1. What is the TEN Framework?

TEN (Transformative Extensions Network) is an open-source framework for building **real-time multimodal conversational AI agents**. It is developed and maintained by **Agora** (the real-time engagement platform company).

- **GitHub:** [TEN-framework/ten_framework](https://github.com/TEN-framework/ten_framework) (core) + [TEN-framework/TEN-Agent](https://github.com/TEN-framework/TEN-Agent) (agent examples & extensions)
- **Stars:** ~10,300 (TEN-Agent repo)
- **Created:** June 2024
- **Language:** Core runtime in C++/Rust with Python, Go, TypeScript, and Node.js extension runtimes
- **Ecosystem:** TEN Framework core, TEN Agent (examples), TEN VAD, TEN Turn Detection, TEN Portal (docs)

### Core Architecture

TEN uses a **graph-based agent composition** model. Agents are defined declaratively in `property.json` files as directed graphs of extensions:

```json
{
  "ten": {
    "predefined_graphs": [{
      "name": "voice_assistant",
      "auto_start": true,
      "graph": {
        "nodes": [
          {"name": "agora_rtc", "addon": "agora_rtc", ...},
          {"name": "stt", "addon": "deepgram_asr_python", ...},
          {"name": "llm", "addon": "openai_llm2_python", ...},
          {"name": "tts", "addon": "elevenlabs_tts2_python", ...}
        ],
        "connections": [
          {"extension": "main_control", "data": [{"name": "asr_result", "source": [{"extension": "stt"}]}]},
          {"extension": "llm", "audio_frame": [{"name": "pcm_frame", "source": [...]}]}
        ]
      }
    }]
  }
}
```

**Connection types** between nodes:
- `data` -- structured data messages (ASR results, text)
- `cmd` -- command messages (on_user_joined, tool_register)
- `audio_frame` -- PCM audio streams
- `video_frame` -- video streams

### Extension System

Extensions are modular components with a standard structure:
- `manifest.json` -- metadata, dependencies, API interface
- `property.json` -- default config (supports `${env:VAR_NAME}` syntax)
- `addon.py` -- registration via `@register_addon_as_extension` decorator
- `extension.py` -- main logic inheriting from base classes (`AsyncASRBaseExtension`, `AsyncTTSBaseExtension`, `LLMBaseExtension`, etc.)

The framework includes **60+ built-in extensions** covering ASR, TTS, LLM, tools, and transport.

---

## 2. What TEN Provides Out of the Box

### Pre-built Extensions (from TEN-Agent repository)

| Category | Extensions |
|----------|-----------|
| **STT/ASR** | Deepgram, Azure ASR, Agora built-in ASR |
| **LLM** | OpenAI (GPT-4o, etc.), plus extensible base class |
| **TTS** | ElevenLabs, plus extensible base class |
| **Real-time Transport** | Agora RTC (WebRTC), WebSocket, RTM Transport |
| **SIP/Telephony** | Plivo, Telnyx, Twilio (via SIP extensions) |
| **VAD** | TEN VAD (dedicated low-latency streaming VAD) |
| **Turn Detection** | TEN Turn Detection (full-duplex dialogue) |
| **Video/Vision** | Voice-assistant-video example with vision capabilities |
| **Memory** | Memory extensions (EverMemOS, PowerMem, memU) |
| **Avatars** | Live2D, lip-sync avatar support |
| **Speaker Diarization** | Speechmatics-based diarization |

### Infrastructure

- **TMAN Designer** -- visual graph editor (localhost:49483) for composing agent pipelines
- **Next.js Playground** -- pre-built frontend UI for testing agents
- **Go API Server** -- HTTP server for agent lifecycle management
- **Docker-first deployment** -- Dockerfile per example, docker-compose for dev
- **ESP32 hardware client** -- IoT device integration

### Agent Examples

- Voice Assistant (chained: STT -> LLM -> TTS)
- Voice Assistant Realtime (OpenAI Realtime API, speech-to-speech)
- Voice Assistant with Video (vision capabilities)
- Voice Assistant with SIP (Plivo, Telnyx, Twilio)
- Doodler (voice-to-sketch)
- Speaker Diarization
- Transcription
- HTTP Control (API-driven agent)
- WebSocket Example

---

## 3. LiveKit Agents Overview

LiveKit Agents is a Python framework for building real-time AI voice/video agents, built on top of the **LiveKit** open-source WebRTC SFU (Selective Forwarding Unit).

- **GitHub:** [livekit/agents](https://github.com/livekit/agents) (~9,900 stars) + [livekit/livekit](https://github.com/livekit/livekit) (~17,800 stars)
- **Created:** October 2023
- **Language:** Python (agents), Go (LiveKit server)
- **License:** Apache 2.0 (pure, no additional restrictions)

### Core Concepts

- **Agent** -- LLM-based application with defined instructions
- **AgentSession** -- container managing user interactions (VAD + STT + LLM + TTS pipeline)
- **AgentServer** -- main process coordinating job scheduling
- **entrypoint** -- starting point for interactive sessions (like a request handler)

### Plugin Ecosystem (~60+ plugins)

| Category | Plugins |
|----------|---------|
| **STT/ASR** | Deepgram, AssemblyAI, Azure, Google, Gladia, Clova, Soniox, Speechmatics, Sarvam, RTZR |
| **LLM** | OpenAI, Anthropic, Google, Groq, MistralAI, xAI, FireworksAI, NVIDIA, Ultravox, Baseten |
| **TTS** | Cartesia, ElevenLabs, Google, Azure, LMNT, Rime, Neuphonic, Speechify, FishAudio, Murf, Phonic, SmallestAI, CambAI, Spitch, Resemble |
| **Realtime** | OpenAI Realtime API |
| **Avatars** | Anam, Avatario, AvatarTalk, Bey, BitHuman, Hedra, Simli, Tavus, LiveAvatar, Keyframe |
| **Telephony** | Telnyx (SIP), LiveKit SIP stack |
| **Utilities** | Silero VAD, Turn Detector, BlingFire (NLP), NLTK, LangChain, Durable, BlockGuard |
| **Browser** | Browser plugin for web automation |

### Key Features

- **Flexible STT/LLM/TTS composition** -- mix and match any providers
- **Built-in job scheduling** -- dispatch APIs for routing users to agents
- **Extensive WebRTC client SDKs** -- all major platforms (Web, iOS, Android, Flutter, React Native, Unity)
- **SIP telephony** -- built into LiveKit server natively
- **Semantic turn detection** -- transformer-based model
- **MCP support** -- native Model Context Protocol for tool integration
- **Multi-agent handoff** -- first-class support for agent-to-agent transitions
- **Built-in test framework** -- test judges for agent behavior validation
- **LangChain plugin** -- direct integration for RAG and chain-based workflows

---

## 4. Head-to-Head Comparison

| Criteria | TEN Framework | LiveKit Agents + Custom |
|----------|--------------|------------------------|
| **Architecture** | Graph-based declarative (JSON config) | Code-first Python (imperative) |
| **WebRTC Transport** | Agora RTC (proprietary SDK, requires Agora account) | LiveKit SFU (fully open-source, self-hostable) |
| **STT Providers** | Deepgram, Azure (~2-3 built-in) | Deepgram, AssemblyAI, Azure, Google, Gladia, Speechmatics, ~10+ |
| **TTS Providers** | ElevenLabs (~1-2 built-in) | Cartesia, ElevenLabs, Google, Azure, LMNT, Rime, ~15+ |
| **LLM Providers** | OpenAI (extensible base class) | OpenAI, Anthropic, Google, Groq, Mistral, xAI, NVIDIA, ~10+ |
| **SIP/Telephony** | Plivo, Telnyx, Twilio (example-level) | Native SIP in LiveKit server + Telnyx plugin |
| **WhatsApp Calls** | Not built-in (would need custom extension) | Not built-in (would need custom integration) |
| **Embeddable Chatbot** | Not built-in (playground UI only) | Not built-in (WebRTC SDK can be embedded) |
| **Custom RAG Pipeline** | Write a custom LLM extension (Python/Go/TS) | function_tool decorator + LangChain plugin + any Python RAG lib |
| **Multi-agent Support** | Graph switching possible | First-class multi-agent handoff API |
| **Visual Graph Editor** | TMAN Designer (built-in) | None (code-only) |
| **Client SDKs** | Agora SDKs (Web, iOS, Android) | LiveKit SDKs (Web, iOS, Android, Flutter, React Native, Unity, Rust) |
| **Self-hosted WebRTC** | No -- requires Agora cloud service (paid) | Yes -- LiveKit server is fully open-source (Go) |
| **MCP Support** | Not documented | Native, one-line integration |
| **Testing Framework** | ASR/TTS guarder integration tests | Built-in test framework with judge models |
| **Docker Support** | First-class (docker-compose, per-example Dockerfiles) | Standard Python packaging, deploy anywhere |
| **Multi-tenancy** | Not built-in (single agent per container) | Room-based isolation via LiveKit server; dispatch API for routing |
| **Scalability** | Agora handles media scaling; agent server is single-instance | LiveKit server scales horizontally; agents auto-dispatch across workers |
| **Extension Languages** | Python, Go, TypeScript, C++ | Python only (agents); Go (server) |
| **Community Size** | ~10K stars, smaller community, strong Chinese presence | ~10K stars (agents) + ~18K stars (server), large global community |
| **Documentation** | Portal site + examples, moderate | Extensive docs site + MCP server + Agent Skill for AI coding |
| **Maturity** | ~2 years old, active development, fewer production references | ~2.5 years old, widely used in production, backed by LiveKit Inc. |
| **License** | Apache 2.0 **with additional restrictions** (see below) | Apache 2.0 (pure, no restrictions) |
| **Vendor Lock-in** | **High** -- Agora RTC is the only WebRTC transport | **Low** -- fully self-hostable, open-source SFU |

---

## 5. Critical License Analysis

### TEN Framework License (CRITICAL CONCERN)

The TEN Framework uses Apache 2.0 **with additional conditions** imposed by Agora:

> **Condition 1:** You may not (i) host the TEN Framework or the Derivative Works on any End User devices, including but not limited to any mobile terminal devices or (ii) **Deploy the TEN Framework in a way that competes with Agora's offerings** and/or that allows others to compete with Agora's offerings, including without limitation enabling any third party to develop or deploy Applications.

> **Condition 2:** You may Deploy the TEN Framework solely to create and enable deployment of your Application(s) **solely for your benefit and the benefit of your direct End Users.**

> **Condition 3:** Derivative Works of the TEN Framework remain subject to this Open Source License.

**Impact for Raven:**
- Condition 1 is vague and potentially problematic. If Raven is a platform that enables customers to build/deploy AI voice applications, this could be interpreted as "enabling third parties to develop or deploy Applications" -- which is explicitly prohibited.
- Condition 2 limits deployment to "your benefit and direct end users" -- a multi-tenant SaaS platform serving multiple organizations may not qualify.
- The "competes with Agora's offerings" clause is subjective and creates legal uncertainty.

### LiveKit License

Pure Apache 2.0. No additional restrictions. The entire stack (server + agents + SDKs) is fully open source with a permissive license. Full commercial use is permitted without restriction.

---

## 6. Custom RAG Pipeline Support

### TEN Framework

- You would write a **custom LLM extension** in Python (or Go/TypeScript) that inherits from `LLMBaseExtension`
- The extension receives text from the STT node, performs RAG lookup, augments the prompt, and forwards to the actual LLM
- Alternatively, you can modify the `main_cascade_python` extension to inject RAG context
- No built-in RAG support or vector DB integrations
- No LangChain integration

### LiveKit Agents

- **function_tool decorator** -- define tools that the LLM can call (e.g., knowledge base lookup)
- **LangChain plugin** (`livekit-plugins-langchain`) -- direct integration with LangChain's RAG chains, retrievers, and vector stores
- Standard Python -- use any RAG library (LlamaIndex, Haystack, custom) directly in your agent code
- The imperative code-first approach makes RAG integration natural and straightforward

**Winner for RAG:** LiveKit Agents -- significantly easier to integrate custom RAG pipelines. The code-first approach means you write Python functions that are called as tools. No need to create a formal extension package with manifest files.

---

## 7. Can TEN Handle the Full Raven Stack?

| Raven Mode | TEN Framework | LiveKit Agents + Custom |
|-----------|--------------|------------------------|
| **Embeddable Chatbot** | No. No web component / embeddable widget. Would need to build from scratch. | Partial. LiveKit Web SDK can be embedded. Chat-only mode would still need custom work. |
| **Voice Agent (STT+RAG+TTS)** | Yes. Graph-based pipeline with custom RAG extension. | Yes. AgentSession with function tools + RAG. |
| **WebRTC Voice Calls** | Yes, via Agora RTC (proprietary, paid). | Yes, via LiveKit SFU (open-source, self-hostable). |
| **WhatsApp Voice Calls** | No built-in support. | No built-in support. |

**Neither framework provides an out-of-the-box embeddable chatbot widget or WhatsApp integration.** Both require custom development for these modes.

---

## 8. Multi-Tenant Deployment

### TEN Framework
- No built-in multi-tenancy concepts
- Agent runs as a single instance per container
- Agora channels provide session isolation but not tenant isolation
- Would need to build tenant routing, configuration management, and isolation yourself
- Scaling relies on Agora's cloud infrastructure for media; agent containers need orchestration

### LiveKit Agents
- **Room-based isolation** -- each conversation is a LiveKit room with access control
- **Dispatch APIs** -- built-in routing of users to agent workers
- **Agent workers** can handle multiple sessions concurrently
- LiveKit server supports **multi-region deployment** and horizontal scaling
- Token-based authentication with per-room permissions enables multi-tenant patterns
- Well-documented Kubernetes deployment guides

**Winner for Multi-Tenancy:** LiveKit Agents -- the room + dispatch + token model maps naturally to multi-tenant SaaS.

---

## 9. Production Readiness

### TEN Framework
- ~2 years old (created June 2024)
- Active development with frequent commits
- Few public production case studies outside Agora's own demos
- Strong demo/example quality but limited battle-tested production documentation
- Agora (the company) backs it, but as a framework play, not their core business
- Community skews heavily Chinese (WeChat group, Chinese docs)

### LiveKit Agents
- ~2.5 years old (created October 2023)
- LiveKit server itself is mature (~5 years, 17K+ stars)
- Widely used in production by companies of various sizes
- LiveKit Inc. is a funded company with the SFU as their core business
- Extensive documentation, tutorials, and production guides
- Global community with active Discord, GitHub discussions
- Built-in test framework with judge models for CI/CD
- LiveKit Cloud available as managed service, with self-hosted as first-class option

**Winner for Production Readiness:** LiveKit Agents -- more mature ecosystem, more production deployments, better documentation, stronger company backing.

---

## 10. Recommendation for Raven

### Summary Verdict: **Stay with LiveKit Agents + Custom RAG Pipeline**

The TEN Framework is an impressive project for rapid prototyping of voice agents, especially if you are already an Agora customer. However, it has several **dealbreaker issues** for Raven:

#### Dealbreakers

1. **License restrictions** -- The "no competing with Agora" and "no enabling third parties" clauses are legally risky for a multi-tenant knowledge-base platform. Raven enables customers to deploy AI-powered chatbots and voice agents, which could be interpreted as "enabling third parties to develop or deploy Applications."

2. **Agora vendor lock-in** -- TEN's WebRTC transport is exclusively Agora RTC, a proprietary paid service. LiveKit is a fully open-source, self-hostable WebRTC SFU. This is a fundamental architectural difference -- with TEN, you cannot self-host the real-time transport layer.

3. **No multi-tenancy primitives** -- LiveKit's room + dispatch + token model naturally supports multi-tenant SaaS. TEN has no equivalent.

4. **Weaker RAG integration story** -- LiveKit Agents' code-first Python approach + LangChain plugin makes custom RAG trivial. TEN requires building a formal extension package.

#### Where TEN Wins (but not enough)

- **Visual graph editor** (TMAN Designer) is great for prototyping and non-developer configuration
- **Declarative graph composition** is elegant for simple, fixed pipelines
- **Multi-language extensions** (Go, TypeScript, C++) provide flexibility LiveKit lacks (Python-only agents)
- **Built-in SIP examples** (Plivo, Telnyx, Twilio) are more turnkey than LiveKit's SIP documentation

#### Recommended Architecture for Raven

```
+------------------+     +-------------------+     +------------------+
|  Embeddable      |     |  Voice Agent      |     |  WebRTC/WhatsApp |
|  Chatbot         |     |  (STT+RAG+TTS)    |     |  Voice Calls     |
|  (Web Component) |     |                   |     |                  |
+--------+---------+     +--------+----------+     +--------+---------+
         |                        |                          |
         v                        v                          v
+--------+---------+     +--------+----------+     +--------+---------+
|  Custom API      |     |  LiveKit Agents   |     |  LiveKit Server  |
|  (REST/WebSocket)|     |  (Python)         |     |  (WebRTC SFU)    |
+--------+---------+     +--------+----------+     +--------+---------+
         |                        |                          |
         +----------+-------------+----------+---------------+
                    |                        |
                    v                        v
         +---------+----------+    +---------+----------+
         |  Custom RAG        |    |  LiveKit SIP       |
         |  Pipeline          |    |  (Phone/WhatsApp)  |
         |  (Python)          |    |                    |
         +--------------------+    +--------------------+
```

**Key decisions:**
- **LiveKit Agents** for voice agent mode and WebRTC calls (single framework for both)
- **LiveKit Server** (self-hosted) for WebRTC media transport (no vendor lock-in)
- **LiveKit SIP** for phone/WhatsApp call ingress
- **Custom Python RAG pipeline** integrated via function_tools or LangChain plugin
- **Custom web component** for embeddable chatbot (this is custom work regardless of framework choice)
- **LiveKit room + token model** for multi-tenant isolation

---

## Appendix A: Extension/Plugin Ecosystem Comparison

### LiveKit Agents Plugins (~63 plugins)

**STT:** AssemblyAI, Azure, Clova, Deepgram, Gladia, Google, RTZR, Sarvam, Soniox, Speechmatics, Spitch
**LLM:** Anthropic, AWS (Bedrock), Azure, Baseten, FireworksAI, Google, Groq, MistralAI, NVIDIA, OpenAI, SimpliSmart, Ultravox, xAI
**TTS:** Azure, CambAI, Cartesia, ElevenLabs, FishAudio, Google, LMNT, MiniMax, Murf, Neuphonic, Phonic, Resemble, Rime, Sarvam, SmallestAI, Speechify, Spitch
**Avatars:** Anam, Avatario, AvatarTalk, Bey, BitHuman, Hedra, Keyframe, LiveAvatar, Simli, Tavus, TruGen, UpliftAI
**Utilities:** BlingFire, BlockGuard, Browser, Durable, LangChain, NLTK, Silero VAD, Turn Detector
**Telephony:** Telnyx

### TEN Agent Extensions (~60+ extensions)

**STT/ASR:** Deepgram, Azure (via Agora ASR)
**LLM:** OpenAI
**TTS:** ElevenLabs
**Transport:** Agora RTC, WebSocket, RTM
**SIP:** Plivo, Telnyx, Twilio (example-level integrations)
**Memory:** EverMemOS, PowerMem, memU, Memos
**Specialized:** Doodler, Speaker Diarization, Live2D, Video/Vision
**Utilities:** HTTP Control, Message Collector, Transcription

*Note: TEN's extension count of "60+" includes many internal/framework extensions. The user-facing provider integrations are fewer than LiveKit's.*

---

## Appendix B: Source Links

| Resource | URL |
|----------|-----|
| TEN Framework (core) | https://github.com/TEN-framework/ten_framework |
| TEN Agent (examples + extensions) | https://github.com/TEN-framework/TEN-Agent |
| TEN Framework License | https://github.com/TEN-framework/ten_framework/blob/main/LICENSE |
| TEN Portal (docs) | https://github.com/TEN-framework/portal |
| LiveKit Agents | https://github.com/livekit/agents |
| LiveKit Server | https://github.com/livekit/livekit |
| LiveKit Docs | https://docs.livekit.io/ |
| LiveKit SIP | https://docs.livekit.io/sip/ |
