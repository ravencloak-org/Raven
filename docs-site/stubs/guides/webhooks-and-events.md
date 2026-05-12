---
title: "Webhooks & Events"
---

# Webhooks & Events

Raven can notify external systems when interesting things happen in your
workspace by sending a signed JSON `POST` to a URL you register. This page
documents the data model, the HTTP contract, the signing scheme, the
delivery semantics, and the current state of the implementation.

::: info Current state — three of four event types fire from production code
The webhook **data model**, **management API**, **delivery worker**,
**HMAC signing**, **and** the producer call sites for three of the four
event types are implemented in the Go backend
(`internal/handler/webhook.go`, `internal/service/webhook.go`,
`internal/jobs/webhook_delivery.go`,
`migrations/00024_webhook_configs.sql`, plus the wiring listed below).

The remaining event (`conversation.escalated`) has **no producer code
path in OSS** today — the conversation lifecycle does not yet have an
"escalate to human" verb. The event type is still accepted at
registration so receivers can subscribe in anticipation of the EE
emitter (`internal/ee/webhooks`, currently a doc-comment placeholder).
:::

## What's wired

| Component | File | Status |
|-----------|------|--------|
| `webhook_configs` + `webhook_deliveries` tables, RLS, indexes | `migrations/00024_webhook_configs.sql` | Real |
| CRUD API (`/orgs/:org_id/webhooks`) | `internal/handler/webhook.go`, `cmd/api/main.go` | Real, admin-only |
| List deliveries (`/orgs/:org_id/webhooks/:id/deliveries`) | `internal/handler/webhook.go` | Real, last 50 |
| Async delivery worker with HMAC-SHA256 signing, SSRF guard, retries | `internal/jobs/webhook_delivery.go` | Real |
| Dispatch fan-out (look up active subscribers and enqueue) | `internal/service/webhook.go` `WebhookService.Dispatch` | Real, called from three producers |
| `document.processed` emission | `internal/jobs/document_process.go` | Real, end of success path |
| `sync.completed` emission | `internal/jobs/airbyte_sync.go` | Real, end of success path |
| `lead.generated` emission | `internal/service/lead.go` `LeadService.Upsert` | Real, on each successful upsert |
| `conversation.escalated` emission | — | **Not wired** (no escalation verb in OSS) |
| Replay / resend endpoint | — | Not implemented |

## Event types

Defined in `internal/model/webhook.go` and enforced by
`supportedWebhookEvents` in `internal/handler/webhook.go`. Subscribing to
any other name is rejected with HTTP 400.

| Event | Fires when | Status |
|-------|-----------|--------|
| `lead.generated` | `LeadService.Upsert` succeeds (capture or merge by org+email). | Wired |
| `conversation.escalated` | A conversation is escalated to a human operator. | **Not wired** — no escalation verb in OSS today; reserved for the EE emitter. |
| `document.processed` | The document processing job finishes successfully (with or without chunks). | Wired |
| `sync.completed` | An Airbyte connector sync run completes successfully. | Wired |

### Wired payload shapes

```json
// document.processed
{
  "document_id": "uuid",
  "knowledge_base_id": "uuid",
  "status": "ready",
  "chunk_count": 7
}

// sync.completed
{
  "connector_id": "uuid",
  "source_id": "uuid",            // knowledge_base_id of the connector
  "sync_run_id": "uuid",
  "records_synced": 0,             // placeholder until real Airbyte API lands (TODO #111)
  "status": "completed"
}

// lead.generated
{
  "lead_id": "uuid",
  "email": "visitor@example.com",
  "name": "Visitor Name",
  "knowledge_base_id": "uuid",
  "session_ids": ["uuid", "..."]
}
```

`lead.generated` is emitted on every successful `UpsertLead` call — the
repo merges by `(org_id, email)` so receivers may see repeated events
for the same `lead_id` as the visitor's profile is enriched. Dedupe on
`lead_id` plus the delivery row's `id`.

