# Raven Test Plan Design

**Date:** 2026-04-09  
**Status:** Approved  
**Approach:** Gap-analysis-driven, layered test suite

---

## 1. Goals & Constraints

- **Goal:** Comprehensive test coverage across all Raven features — core, enterprise (EE), and eBPF — using the right tool at each layer.
- **Approach:** Audit existing tests, write only what is missing. Do not rewrite passing tests.
- **Coverage targets:**
  - Go API: 80% line coverage (codecov gate)
  - Python AI worker: 70% line coverage
  - Frontend E2E: 100% of named user journeys covered by at least one Playwright test
  - eBPF: all 3 program types (XDP, observability, audit) exercised

---

## 2. Test Structure (Approach C — Co-located + Dedicated Suites)

```
raven/
├── internal/                  # Go unit tests co-located (*_test.go) ← existing
├── ai-worker/tests/           # Python pytest suite ← existing
├── tests/
│   ├── e2e/                   # NEW — Playwright (frontend journeys + API flows)
│   │   ├── auth/
│   │   ├── chat/
│   │   ├── documents/
│   │   ├── knowledge-bases/
│   │   ├── voice/
│   │   ├── whatsapp/
│   │   ├── api-keys/
│   │   ├── webhooks/
│   │   └── ee/
│   └── ebpf/                  # NEW — privileged Go kernel harness
│       ├── xdp/
│       ├── observability/
│       └── audit/
```

---

## 3. Go Backend Tests (testify + testcontainers)

### 3.1 Unit Tests (co-located `*_test.go`)

| Package | Test Cases |
|---------|-----------|
| `internal/handler` | Request validation (required fields, type checks), response shape per endpoint, 4xx error codes for bad input, middleware passthrough |
| `internal/service` | Business logic with mocked repositories; ChatService message flow, DocumentService upload orchestration, SearchService hybrid retrieval scoring, SecurityService WAF rule evaluation, WhatsAppService webhook event dispatch, VoiceService session lifecycle |
| `internal/repository` | SQL correctness against real PostgreSQL (testcontainers); CRUD for all entities, RLS cross-tenant isolation (attempt cross-org reads, verify zero rows), pagination, soft deletes |
| `internal/middleware` | JWT validation (valid, expired, wrong audience), API key auth (valid scoped key, revoked key, wrong scope), rate limiter (under threshold pass, over threshold 429), WAF rule middleware evaluation |
| `internal/queue` + `internal/jobs` | Task enqueue (correct payload serialisation), job handler execution (mock dependencies), scheduler cron registration, dead-letter on repeated failure |
| `internal/grpc` | gRPC client with stubbed AI Worker: ParseAndEmbed happy path, QueryRAG streaming chunk assembly, GetEmbedding response mapping, timeout/cancellation handling, connection refused → caller receives wrapped error, TLS handshake failure → error surfaced, gRPC status codes propagated correctly (UNAVAILABLE → 503, RESOURCE_EXHAUSTED → 429) |
| `internal/crypto` | Encrypt/decrypt round-trip, key rotation, hash comparison for API keys |
| `internal/cache` | Get/Set/Delete with TTL, cache miss fallback, Valkey connection failure handling |
| `internal/storage` | SeaweedFS upload, download, delete — mocked HTTP responses; cross-tenant file access attempt → mocked 403 response → caller receives permission error (multi-tenant isolation) |

### 3.2 Integration Tests

| Scenario | Description |
|----------|-------------|
| RLS enforcement | Spin up PG via testcontainers; insert data for org A; query as org B; assert zero rows returned |
| Document processing pipeline | Upload → async parse → chunk → embed (stub gRPC) → store chunks → verify retrieval |
| Hybrid search | Insert chunks with known vectors + BM25 index; query with known embedding; verify RRF-fused ranked results |
| SSE streaming | Hit `/api/v1/chat` with streaming enabled; verify chunked response arrives and assembles to coherent text |
| Webhook delivery | Enqueue webhook task; verify HTTP delivery attempt; mock 500 response; verify retry at canonical backoff intervals (1s, 5s, 30s — matching section 7.3); verify dead-letter after max attempts |
| Asynq job scheduler | Verify scheduled jobs fire at correct intervals; verify `TypeRecrawl`, `TypeCleanup`, `TypeVoiceUsage` handlers execute |
| Migration correctness | Run all 32 migrations up; verify schema snapshot matches expected structure at each checkpoint; run down only if down migrations are maintained (skip otherwise) |

---

## 4. Python AI Worker Tests (pytest)

### 4.1 gRPC Service Tests

| RPC | Cases |
|-----|-------|
| `ParseAndEmbed` | Valid document → chunks + embeddings returned; empty document → error; oversized document → chunked correctly |
| `QueryRAG` | Query returns streaming chunks; sources attributed correctly; empty KB → graceful empty response |
| `GetEmbedding` | Text → vector with correct dimensions; empty string → error |

### 4.2 Document Processing

