---
title: "Try the Voice Agent"
---

# Try the Voice Agent

Raven ships a real-time voice agent that joins a [LiveKit](https://livekit.io)
room and talks to a user from the browser — no special hardware, no native
client. This page walks through bringing it up against your own
knowledge base on a full Compose stack.

::: info Prefer the deeper guide?
This is the get-started path. For sizing, retention tuning, and production
deployment notes, see the [Voice guide](/guides/voice).
:::

## Prerequisites

The voice agent depends on two extra services that are **only present in
the full Compose file**:

| Service          | Image                            | Purpose                                |
|------------------|----------------------------------|----------------------------------------|
| `livekit-server` | `livekit/livekit-server:latest`  | WebRTC SFU; brokers audio between participants |
| `python-agent`   | built from `./ai-worker`         | LiveKit Agents worker that joins rooms and runs the AI loop |

Both are wired up in [`docker-compose.yml`][compose] and **excluded from**
[`docker-compose.edge.yml`][compose-edge] — the edge variant is too small
to host real-time media. Run the voice tutorial on the full stack:

```bash
docker compose up -d livekit-server python-agent go-api postgres valkey
```

You also need:

- A workspace with at least one knowledge base (see
  [First Knowledge Base](/get-started/first-knowledge-base)).
- A microphone-capable browser (any modern Chromium or Firefox build).
- The following environment variables set in your root `.env` before
  bringing the stack up:

```env
LIVEKIT_API_KEY=devkey
LIVEKIT_API_SECRET=devsecret
```

The agent container reads them as `RAVEN_LIVEKIT_API_KEY` and
`RAVEN_LIVEKIT_API_SECRET`; the Go API reads them as the `livekit.api_key`
and `livekit.api_secret` config keys. Defaults are deliberately the
LiveKit `devkey`/`devsecret` pair — **rotate before exposing the stack to
the internet**. See [Configuration](/reference/configuration) for the
full key list.

## How it works

```
┌──────────┐   1 join     ┌────────────────┐   2 join    ┌─────────────┐
│ Browser  │ ───────────► │ livekit-server │ ◄────────── │ python-agent│
│ (Vue UI) │ ◄ audio ────►│   (SFU :7880)  │◄─ audio ────│ (LiveKit    │
└─────┬────┘              └────────────────┘             │  Agents)    │
      │                                                  └──────┬──────┘
      │ 3 create session + fetch token                          │
      ▼                                                         │
┌──────────┐                                                    │
│ go-api   │                                                    │
│ (Gin)    │                                                    │
└─────┬────┘                                                    │
      │ 4 RAG retrieval (Phase 2, not yet wired in the agent)   │
      ▼                                                         │
┌──────────┐                                                    │
│ ai-worker│ ◄──────────────────────────────────────────────────┘
│  (gRPC)  │
└──────────┘
```

1. The browser asks the Go API for a voice session and gets back a
   LiveKit access token.
2. The browser connects to `livekit-server` on `ws://…:7880` using
   [`livekit-client`][livekit-client] and joins the room named in the
   session row.
3. `python-agent` is a long-running [LiveKit Agents](https://docs.livekit.io/agents/)
   worker that auto-subscribes to new rooms. Its entrypoint is
   [`ai-worker/raven_worker/agent.py`][agent-py]:

   ```python
   # ai-worker/raven_worker/agent.py
   return WorkerOptions(
       entrypoint_fnc=_entrypoint,
       ws_url=settings.livekit_url,
       api_key=settings.livekit_api_key,
       api_secret=settings.livekit_api_secret,
       auto_subscribe=AutoSubscribe.AUDIO_ONLY,
   )
   ```

4. When a room is created, `_entrypoint` joins, builds an `AgentSession`,
   and (once wired — see below) hands user audio off to STT → LLM with
   RAG retrieval → TTS, sending the synthesised reply back into the room.

## Start a voice session from the dashboard

The frontend already ships a complete UI for this. Routes are registered
in `frontend/src/router/index.ts`:

| Path                              | Page                       |
|-----------------------------------|----------------------------|
| `/orgs/:orgId/voice`              | `VoiceSessionListPage.vue` |
| `/orgs/:orgId/voice/:sessionId`   | `VoiceSessionDetailPage.vue` |

1. Sign in and switch to your org.
2. Navigate to **Voice Sessions** (or `/orgs/<your-org-id>/voice`
   directly).
3. Click **New Session**. The
   [`CreateSessionModal`][create-modal] lets you supply a room name
   *or* leave it blank — the service layer auto-generates one of the form
   `voice-<orgShort>-<uuidShort>` (see `generateRoomName` in
   `internal/service/voice.go`).
4. You'll be redirected to the session detail page. Click **Start
   Session** (transitions state from `created` → `active`) and then
   **Join LiveKit Room** — the page calls the token endpoint and opens
   `${url}?token=<jwt>` in a new tab.
5. Speak. The `python-agent` worker, which is already listening on the
   LiveKit server, will join the room as a second participant.
6. Click **End Session** to transition state to `ended`. This also
   deletes the LiveKit room server-side.

## Start a voice session from the API

Every dashboard action is a thin wrapper over a REST call. The four you
care about are defined in [`internal/handler/voice.go`][voice-handler]:

```bash
# 1. Create the session. Returns a row with state="created" and an
#    auto-generated livekit_room.
curl -X POST "https://raven.example.com/api/v1/orgs/$ORG_ID/voice-sessions" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}'
```

Response (truncated):

```json
{
  "id": "f1d3b7c2-…",
  "org_id": "…",
  "livekit_room": "voice-ab12-cd34ef56",
  "state": "created",
  "created_at": "2026-05-12T10:14:22Z",
  "updated_at": "2026-05-12T10:14:22Z"
}
```

```bash
# 2. Mint a LiveKit join token. URL is the same WS endpoint the agent
#    uses; clients connect with livekit-client.
curl -X POST \
  "https://raven.example.com/api/v1/orgs/$ORG_ID/voice-sessions/$SESSION_ID/token" \
  -H "Authorization: Bearer $TOKEN"
```

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9…",
  "url":   "ws://livekit-server:7880"
}
```

```bash
# 3. Transition to active when the participant joins.
curl -X PATCH \
  "https://raven.example.com/api/v1/orgs/$ORG_ID/voice-sessions/$SESSION_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"state":"active"}'

# 4. End the session — derives call_duration_seconds and removes the room.
curl -X PATCH \
  "https://raven.example.com/api/v1/orgs/$ORG_ID/voice-sessions/$SESSION_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"state":"ended"}'
```

The token returned at step 2 is a standard LiveKit JWT scoped to a single
room. Hand it to `livekit-client` in the browser:

```ts
import { Room } from 'livekit-client'

const { token, url } = await generateVoiceToken(orgId, sessionId)
const room = new Room()
await room.connect(url, token)
```

## Configure the LLM and STT/TTS providers

The python-agent is wired up end-to-end at the **transport layer** today.
The STT, LLM, and TTS plugins inside `_entrypoint` are tracked as separate
work items — the source explicitly calls this out:

```python
# ai-worker/raven_worker/agent.py — inside _entrypoint
session = AgentSession()

# TODO (#59): Add STT plugin (Deepgram or faster-whisper)
# TODO (#60): Add TTS plugin (Cartesia or Piper)
# TODO: Connect to RAG pipeline via gRPC for knowledge retrieval
```

What this means in practice:

- Audio flows between browser and agent over LiveKit today — you can
  observe the agent joining the room and the
  `agent_session_started` structlog event firing.
- Transcription, LLM responses, and synthesised speech land in a
  follow-up issue. Until then, the conversation is one-sided.

The provider plumbing is already declared at the **Go side** in
`internal/config/config.go` for the parallel HTTP-mode endpoints
(`/v1/tts/*`, plus a future direct STT call). When the agent wires its
plugins it will read the same settings:

| Concern | Field        | Allowed values             | Default        |
|---------|--------------|----------------------------|----------------|
| TTS     | `tts.provider`         | `cartesia`, `piper`        | `cartesia`     |
| TTS     | `tts.cartesia_model`   | Cartesia Sonic model id    | `sonic-2`      |
| TTS     | `tts.piper_voice`      | Piper voice file           | `en_US-amy-medium` |
| STT     | `stt.provider`         | `deepgram`, `whisper`      | `""` (unset)   |
| STT     | `stt.whisper_model`    | Whisper model name         | (unset)        |
| LiveKit | `livekit.ws_url`       | LiveKit WSS endpoint       | `ws://localhost:7880` |
| LiveKit | `livekit.api_url`      | LiveKit HTTPS endpoint     | `http://localhost:7880` |

Set these via Viper env vars (`RAVEN_TTS_PROVIDER`, `RAVEN_STT_PROVIDER`,
etc.) — see [Configuration](/reference/configuration) for the env-var
mapping rules.

## Voice limits

The service enforces two limits in `internal/service/voice.go`:

- **Concurrent sessions per org** — resolved from the subscription tier
  via `QuotaChecker.GetConcurrentVoiceLimit`. Free tier defaults to **1
  active session**; exceeding it returns `429 Too Many Requests`. The
  check uses a PostgreSQL advisory lock keyed by org id so concurrent
  creates can't race past the cap.
- **Monthly voice-minute quota** — `QuotaChecker.CheckVoiceMinuteQuota`
  runs before each `CreateSession` and returns `402 Payment Required`
  when the org is over plan.

Per-turn state lives in `voice_turns` (transcript, `started_at`,
`ended_at`, `speaker`). Session-level state lives in `voice_sessions`,
with `call_duration_seconds` derived as a `GENERATED ALWAYS` column from
`ended_at − started_at`. The schema lives in the migration introduced
under [Issue #61][issue-61] and rolled up for billing by
[Issue #69][issue-69]'s `voice_usage_summaries` table.

## Storage and retention

By default, Raven stores transcripts in PostgreSQL (`voice_turns`) and
short-form audio clips in **SeaweedFS** alongside attachments. Retention
defaults to **30 days** for voice audio, after which clips are purged and
only the transcript remains; the privacy posture is documented in
[`PRIVACY.md`][privacy], which calls out voice audio as a distinct
processing category requiring consent for any retention beyond the
default.

If you need longer retention for compliance reasons, configure it
explicitly per workspace — see the [Voice guide](/guides/voice) once it
covers the retention knob.

## Demo on the public docs site

A hosted demo against the Raven docs workspace is **coming soon**. Until
that lands, the closest you can get without standing up the full stack
is the embedded chat widget on the
[Installation page](/get-started/installation) — text-only, but it talks
to the same RAG pipeline the voice agent will use once STT/TTS land.

## Troubleshooting

**The agent doesn't join the room.** Check
`docker compose logs python-agent`. The most common errors come from
[`agent.py`][agent-py] itself:

- `livekit_credentials_missing` → set `RAVEN_LIVEKIT_API_KEY` /
  `RAVEN_LIVEKIT_API_SECRET` and restart `python-agent`.
- `livekit_agents_not_installed` → the container build is broken; run
  `docker compose build python-agent`.

**Browser connects but no audio flows.** LiveKit needs UDP ports
`50000–60000` open. The compose file publishes them, but if you're
behind a corporate firewall or a NAT that blocks UDP, switch to LiveKit
Cloud (set `RAVEN_LIVEKIT_URL=wss://<your-project>.livekit.cloud` and
the cloud project's API key/secret) or front the SFU with a TURN server.

**Token returns 400 "cannot generate token for ended session".** Self-
explanatory — you patched the session to `ended`. Create a new one.

**Microphone permission silently denied.** Browsers require HTTPS for
`getUserMedia` outside `localhost`. If you're testing on a LAN IP, either
expose the frontend through Traefik with a real cert (see
[Traefik & TLS](/guides/self-hosting/traefik-and-tls)) or run on
`http://localhost:5173` directly.

**Self-hosted vs LiveKit Cloud.** Self-hosting (the default) keeps audio
on your network and uses the SFU container in compose. LiveKit Cloud
trades that for a managed media plane; swap `RAVEN_LIVEKIT_URL` /
`RAVEN_LIVEKIT_API_KEY` / `RAVEN_LIVEKIT_API_SECRET` to your cloud
project values and leave everything else unchanged.

## What's next

- Read [Voice](/guides/voice) for production sizing and deployment knobs.
- Read [AI Worker Design](/concepts/ai-worker-design) for the gRPC
  service the agent will call into for RAG retrieval.
- Read [Configuration](/reference/configuration) for the full LiveKit /
  STT / TTS env-var matrix.

[compose]: https://github.com/ravencloak-org/Raven/blob/main/docker-compose.yml
[compose-edge]: https://github.com/ravencloak-org/Raven/blob/main/docker-compose.edge.yml
[agent-py]: https://github.com/ravencloak-org/Raven/blob/main/ai-worker/raven_worker/agent.py
[voice-handler]: https://github.com/ravencloak-org/Raven/blob/main/internal/handler/voice.go
[create-modal]: https://github.com/ravencloak-org/Raven/blob/main/frontend/src/components/voice/CreateSessionModal.vue
[livekit-client]: https://github.com/livekit/client-sdk-js
[privacy]: https://github.com/ravencloak-org/Raven/blob/main/PRIVACY.md
[issue-61]: https://github.com/ravencloak-org/Raven/issues/61
[issue-69]: https://github.com/ravencloak-org/Raven/issues/69