No other event names are accepted. Billing webhooks (`payment_succeeded`,
etc.) and the Meta / WhatsApp webhook endpoints (`/webhooks/meta`,
`/webhooks/whatsapp`) are **inbound** — they are a separate concern and
do not flow through this system.

## Configure a webhook

`POST /api/v1/orgs/{org_id}/webhooks` — requires the `org_admin` role.

Request body (`model.CreateWebhookRequest`):

```json
{
  "name": "ops-pager",
  "url": "https://ops.example.com/raven-hooks",
  "secret": "a-long-random-string-min-8-chars",
  "events": ["lead.generated", "document.processed"],
  "headers": {
    "X-Internal-Routing": "leads-team"
  },
  "max_retries": 5
}
```

Validation rules (from the model tags and `validate*` helpers):

- `name` — required, 2–255 chars, unique per org.
- `url` — required, must parse as a URL, must use `http://` or `https://`.
  Non-HTTP schemes and private/loopback IPs are blocked at delivery time
  by the SSRF guard in `internal/jobs/webhook_delivery.go`.
- `secret` — required, minimum 8 chars. Stored as plain text in the
  `secret` column. Marked `json:"-"` so it is never returned in API
  responses after creation.
- `events` — at least one entry; every entry must be one of the supported
  event types above.
- `headers` — optional custom headers. Cannot override `Content-Type`,
  `X-Raven-Signature`, or `X-Raven-Event` (silently dropped by the
  delivery worker).
- `max_retries` — optional, 0–20 (default 5).

Full set of endpoints:

| Method | Path | Purpose |
|--------|------|---------|
| `POST` | `/orgs/{org_id}/webhooks` | Create |
| `GET` | `/orgs/{org_id}/webhooks` | List |
| `GET` | `/orgs/{org_id}/webhooks/{id}` | Fetch one |
| `PUT` | `/orgs/{org_id}/webhooks/{id}` | Update (partial) |
| `DELETE` | `/orgs/{org_id}/webhooks/{id}` | Delete |
| `GET` | `/orgs/{org_id}/webhooks/{id}/deliveries` | List recent attempts (limit 50) |

## Payload shape

Every outbound delivery is a JSON `POST` with this body (constructed in
`WebhookDeliveryHandler.Process`):

```json
{
  "event_type": "lead.generated",
  "org_id": "0c9d4f1a-...-...",
  "timestamp": "2026-05-12T08:14:33Z",
  "data": {
    "lead_id": "...",
    "...": "event-specific fields"
  }
}
```

Fixed headers set by Raven:

```
Content-Type: application/json
X-Raven-Event: lead.generated
X-Raven-Signature: sha256=<hex-encoded HMAC>
```