| Module | Cases |
|--------|-------|
| HTML parser (BeautifulSoup4) | Strips scripts/styles, extracts body text, handles malformed HTML |
| Text splitter (langchain) | Chunk size respected, overlap correct, no orphan tokens |
| Chunk metadata | Source URL, document ID, chunk index all propagated |

### 4.3 Retrieval Pipeline

| Component | Cases |
|-----------|-------|
| Embedding generation | Returns float array of correct dimension for each provider mock |
| Cosine similarity ranking | Higher similarity → higher rank, tied scores handled |
| BM25 scoring | Term frequency counted correctly, IDF weighted |
| RRF fusion | Combined score > either individual score for relevant docs |

### 4.4 Voice Agent

| Component | Cases |
|-----------|-------|
| STT pipeline | Audio input → transcript (mocked LiveKit input) |
| LLM call | Transcript → response via mocked provider |
| TTS pipeline | Response text → audio output (mocked) |
| Session lifecycle | Join → active → disconnect → cleanup |

### 4.5 LLM Provider Mocking

All tests use deterministic stub providers. One manual-trigger smoke test per real provider (OpenAI, Cohere, Anthropic) via separate CI job — not run on every PR.

---

## 5. Playwright E2E Tests (`tests/e2e/`)

### 5.1 Frontend Journeys

| Domain | Journeys |
|--------|---------|
| **Auth** | Login via Keycloak SSO, logout, session expiry redirect, invalid credentials error |
| **Org/Workspace** | Create organisation, create workspace, invite member, remove member; member attempts workspace-admin action → denied (RBAC enforcement); viewer role cannot access KB settings |
| **Knowledge Base** | Create KB, edit settings, delete KB, list KBs |
| **Documents** | Upload file (PDF, DOCX, TXT), add URL source, view processing status (polling), view chunk list, delete document |
| **Chat** | Send message, receive streaming response, citation links open source, view session history, start new session |
| **API Keys** | Create key scoped to workspace, create key scoped to KB, copy key value, revoke key, list keys |
| **LLM Providers** | Add BYOK config (OpenAI), test connection (mocked), edit provider, delete provider |
| **Voice** | Initiate LiveKit session (mocked SFU), view active sessions list, end session |
| **WhatsApp** | View incoming webhook events, trigger test callback endpoint, view delivery status |
| **Chat Widget** | Load sandbox page with embedded `<raven-chat>`, authenticate via API key, send message, receive response; invalid API key → widget displays error state (not blank/crash) |
| **Analytics** | View usage dashboard, filter by date range, export data |
| **Notifications** | Create notification rule, receive in-app notification |

### 5.2 API Mode Tests (REST Endpoints)

| Category | Cases |
|----------|-------|
| **Auth** | Valid JWT → 200; expired JWT → 401; valid API key → 200; revoked API key → 401; wrong scope API key → 403 |
| **Rate limiting** | Burst requests to threshold → 429 with `Retry-After` header |
| **Webhook reception** | POST to `/webhooks/meta` with valid HMAC → 200; invalid HMAC → 403 |
| **SSE streaming** | `GET /api/v1/chat` with `Accept: text/event-stream` → chunked `data:` events arrive |
| **Health** | `GET /healthz` → 200 with DB + cache status |
| **API key scoping** | KB-scoped key cannot access other KB's chat endpoint |

---

## 6. eBPF Test Harness (`tests/ebpf/`)

Runs as a privileged Go test binary requiring `CAP_BPF` + `CAP_NET_ADMIN`.

### 6.1 XDP Pre-filter (`tests/ebpf/xdp/`)

| Case | Description |
|------|-------------|
| Allow legitimate traffic | Craft TCP packet to port 8080; verify XDP_PASS decision |
| Drop known-bad source | Craft packet from blocklisted CIDR; verify XDP_DROP |
| Rate threshold drop | Flood loopback beyond configured PPS limit; verify XDP_DROP kicks in |
| Allowlist bypass | Packet from trusted IP exceeds rate limit but is allowlisted; verify XDP_PASS |

### 6.2 Observability (`tests/ebpf/observability/`)

| Case | Description |
|------|-------------|
| Ring buffer capture | Generate network event; verify ring buffer entry has correct src IP, dst port, timestamp, direction |
| Multiple concurrent events | 100 concurrent connections; verify all captured without dropped events |
| Metadata accuracy | Verify captured metadata matches actual packet fields |

### 6.3 Audit Trail (`tests/ebpf/audit/`)

| Case | Description |
|------|-------------|
| Port scan detection | Rapid sequential port probes; verify audit log entry written with `threat_type: port_scan` |
| Rate threshold event | Sustained high-rate traffic; verify audit entry with `threat_type: rate_exceeded` |
| Audit entry schema | All required fields present: timestamp, src_ip, dst_port, threat_type, action_taken |
| Audit persistence | Entries survive process restart; ClickHouse write verified via real ClickHouse sidecar container started alongside the privileged eBPF test container (not the unit-test in-memory sink) |

