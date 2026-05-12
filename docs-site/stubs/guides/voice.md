---
title: "Voice"
---

# Voice

This guide is the operator-and-developer surface for Raven's voice agent: how
to deploy LiveKit, how to wire speech-to-text (STT) and text-to-speech (TTS)
providers, how voice sessions are persisted, what gets billed, and how to keep
recordings inside the retention promise.

If you just want a five-minute "is my mic plugged in" demo, start at
[/get-started/try-the-voice-agent](/get-started/try-the-voice-agent) and
come back here once a turn round-trips end-to-end.

## Architecture recap

The voice path crosses three processes. A browser (or mobile client) joins a
**LiveKit room** over WebRTC; a separate Python process called **`python-agent`**
joins the same room as a server-side participant, ferries audio between
LiveKit and the configured STT/TTS plugins, and calls into the gRPC `RAGServicer`
to answer using the workspace's knowledge bases. The Go API tracks the session
lifecycle, mints LiveKit access tokens, and persists turns and usage rows in
Postgres.

`python-agent` is **not** the same binary as `python-worker` — the gRPC AI worker
runs `python -m raven_worker` (entrypoint `raven_worker/__main__.py`), while
the voice agent runs `python -m raven_worker.agent`
([`ai-worker/raven_worker/agent.py`](https://github.com/ravencloak-org/Raven/blob/main/ai-worker/raven_worker/agent.py))
as a standalone long-lived worker that calls `livekit.agents.cli.run_app`. They
ship in the same Python package and the same Docker image, but they are
deployed and scaled as distinct services in `docker-compose.yml`.

For the full module map see
[/concepts/ai-worker-design](/concepts/ai-worker-design#voice-agent).

## LiveKit deployment options

### Self-hosted (default in Compose)

The bundled `docker-compose.yml` ships a `livekit-server` service running the
official `livekit/livekit-server:latest` image, with its config at
[`deploy/livekit/livekit.yaml`](https://github.com/ravencloak-org/Raven/blob/main/deploy/livekit/livekit.yaml).
Key ports:

| Port           | Protocol | Purpose                                         |
|----------------|----------|-------------------------------------------------|
| `7880`         | HTTP/WS  | Signalling + RoomService HTTP API               |
| `7882`         | TCP      | WebRTC TCP fallback for restricted networks     |
| `50000–60000`  | UDP      | WebRTC media (`rtc.port_range_start/end`)       |

`use_external_ip: true` is set so LiveKit advertises the host's public IP in
its ICE candidates. The two Google STUN servers in `livekit.yaml` are enough
for the open internet — add a TURN server only when you have clients behind
symmetric NATs that block UDP entirely.

Behind Traefik, the included labels publish LiveKit's HTTP/WS API on
`/livekit/*` with TLS via the configured cert resolver. UDP media ports must
be exposed on the host (not routed through Traefik).

### LiveKit Cloud

To use the managed service instead, stop the `livekit-server` container,
remove the `depends_on: livekit-server` entry from `python-agent`, and point
both the agent and the Go API at your cloud URL:

```bash
RAVEN_LIVEKIT_API_URL=https://<project>.livekit.cloud
RAVEN_LIVEKIT_WS_URL=wss://<project>.livekit.cloud
RAVEN_LIVEKIT_API_KEY=<from console>
RAVEN_LIVEKIT_API_SECRET=<from console>
```

The same env vars are read by `internal/config/config.go` (`LiveKitConfig`)
and by the agent's `Settings` in `ai-worker/raven_worker/config.py`. No code
changes are required for the switch — only credentials.

## STT and TTS provider selection

Provider choice is configured Go-side and consumed by the agent process at
startup. The supported set, as wired today, is:

| Surface | Provider key | Where it lives                              | Env prefix              |
|---------|--------------|---------------------------------------------|-------------------------|
| STT     | `deepgram`   | Deepgram Nova cloud API                     | `RAVEN_STT_DEEPGRAM_*`  |
| STT     | `whisper`    | Self-hosted faster-whisper / Whisper.cpp    | `RAVEN_STT_WHISPER_*`   |
| TTS     | `cartesia`   | Cartesia Sonic cloud API                    | `RAVEN_TTS_CARTESIA_*`  |
| TTS     | `piper`      | Self-hosted Piper voices                    | `RAVEN_TTS_PIPER_*`     |

`STTConfig.Provider` selects the active backend; when left empty,
`internal/config/config.go` falls back to Deepgram if `RAVEN_STT_DEEPGRAM_API_KEY`
is set and to Whisper otherwise. `TTSConfig.Provider` accepts `cartesia` or
`piper`. The full bind list:

```text
RAVEN_STT_PROVIDER            deepgram | whisper (or empty for auto)
RAVEN_STT_DEEPGRAM_API_KEY    Deepgram key
RAVEN_STT_DEEPGRAM_MODEL      e.g. nova-2
RAVEN_STT_DEEPGRAM_BASE_URL   override for self-hosted Deepgram on-prem
RAVEN_STT_WHISPER_ENDPOINT    http://whisper:9000 (your faster-whisper server)
RAVEN_STT_WHISPER_MODEL       e.g. base | small | medium

RAVEN_TTS_PROVIDER            cartesia | piper
RAVEN_TTS_CARTESIA_API_KEY    Cartesia key
RAVEN_TTS_CARTESIA_VOICE_ID   Cartesia voice slug
RAVEN_TTS_CARTESIA_MODEL      e.g. sonic-english
RAVEN_TTS_CARTESIA_BASE_URL   override for staging endpoints
RAVEN_TTS_PIPER_ENDPOINT      http://piper:5000
RAVEN_TTS_PIPER_VOICE         default en_US-amy-medium
```

### Plugin attachment status

The agent process is currently a thin shell: `agent.py` constructs a
`livekit.agents.voice.AgentSession()` with `auto_subscribe=AUDIO_ONLY` and
attaches it to incoming rooms, but it does **not yet** instantiate STT or
TTS plugins. The provider configuration above is fully read and validated
Go-side and is ready for the agent to consume; wiring the actual
`livekit-agents` STT and TTS plugin classes is tracked as issues **#59**
(STT) and **#60** (TTS). Operators should treat this guide as the **target
contract** — the env vars listed here will not change once the plugins
land, but the agent will only start answering with audio after #59 and #60
are merged.

## Voice session lifecycle

Every voice call is anchored on a row in `voice_sessions` (see migration
[`00029_voice_sessions.sql`](https://github.com/ravencloak-org/Raven/blob/main/migrations/00029_voice_sessions.sql)).
The columns:

| Column                  | Type                  | Notes                                       |
|-------------------------|-----------------------|---------------------------------------------|
| `id`                    | `UUID`                | primary key                                 |
| `org_id`                | `UUID`                | tenant scope; enforced by RLS               |
| `user_id`               | `UUID?`               | mutually exclusive with `stranger_id`       |
| `stranger_id`           | `UUID?`               | for anonymous public widget callers         |
| `livekit_room`          | `VARCHAR(255)`        | LiveKit room name; auto-generated on create |
| `state`                 | `voice_session_state` | `created` → `active` → `ended`              |
| `started_at`            | `TIMESTAMPTZ?`        | set on first audio                          |
| `ended_at`              | `TIMESTAMPTZ?`        | set on hangup / timeout                     |
| `call_duration_seconds` | `INTEGER` (generated) | `ended_at - started_at`, materialised       |
| `created_at`            | `TIMESTAMPTZ`         |                                             |
| `updated_at`            | `TIMESTAMPTZ`         | maintained by trigger                       |

State machine (defined as the `voice_session_state` Postgres enum and mirrored
in `internal/model/voice.go`):

```text
created ──► active ──► ended
   │                      ▲
   └──────────────────────┘   (hangup before audio)
```

The Go service auto-generates `livekit_room` if the create request omits it.
A `chk_voice_session_actor` constraint enforces that a session is bound to
either a known user, an anonymous stranger, or neither — never both.

Endpoints (from `internal/handler/voice.go`, all under
`/v1/orgs/:org_id/voice-sessions`):

| Method | Path                                | Action                              |
|--------|-------------------------------------|-------------------------------------|
| `POST` | `/`                                 | Create session (`created`)          |
| `GET`  | `/`                                 | List sessions (paginated)           |
| `GET`  | `/:session_id`                      | Fetch a session                     |
| `PATCH`| `/:session_id`                      | Transition state (`active`/`ended`) |
| `POST` | `/:session_id/token`                | Mint a LiveKit JWT for a participant|
| `POST` | `/:session_id/turns`                | Append a transcribed turn           |
| `GET`  | `/:session_id/turns`                | List turns ordered by `started_at`  |

## Voice turns and transcripts

Per-turn transcripts live in `voice_turns` (same migration):

| Column       | Type            | Notes                                |
|--------------|-----------------|--------------------------------------|
| `id`         | `UUID`          |                                      |
| `session_id` | `UUID`          | FK → `voice_sessions(id)` `ON DELETE CASCADE` |
| `org_id`     | `UUID`          | denormalised for RLS efficiency      |
| `speaker`    | `voice_speaker` | `agent` or `user`                    |
| `transcript` | `TEXT`          | STT output for user; final assistant text for agent |
| `started_at` | `TIMESTAMPTZ`   | from the audio frame timeline        |
| `ended_at`   | `TIMESTAMPTZ?`  | end of utterance                     |

Turns are indexed by `(session_id, started_at)`. They share the
`tenant_isolation` row-level-security policy on `org_id`, so transcripts from
one workspace are never visible to another even with shared connection
pooling.

Transcripts inherit retention from the parent conversation (see
[Privacy and recording](#privacy-and-recording) below); deleting a session
cascades to its turns.

## Usage summaries

`voice_usage_summaries` (migration
[`00030_voice_usage.sql`](https://github.com/ravencloak-org/Raven/blob/main/migrations/00030_voice_usage.sql))
aggregates ended sessions into one row per `(org_id, period_start)` hour
window for billing dashboards and tenant-facing usage reports:

| Column                   | Type           | Notes                                  |
|--------------------------|----------------|----------------------------------------|
| `id`                     | `UUID`         |                                        |
| `org_id`                 | `UUID`         |                                        |
| `period_start`           | `TIMESTAMPTZ`  | hourly bucket, unique per org          |
| `total_sessions`         | `INTEGER`      | sessions ended in the window           |
| `total_duration_seconds` | `INTEGER`      | summed `call_duration_seconds`         |

Cost columns (STT minutes, LLM tokens, TTS characters) are **not** stored
today — operators back into them from provider invoices using
`total_duration_seconds` as the denominator. The unique constraint
`uq_voice_usage_org_period` makes the upsert from the aggregator idempotent.

## Audio storage

LiveKit forwards raw RTP to the agent in real time and does not persist
audio by itself. When you enable session recording (a LiveKit Egress job),
the resulting WebM/MP4 blob lands in **SeaweedFS**, the same object store
used for document attachments — see `SeaweedFSConfig` in
`internal/config/config.go`. There is no separate audio bucket; recordings
are scoped by the same per-org prefix scheme as uploads.

Default retention for voice audio is **30 days**, as documented in
[`PRIVACY.md` §9](https://github.com/ravencloak-org/Raven/blob/main/PRIVACY.md#9-retention).
Callers may opt in to extended retention (Art. 6(1)(a) consent under GDPR;
the matching DPDP basis applies for Indian deployments). To extend the
default for a whole workspace you currently need to schedule your own
SeaweedFS lifecycle job — there is no Raven-level setting for this yet.

## Costs

Voice billing is provider-driven and add-on to your regular LLM bill.
Rough per-minute order of magnitude (always check the provider's current
pricing page):

- **STT minutes** — Deepgram Nova is billed per audio minute streamed.
  Self-hosted faster-whisper has zero per-minute cost but pins one CPU core
  (or one GPU slot) per concurrent stream.
- **LLM tokens** — every user turn pulls retrieved chunks into the prompt
  via the standard RAG pipeline (`raven_worker/services/rag.py`), so cost
  scales with conversation length and `top_k`. The same model and BYOK key
  resolved for text chat applies — voice does not bypass `llm_provider_configs`.
- **TTS characters** — Cartesia Sonic is billed per character of synthesised
  speech. Piper is free at the cost of one CPU core per stream.

Because `voice_usage_summaries` does not yet aggregate token or character
counts, link your provider account to the same workspace and reconcile via
their dashboards.

## Limits

The current build has **no built-in concurrency cap** on voice sessions —
the limit is whichever runs out first: LiveKit room slots, agent CPU, or
your STT provider's concurrent-connection quota. Plan for one CPU core per
self-hosted STT/TTS stream and at least 1 vCPU per agent process.

The Go API rejects `POST /voice-sessions/:id/turns` once a session has
transitioned to `ended`, which gives you a natural per-session turn ceiling
via your client logic — there is no hard server-side `max_turns` enforced
today.

Maximum session length is governed by LiveKit's `room_empty_timeout`
(currently the LiveKit default; no override in
[`deploy/livekit/livekit.yaml`](https://github.com/ravencloak-org/Raven/blob/main/deploy/livekit/livekit.yaml)
right now). Add `room_empty_timeout: 300` to that file to hang up rooms
five minutes after the last participant leaves.

## Tuning

The voice agent re-uses the gRPC `QueryRAG` pipeline, so retrieval knobs are
the same ones documented in [/guides/retrieval](/guides/retrieval) —
`top_k`, the hybrid-search RRF blend, and the Cohere re-ranker toggle all
apply unchanged. Voice-specific guidance:

- **System prompt** — the voice agent does not maintain its own prompt
  file; whatever system prompt your tenant has configured for chat is
  applied. Once the STT/TTS plugins land, expect the agent to layer a short
  "speak in short sentences, never read URLs aloud" preamble on top of the
  tenant prompt inside `agent.py`. Treat that as the single place to edit
  voice-only persona.
- **Retrieval top-K** — voice answers should be short, so a `top_k` of
  **3–4** typically reads more natural than the chat default of 6–8.
- **`max_tokens` for spoken output** — `services/rag.py` caps streaming
  responses at `max_tokens=1024` (OpenAI) and `max_tokens=2048` (Anthropic);
  for voice you generally want shorter, so override the per-tenant model
  config rather than the worker default.

## Privacy and recording

Voice audio, transcripts, and embeddings are **personal data** under GDPR
Art. 4(1) and may incidentally capture special-category data under Art. 9
if the caller volunteers it — see `PRIVACY.md` §6. Operator responsibilities:

- **Notice on connect.** Inform the caller that the call is being processed
  by an AI agent and (where applicable) recorded — the widget should
  surface this before joining the LiveKit room.
- **Consent for extended retention.** The 30-day audio default is the only
  one we rely on Art. 6(1)(f) legitimate-interests for; anything longer is
  Art. 6(1)(a) **consent**.
- **Right to erasure.** `voice_sessions` and `voice_turns` cascade-delete
  cleanly on `org_id` / `user_id`. SeaweedFS objects must be deleted via the
  same DSAR pipeline as document attachments.

The public-facing privacy notice is still in `/trust/privacy` (held back).
Defer to the in-repo `PRIVACY.md` until it ships.

## Troubleshooting

**`401 Unauthorized` when creating a LiveKit room.**
The Go API and the agent must share the same `RAVEN_LIVEKIT_API_KEY` /
`RAVEN_LIVEKIT_API_SECRET` pair. Compose defaults to `devkey` / `devsecret`
— check both services are reading the same `.env` and that you didn't
override only one side.

**Browser connects, no audio in either direction.**
Almost always blocked UDP. Confirm `50000–60000/udp` is reachable from the
client to your host, then fall back to TCP via port `7882` and verify with
a different network (mobile hotspot). If you sit behind symmetric NAT,
deploy a TURN server (e.g. Coturn) and add it to `rtc.turn_servers` in
`livekit.yaml`.

**Browser asks for mic permission and gets denied.**
Browsers only prompt on **secure origins**. Local dev over `http://localhost`
works; anything else needs HTTPS via the bundled Traefik resolver.

**Agent doesn't respond at all.**
Check `docker compose logs python-agent`. Common failures:
`livekit_credentials_missing` (env not threaded), `livekit_agents_not_installed`
(the `livekit-agents>=1.0.0` dep didn't make it into the image), or the
agent attaching to the room but not subscribing because `auto_subscribe`
got changed away from `AUDIO_ONLY`.

**First turn takes 10+ seconds.**
If you're on self-hosted Whisper, model load happens on first request —
pre-warm by issuing a one-byte test transcription at container start, or
size your Whisper model down from `medium` to `small`/`base` on edge
hardware.

**`voice_session_state` check constraint violated.**
The state column is a Postgres enum with only three legal values
(`created`, `active`, `ended`). Callers updating state directly via SQL must
respect that — the service layer is the only sanctioned writer.

## Related reading

- [/get-started/try-the-voice-agent](/get-started/try-the-voice-agent) — five-minute hands-on demo
- [/concepts/ai-worker-design](/concepts/ai-worker-design) — where `agent.py` fits in the worker
- [/guides/retrieval](/guides/retrieval) — RAG knobs that apply to voice too
- [/guides/llm-providers](/guides/llm-providers) — BYOK setup for the per-turn LLM call
- [/reference/configuration](/reference/configuration) — exhaustive `RAVEN_*` env-var list
