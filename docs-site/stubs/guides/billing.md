---
title: "Billing"
---

# Billing

Raven uses [Hyperswitch](https://opensource.hyperswitch.io/) as its payment
gateway. Hyperswitch is an open-source unified payment-orchestration API that
routes each charge to a downstream processor — Razorpay for UPI, RuPay, and
domestic Indian cards; international card networks for everything else.

We did not pick Stripe. Stripe does not onboard merchants in India for our
use case, which is the home market for the Raven-hosted offering. Hyperswitch
gives us a single API surface while letting the routing layer pick the right
local rails per transaction.

This page is about how the Raven-hosted SaaS bills its customers. Customers
also pay LLM providers directly — that part of the bill is **not** routed
through Raven. See [LLM Providers](/guides/llm-providers) for the BYOK model.

## What gets billed

Raven only bills you for the platform itself. LLM token costs, embedding
costs, and any provider-level charges are between you and Anthropic / OpenAI
/ Cohere / your chosen provider.

| Charge | Frequency | Notes |
|---|---|---|
| Subscription plan (Free / Pro / Enterprise) | Monthly | Flat fee per organisation. Defined in `model.DefaultPlans()` (`internal/model/billing.go`). |
| Seats (users above plan limit) | Monthly | Enforced by quota check — see `MaxUsers` on each plan. Today over-limit users are blocked, not auto-billed. |
| Knowledge bases above plan limit | Monthly | Same shape as seats. `MaxKBs` is enforced; over-limit creation is blocked. |
| Storage above plan limit | Monthly | `MaxStorageMB`; enforced, not metered overage. |
| Voice minutes above plan limit | Monthly | `MaxVoiceMinutesMonthly`; enforced, not metered overage. |
| LLM tokens | n/a | Not billed by Raven. BYOK, paid to provider. |

The codebase is honest about its current shape: the plan structure exists,
the **quotas are enforced**, but **metered overage billing is not yet wired**.
Going over a limit returns `402 Payment Required` from `/billing/usage` and
related endpoints. Upgrading the plan is the supported path; per-unit
overage invoices are a future addition.

## Hyperswitch architecture

Raven's API server talks to Hyperswitch over HTTP. Hyperswitch then decides
which downstream processor handles the actual money movement.

```
┌──────────────┐   POST /payments   ┌──────────────┐   ┌───────────────┐
│  Raven API   │ ─────────────────▶ │ Hyperswitch  │ ─▶│  Razorpay     │
│  (Go, Gin)   │                    │ orchestrator │   │  (UPI/card)   │
│              │ ◀───── webhook ─── │              │ ─▶│  Card networks│
└──────────────┘  HMAC-SHA256 sig   └──────────────┘   └───────────────┘
```

The HTTP client lives at `internal/hyperswitch/client.go`. It is a thin
wrapper around `POST /payments`, `POST /payments/{id}/cancel`, and
`GET /payments/{id}`. The package doc-comment is the canonical description:

> Package hyperswitch provides an HTTP client for the Hyperswitch payment
> orchestration API. Hyperswitch acts as the routing layer and connects to
> downstream payment processors (e.g. Razorpay for UPI/card in India).

Three environment variables drive the integration. They are bound in
`internal/config/config.go` and consumed by the billing service:

| Env var | Maps to | Purpose |
|---|---|---|
| `RAVEN_HYPERSWITCH_BASE_URL` | `hyperswitch.base_url` | Hyperswitch instance URL. Defaults to `http://localhost:8090`. |
| `RAVEN_HYPERSWITCH_API_KEY` | `hyperswitch.api_key` | Merchant API key. Empty in dev defaults. |
| `RAVEN_HYPERSWITCH_WEBHOOK_SECRET` | `hyperswitch.webhook_secret` | HMAC-SHA256 secret for inbound webhook verification. |
| `RAVEN_HYPERSWITCH_RAZORPAY_KEY_ID` | `hyperswitch.razorpay_key_id` | Razorpay merchant key (used when Razorpay is the routed processor). |

If `RAVEN_HYPERSWITCH_WEBHOOK_SECRET` is empty, the webhook handler **rejects
every event** with the error `webhook signature verification is not
configured`. That is by design — silently accepting unsigned events would let
anyone with the URL activate subscriptions.

See [Configuration](/reference/configuration) for the full env-var reference.

## Plans and tiers

Three plans ship in the codebase today. Prices are in cents and live in
`model.DefaultPlans()`:

| Plan | ID | Monthly | Users | Workspaces | KBs | Storage | Voice mins/mo |
|---|---|---|---|---|---|---|---|
| Free | `plan_free` | $0.00 | 5 | 2 | 3 | 500 MB | 60 |
| Pro | `plan_pro` | $29.00 | 25 | 10 | 50 | 10 GB | 1 200 |
| Enterprise | `plan_enterprise` | $99.00 | unlimited | unlimited | unlimited | unlimited | unlimited |

Plans are returned by `GET /api/v1/billing/plans`. Frontend types live in
`frontend/src/api/billing.ts` (`PlanName = 'free' | 'pro' | 'enterprise'`).

Currency is reported in USD in code but Hyperswitch + Razorpay routing is
expected to convert / display INR for Indian merchants — the `payment_intents`
table defaults to `INR` (see `migrations/00033_payment_intents.sql`). The
canonical currency for a billed transaction is whatever the `PaymentIntent`
row stores.

## Subscription lifecycle

A subscription is one row in the `subscriptions` table (`migrations/00019_subscriptions.sql`).
The valid statuses are defined as a `CHECK` constraint in the migration and
as Go constants in `model.SubscriptionStatus`:

```
trialing ─┐
          ├─▶ active ─┬─▶ past_due ──▶ canceled
          │           │
          │           └─▶ paused ──▶ active
          │
          └─▶ canceled
                          expired
```

| Status | Set by | Meaning |
|---|---|---|
| `trialing` | Service (future) | Trial period; access granted, no charge. |
| `active` | Initial insert / webhook `payment_succeeded` | Plan is paid and current. |
| `past_due` | Webhook `payment_failed` | Renewal payment failed; access policy is per-handler. |
| `canceled` | `DELETE /billing/subscriptions/:id` or webhook `subscription_cancelled` | Tenant or processor cancelled. |
| `paused` | Reserved | Defined in schema; not currently emitted by the service. |
| `expired` | Reserved | Defined in schema; not currently emitted by the service. |

A partial unique index enforces **one active-ish subscription per org**:

```sql
CREATE UNIQUE INDEX idx_subscriptions_org_active
    ON subscriptions (org_id)
    WHERE status IN ('active', 'trialing', 'past_due');
```

Honest gap: the service today only emits `active` (on create), and transitions
to `canceled` or `past_due` driven by webhooks. `trialing`, `paused`, and
`expired` are schema-ready but not yet wired into a service path.

## Payment methods

Whatever Hyperswitch routes to. The two production routes wired today:

- **UPI, RuPay, and domestic Indian cards** via the Razorpay connector.
  `RAVEN_HYPERSWITCH_RAZORPAY_KEY_ID` is the merchant key.
- **International cards** via the default Hyperswitch connector chain. The
  exact routing rules live in the Hyperswitch dashboard, not in Raven.

The frontend opens the Hyperswitch SDK / Razorpay checkout using the
`client_secret` returned from `POST /api/v1/billing/payment-intents`. The
secret is a transient field on the `Subscription` and `PaymentIntent` models
(`json:"client_secret,omitempty"`); it is **never persisted**.

## Webhooks

Hyperswitch posts payment events to `POST /api/v1/billing/webhook`. This
endpoint **does not** use JWT auth — it verifies an `X-Webhook-Signature`
header (HMAC-SHA256 of the raw body, hex-encoded, keyed by
`RAVEN_HYPERSWITCH_WEBHOOK_SECRET`).

Implementation: `internal/handler/billing.go` (`(h *BillingHandler).Webhook`)
delegates to `internal/service/billing.go` (`HandleWebhook`). The handler
caps body size at 1 MB.

The payload shape is intentionally generic:

```go
type HyperswitchWebhookPayload struct {
    EventType string         `json:"event_type"`
    Content   map[string]any `json:"content"`
}
```

The service handles three event types and ignores the rest with an info log:

| `event_type` | Action |
|---|---|
| `payment_succeeded` | Insert into `payment_events` (idempotent by `(payment_id, event_type)`), extend `current_period_end` on the matching subscription. |
| `payment_failed` | Insert into `payment_events`, leave subscription state alone (move to `past_due` is handled elsewhere by caller policy). |
| `subscription_cancelled` | Insert into `payment_events`, mark subscription `canceled`. |

`payment_events` exists for **idempotency**: replaying any event is a no-op.
A unique index on `(payment_id, event_type)` blocks double-application. See
`migrations/00032_billing_rls_and_payment_events.sql`.

Both `subscriptions` and `payment_events` have row-level security enabled.
The webhook handler bypasses RLS for one transaction by calling
`SELECT set_config('app.bypass_rls', 'true', true)`, because the caller has
no org context. Org attribution comes from `content.metadata.org_id` in the
event payload, which Raven puts there when it creates the payment.

## Tax and invoicing

Honest about the MVP:

- **GST / VAT computation**: Not done by Raven. Hyperswitch / Razorpay are
  expected to handle this at the processor level for Indian transactions.
- **Invoice generation**: Not implemented. There is no `invoices` table and
  no PDF rendering path. Customers see line-items in the Hyperswitch /
  Razorpay receipt email.
- **Dunning** (retry-on-failure for `past_due`): Not implemented in Raven;
  whatever Hyperswitch does at the connector layer is what you get.
- **Proration on plan change**: Not implemented. Plan changes apply at the
  next period boundary; the migration doesn't track mid-period changes.

These are tracked gaps. If you need a tax-compliant invoice today, pull it
from the Hyperswitch dashboard, not the Raven UI.

## Test mode

Raven ships a local Hyperswitch stack under `deploy/hyperswitch/` for
development. It is **not** part of the main `docker-compose.yml` — you bring
it up separately:

```bash
docker compose -f deploy/hyperswitch/docker-compose.hyperswitch.yml up -d
```

That brings up:

- The Hyperswitch application server (port `8090` by default — matches the
  `RAVEN_HYPERSWITCH_BASE_URL` default).
- Its backing PostgreSQL.
- Its Redis instance.
- The Hyperswitch web client (control center) for managing connectors and
  API keys.

Default admin API key: `test_admin`. Merchant API keys are minted through
the dashboard or API once the stack is up.

For sandbox card / UPI numbers, use the test credentials documented at
[opensource.hyperswitch.io](https://opensource.hyperswitch.io/) — the test
data is upstream, not duplicated in this repo.

To test webhook delivery: configure the webhook destination in the
Hyperswitch dashboard to point at your Raven API's
`/api/v1/billing/webhook`. Use the dashboard's "send test event" feature
once `RAVEN_HYPERSWITCH_WEBHOOK_SECRET` matches on both sides.

## Self-host vs Raven-hosted

This page is about the **Raven-hosted SaaS**. If you are running Raven on
your own infrastructure — Docker Compose, an edge node, a Raspberry Pi — you
generally do **not** need Hyperswitch at all:

- There is no SaaS subscription to charge for; the software is yours.
- The `subscriptions` table will simply not be populated, and your org
  defaults to the implicit Free-tier subscription returned by
  `model.DefaultFreeSubscription()`.
- Plan limits still apply in code, but you can either change the defaults
  or run on Enterprise quotas (all limits `-1`).

Self-hosters can safely skip the rest of this guide. If you are building
your own multi-tenant offering on top of Raven and need billing of your own,
you can either point Raven at your own Hyperswitch tenant or wire a
different gateway behind the same `BillingService` interface — the
hyperswitch package is the only place the gateway choice leaks.

See [Self-Hosting → Docker Compose](/guides/self-hosting/docker-compose) for
the operator path that skips billing entirely.

## Related

- [Configuration reference](/reference/configuration) — full env-var list
  including the `RAVEN_HYPERSWITCH_*` variables.
- [Workspaces & Tenancy](/guides/workspaces-and-tenancy) — how plans, quotas,
  and org isolation tie together.
- [LLM Providers](/guides/llm-providers) — BYOK model for the
  Anthropic / OpenAI / Cohere bill that Raven does **not** collect.
- [Webhooks & Events](/guides/webhooks-and-events) — Raven's own outbound
  webhook system (separate from the inbound Hyperswitch webhook discussed
  here).