---

## 7. Enterprise Features (EE)

### 7.1 Security Rules (WAF)

| Case | Description |
|------|-------------|
| Block rule | Create block rule for pattern; send matching request; assert 403 |
| Allow rule | Allowlist rule overrides block; verify 200 |
| Log rule | Log-only rule; request passes; verify audit log entry created |
| Rule priority | Higher-priority rule wins when two rules match same request |
| Regex rule | Complex regex pattern; test matching and non-matching inputs |

### 7.2 SSO

| Case | Description |
|------|-------------|
| OIDC login flow | Redirect to Keycloak, authenticate, redirect back with tokens |
| Token exchange | ID token → session; verify claims mapped to user profile |
| Attribute mapping | Keycloak group → Raven workspace role |
| SSO-only enforcement | When SSO-only enabled, password login returns 403 |

### 7.3 Webhooks

| Case | Description |
|------|-------------|
| Delivery | Event fires → HTTP POST to configured URL with correct payload |
| HMAC signature | Delivery includes `X-Raven-Signature` header; verify HMAC-SHA256 |
| Retry with backoff | Mock endpoint returns 500; verify retry at 1s, 5s, 30s intervals |
| Dead-letter | After max retries, event moves to dead-letter queue |
| Replay | Dead-lettered event can be manually replayed |

### 7.4 Connectors (Airbyte)

| Case | Description |
|------|-------------|
| Sync trigger | POST to trigger sync; verify Airbyte job created |
| Status polling | Poll job status; mock progression to SUCCEEDED; verify KB updated |
| Data ingestion | Synced records appear as documents in KB |
| Failure handling | Airbyte job FAILED; verify error surfaced to user |

### 7.5 Licensing

| Case | Description |
|------|-------------|
| EE feature gated | Call EE endpoint without valid license → 402 |
| Valid license | Call EE endpoint with valid license → 200 |
| Expired license | Expired license → 402 with expiry message |
| Feature flag granularity | License for feature A does not unlock feature B |

### 7.6 Analytics

| Case | Description |
|------|-------------|
| ClickHouse event write | Action triggers event; verify row written to ClickHouse |
| PostHog event shape | Event emitted with correct distinct_id, event name, properties (mocked sink) |
| Usage aggregation | Query aggregated metrics; verify counts match raw event count |

### 7.7 Lead Profiles

| Case | Description |
|------|-------------|
| Create lead | POST lead profile; verify stored with correct fields |
| CRM field mapping | Lead fields map correctly to configured CRM schema |
| Update lead | PATCH updates only specified fields |
| List leads | Pagination, filtering by status |

### 7.8 Audit Logs

| Case | Description |
|------|-------------|
| Create event | KB created → audit log entry with actor, timestamp, entity_type, action |
| Update event | Document updated → audit entry with before/after diff |
| Delete event | User deleted → audit entry retained (immutable) |
| Query audit log | Filter by actor, date range, entity type |

---

## 8. CI Integration

| Job | Trigger | Tests run | Gate |
|-----|---------|-----------|------|
| `go.yml` | Every PR + push to main | Go unit + integration | 80% coverage |
| `python.yml` | Every PR + push to main | pytest + mypy + ruff | 70% coverage |
| `frontend.yml` | Every PR + push to main | Step 1: Vitest unit (fast-fail gate); Step 2: Playwright E2E headless (only runs if Step 1 passes) | Vitest: all pass; Playwright: all journeys pass |
| `ebpf` (new, in go.yml) | Push to main only | eBPF kernel harness (privileged container) | All cases pass |
| `smoke` (new manual job) | Manual trigger only | Real LLM provider smoke tests | Pass/fail reported |

### eBPF CI Container Config

```yaml
ebpf-tests:
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@v4
    - name: Run eBPF tests
      run: |
        docker run --privileged \
          --cap-add CAP_BPF \
          --cap-add CAP_NET_ADMIN \
          -v ${{ github.workspace }}:/workspace \
          golang:1.26.1 \
          bash -c "cd /workspace && go test ./tests/ebpf/... -v -tags ebpf"
```

---

## 9. Mocking Strategy

| External System | Mock Approach |
|----------------|--------------|
| LLM providers (OpenAI, Cohere, Anthropic) | Interface stub returning deterministic vectors/text |
| Meta WhatsApp API | HTTP mock server (httptest) verifying request shape |
| LiveKit SFU | Mocked livekit-server SDK responses |
| SeaweedFS | Mocked HTTP responses via httptest |
| Keycloak (unit tests) | JWT signed with test key; JWKS endpoint mocked |
| Keycloak (E2E) | Real Keycloak in Docker Compose test stack |
| ClickHouse | In-memory sink for unit tests; real container for integration |
| Airbyte | Mocked HTTP API responses |

---

## 10. Out of Scope

- Load / performance testing (separate initiative)
- Chaos engineering
- Mobile browser testing (Playwright desktop only for now)
- Real LLM cost testing (gated behind manual smoke job)
