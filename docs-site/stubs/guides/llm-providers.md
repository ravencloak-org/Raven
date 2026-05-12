---
title: "LLM Providers"
---

# LLM Providers

Raven is **bring-your-own-key (BYOK)**. You supply credentials for the LLM
providers you want to use; Raven encrypts them at rest with AES-256-GCM,
decrypts them in-memory per request, and forwards the call to the upstream
provider. Token costs are billed by the provider directly to your account —
Raven never proxies money through itself.

Supported providers, in the enum that backs `llm_provider_configs.provider`:

- `anthropic` — Claude family (chat / RAG generation)
- `openai` — GPT models (chat) and `text-embedding-3-*` (embeddings)
- `cohere` — `command-*` (chat), `embed-*` (embeddings), `rerank-*` (reranker)
- `ollama` — local self-hosted models (chat + embeddings, **no API key**)
- `google`, `azure_openai`, `custom` — enum values reserved; concrete
  provider classes not wired yet

The supported set is enforced in two places: the Go enum in
[`internal/model/llm_provider.go`](https://github.com/ravencloak-org/Raven/blob/main/internal/model/llm_provider.go)
and the Python registry in
[`ai-worker/raven_worker/providers/registry.py`](https://github.com/ravencloak-org/Raven/blob/main/ai-worker/raven_worker/providers/registry.py).

## Provider matrix

| Provider    | Default chat / generation model       | Default embedding model               | Reranking            | API key required | Notes                                                            |
|-------------|---------------------------------------|---------------------------------------|----------------------|------------------|------------------------------------------------------------------|
| `anthropic` | `claude-*` (passed per request)       | — (stub: `NotImplementedError`)       | —                    | yes              | Supports prompt caching + memory tool in the streaming RAG path. |
| `openai`    | `gpt-*` (passed per request)          | `text-embedding-3-small` (1536 dims)  | —                    | yes              | 429 rate-limit errors retried with `tenacity` exponential back-off (max 5 attempts). |
| `cohere`    | `command-*` (passed per request)      | `embed-english-v3.0` (1024 dims)      | `rerank-english-v3.0` | yes             | **Default reranker** for hybrid retrieval. Required only if `filters.rerank == "cohere"`. |
| `ollama`    | model name passed per request         | `nomic-embed-text` (768 dims)         | —                    | **no**           | Talks to a local Ollama daemon at `OLLAMA_BASE_URL` (default `http://ollama:11434`). |

Provider keys are scoped per **organisation**, not per workspace or KB.
The active model and provider are selected **per request**.

## Configure a provider (UI)

1. Sign in and open the workspace switcher → **Settings** → **LLM Providers**.
2. Click **Add provider**, pick a type (`openai` / `anthropic` / `cohere` /
   `ollama`), and give it a `display_name` (the friendly label the dashboard
   shows).
3. Paste the upstream API key. The frontend submits it to
   `POST /api/v1/orgs/{org_id}/llm-providers`; the server encrypts it with
   AES-256-GCM before INSERT.
4. Tick **Set as default** if you want this provider used when a request does
   not name one explicitly. Only one provider per organisation can be the
   default (`is_default = true` on `llm_provider_configs`).

The UI lives in `frontend/src/api/llm-providers.ts` and its consuming Vue
components.

## Configure a provider (API)

`org_admin` is required for every write. The path is nested under the
organisation:

```http
POST /api/v1/orgs/{org_id}/llm-providers
```

```bash
curl -X POST "https://raven.example.com/api/v1/orgs/$ORG_ID/llm-providers" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "anthropic",
    "display_name": "Production Anthropic",
    "api_key": "sk-ant-api03-...",
    "is_default": true
  }'
```

The full request schema (`CreateLLMProviderRequest`) accepts:

| Field          | Required | Notes                                                     |
|----------------|----------|-----------------------------------------------------------|
| `provider`     | yes      | One of the enum values above.                             |
| `display_name` | yes      | 1–255 chars. Surfaced in the UI.                          |
| `api_key`      | yes      | Plaintext key. Encrypted before write; never returned.    |
| `base_url`     | no       | Override the upstream base URL (e.g. for Ollama, custom). |
| `config`       | no       | Free-form JSONB for provider-specific knobs.              |
| `is_default`   | no       | Marks this row as the org default for its provider type.  |

The response (`LLMProviderResponse`) intentionally omits
`api_key_encrypted` and `api_key_iv` — they are tagged `json:"-"` in
[`internal/model/llm_provider.go`](https://github.com/ravencloak-org/Raven/blob/main/internal/model/llm_provider.go).
Only an `api_key_hint` (last few characters) is ever returned.

The other endpoints on the same group:

| Method | Path                                                            | Role        | Purpose                       |
|--------|-----------------------------------------------------------------|-------------|-------------------------------|
| GET    | `/api/v1/orgs/{org_id}/llm-providers`                           | any member  | List configured providers     |
| GET    | `/api/v1/orgs/{org_id}/llm-providers/{provider_id}`             | any member  | Fetch one provider            |
| PUT    | `/api/v1/orgs/{org_id}/llm-providers/{provider_id}`             | `org_admin` | Update (re-encrypts new key)  |
| DELETE | `/api/v1/orgs/{org_id}/llm-providers/{provider_id}`             | `org_admin` | Remove a provider             |
| PUT    | `/api/v1/orgs/{org_id}/llm-providers/{provider_id}/default`     | `org_admin` | Mark as the org default       |

Routes are registered in
[`cmd/api/main.go`](https://github.com/ravencloak-org/Raven/blob/main/cmd/api/main.go)
under the `llm := api.Group("/orgs/:org_id/llm-providers")` block.

## Selecting a provider per workspace / KB

Provider selection happens **per request**, not per workspace. The flow in
`ai-worker/raven_worker/providers/registry.py` is:

1. The caller (RAG service, embedding service) passes
   `(org_id, provider_name, model)` to `get_provider_for_request(...)`.
2. The registry queries `llm_provider_configs` filtering on
   `org_id`, `provider = $provider_name`, `status = 'active'`, ordered by
   `is_default DESC LIMIT 1`. The active default for that provider wins.
3. The encrypted API key is decrypted with `decrypt_api_key`, except for
   providers in `_KEYLESS_PROVIDERS = {"ollama"}`.
4. The matching Python provider class (`AnthropicEmbeddingProvider`,
   `OpenAIEmbeddingProvider`, `CohereEmbeddingProvider`,
   `OllamaEmbeddingProvider`) is instantiated and cached at module level
   keyed by `(org_id, provider_name, model)`.

What can override per request:

- **Provider** — chat requests carry a `provider` field. RAG explicitly
  threads `provider_name` through `_stream_anthropic` / OpenAI / Cohere
  branches.
- **Model** — passed alongside `provider` on the request; falls back to
  the provider's `_DEFAULT_MODEL` constant.
- **`base_url`** — read from the `llm_provider_configs.base_url` column on
  decrypt. Lets you point `ollama` at a non-default host or front
  `openai`/`cohere` traffic through a corporate proxy.

There is currently no `default_provider_id` foreign key on the `workspaces`
or `knowledge_bases` tables — defaults are at the organisation level. If you
need per-KB routing, use the **routing rules** subsystem
(`/api/v1/orgs/{org_id}/routing-rules`).

## Encryption

Keys are encrypted with **AES-256-GCM** by
[`internal/crypto/aes.go`](https://github.com/ravencloak-org/Raven/blob/main/internal/crypto/aes.go):

```go
// Encrypt encrypts plaintext using AES-256-GCM with a random nonce.
// The key must be exactly 32 bytes. Returns (ciphertext, iv/nonce, error).
func Encrypt(plaintext []byte, key []byte) ([]byte, []byte, error)
```

- **Algorithm:** AES-256-GCM (authenticated encryption, no padding).
- **Master key:** 32 bytes. Anything else returns `ErrInvalidKeyLength`.
- **Nonce:** 12 bytes (GCM default), generated fresh per encryption via
  `crypto/rand` and stored next to the ciphertext in `api_key_iv`.
- **Storage layout** (`migrations/00011_llm_provider_configs.sql`):
  - `api_key_encrypted BYTEA` — ciphertext including GCM tag
  - `api_key_iv BYTEA` — per-row nonce
  - `api_key_hint VARCHAR(20)` — last few characters for UI display

The master key is provided as `RAVEN_ENCRYPTION_AES_KEY` — a 32-byte
key, conventionally a 64-character hex string. Generate one with:

```bash
openssl rand -hex 32
```

Set it before booting the API; compose enforces presence via
`${RAVEN_ENCRYPTION_AES_KEY:?... is required}`. The variable is bound to
`config.Encryption.AESKey` in
[`internal/config/config.go`](https://github.com/ravencloak-org/Raven/blob/main/internal/config/config.go):

```go
_ = v.BindEnv("encryption.aes_key", "RAVEN_ENCRYPTION_AES_KEY")
```

### Key rotation

Raven does not yet support online re-encryption. To rotate the master key:

1. Generate the new key: `openssl rand -hex 32`.
2. Export both `RAVEN_ENCRYPTION_AES_KEY_OLD` (current) and
   `RAVEN_ENCRYPTION_AES_KEY` (new) on a maintenance host.
3. For each row in `llm_provider_configs`, `Decrypt(api_key_encrypted,
   api_key_iv, old_key)` then `Encrypt(plaintext, new_key)` and `UPDATE`
   the row with the new ciphertext and nonce.
4. Restart the API and AI worker with the new key only.
5. Have each `org_admin` re-test their providers via the dashboard "Test
   connection" button to confirm successful decryption.

Re-issuing keys at the upstream provider (Anthropic, OpenAI, Cohere) and
running `PUT /api/v1/orgs/{org_id}/llm-providers/{provider_id}` with the new
`api_key` is the simpler alternative if you suspect a master-key compromise.

## Capabilities matrix

| Capability        | `anthropic` | `openai` | `cohere` | `ollama` |
|-------------------|-------------|----------|----------|----------|
| Chat / RAG stream | yes         | yes      | yes      | yes      |
| Embeddings        | **no** (stub raises `NotImplementedError`) | yes (`text-embedding-3-small`, 1536 dims) | yes (`embed-english-v3.0`, 1024 dims) | yes (`nomic-embed-text`, 768 dims) |
| Reranking         | no          | no       | **yes** (`rerank-english-v3.0`, default) | no |
| Prompt caching    | yes (`cache_control: ephemeral` on system prompt) | no | no | no |
| Web search tool   | yes (`web_search_20260209`, opt-in via `RAVEN_ENABLE_WEB_SEARCH=true`) | no | no | no |
| Memory tool       | yes (opt-in via `RAVEN_MEMORY_DIR`) | no | no | no |

The Anthropic embedding class exists only so the registry returns a
consistent error instead of "unknown provider" when an Anthropic-only org
asks for embeddings — pair Anthropic chat with OpenAI / Cohere / Ollama
for embeddings.

## Ollama (self-hosted models)

Ollama is the recommended provider for **air-gapped deployments** and
**PII-sensitive workloads** — nothing leaves the host. The defaults in
`ai-worker/raven_worker/providers/ollama_provider.py`:

```python
_DEFAULT_MODEL = "nomic-embed-text"
_DEFAULT_DIMENSIONS = 768
_DEFAULT_BASE_URL = "http://ollama:11434"
```

To wire it up:

```bash
curl -X POST "https://raven.example.com/api/v1/orgs/$ORG_ID/llm-providers" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "ollama",
    "display_name": "On-prem Ollama",
    "api_key": "unused",
    "base_url": "http://ollama:11434",
    "config": {"is_local": true}
  }'
```

Notes:

- The `api_key` field is required by the request schema but the registry
  treats `ollama` as a member of `_KEYLESS_PROVIDERS` and skips the
  decryption step. Pass any non-empty placeholder.
- For Raven Local (single-user desktop mode) a row is auto-seeded by
  [`migrations/00039_seed_ollama_local_provider.sql`](https://github.com/ravencloak-org/Raven/blob/main/migrations/00039_seed_ollama_local_provider.sql)
  with `base_url = 'http://ollama:11434/v1'` and `is_default = true`.
- Pull the model on the Ollama host before first use:
  `ollama pull nomic-embed-text`. The provider raises `ModelNotPulledError`
  (mapped to a friendly UI affordance) on 404.

## Default provider

The Go API does **not** ship a fallback BYOK key — defaults are per-org
rows with `is_default = true`. The `.env.example` file exposes the
upstream keys (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `COHERE_API_KEY`)
only for the **dev-agent and worker bootstrap scripts**; production
traffic goes through the encrypted `llm_provider_configs` rows.

The org-level default is set by:

```bash
curl -X PUT "https://raven.example.com/api/v1/orgs/$ORG_ID/llm-providers/$PROVIDER_ID/default" \
  -H "Authorization: Bearer $TOKEN"
```

Per-workspace overrides are not modelled at the schema level today; prefer
explicit `provider` + `model` on each request, or use
[routing rules](/reference/configuration) to map traffic patterns to a
specific provider.

## Rate limits and retries

Two layers of retry:

- **In-provider** — `OpenAIEmbeddingProvider.embed()` uses `tenacity` with
  `wait_exponential(multiplier=1, min=1, max=60)` and
  `stop_after_attempt(5)`, retrying only on
  `openai.RateLimitError` (HTTP 429). Anthropic and Cohere rely on the
  worker's task retry.
- **Async job queue** — `internal/queue` enqueues all worker tasks with
  `asynq.MaxRetry(c.maxRetry)`. The default is `5`
  (`v.SetDefault("queue.max_retry", 5)` in `internal/config/config.go`);
  override with `RAVEN_QUEUE_MAX_RETRY`. Webhook delivery is a deliberate
  exception (`asynq.MaxRetry(0)` — the handler manages its own retry
  schedule).

Synchronous RAG streaming is not retried — a transient upstream failure
surfaces as an error event on the SSE stream and the client decides
whether to retry.

## Costs

Raven does not bill customers for LLM tokens. Anthropic, OpenAI, and
Cohere bill your account directly for the keys you upload. The only
Raven-side costs are:

- The platform subscription (handled by Hyperswitch — see
  [/guides/billing](/guides/billing)).
- Voice usage (Deepgram / LiveKit), tracked separately in `voice_usage`.

Ollama is free in absolute terms — the cost is the hardware running the
daemon.

Cache aggressively to cut your provider bill: the Anthropic provider
adds `cache_control: ephemeral` on the system prompt, and the semantic
response cache (KB-level, see [/guides/retrieval](/guides/retrieval))
deduplicates near-identical questions before any token is spent.

## Troubleshooting

| Symptom                                              | Likely cause                                              | Fix                                                                                                          |
|------------------------------------------------------|-----------------------------------------------------------|--------------------------------------------------------------------------------------------------------------|
| Upstream returns **401 Unauthorized**                | Provider key was revoked or pasted incorrectly             | `PUT /llm-providers/{id}` with a fresh `api_key`. Confirm via the upstream provider dashboard.               |
| Upstream returns **429 Too Many Requests**           | Per-minute / per-day quota exhausted                       | OpenAI auto-retries up to 5 times. For sustained pressure, raise the quota upstream or split across orgs.    |
| **`model not found`** from OpenAI / Anthropic        | Model name typo or model deprecated                        | Check the upstream provider's current model list — Raven passes the string through unchanged.               |
| **`ModelNotPulledError`** from Ollama                | The named model has not been pulled on the Ollama host     | `ollama pull <model>` on the host. Verify with `curl http://ollama:11434/api/tags`.                          |
| Worker logs **`decrypt: cipher: message authentication failed`** | Master key changed without re-encrypting rows | Restore the previous `RAVEN_ENCRYPTION_AES_KEY` or follow the rotation procedure above to re-encrypt rows.   |
| `decrypt_api_key` returns `None`                     | No active provider config matching `(org_id, provider)`    | Add one via `POST /llm-providers` or mark an existing row `active`.                                          |
| Cohere rerank silently disabled                      | `cohere` Python package missing, or API call failed        | Check worker logs for `cohere_rerank_unavailable` / `cohere_rerank_failed`. RRF results are returned anyway. |

---

**Related**

- [/guides/retrieval](/guides/retrieval) — how the chosen provider plugs
  into hybrid search and Cohere reranking
- [/guides/ingestion](/guides/ingestion) — where the embedding provider
  is called from
- [/reference/configuration](/reference/configuration) — full env-var
  reference, including `RAVEN_ENCRYPTION_AES_KEY` and queue knobs