The `data` field is a free-form `map[string]any` chosen by the caller of
`Dispatch`. Concrete shapes per wired event are listed under
[Event types](#event-types) above.

## Signing + verification

Signing scheme (`internal/jobs/webhook_delivery.go`):

```go
mac := hmac.New(sha256.New, []byte(hook.Secret))
mac.Write(bodyBytes)
signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
req.Header.Set("X-Raven-Signature", signature)
```

The HMAC is over the **raw request body**, not over headers or the URL.

A Node receiver verifies it like this:

```js
import crypto from "node:crypto";

function verify(rawBody, signatureHeader, secret) {
  const expected =
    "sha256=" +
    crypto.createHmac("sha256", secret).update(rawBody).digest("hex");
  const a = Buffer.from(expected);
  const b = Buffer.from(signatureHeader || "");
  return a.length === b.length && crypto.timingSafeEqual(a, b);
}
```

Always compare with a constant-time comparison
(`crypto.timingSafeEqual` in Node, `hmac.compare_digest` in Python).
Read the body as bytes before parsing it as JSON — re-serialising will
change whitespace and break the HMAC.

## Delivery semantics

Implemented in `internal/jobs/webhook_delivery.go` and
`internal/queue/client.go`:

- **Transport** — Asynq task `webhook:deliver` on the `default` queue.
- **Retries** — Asynq itself is enqueued with `MaxRetry(0)`; the job
  handler manages retry counting against the webhook's `max_retries`
  column. Each attempt increments `failure_count` on the config; a
  successful delivery resets it to 0.
- **Termination** — once `failure_count >= max_retries`, the webhook's
  status is flipped to `failed` and no further deliveries are
  attempted. A human re-enables it by `PUT`-ing
  `{"status": "active"}` and clearing the failure count via update.
- **Backoff** — the schema includes a `next_retry_at` column on
  `webhook_deliveries` for scheduled re-attempts, but the current job
  handler does not populate it. Today, retries happen only if a caller
  enqueues a fresh task; there is no automatic scheduled re-attempt.
- **Delivery guarantee** — **at-least-once when emitted**. A
  `webhook_deliveries` row is created *before* the Asynq task is
  enqueued (`WebhookService.Dispatch`), so receivers may see retries
  with the same `id` and must be idempotent on it.
- **Success** — HTTP `2xx` from the receiver.
- **Failure** — any non-`2xx`, timeout, DNS failure, or SSRF rejection
  (private IPs, non-HTTP schemes).
- **HTTP client** — wraps a dialer that resolves DNS and rejects
  private/loopback IPs to prevent SSRF against internal services. The
  response body is read with `io.LimitReader` at 4096 bytes and stored
  in `webhook_deliveries.response_body` for inspection.

## Inspecting deliveries

```
GET /api/v1/orgs/{org_id}/webhooks/{id}/deliveries
```

Returns up to 50 most recent `WebhookDelivery` rows for the given
webhook. Fields (`internal/model/webhook.go`):

```json
{
  "id": "uuid",
  "webhook_id": "uuid",
  "org_id": "uuid",
  "event_type": "lead.generated",
  "payload": { "...": "the data sent" },
  "response_status": 200,
  "response_body": "ok",
  "attempt_count": 1,
  "success": true,
  "delivered_at": "2026-05-12T08:14:34Z",
  "created_at": "2026-05-12T08:14:33Z"
}
```

The underlying `webhook_deliveries` table is RLS-scoped to the org via
`app.current_org_id` (see migration). Admins have a `raven_admin`
bypass policy.

## Replaying a failed delivery

**Not yet implemented.** There is no replay or resend endpoint. To
re-fire an event today you would have to enqueue a new
`WebhookDeliveryPayload` directly against the Asynq queue (see
`internal/queue/client.go` `EnqueueWebhookDelivery`), or call
`WebhookService.Dispatch` from a controlled script. The
`webhook_deliveries.next_retry_at` column exists in anticipation of
this but is not populated by current code.

## Rate limits

There are **no per-webhook or per-org rate limits** on outbound
deliveries today. The only relevant caps are:

- `max_retries` per webhook config (default 5, max 20).
- A 4096-byte cap on the captured response body per delivery row.
- The global Asynq worker concurrency configured for the `default`
  queue in the worker process.

Build a receiver that can absorb bursts. If you need throttling, the
right place to add it is in `WebhookDeliveryHandler.Process` or as a
queue-level rate limiter; neither is in place today.

## Best practices

- **Be idempotent on `id`.** A retry will arrive with the same
  `webhook_deliveries.id`; treat it as the dedup key.
- **Respond fast.** The job handler uses a single HTTP client; long
  receivers block worker capacity. Acknowledge with `200` and process
  asynchronously on your side.
- **Verify the signature before parsing JSON.** Use the raw bytes.
- **Rotate `secret` regularly** by `PUT`-ing a new `secret` on the
  webhook config. There is no overlap window today, so plan a short
  cutover on the receiver.
- **Use HTTPS.** `http://` is accepted but unsigned over the wire is a
  bad idea outside a trusted network.
- **Pin which events you subscribe to.** Don't subscribe to all four
  and filter on the receiver — extra deliveries are extra failure
  surface.

## See also

- [API overview](../reference/api-overview.md)
- [Configuration reference](../reference/configuration.md)
