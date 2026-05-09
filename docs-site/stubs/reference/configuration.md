---
title: "Configuration"
---

# Configuration

Raven is configured entirely via environment variables. There is no central
runtime config file — every binary reads its environment at startup, validates
required values, and fails fast if the set is incomplete.

Three runtimes consume those variables:

- **Go API and Asynq worker** — [Viper](https://github.com/spf13/viper) loads
  defaults, optional `config.yaml`, then env overrides. Defined in
  [`internal/config/config.go`](https://github.com/ravencloak-org/Raven/blob/main/internal/config/config.go).
  All keys are bound under the `RAVEN_` prefix (with a few legacy bindings such
  as `GOOGLE_CLIENT_ID`, `AWS_SES_*`, `TMDB_API_KEY`).
- **Python AI worker** — [pydantic-settings](https://docs.pydantic.dev/latest/concepts/pydantic_settings/)
  reads the same `RAVEN_` prefix. Defined in
  [`ai-worker/raven_worker/config.py`](https://github.com/ravencloak-org/Raven/blob/main/ai-worker/raven_worker/config.py)
  with `env_prefix="RAVEN_"`.
- **Frontend (Vue 3 + Vite)** — `import.meta.env.VITE_*` substitutions are
  baked in at build time. Defined per build in `frontend/.env.example`,
  `frontend/.env.production`, and the deploy config at
  `deploy/cloudflare-pages.json`.

## Precedence

For the Go binaries, Viper resolves each key in this order (later wins):

1. Built-in `SetDefault` value in `config.go`.
2. Optional `config.yaml` under `.` or `./config`. Missing file is not an error.
3. Process environment (the Compose stack mounts `.env` into containers and
   relies on Compose's variable substitution + `env_file` loading).
4. Explicit `BindEnv` mappings — both `RAVEN_*` and the legacy aliases noted
   above.

Frontend `VITE_*` values are read once at `vite build` and frozen into the
JS bundle; you cannot change them without a rebuild.

## Secrets handling

- All secrets are environment-only. The repo ships `.env.example` files;
  there is no committed `.env`.
- The Compose stack supports `dotenvx`-encrypted `.env` files: each service
  bind-mounts `./.env:/app/.env:ro` and receives `DOTENV_PRIVATE_KEY`. Never
  commit `.env.keys`.
- Required secrets fail loudly via Compose's `${VAR:?...}` syntax — see
  `RAVEN_ENCRYPTION_AES_KEY` and `DATABASE_URL` in `docker-compose.yml` and
  `docker-compose.edge.yml`.
- For production, push `.env` into a secrets manager (Vault, SSM Parameter
  Store, 1Password Connect) and inject at boot — see the hardening guide.

## Quick start

The minimum env vars to bring up the full Compose stack:

```bash
# postgres + valkey
POSTGRES_USER=raven
POSTGRES_PASSWORD=changeme
POSTGRES_DB=raven
DATABASE_URL=postgresql://raven:changeme@postgres:5432/raven?sslmode=disable
VALKEY_URL=redis://valkey:6379/0

# Raven API
RAVEN_DATABASE_URL=${DATABASE_URL}
RAVEN_VALKEY_URL=${VALKEY_URL}
RAVEN_GRPC_WORKER_ADDR=python-worker:50051
RAVEN_ENCRYPTION_AES_KEY=<base64 32-byte key>

# auth
SUPERTOKENS_API_KEY=supertokens-dev-key-replace-me

# at least one LLM provider for the AI worker
ANTHROPIC_API_KEY=sk-ant-...
```

Generate `RAVEN_ENCRYPTION_AES_KEY` with `openssl rand -base64 32`. Without it
the API container refuses to start (Compose enforces this with `:?required`).

## Go API — `cmd/api`

The API binary loads config via `config.Load()` in `internal/config/config.go`.

### Server

| Env var | Type | Default | Required | Description |
|---|---|---|---|---|
| `RAVEN_SERVER_PORT` | int | `8081` | no | HTTP listener port (`Server.Port`). |
| `RAVEN_SERVER_MODE` | string | `debug` | no | Gin mode (`debug` or `release`). `Server.Mode`. |
| `RAVEN_HTTP_READ_HEADER_TIMEOUT` | duration | `5s` | no | `http.Server.ReadHeaderTimeout`. |
| `RAVEN_HTTP_READ_TIMEOUT` | duration | `30s` | no | Full request read timeout. |
| `RAVEN_HTTP_WRITE_TIMEOUT` | duration | `60s` | no | Full response write timeout. |
| `RAVEN_HTTP_IDLE_TIMEOUT` | duration | `120s` | no | Keep-alive idle timeout. |
| `RAVEN_AI_WORKER_TIMEOUT` | duration | `5s` | no | Per-call gRPC deadline against the Python worker. |
| `RAVEN_AI_WORKER_BREAKER_THRESHOLD` | uint | `5` | no | Consecutive AI-worker failures that open the circuit breaker. |
| `RAVEN_AI_WORKER_BREAKER_COOLDOWN` | duration | `30s` | no | Cooldown before the breaker probes again. |
| `RAVEN_SINGLE_USER` | bool | `false` | no | Raven Local / desktop mode. Injects a synthetic `user_id=local`, `org_id=local` session and disables `/auth/*`. |
| `RAVEN_CORS_ALLOWED_ORIGINS` | csv | `http://localhost:5173,https://raven-frontend.pages.dev` | no | Comma-separated origin allowlist. |

Durations accept Go `time.Duration` syntax (`5s`, `500ms`, `2m`).

### Database & cache

| Env var | Type | Default | Required | Description |
|---|---|---|---|---|
| `RAVEN_DATABASE_URL` | URL | — | yes | PostgreSQL DSN (with `pgvector`). Validated as non-empty in `Load()`. |
| `RAVEN_VALKEY_URL` | URL | — | no | Valkey/Redis DSN, used by Asynq + rate-limit buckets + cache. |

`config.Load()` returns an error if `RAVEN_DATABASE_URL` is unset:
`database.url is required`.

### gRPC client (AI worker)

| Env var | Type | Default | Required | Description |
|---|---|---|---|---|
| `RAVEN_GRPC_WORKER_ADDR` | host:port | `localhost:50051` | no (yes in Compose) | Address of the Python AI worker gRPC server. No scheme prefix. |

### Auth — SuperTokens

| Env var | Type | Default | Required | Description |
|---|---|---|---|---|
| `RAVEN_SUPERTOKENS_CONNECTION_URI` | URL | `http://supertokens:3567` | no | SuperTokens core URI. |
| `RAVEN_SUPERTOKENS_API_KEY` | string | _(empty)_ | yes (prod) | SuperTokens core API key. Never commit. |
| `RAVEN_SUPERTOKENS_API_DOMAIN` | URL | `http://localhost:8081` | no | Public API domain that frontend calls. |
| `RAVEN_SUPERTOKENS_WEBSITE_DOMAIN` | URL | `http://localhost:5173` | no | Public frontend domain (used for cookie scoping + CORS). |
| `GOOGLE_CLIENT_ID` | string | _(empty)_ | no | Google OAuth client ID for SuperTokens social login. |
| `GOOGLE_CLIENT_SECRET` | string | _(empty)_ | no | Google OAuth client secret. Never commit. |

### Encryption

| Env var | Type | Default | Required | Description |
|---|---|---|---|---|
| `RAVEN_ENCRYPTION_AES_KEY` | base64(32 bytes) | — | yes | AES-256-GCM key for at-rest encryption of tenant LLM API keys (BYOK). Never commit. |

Compose enforces this with `${RAVEN_ENCRYPTION_AES_KEY:?... is required}`.

### Rate limits

All values are requests-per-minute unless noted. Tier limits override
`default_*` for orgs on that tier.

| Env var | Type | Default | Required | Description |
|---|---|---|---|---|
| `RAVEN_RATELIMIT_DEFAULT_USER_LIMIT` | int | `1000` | no | Per-user fallback (must be > 0). |
| `RAVEN_RATELIMIT_DEFAULT_ORG_LIMIT` | int | `10000` | no | Per-org fallback (must be > 0). |
| `RAVEN_RATELIMIT_FREE_GENERAL_RPM` | int | `60` | no | Free-tier general endpoints. |
| `RAVEN_RATELIMIT_FREE_COMPLETION_RPM` | int | `10` | no | Free-tier LLM completions. |
| `RAVEN_RATELIMIT_PRO_GENERAL_RPM` | int | `600` | no | Pro tier general. |
| `RAVEN_RATELIMIT_PRO_COMPLETION_RPM` | int | `120` | no | Pro tier completions. |
| `RAVEN_RATELIMIT_ENTERPRISE_GENERAL_RPM` | int | `6000` | no | Enterprise general. |
| `RAVEN_RATELIMIT_ENTERPRISE_COMPLETION_RPM` | int | `-1` | no | Enterprise completions; `-1` means unlimited. |
| `RAVEN_RATELIMIT_WIDGET_RPM` | int | `30` | no | Public widget endpoints. |
| `RAVEN_RATELIMIT_APIKEY_HASH_SECRET` | string | _(empty)_ | yes (prod) | HMAC-SHA-256 key for deriving Valkey bucket IDs from raw API keys. Must be stable across nodes; never commit. |

`Load()` returns an error if either `default_user_limit` or
`default_org_limit` is zero or negative.

### Asynq queue (used by the API to enqueue, by the worker to consume)

| Env var | Type | Default | Required | Description |
|---|---|---|---|---|
| `RAVEN_QUEUE_CONCURRENCY` | int | `10` | no | Worker concurrency. |
| `RAVEN_QUEUE_MAX_RETRY` | int | `5` | no | Max retries per task. |

### Storage — SeaweedFS

| Env var | Type | Default | Required | Description |
|---|---|---|---|---|
| `RAVEN_SEAWEEDFS_MASTER_URL` | URL | `http://seaweedfs-master:9333` | no | SeaweedFS master node URL. |

### Uploads

| Env var | Type | Default | Required | Description |
|---|---|---|---|---|
| `RAVEN_UPLOAD_MAX_SIZE_BYTES` | int64 | `52428800` (50 MB) | no | Max upload size in bytes. |
| `RAVEN_UPLOAD_ALLOWED_TYPES` | csv | _PDF, DOCX, PPTX, HTML, MD, TXT, CSV_ | no | MIME allowlist (Viper accepts a comma list). |

### LLM provider keys (BYOK + summary job)

These are not bound by Viper; they are read directly by the AI worker (and by
the dev-agent script). The Go API receives tenant-scoped keys via the
`/api/v1/byok` endpoints and stores them encrypted with
`RAVEN_ENCRYPTION_AES_KEY`.

| Env var | Type | Default | Required | Description |
|---|---|---|---|---|
| `ANTHROPIC_API_KEY` | string | — | yes (one provider) | Anthropic key. Never commit. |
| `OPENAI_API_KEY` | string | — | no | OpenAI key. Never commit. |
| `COHERE_API_KEY` | string | — | no | Cohere key (rerank + embeddings). Never commit. |

### Voice — LiveKit

| Env var | Type | Default | Required | Description |
|---|---|---|---|---|
| `RAVEN_LIVEKIT_API_URL` | URL | `http://localhost:7880` | no | LiveKit server HTTP(S) URL for RoomService API calls. |
| `RAVEN_LIVEKIT_WS_URL` | URL | `ws://localhost:7880` | no | WebSocket URL returned to clients (use `wss://` in production). |
| `RAVEN_LIVEKIT_API_KEY` | string | `devkey` | yes (prod) | LiveKit API key. Never commit. |
| `RAVEN_LIVEKIT_API_SECRET` | string | `devsecret` | yes (prod) | LiveKit API secret. Never commit. |

### Voice — TTS (text-to-speech)

| Env var | Type | Default | Required | Description |
|---|---|---|---|---|
| `RAVEN_TTS_PROVIDER` | enum | `cartesia` | no | `cartesia` or `piper`. |
| `RAVEN_TTS_CARTESIA_API_KEY` | string | _(empty)_ | yes (cartesia) | Cartesia Sonic API key. Never commit. |
| `RAVEN_TTS_CARTESIA_VOICE_ID` | string | _(empty)_ | no | Cartesia voice UUID. |
| `RAVEN_TTS_CARTESIA_MODEL` | string | `sonic-2` | no | Cartesia model id. |
| `RAVEN_TTS_CARTESIA_BASE_URL` | URL | _(empty)_ | no | Override Cartesia API base. |
| `RAVEN_TTS_PIPER_ENDPOINT` | URL | `http://localhost:5000` | no | Self-hosted Piper HTTP endpoint. |
| `RAVEN_TTS_PIPER_VOICE` | string | `en_US-amy-medium` | no | Piper voice file id. |

### Voice — STT (speech-to-text)

| Env var | Type | Default | Required | Description |
|---|---|---|---|---|
| `RAVEN_STT_PROVIDER` | enum | _(empty)_ | no | `deepgram`, `whisper`, or empty. Empty falls back to Deepgram if a key is set, otherwise Whisper. |
| `RAVEN_STT_DEEPGRAM_API_KEY` | string | _(empty)_ | yes (deepgram) | Deepgram key. Never commit. |
| `RAVEN_STT_DEEPGRAM_MODEL` | string | `nova-2` | no | Deepgram model id. |
| `RAVEN_STT_DEEPGRAM_BASE_URL` | URL | `https://api.deepgram.com` | no | Deepgram API base. |
| `RAVEN_STT_WHISPER_ENDPOINT` | URL | `http://localhost:8000` | no | Self-hosted Whisper HTTP endpoint. |
| `RAVEN_STT_WHISPER_MODEL` | string | `large-v3` | no | Whisper model id. |

### ClickHouse (large-tenant vector backend)

ClickHouse is opt-in. The API uses `pgvector` by default and switches per-org
once the chunk count exceeds `chunk_threshold`.

| Env var | Type | Default | Required | Description |
|---|---|---|---|---|
| `RAVEN_CLICKHOUSE_HOST` | host | _(empty)_ | no | Empty disables ClickHouse entirely. |
| `RAVEN_CLICKHOUSE_PORT` | int | `9000` | no | Native protocol port. |
| `RAVEN_CLICKHOUSE_DATABASE` | string | `raven` | no | ClickHouse database name. |
| `RAVEN_CLICKHOUSE_USER` | string | `default` | no | ClickHouse user. |
| `RAVEN_CLICKHOUSE_PASSWORD` | string | _(empty)_ | yes (if host set) | ClickHouse password. Never commit. |
| `RAVEN_CLICKHOUSE_VECTOR_BACKEND` | enum | `pgvector` | no | Default vector backend: `pgvector` or `clickhouse`. |
| `RAVEN_CLICKHOUSE_CHUNK_THRESHOLD` | int64 | `5000000` | no | Per-org chunk count above which ClickHouse is preferred. |

### Telemetry — OpenTelemetry / OpenObserve

| Env var | Type | Default | Required | Description |
|---|---|---|---|---|
| `RAVEN_OTEL_ENABLED` | bool | `false` | no | Master switch for OTEL exporters. |
| `RAVEN_OTEL_ENDPOINT` | host:port | _(empty)_ | yes (if enabled) | OTLP gRPC endpoint, e.g. `openobserve:5081`. |
| `RAVEN_OTEL_SERVICE_NAME` | string | `raven-api` | no | `service.name` resource attribute. |

### Payments — Hyperswitch + Razorpay

Raven uses Hyperswitch as the payment orchestrator with Razorpay underneath
for Indian rails (UPI, RuPay, cards).

| Env var | Type | Default | Required | Description |
|---|---|---|---|---|
| `RAVEN_HYPERSWITCH_BASE_URL` | URL | `http://localhost:8090` | yes (billing) | Hyperswitch API base. Use `https://sandbox.hyperswitch.io` for testing. |
| `RAVEN_HYPERSWITCH_API_KEY` | string | _(empty)_ | yes (billing) | Hyperswitch API key. Never commit. |
| `RAVEN_HYPERSWITCH_WEBHOOK_SECRET` | string | _(empty)_ | yes (billing) | HMAC-SHA-256 secret for webhook verification. Never commit. |
| `RAVEN_HYPERSWITCH_RAZORPAY_KEY_ID` | string | _(empty)_ | yes (Razorpay) | Razorpay public key id (surfaced to the frontend checkout SDK). |

### Email — AWS SES (SMTP)

Credentials are SES SMTP credentials, **not** IAM access keys. Generate via
AWS Console → SES → SMTP settings.

| Env var | Type | Default | Required | Description |
|---|---|---|---|---|
| `AWS_SES_REGION` | string | `ap-south-1` | no | SES region. |
| `AWS_SES_FROM_ADDRESS` | email | _(empty)_ | yes (email) | Verified SES identity used as the From address. |
| `AWS_SES_FROM_NAME` | string | `Raven` | no | Friendly From name. |
| `AWS_SES_SMTP_USERNAME` | string | _(empty)_ | yes (email) | SES SMTP username. Never commit. |
| `AWS_SES_SMTP_PASSWORD` | string | _(empty)_ | yes (email) | SES SMTP password. Never commit. |

### Email summary job (M9 #257)

Required when the post-session summary email pipeline is on. Setting
`RAVEN_UNSUBSCRIBE_BASE_URL` flips the pipeline on; that requires
`UNSUBSCRIBE_TOKEN_SECRET` to be at least 32 bytes — `Load()` rejects shorter
secrets at startup.

| Env var | Type | Default | Required | Description |
|---|---|---|---|---|
| `RAVEN_FRONTEND_BASE_URL` | URL | _(empty)_ | yes (email) | Public frontend URL used to build deep-links in emails. |
| `RAVEN_SUPPORT_ADDRESS` | email | `support@ravencloak.io` | no | Reply-To / footer contact. |
| `RAVEN_UNSUBSCRIBE_BASE_URL` | URL | _(empty)_ | yes (email) | Public URL of the one-click unsubscribe endpoint, proxied by the API. |
| `UNSUBSCRIBE_TOKEN_SECRET` | string (≥ 32 bytes) | _(empty)_ | yes (email) | HMAC-SHA-256 key for unsubscribe tokens. Generate with `openssl rand -hex 32`. Never commit. |
| `RAVEN_AI_WORKER_HTTP_URL` | URL | `http://ai-worker:8090` | no | AI-worker FastAPI base for `POST /internal/summarize`. |

### PostHog (server-side, opt-in)

| Env var | Type | Default | Required | Description |
|---|---|---|---|---|
| `RAVEN_POSTHOG_API_KEY` | string | _(empty)_ | no | PostHog project API key. Empty disables event capture. |
| `RAVEN_POSTHOG_HOST` | URL | `https://us.i.posthog.com` | no | PostHog ingestion host (use `https://eu.i.posthog.com` for the EU region). |

### Meta WhatsApp Business API

| Env var | Type | Default | Required | Description |
|---|---|---|---|---|
| `RAVEN_META_ACCESS_TOKEN` | string | _(empty)_ | yes (WhatsApp) | Permanent access token. Never commit. |
| `RAVEN_META_PHONE_NUMBER_ID` | string | _(empty)_ | yes (WhatsApp) | WhatsApp Cloud API phone-number id. |
| `RAVEN_META_APP_SECRET` | string | _(empty)_ | yes (WhatsApp) | App secret used to verify webhook HMAC signatures. Never commit. |
| `RAVEN_META_WEBHOOK_TOKEN` | string | _(empty)_ | yes (WhatsApp) | `hub.verify_token` for webhook subscription handshake. |

> Note: the variables above match the field names on `MetaConfig` in
> `config.go` and Viper's default `RAVEN_<SECTION>_<FIELD>` derivation
> (`SetEnvPrefix("RAVEN")` + `EnvKeyReplacer(".", "_")`). They are not
> additionally bound via `BindEnv`.

### TMDB (demo seed)

The demo knowledge base uses TMDB. Both names work because the legacy
`TMDB_API_KEY` is bound via `BindEnv`.

| Env var | Type | Default | Required | Description |
|---|---|---|---|---|
| `TMDB_API_KEY` | string | _(empty)_ | yes (demo seed) | TMDB API key. Never commit. |
| `RAVEN_TMDB_BASE_URL` | URL | `https://api.themoviedb.org/3` | no | TMDB API base. |

### Seed endpoint

| Env var | Type | Default | Required | Description |
|---|---|---|---|---|
| `RAVEN_SEED_KEY` | string | _(empty)_ | yes (admin seed) | Value required in the `X-Seed-Key` header to call the protected seed endpoint. Never commit. |

### eBPF (Linux ≥ 5.8 with `CONFIG_DEBUG_INFO_BTF=y`)

All flags default to `false`. The subsystem is a no-op on non-Linux platforms
regardless. `Load()` rejects `RAVEN_EBPF_AUDIT_RING_BUFFER_SIZE` values that
are not a power of two when `RAVEN_EBPF_ENABLED=true`.

| Env var | Type | Default | Required | Description |
|---|---|---|---|---|
| `RAVEN_EBPF_ENABLED` | bool | `false` | no | Master switch. When `false`, all sub-flags are ignored. |
| `RAVEN_EBPF_OBSERVABILITY_ENABLED` | bool | `false` | no | Enable eBPF-based observability hooks. |
| `RAVEN_EBPF_AUDIT_ENABLED` | bool | `false` | no | Enable eBPF audit. |
| `RAVEN_EBPF_AUDIT_IP_ALLOWLIST` | csv | _(empty)_ | no | IPs excluded from audit logging. |
| `RAVEN_EBPF_AUDIT_EXEC_ALLOWLIST` | csv | _(empty)_ | no | Exec paths excluded from audit logging. |
| `RAVEN_EBPF_AUDIT_RING_BUFFER_SIZE` | int (power of 2) | `1048576` | no | BPF ring buffer size in bytes. |
| `RAVEN_EBPF_XDP_ENABLED` | bool | `false` | no | Enable XDP rate-limit dataplane. |
| `RAVEN_EBPF_XDP_INTERFACE` | string | `eth0` | no | Network interface to attach XDP to. |

## Asynq worker — `cmd/worker`

The Asynq worker (`cmd/worker/main.go`) calls `config.Load()` and reuses
**every** variable above. The environment is identical to the API's. The
worker additionally consumes:

- `RAVEN_QUEUE_CONCURRENCY`, `RAVEN_QUEUE_MAX_RETRY` — Asynq runtime knobs.
- `AWS_SES_*` and `RAVEN_*` summary keys — the post-session summary job runs
  inside this binary (M9 #257).
- `RAVEN_AI_WORKER_HTTP_URL` — used to call `POST /internal/summarize` on the
  Python worker.

When SES SMTP credentials are unset and the binary is in `debug` server mode,
the worker falls back to a stub email sender; in `release` mode it returns
an init error from `email.Config.Validate()`.

## AI worker — Python (`ai-worker`)

Defined in
[`ai-worker/raven_worker/config.py`](https://github.com/ravencloak-org/Raven/blob/main/ai-worker/raven_worker/config.py)
as a `pydantic_settings.BaseSettings` subclass with
`SettingsConfigDict(env_prefix="RAVEN_")`. Version `0.3.0`
(`ai-worker/pyproject.toml`).

| Env var | Type | Default | Required | Description |
|---|---|---|---|---|
| `RAVEN_GRPC_PORT` | int | `50051` | no | gRPC listen port. |
| `RAVEN_GRPC_MAX_WORKERS` | int | `10` | no | gRPC server thread-pool size. |
| `RAVEN_DATABASE_URL` | URL | `postgresql://raven:raven@localhost:5432/raven` | yes | Same DSN the Go API uses. |
| `RAVEN_VALKEY_URL` | URL | `redis://localhost:6379/0` | yes | Same Valkey/Redis URL. |
| `RAVEN_OTEL_ENDPOINT` | host:port | _(empty)_ | no | OTLP endpoint (if enabled). |
| `RAVEN_OTEL_ENABLED` | bool | `false` | no | Toggle OTEL exporters. |
| `RAVEN_LOG_LEVEL` | string | `INFO` | no | Python logging level. |
| `RAVEN_LITEPARSE_PATH` | path | `liteparse` | no | Path to the `liteparse` binary used for document parsing. |
| `RAVEN_ENCRYPTION_KEY` | base64(32 bytes) | _(empty)_ | yes (BYOK) | Same key used by the Go API to decrypt tenant LLM keys. Never commit. |
| `RAVEN_LIVEKIT_URL` | URL | `ws://localhost:7880` | yes (voice) | LiveKit WebSocket URL. |
| `RAVEN_LIVEKIT_API_KEY` | string | _(empty)_ | yes (voice) | LiveKit API key. Never commit. |
| `RAVEN_LIVEKIT_API_SECRET` | string | _(empty)_ | yes (voice) | LiveKit API secret. Never commit. |
| `RAVEN_MEMORY_DIR` | path | _(empty)_ | no | Per-session memory directory. Empty disables memory. In Compose this is mounted from a named volume. |
| `RAVEN_ENABLE_WEB_SEARCH` | bool | `false` | no | Allow Anthropic `web_search` tool in RAG responses. |
| `RAVEN_HTTP_PORT` | int | `8090` | no | FastAPI internal endpoints (`POST /internal/summarize`). |
| `RAVEN_HTTP_ENABLED` | bool | `true` | no | Toggle the FastAPI side-server. |
| `RAVEN_HTTP_BIND_HOST` | string | `127.0.0.1` | no | Binds loopback by default; override only when callers live on a separate node and a network policy gates external access. |
| `RAVEN_SEMANTIC_CACHE_ENABLED` | bool | `true` | no | Toggle the pgvector semantic-response cache (M9 #256). When false the exact-match Valkey cache still applies. |

In addition, the AI worker reads provider keys directly from the environment
at request time (not via pydantic-settings):

- `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `COHERE_API_KEY` — used as
  fallbacks when a tenant has not provided a BYOK key.

## Frontend — Vite (`frontend`)

Vite substitutes `import.meta.env.VITE_*` at `vite build`. Defaults defined
in `frontend/.env.example`, production overrides in `frontend/.env.production`,
and the canonical Cloudflare Pages build env in `deploy/cloudflare-pages.json`.

| Env var | Type | Default (build) | Required | Where used | Description |
|---|---|---|---|---|---|
| `VITE_API_URL` | URL | `http://localhost:8081/api` | no | `frontend/src/api/client.ts` | Legacy/base API URL fallback. |
| `VITE_API_BASE_URL` | URL | `http://localhost:8081/api/v1` | yes | API store + auth callback wiring | The `/api/v1` prefix used by the bulk of the SPA. |
| `VITE_API_DOMAIN` | URL | `http://localhost:8081` | yes | SuperTokens `apiDomain` config in the SPA | Public API origin (no path). |
| `VITE_LIVEKIT_URL` | URL | _(empty)_ | no | Voice room joining | `wss://` LiveKit endpoint. |
| `VITE_POSTHOG_API_KEY` | string | _(empty)_ | no | Frontend PostHog client | Empty disables client-side analytics. |
| `VITE_POSTHOG_HOST` | URL | `https://us.i.posthog.com` | no | Frontend PostHog client | EU region: `https://eu.i.posthog.com`. |
| `VITE_WIDGET_URL` | URL | `https://cdn.raven.example/widget.js` | no | Embed widget loader | Override for self-hosted CDN. |
| `VITE_KEYCLOAK_URL` | URL | _(empty)_ | no | Legacy auth path (kept in `.env.production`) | Pre-SuperTokens Keycloak base URL. |
| `VITE_KEYCLOAK_REALM` | string | _(empty)_ | no | Legacy auth path | Keycloak realm. |
| `VITE_KEYCLOAK_CLIENT_ID` | string | _(empty)_ | no | Legacy auth path | Keycloak client id. |

> Reminder: Vite freezes these values into the bundle. To change them in
> production you must rebuild the frontend image (or redeploy Cloudflare
> Pages with new build env). They cannot be overridden at container start.

## Edge variant differences — `docker-compose.edge.yml`

The edge compose file (`docker-compose.edge.yml`) ships only the Go API, a
local PostgreSQL, and Traefik. The Python AI worker runs remotely. Env
differences vs the full stack:

| Env var | Edge value / behaviour | Notes |
|---|---|---|
| `RAVEN_SERVER_PORT` | `${RAVEN_SERVER_PORT:-8080}` | Edge default is `8080` (vs API default `8081`). |
| `RAVEN_DATABASE_URL` | `${DATABASE_URL:?required}` | Compose enforces required; uses local Postgres. |
| `RAVEN_GRPC_WORKER_ADDR` | `${GRPC_AI_WORKER_ADDR:?required}` | Points to a remote AI worker; required and validated by Compose. |
| `RAVEN_EDGE_MODE` | `"true"` (set in compose) | Signals the API to skip local-worker checks. |
| `RAVEN_API_IMAGE` | `ghcr.io/ravencloak-org/raven-api:latest` (default) | Override for pinned tags, e.g. `:latest-arm64`. |
| `DOMAIN`, `ACME_EMAIL` | optional | Used by Traefik for HTTPS. |

What is **not** present in the edge compose: SuperTokens, OpenObserve,
SeaweedFS, LiveKit, Asynq worker. Voice + WhatsApp + email + storage features
are not available on the edge profile; configure them only when you bring up
the full stack.

## Validation — how invalid configs fail

Go API and worker (`config.Load()`):

- `RAVEN_DATABASE_URL` empty ⇒ returns `database.url is required` and the
  binary exits non-zero on startup (`log.Fatalf` in `cmd/api/main.go` and
  `cmd/worker/main.go`).
- `RAVEN_RATELIMIT_DEFAULT_USER_LIMIT` ≤ 0 ⇒ explicit error.
- `RAVEN_RATELIMIT_DEFAULT_ORG_LIMIT` ≤ 0 ⇒ explicit error.
- `RAVEN_EBPF_ENABLED=true` and `RAVEN_EBPF_AUDIT_RING_BUFFER_SIZE` not a
  power of 2 ⇒ explicit error (`bits.OnesCount` check).
- `RAVEN_UNSUBSCRIBE_BASE_URL` set and `UNSUBSCRIBE_TOKEN_SECRET` shorter
  than 32 bytes ⇒ explicit error.

Compose (`docker-compose.yml`, `docker-compose.edge.yml`):

- `${VAR:?...}` syntax aborts `docker compose up` with the message before any
  container starts. Used for `RAVEN_ENCRYPTION_AES_KEY`, `DATABASE_URL`, and
  `GRPC_AI_WORKER_ADDR` (edge).

Python AI worker (pydantic-settings):

- Type-mismatch (e.g. non-int into `grpc_port`) raises `ValidationError` at
  import time and the gRPC process exits with a non-zero code.

Frontend (Vite):

- No runtime validation. Missing `VITE_*` values fall back to the `??`
  defaults in source. Misconfiguration manifests as runtime API errors in
  the browser console.

## Secrets handling — recommendations

- **Local dev** — copy `.env.example` to `.env`, fill values, never commit.
  `.gitignore` excludes `.env`, `.env.*` (except `.env.example`).
- **CI** — use repo secrets and inject only what the job needs.
- **Production** — use a secrets manager (Vault, AWS Secrets Manager, SSM
  Parameter Store, 1Password Connect, Doppler). The Compose stack supports
  encrypted `.env` files via [`dotenvx`](https://dotenvx.com/) — pass the
  decryption key as `DOTENV_PRIVATE_KEY` and bind-mount the encrypted `.env`
  read-only. Never mount `.env.keys` into a container.
- **Rotation** — `RAVEN_ENCRYPTION_AES_KEY`, `RAVEN_RATELIMIT_APIKEY_HASH_SECRET`,
  and `UNSUBSCRIBE_TOKEN_SECRET` are sticky: rotating them invalidates
  encrypted BYOK keys, existing rate-limit buckets, and outstanding
  unsubscribe tokens respectively. Plan rotations explicitly.

## See also

- [Self-hosting with Docker Compose](/guides/self-hosting/docker-compose)
- [Hardening guide](/guides/self-hosting/hardening)
