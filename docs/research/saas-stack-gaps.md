# Raven Platform -- SaaS Stack Gap Analysis

**Date:** 2026-03-27
**Status:** Research document
**Purpose:** Identify every missing component needed for a fully functional, production-grade, multi-tenant SaaS product. Each gap includes rationale, recommended tool, priority classification, and resource impact for edge deployment.

---

## Priority Legend

| Label | Meaning | Timeline |
|-------|---------|----------|
| **MUST-HAVE** | Cannot launch without it; blocks MVP or creates legal/security liability | Before MVP launch |
| **SHOULD-HAVE** | Expected by paying customers; absence will cause churn or support burden | Before v1.0 GA |
| **NICE-TO-HAVE** | Enhances product maturity; absence is acceptable for early adopters | Future roadmap |

---

## Confirmed Stack (Reference)

Before cataloging gaps, here is what already exists:

| Category | Component |
|----------|-----------|
| Backend API | Go (Gin framework), pgx, sqlc, goose migrations |
| AI Workers | Python (gRPC), LangChain/LlamaIndex ecosystem |
| Frontend | Vue.js 3.5 + Tailwind CSS 4 + Tailwind Plus |
| CMS | Strapi 5 (Community, MIT) |
| Auth | Keycloak 26 + reavencloak custom SPI |
| Database | PostgreSQL 18 + pgvector + ParadeDB (or tsvector fallback) |
| Cache/Queue | Valkey 8.1 (BSD-3-Clause, Redis replacement) |
| Object Storage | SeaweedFS (Apache 2.0) or local FS |
| Document Parsing | LiteParse (Apache 2.0) |
| Web Scraping | Crawl4AI (Apache 2.0) |
| Voice STT | faster-whisper (MIT) |
| Voice TTS | Piper TTS (MIT archived) |
| Voice VAD | Silero VAD (MIT) |
| Voice/WebRTC | LiveKit Server + LiveKit Agents (Apache 2.0) |
| Reverse Proxy | Traefik 3.3 (MIT) |
| Analytics | PostHog |
| Observability | OpenObserve + OpenTelemetry |
| Security Scanning | Dependabot, CodeRabbit |
| Compliance Design | GDPR, SOC 2 |
| DB Migrations | goose v3.24 (MIT) -- already in tech stack |
| Config Management | viper (MIT) -- already in tech stack |

---

## Gap 1: Email / Transactional Notifications

### Why It Is Needed

A multi-tenant SaaS cannot operate without transactional email. Keycloak itself requires an SMTP relay for password reset and email verification flows. Beyond auth, Raven needs email for:

- User invitations (org admin invites a team member to a workspace)
- Password resets and MFA enrollment (delegated to Keycloak, but Keycloak needs SMTP)
- Document processing notifications (your upload is ready / failed)
- Usage alerts (approaching rate limits, API key expiration)
- System alerts to platform operators (failed jobs, disk space, etc.)

Without email, users have no way to recover accounts, and admins have no way to invite collaborators.

### Recommended Tool

**Listmonk** -- self-hosted newsletter and transactional email manager.

| Attribute | Detail |
|-----------|--------|
| License | AGPL-3.0 |
| Language | Go + PostgreSQL |
| AGPL Risk | Low -- Listmonk runs as a standalone service that Raven calls via API. Raven does not embed or modify Listmonk code. AGPL copyleft applies to Listmonk itself, not to Raven's code that calls it over HTTP. This is the same deployment model as using any AGPL web application as a service. However, if legal counsel is uncomfortable, alternatives below apply. |
| Alternative | **Mailtrain** (GPL-3.0, server-side so SaaS-safe) or skip the management UI and use a lightweight Go library (`gomail` / `go-mail`) to send directly via SMTP relay. |

For the SMTP relay itself (the actual delivery infrastructure):

| Option | Notes |
|--------|-------|
| **Postal** (MIT) | Self-hosted mail server with delivery tracking, bounce handling, IP reputation management. Heavy to operate. |
| **Mailpit** (MIT) | Development/testing SMTP trap. Not for production delivery. |
| **External relay** | AWS SES, Postmark, Resend, Mailgun. The pragmatic production choice -- deliverability is hard, and dedicated providers handle IP warming, DKIM/SPF/DMARC, bounce processing, and compliance. Cost: ~$0.10 per 1,000 emails. |

**Recommendation:** Use an external SMTP relay (AWS SES or Resend) for production deliverability, with Keycloak configured to use it directly. For application-level transactional emails (processing complete, invitations), build a thin Go email service using `go-mail` that templates and sends via the same relay. No need for Listmonk unless marketing emails are required.

### Priority

**MUST-HAVE (MVP)** -- Keycloak password resets and user invitations are blocked without SMTP.

### Resource Impact

Negligible if using an external relay. If self-hosting Postal: ~512 MB RAM, 1 CPU core, PostgreSQL database. Not suitable for edge deployment; email should always be cloud-hosted.

---

## Gap 2: Payment / Billing

### Why It Is Needed

Raven is a SaaS product. Without billing, there is no revenue. Multi-tenant platforms need:

- Subscription plan management (free tier, pro, enterprise)
- Usage-based metering (documents processed, chat queries, storage consumed)
- Invoice generation and tax compliance
- Payment method management (credit cards, wire transfers)
- Dunning (failed payment retries, grace periods, account suspension)
- Self-service plan upgrades/downgrades
- Webhook-driven account provisioning (payment succeeds -> org becomes active)

### Recommended Approach

**Stripe** (proprietary, industry standard) for payment processing -- there is no viable open-source alternative for actual payment processing (PCI DSS compliance alone makes self-hosting prohibitive).

For billing orchestration and usage metering on top of Stripe:

| Tool | License | Description |
|------|---------|-------------|
| **Lago** | AGPL-3.0 (core) | Open-source billing and metering. Usage-based billing, subscription management, invoice generation. Integrates with Stripe for payment collection. Same AGPL deployment model as Listmonk -- runs as a standalone service, Raven calls via API. |
| **Kill Bill** | Apache 2.0 | Open-source subscription billing. Java-based, heavier footprint. More mature but more complex. |
| **OpenMeter** | Apache 2.0 | Usage metering only (not full billing). Feeds into Stripe or Lago. Lightweight Go service. |

**Recommendation:** For MVP, integrate Stripe directly using their Billing and Checkout APIs. The Go API server manages Stripe customer/subscription objects and listens for Stripe webhooks. This avoids deploying a separate billing service initially.

When usage-based billing becomes necessary (Phase 2), evaluate Lago or OpenMeter. Lago AGPL-3.0 risk is low as a standalone service, but if legal prefers Apache 2.0, OpenMeter (metering) + Stripe (billing) is fully permissive.

### Priority

**MUST-HAVE (MVP)** -- Cannot charge customers without it. Even a free-tier MVP needs Stripe integration designed into the org/subscription data model from day one to avoid painful retrofitting.

### Resource Impact

Stripe is SaaS (zero self-hosted resources). Lago, if deployed later: ~1 GB RAM, PostgreSQL database, Redis. Not for edge.

---

## Gap 3: Error Tracking / Crash Reporting

### Why It Is Needed

OpenObserve + OpenTelemetry handle logs, metrics, and traces -- but error tracking is a distinct concern. Error tracking provides:

- Automatic grouping of exceptions by stack trace signature
- Per-release regression detection ("this error is new in v1.2.3")
- Assignment of errors to team members
- Integration with issue trackers (GitHub Issues, Linear)
- Source map support for Vue.js frontend errors
- User impact analysis ("this error affected 47 unique orgs")

OpenTelemetry can capture exceptions as span events, but it lacks the deduplication, grouping, alerting, and workflow features that a dedicated error tracker provides.

### Recommended Tool

**GlitchTip** -- self-hosted error tracking, Sentry-compatible.

| Attribute | Detail |
|-----------|--------|
| License | MIT |
| Language | Python (Django) |
| Compatibility | Sentry SDK compatible -- use the official Sentry SDKs for Go, Python, and Vue.js, pointed at the GlitchTip server |
| Features | Error grouping, release tracking, uptime monitoring, performance monitoring (basic) |
| Alternative | **Sentry self-hosted** (BSL-1.1, converts to Apache 2.0 after 36 months). More features than GlitchTip but heavier and BSL-licensed. |

**Recommendation:** Deploy GlitchTip (MIT, lightweight) for self-hosted error tracking. Use official Sentry SDKs (`sentry-go`, `sentry-sdk` for Python, `@sentry/vue` for Vue.js) pointed at the GlitchTip endpoint. If GlitchTip's feature set proves insufficient, evaluate Sentry self-hosted or Sentry cloud (SaaS).

### Priority

**SHOULD-HAVE (v1.0)** -- During MVP development, console logs and OpenTelemetry traces suffice. Before GA with paying customers, structured error tracking is essential for operational maturity.

### Resource Impact

GlitchTip: ~512 MB RAM, PostgreSQL database (can share with Raven's existing PostgreSQL or use a separate instance), Redis. Moderate footprint. Not for edge deployment.

---

## Gap 4: API Documentation

### Why It Is Needed

Raven exposes a REST API consumed by:

- The Vue.js SPA (primary frontend)
- The `<raven-chat>` embeddable web component
- Customer integrations (webhooks, API keys)
- Third-party developers building on top of Raven

Without generated, versioned API documentation, every integration becomes a support ticket.

### Recommended Approach

The Go API server using Gin already has a natural path to API docs:

| Tool | License | Description |
|------|---------|-------------|
| **swaggo/swag** | MIT | Generates OpenAPI 2.0/3.0 spec from Go struct annotations and Gin route definitions. Produces `swagger.json` at build time. |
| **Scalar** | MIT | Modern, beautiful API reference UI. Drop-in replacement for Swagger UI. Reads OpenAPI spec, renders interactive docs. |

**Recommendation:**

1. Add `swaggo/swag` annotations to Gin route handlers. Run `swag init` in CI to generate `docs/swagger.json`.
2. Serve Scalar UI at `/api/docs` from the Go API server, loading the generated spec. Zero external dependencies.
3. Publish the OpenAPI spec as a versioned artifact alongside each release.

The Python AI Worker's gRPC interface should have its `.proto` files published in the repository with `buf` linting, but does not need a public developer portal (it is an internal interface).

### Priority

**SHOULD-HAVE (v1.0)** -- API consumers need docs before Raven can have external integrations. For MVP, the Vue.js frontend team can work from `.proto` files and route definitions directly.

### Resource Impact

Zero additional runtime resources. `swag init` runs at build time. Scalar is a static JS bundle served from the existing Go API server.

---

## Gap 5: CDN / Static Asset Delivery

### Why It Is Needed

Two assets need global distribution:

1. **Vue.js SPA** -- the admin dashboard. JS/CSS bundles, fonts, images.
2. **`<raven-chat>` web component** -- the embeddable chatbot widget JS bundle. Loaded by customers' websites worldwide; latency directly impacts their page load time.

Without a CDN, every static asset request hits the origin server, increasing latency for geographically distant users and increasing load on Traefik.

### Recommended Approach

| Option | License / Model | Description |
|--------|----------------|-------------|
| **Cloudflare (free tier)** | Proprietary (free) | Reverse-proxy CDN. Free tier includes unlimited bandwidth, global edge caching, DDoS protection. Point DNS at Cloudflare, configure cache rules. |
| **Bunny CDN** | Proprietary (paid) | Pull-zone CDN. ~$0.01/GB. Simple, fast, no vendor lock-in. |
| **Traefik + Cache-Control headers** | Already in stack | For self-hosted/edge deployments, configure aggressive `Cache-Control` headers on static assets. Traefik can serve cached responses from Valkey or a file cache plugin. Not a true CDN but sufficient for single-region deployments. |

**Recommendation:** For the SaaS offering, place Cloudflare in front of Traefik. The `<raven-chat>` widget bundle is the most latency-sensitive asset and benefits most from edge caching. For self-hosted deployments, rely on Traefik cache headers plus the customer's own CDN.

### Priority

**SHOULD-HAVE (v1.0)** -- MVP can serve static assets directly from Traefik. Before scaling to multiple customer geographies, a CDN is needed.

### Resource Impact

Zero self-hosted resources (Cloudflare is SaaS). If using Traefik caching plugin, minimal additional memory.

---

## Gap 6: SSL/TLS Certificate Management

### Why It Is Needed

All traffic must be encrypted. Keycloak, the Go API, Strapi, and the Vue.js SPA all need valid TLS certificates. Without automated certificate management, certificates expire silently, causing outages.

### Current State

Traefik has built-in Let's Encrypt support via its ACME provider. This is likely already planned but needs explicit confirmation and configuration.

### Recommended Approach

**Traefik ACME (Let's Encrypt)** -- already in the stack.

| Configuration | Detail |
|---------------|--------|
| Challenge type | `TLS-ALPN-01` (preferred, no port 80 needed) or `HTTP-01` |
| Storage | File-based (`acme.json`) or Consul/etcd for HA |
| Wildcard certs | Requires `DNS-01` challenge with a supported DNS provider (Cloudflare, Route53, etc.) |
| Renewal | Automatic, 30 days before expiry |

**Recommendation:** Configure Traefik's built-in ACME resolver for Let's Encrypt certificates. Use `DNS-01` challenge with Cloudflare DNS for wildcard certificates (`*.raven.example.com`). This is a configuration task, not a new dependency.

For edge/self-hosted deployments where customers bring their own domain, document the Traefik ACME configuration in the deployment guide.

### Priority

**MUST-HAVE (MVP)** -- No production deployment should serve unencrypted traffic. This is a Traefik configuration item, not a new tool.

### Resource Impact

Zero additional resources. Traefik handles it natively.

---

## Gap 7: Backup Strategy

### Why It Is Needed

Data loss in a multi-tenant SaaS is existential. Customer trust requires demonstrated backup and recovery capability. SOC 2 compliance (already targeted) mandates documented backup procedures.

### Components Requiring Backup

| Component | Data at Risk | Backup Method |
|-----------|-------------|---------------|
| **PostgreSQL** | All application data, embeddings, tenant data | Logical + physical backups |
| **SeaweedFS** | Uploaded documents (PDFs, DOCX, etc.) | Object replication + snapshots |
| **Valkey** | Job queue state (ephemeral, but helpful for recovery) | RDB snapshots (optional) |
| **Keycloak** | User identities, realm config | PostgreSQL backup (Keycloak uses PG as its store) |

### Recommended Tools

| Tool | License | Purpose |
|------|---------|---------|
| **pgBackRest** | MIT | PostgreSQL backup with incremental backups, parallel compression, point-in-time recovery (PITR), and S3/GCS/Azure storage targets. Industry standard for PG backups. |
| **WAL-G** | Apache 2.0 | Alternative to pgBackRest. Simpler, Go-based, supports S3/GCS. Good for edge deployments due to small binary size. |
| **SeaweedFS replication** | Built-in | SeaweedFS supports configurable replication (e.g., `001` = 1 copy on a different rack). For disaster recovery, schedule periodic `weed export` to a separate storage target. |
| **Restic** | BSD-2-Clause | General-purpose encrypted backup tool. Can back up SeaweedFS FUSE mount or local filesystem to S3, B2, or local disk. Deduplication and encryption built in. |

**Recommendation:**

1. **PostgreSQL:** Deploy pgBackRest with daily full backups, continuous WAL archiving for PITR, and a 30-day retention policy. Store backups in a separate S3 bucket (or SeaweedFS bucket in a different availability zone).
2. **SeaweedFS:** Enable replication factor of at least 2. Schedule daily `restic` backups of critical buckets to a separate storage target.
3. **Valkey:** Enable RDB snapshots every 15 minutes. Valkey data is ephemeral (job queue), so loss is recoverable by re-enqueueing failed jobs.
4. **Test restores monthly.** A backup that has never been tested is not a backup.

### Priority

**MUST-HAVE (MVP)** -- Data loss before launch is embarrassing; data loss after launch with paying customers is catastrophic. SOC 2 requires documented backup/restore procedures.

### Resource Impact

pgBackRest: ~256 MB RAM during backup operations, minimal when idle. Storage cost depends on retention and database size. Restic: ~128 MB RAM during backup runs. Both are scheduled batch processes, not persistent services. Suitable for edge if backing up to remote storage.

---

## Gap 8: Rate Limiting (Explicit Design)

### Why It Is Needed

Valkey is in the stack and rate limiting is mentioned in the `api_keys` table (`rate_limit` column) and `organizations.settings` JSONB. However, the actual rate limiting middleware and strategy need explicit design:

- Per-tenant rate limits (org-level: prevent one org from starving others)
- Per-API-key rate limits (widget-level: each chatbot embed has its own quota)
- Per-endpoint rate limits (protect expensive endpoints like `/chat` and `/documents/upload`)
- Global rate limits (DDoS protection at the Traefik level)

### Recommended Approach

| Layer | Tool | Strategy |
|-------|------|----------|
| **Traefik** (edge) | Traefik RateLimit middleware (built-in) | Global per-IP rate limiting. First line of defense against DDoS. |
| **Go API** (application) | Custom middleware using Valkey + sliding window counter | Per-org and per-API-key rate limiting. Read limits from `organizations.settings` and `api_keys.rate_limit`. Use Valkey `INCR` + `EXPIRE` for sliding window counters. |
| **Python AI Worker** | Semaphore / token bucket in gRPC interceptor | Limit concurrent AI operations per org to prevent GPU/LLM API starvation. |

**Recommendation:** This is an architectural design task, not a new tool. The building blocks (Valkey, Traefik) are already in the stack. The work is:

1. Define the rate limit tiers per plan (free: 100 req/min, pro: 1,000 req/min, enterprise: custom).
2. Implement a Go middleware that reads org/API-key limits from Valkey-cached config and enforces sliding window counters.
3. Return standard `429 Too Many Requests` with `Retry-After` header.
4. Configure Traefik's built-in rate limiter for global per-IP protection.

### Priority

**MUST-HAVE (MVP)** -- Without rate limiting, a single abusive tenant can exhaust LLM API quotas (which cost real money via BYOK) and degrade service for all tenants.

### Resource Impact

Zero additional services. Valkey already handles the counter storage. The middleware adds microseconds of latency per request.

---

## Gap 9: Feature Flags

### Why It Is Needed

Feature flags enable:

- Gradual rollout of new features to specific orgs or percentage of users
- A/B testing of UI changes
- Kill switches for problematic features without redeployment
- Plan-gated features (enterprise-only capabilities)

### Current State

PostHog (already added to the stack) includes a feature flags module with:

- Boolean and multivariate flags
- Percentage-based rollout
- User/group targeting (can target by org_id)
- Local evaluation SDK for Go and JavaScript (no network call per flag check)

### Recommendation

**PostHog Feature Flags** -- already in the stack. No new tool needed.

Action items:

1. Integrate the PostHog Go SDK (`posthog-go`) into the Go API server for server-side flag evaluation.
2. Integrate the PostHog JavaScript SDK into the Vue.js frontend for client-side flags.
3. Define a naming convention for flags: `feature.<area>.<name>` (e.g., `feature.voice.webrtc-agent`).
4. Use org_id as the group key for per-tenant targeting.

### Priority

**SHOULD-HAVE (v1.0)** -- MVP can hard-code feature gates. Before GA, flags are needed for safe rollouts.

### Resource Impact

Zero additional services. PostHog is already deployed. SDK adds ~2 MB to the Go binary and performs local flag evaluation (no per-request API call).

---

## Gap 10: Status Page / Uptime Monitoring

### Why It Is Needed

Paying customers expect transparency when services are degraded. A public status page:

- Communicates real-time system health (API, chatbot, voice, dashboard)
- Provides incident history and resolution timelines
- Reduces support volume during outages ("Is it just me or is Raven down?")
- Is a SOC 2 expectation for incident communication

### Recommended Tool

**Upptime** -- open-source uptime monitor and status page powered by GitHub Actions.

| Attribute | Detail |
|-----------|--------|
| License | MIT |
| Hosting | GitHub Pages (free) -- no server needed |
| Monitoring | GitHub Actions cron jobs ping endpoints every 5 minutes |
| Incident management | GitHub Issues as incident tracker |
| Customization | YAML config, custom branding |
| Alternative | **Gatus** (Apache 2.0) -- self-hosted health dashboard with alerting. More powerful but requires a server. |

**Recommendation:** Start with Upptime for zero infrastructure cost. It monitors HTTP endpoints, generates a static status page on GitHub Pages, and uses GitHub Issues for incident communication. If more sophistication is needed (internal health checks, dependency graphs), migrate to Gatus.

### Priority

**SHOULD-HAVE (v1.0)** -- Not needed for MVP, but expected by paying customers at GA.

### Resource Impact

Upptime: zero self-hosted resources (runs on GitHub Actions + GitHub Pages). Gatus: ~64 MB RAM, single binary.

---

## Gap 11: User-Facing Documentation Site

### Why It Is Needed

Customers need:

- Getting started guides (how to create an org, set up a workspace, upload documents)
- Embeddable chatbot integration guide (how to add `<raven-chat>` to their website)
- API reference (linked from the OpenAPI spec)
- Voice agent configuration guide
- Admin and billing documentation
- Troubleshooting and FAQ

Strapi is in the stack as a CMS, but it is positioned for marketing content and admin tooling, not structured technical documentation with versioning, search, and code samples.

### Recommended Tool

**VitePress** -- static site generator powered by Vite and Vue.

| Attribute | Detail |
|-----------|--------|
| License | MIT |
| Language | Vue.js (aligns with frontend stack) |
| Features | Markdown-based, Vue components in docs, full-text search (built-in or Algolia), dark mode, versioning via git branches |
| Build output | Static HTML/CSS/JS, deployable to any CDN or GitHub Pages |
| Alternative | **Docusaurus** (MIT, React-based). Excellent but React introduces a second frontend framework. VitePress aligns with the Vue.js stack. |

**Recommendation:** Use VitePress. Write docs in Markdown, co-locate in the repository under `docs/`, build in CI, deploy to GitHub Pages or the CDN. The Vue.js alignment means custom interactive components (embedded API playground, code samples) use the same framework as the main product.

### Priority

**SHOULD-HAVE (v1.0)** -- MVP can launch with a README and basic guides. GA requires a polished documentation site.

### Resource Impact

Zero runtime resources. Static site deployed to CDN/GitHub Pages. Build is a CI step (~30 seconds).

---

## Gap 12: Changelog / Release Notes

### Why It Is Needed

Customers and internal stakeholders need to know what changed in each release. A changelog:

- Communicates new features, bug fixes, and breaking changes
- Builds trust with customers (the product is actively maintained)
- Is required for API versioning communication
- Supports SOC 2 change management documentation

### Recommended Approach

| Tool | License | Description |
|------|---------|-------------|
| **Conventional Commits** | -- | Commit message convention (`feat:`, `fix:`, `breaking:`) that enables automated changelog generation. |
| **git-cliff** | MIT (Apache 2.0 dual) | Generates changelogs from conventional commits. Written in Rust, single binary. Highly customizable templates. |
| **GitHub Releases** | Built-in | Publish release notes on each tagged release. CI can auto-generate from git-cliff output. |

**Recommendation:**

1. Enforce Conventional Commits via a pre-commit hook or CI check.
2. Run `git-cliff` in CI on each release tag to generate `CHANGELOG.md`.
3. Publish the generated notes as a GitHub Release.
4. Optionally embed the changelog in the VitePress docs site.

### Priority

**NICE-TO-HAVE (future)** -- Can be manual for MVP and v1.0. Automate when release cadence increases.

### Resource Impact

Zero runtime resources. `git-cliff` runs in CI only.

---

## Gap 13: Legal Pages

### Why It Is Needed

GDPR compliance (already targeted) requires:

- **Privacy Policy** -- what data is collected, how it is used, retention periods, data subject rights
- **Terms of Service** -- contractual terms governing use of the platform
- **Cookie Consent** -- banner/modal for cookie opt-in (required in EU/UK)
- **Data Processing Agreement (DPA)** -- required when processing data on behalf of customers (Raven processes customer documents)
- **Acceptable Use Policy** -- what content/use is prohibited

These are legal documents, not engineering artifacts, but they require tooling integration:

- Cookie consent banner in the Vue.js SPA
- Consent storage (Valkey or PostgreSQL) for audit trail
- "Accept ToS" checkbox in the registration flow (Keycloak theming)

### Recommended Tools

| Component | Tool | License |
|-----------|------|---------|
| Cookie consent banner | **vue-cookieconsent** or **cookieconsent** (Osano) | MIT |
| Legal document hosting | VitePress docs site or Strapi pages | Already in stack |
| Consent audit trail | PostgreSQL table (`consent_records`) | Already in stack |

**Recommendation:** The legal documents themselves should be written by a lawyer with SaaS experience. The engineering work is:

1. Add a cookie consent banner to the Vue.js SPA using `cookieconsent` (MIT).
2. Store consent records in PostgreSQL (user_id, consent_type, version, timestamp, ip_address).
3. Add a "I accept the Terms of Service" checkbox to Keycloak's registration theme.
4. Host legal pages on the VitePress docs site or as Strapi-managed content.

### Priority

**MUST-HAVE (MVP)** -- Cannot launch a SaaS product that collects user data (especially in the EU) without Privacy Policy and Terms of Service. GDPR fines are up to 4% of global revenue or 20 million euros.

### Resource Impact

Negligible. Cookie consent is a frontend JS library (~15 KB). Consent records are a lightweight PostgreSQL table.

---

## Gap 14: Webhook System

### Why It Is Needed

Raven processes documents asynchronously. Customers integrating via API need to be notified when events occur:

- Document processing completed / failed
- Knowledge base reindexing finished
- New chat session started (for CRM integration)
- API key approaching rate limit
- Source URL re-crawl completed

Without webhooks, customers must poll the API for status updates, which is inefficient and creates unnecessary load.

### Recommended Approach

This is an architectural pattern, not a third-party tool. The Go API server should implement a webhook dispatch system:

| Component | Implementation |
|-----------|---------------|
| **Webhook registration** | API endpoint: `POST /api/v1/orgs/{org}/webhooks` with `url`, `events[]`, `secret` |
| **Event dispatch** | Valkey queue: `raven:webhooks:dispatch`. When an event occurs (e.g., document status -> ready), enqueue a webhook delivery job. |
| **Delivery** | Go worker consumes from the queue, sends HTTP POST with JSON payload and HMAC-SHA256 signature header (`X-Raven-Signature`). |
| **Retry** | Exponential backoff (1s, 5s, 30s, 5m, 30m) with max 5 attempts. Dead-letter after final failure. |
| **Logging** | `webhook_deliveries` table: event_id, url, status_code, response_body (truncated), attempt_count, next_retry_at. |

**Data model addition:**

```
webhook_endpoints (id, org_id, url, secret_hash, events[], status, created_by, created_at)
webhook_deliveries (id, webhook_endpoint_id, event_type, payload, status_code, attempts, next_retry_at, created_at)
```

### Priority

**SHOULD-HAVE (v1.0)** -- MVP customers can poll. API customers at GA expect webhooks.

### Resource Impact

Zero new services. Uses existing Valkey queue and Go worker. The webhook delivery table adds marginal PostgreSQL storage. Suitable for edge deployment.

---

## Gap 15: API Versioning

### Why It Is Needed

Once external customers integrate with Raven's REST API, breaking changes (renamed fields, removed endpoints, changed response shapes) will break their integrations. An API versioning strategy must be decided before the first public API release because retrofitting is extremely painful.

### Recommended Approach

**URL-path versioning** (`/api/v1/...`, `/api/v2/...`).

| Aspect | Decision |
|--------|----------|
| Versioning scheme | Major version in URL path: `/api/v1/`, `/api/v2/` |
| When to bump | Only for breaking changes (removed fields, changed types, removed endpoints) |
| Deprecation policy | Announce deprecation 6 months before removal. Add `Sunset` header (RFC 8594) to responses from deprecated endpoints. |
| Non-breaking changes | Add new fields, new endpoints, new enum values without bumping version. |
| Implementation | Gin route groups: `v1 := r.Group("/api/v1")`, `v2 := r.Group("/api/v2")`. Shared handlers where possible, version-specific adapters where needed. |

**Recommendation:** This is a design decision, not a tool. Lock in `/api/v1/` for the MVP. Document the versioning policy in the API docs. Use the `swaggo/swag` annotations to generate per-version OpenAPI specs.

### Priority

**MUST-HAVE (MVP)** -- The URL prefix `/api/v1/` must be in place from day one. The actual versioning policy and tooling can mature over time, but the URL structure cannot change retroactively.

### Resource Impact

Zero. This is a routing convention.

---

## Gap 16: Admin Search (Cross-Entity)

### Why It Is Needed

Platform administrators and org admins need to search across organizations, workspaces, knowledge bases, documents, and users. Current search infrastructure (pgvector + ParadeDB/tsvector) is designed for RAG chunk retrieval, not for admin UI search like "find all documents uploaded by user X" or "search orgs by name."

### Recommended Approach

**PostgreSQL native search** -- no new tool needed.

| Use Case | Implementation |
|----------|---------------|
| Admin search (orgs, users, workspaces) | `ILIKE` queries on `name`, `email`, `slug` columns with trigram indexes (`pg_trgm` extension) for fuzzy matching |
| Document search within KB | Already handled by ParadeDB BM25 / tsvector on chunks |
| Full-text search on document titles and metadata | `tsvector` index on `documents.title` + `documents.metadata` |

```sql
-- Trigram index for fuzzy admin search
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX idx_orgs_name_trgm ON organizations USING gin (name gin_trgm_ops);
CREATE INDEX idx_users_email_trgm ON users USING gin (email gin_trgm_ops);
```

**Recommendation:** Use PostgreSQL `pg_trgm` for admin UI search (fuzzy name/email matching) and existing ParadeDB/tsvector for document content search. No new service needed.

### Priority

**SHOULD-HAVE (v1.0)** -- MVP admin can browse lists. GA needs search for operational efficiency.

### Resource Impact

Negligible. `pg_trgm` indexes add marginal storage. No new services.

---

## Gap 17: File Virus Scanning

### Why It Is Needed

Users upload arbitrary files (PDF, DOCX, images) to knowledge bases. Without malware scanning:

- A compromised file could be served back to other users
- Malicious documents could exploit parsing vulnerabilities in LiteParse/Tesseract
- SOC 2 compliance expects input validation including malware scanning
- Enterprise customers will ask "do you scan uploaded files?" during security questionnaires

### Recommended Tool

**ClamAV** -- open-source antivirus engine.

| Attribute | Detail |
|-----------|--------|
| License | GPL-2.0 (server-side, SaaS-safe -- not distributed to users) |
| Language | C/C++ |
| Integration | `clamd` daemon with TCP socket. Go client libraries available (`go-clamd`). |
| Signature updates | `freshclam` daemon updates virus definitions automatically. |
| Alternative | **VirusTotal API** (proprietary, cloud). Higher detection rate but uploads files to a third party -- unacceptable for customer-confidential documents. |

**Recommendation:** Deploy ClamAV (`clamd` + `freshclam`) as a sidecar container. In the Go API document upload flow, before storing to SeaweedFS, stream the file to `clamd` for scanning. Reject files that fail scanning with a `422 Unprocessable Entity` response. Log scan results in the `processing_events` audit table.

**Integration point in the upload flow:**

```
Client upload --> Go API --> ClamAV scan --> SeaweedFS store --> Valkey enqueue
                                |
                           (reject if malware detected)
```

### Priority

**SHOULD-HAVE (v1.0)** -- MVP can launch without it (low initial user count, controlled beta). Before GA with enterprise customers, virus scanning is expected.

### Resource Impact

ClamAV: ~1 GB RAM (virus signature database is loaded into memory), 1 CPU core. The signature database is ~300 MB on disk. This is heavy for edge deployment. For edge mode, skip virus scanning or offload to the cloud component.

---

## Gap 18: CORS / Security Headers

### Why It Is Needed

The `<raven-chat>` web component is embedded on customer websites (different origins). Without proper CORS configuration, browsers will block API requests from the widget. Additionally, security headers protect against:

- Clickjacking (`X-Frame-Options`, `Content-Security-Policy: frame-ancestors`)
- XSS (`Content-Security-Policy`, `X-Content-Type-Options`)
- Protocol downgrade attacks (`Strict-Transport-Security`)
- Information leakage (`X-Powered-By` removal, `Referrer-Policy`)

### Recommended Approach

This is a configuration task across Traefik and the Go API server:

| Header | Configuration |
|--------|--------------|
| **CORS** | `gin-contrib/cors` with `AllowOriginFunc` for dynamic origin validation. Admin dashboard: restrict to dashboard origin. Chatbot widget: `AllowOriginFunc` reads `api_keys.allowed_domains` from Valkey cache and validates per-request. |
| **HSTS** | Traefik middleware: `headers.stsSeconds=63072000, headers.stsIncludeSubdomains=true, headers.stsPreload=true` |
| **CSP** | Traefik middleware or Go response header. Restrictive policy for dashboard; looser for widget embed context. |
| **X-Content-Type-Options** | `nosniff` -- prevents MIME-type sniffing |
| **X-Frame-Options** | `DENY` for dashboard, `ALLOWALL` is not needed (widget is loaded via `<script>`, not `<iframe>`) |
| **Referrer-Policy** | `strict-origin-when-cross-origin` |
| **Permissions-Policy** | Restrict camera, microphone, geolocation access (except microphone for voice agent) |

**Recommendation:** Configure Traefik's `headers` middleware for global security headers. Implement dynamic CORS in the Go API's Gin middleware, reading per-API-key domain allowlists from Valkey-cached `api_keys.allowed_domains`. This is the highest-priority security gap because the embeddable chatbot literally will not work without CORS.

### Priority

**MUST-HAVE (MVP)** -- The `<raven-chat>` widget will not function without CORS. Security headers are table-stakes for any web application.

### Resource Impact

Zero. Configuration only.

---

## Gap 19: Scheduled Jobs / Cron

### Why It Is Needed

Several operations in Raven require periodic execution:

- **Re-crawl web sources** -- sources with `crawl_frequency` = daily/weekly/monthly need scheduled re-crawling
- **Cleanup expired sessions** -- anonymous chat sessions have a 24h TTL (`expires_at`)
- **Cleanup expired API keys** -- revoke keys past `expires_at`
- **Virus definition updates** -- if ClamAV is deployed, `freshclam` runs on schedule
- **Backup jobs** -- pgBackRest, restic
- **Certificate renewal checks** -- Traefik handles this, but monitoring is needed
- **Usage aggregation** -- roll up per-org usage metrics for billing
- **Dead-letter queue processing** -- alert on or retry failed webhook deliveries

### Recommended Approach

| Option | License | Description |
|--------|---------|-------------|
| **Go `robfig/cron`** | MIT | In-process cron scheduler for Go. Runs inside the Go API server. Simple, no external dependency. |
| **Valkey-based distributed scheduler** | Built-in | Use Valkey sorted sets with timestamps as scores. A Go worker polls for due jobs. Supports distributed locking to prevent duplicate execution in multi-instance deployments. |
| **Asynq** | MIT | Go library for distributed task processing with Valkey. Includes a cron scheduler, retry logic, and a web UI (Asynqmon). Built specifically for Valkey/Redis. |

**Recommendation:** Use **Asynq** (MIT, Go, Valkey-backed). It provides both ad-hoc job queuing and cron scheduling in a single library, which consolidates the current Valkey job queue design (`raven:jobs:*`) with scheduled task needs. Asynq supports:

- Cron-style periodic tasks (`asynq.NewPeriodicTaskManager`)
- Distributed locking (safe for multi-instance deployments)
- Retry with exponential backoff
- Dead-letter queues
- Web UI for monitoring (Asynqmon)

This replaces the need for a custom Valkey job queue implementation while adding cron capability.

### Priority

**MUST-HAVE (MVP)** -- Web source re-crawling (`crawl_frequency`) is a core feature. Session cleanup prevents database bloat. Without scheduled jobs, these features do not work.

### Resource Impact

Asynq runs inside the Go process. Zero additional services (uses existing Valkey). Asynqmon web UI: ~64 MB RAM (optional, for development/staging only).

---

## Gap 20: Database Migrations

### Already Addressed

**goose v3.24 (MIT)** is already listed in the final design specification's dependency table (row #8). This gap is closed.

Confirmation: goose is a Go-native migration tool that supports SQL and Go migrations, up/down migrations, and versioned migration files. It integrates cleanly with the pgx driver already in the stack.

### No Action Required

---

## Gap 21: Secrets Management

### Why It Is Needed

The final design spec mentions that LLM API key encryption uses "a master key in secrets manager (AWS Secrets Manager, HashiCorp Vault, or env var fallback)." This needs to be made concrete. In production:

- The database encryption master key cannot live in a `.env` file (if the server is compromised, the `.env` file is the first thing an attacker reads)
- Keycloak admin credentials, PostgreSQL passwords, SeaweedFS keys, and Valkey passwords all need secure storage
- Secret rotation must be possible without downtime
- SOC 2 requires documented secrets management procedures

### Recommended Tool

**Infisical** -- open-source secrets management platform.

| Attribute | Detail |
|-----------|--------|
| License | MIT (core) |
| Language | TypeScript (server), Go SDK available |
| Features | Secret versioning, rotation, audit log, RBAC, environment-specific secrets, Kubernetes integration, Docker integration |
| Deployment | Self-hosted (Docker Compose) or Infisical Cloud |
| Alternative 1 | **HashiCorp Vault** (BSL-1.1 since v1.15, August 2023). Extremely mature but BSL-licensed. For self-hosted use without competing with HashiCorp, BSL is acceptable. However, the license change has caused community fragmentation. |
| Alternative 2 | **OpenBao** (MPL-2.0). Community fork of Vault created after the BSL change. API-compatible with Vault. Still maturing (v2.1 as of early 2026). |
| Alternative 3 | **SOPS** (MPL-2.0, Mozilla). Encrypts secrets in YAML/JSON/ENV files using AWS KMS, GCP KMS, or age keys. Simple, no server needed. Good for small deployments. |

**Recommendation for production SaaS:** Deploy Infisical (MIT) for centralized secrets management. The Go API reads secrets via the Infisical Go SDK or environment variables injected by the Infisical agent.

**Recommendation for edge/self-hosted:** Use SOPS with age encryption for encrypting secrets files. No server dependency.

**Recommendation for MVP:** Use environment variables injected via Docker Compose secrets or Docker Swarm secrets. Graduate to Infisical before GA.

### Priority

**SHOULD-HAVE (v1.0)** -- MVP can use Docker secrets / env vars with appropriate file permissions. Before GA (and for SOC 2 compliance), a proper secrets manager with audit logging is needed.

### Resource Impact

Infisical: ~512 MB RAM, PostgreSQL database (can share), Redis/Valkey. SOPS: zero runtime resources (CLI tool, runs at deploy time).

---

## Gap 22: Load Testing

### Why It Is Needed

Before launch, Raven needs to answer:

- How many concurrent chat sessions can one instance handle?
- What is the p99 latency for RAG queries under load?
- At what point does the PostgreSQL connection pool saturate?
- How does the HNSW index perform with 1M embeddings and 100 concurrent queries?
- What is the document processing throughput (documents/hour)?

Without load testing, capacity planning is guesswork, and the first traffic spike will reveal bottlenecks in production.

### Recommended Tool

**k6** -- modern load testing tool.

| Attribute | Detail |
|-----------|--------|
| License | AGPL-3.0 (core CLI tool) |
| Language | Go (runtime), JavaScript (test scripts) |
| AGPL Risk | k6 is a CLI tool run by developers/CI, not deployed as part of Raven. AGPL does not apply -- you are not providing k6 as a network service. |
| Features | HTTP, WebSocket, gRPC protocol support. Scripting in JavaScript. Threshold-based pass/fail. CI integration. |
| Alternative | **Locust** (MIT, Python). Good alternative if the team prefers Python. Less built-in protocol support than k6. |

**Recommendation:** Use k6 for load testing. Write test scripts that simulate:

1. Concurrent document uploads (test ingestion pipeline throughput)
2. Concurrent RAG chat queries (test gRPC + PostgreSQL + LLM API under load)
3. WebSocket SSE streaming (test concurrent chat streaming connections)
4. Mixed workload (uploads + queries + admin operations simultaneously)

Run k6 tests in CI on a staging environment before each release.

### Priority

**SHOULD-HAVE (v1.0)** -- MVP can skip formal load testing if the user base is controlled (private beta). Before GA, load testing is essential.

### Resource Impact

Zero production resources. k6 runs on a developer machine or CI runner.

---

## Gap 23: Internationalization (i18n)

### Why It Is Needed

If Raven targets international customers, the admin dashboard and chatbot widget need multi-language support. Even if launching English-only:

- The `<raven-chat>` widget is embedded on customer websites that may be in any language. Widget UI strings (placeholder text, "Send", "Powered by Raven") must be translatable.
- Keycloak login/registration pages need localization (Keycloak has built-in i18n support).
- Date, number, and currency formatting varies by locale.

### Recommended Approach

| Component | Tool | License |
|-----------|------|---------|
| Vue.js admin dashboard | **Vue I18n** | MIT |
| `<raven-chat>` web component | **Vue I18n** (if component uses Vue) or lightweight `Intl` API | Built-in |
| Keycloak | Built-in theme localization | Already in stack |
| Go API | Error messages and email templates: use Go `text/template` with locale-specific message files (JSON/YAML) | Built-in |

**Recommendation:** Integrate Vue I18n from the start by externalizing all UI strings into locale JSON files, even if only providing English initially. This is dramatically cheaper to do upfront (extract strings as you write components) than to retrofit (find and replace every hardcoded string across the entire codebase).

### Priority

**NICE-TO-HAVE (future)** -- But the foundation (externalized strings, no hardcoded UI text) should be established from MVP to avoid costly retrofitting.

### Resource Impact

Zero. Vue I18n adds ~8 KB to the frontend bundle. Locale files are static JSON.

---

## Gap 24: Accessibility (a11y)

### Why It Is Needed

- Legal compliance: Section 508 (US federal), EN 301 549 (EU), ADA (US) -- increasingly enforced for web applications
- Enterprise customers often require WCAG 2.1 AA compliance as a procurement condition
- The `<raven-chat>` widget is embedded on customer websites that may have their own accessibility requirements; a non-accessible widget fails their audits too
- Accessibility benefits all users (keyboard navigation, screen reader support, color contrast)

### Recommended Approach

| Tool | License | Purpose |
|------|---------|---------|
| **axe-core** | MPL-2.0 | Accessibility testing engine. Integrates into CI via `@axe-core/playwright` or `@axe-core/cli`. |
| **eslint-plugin-vuejs-accessibility** | MIT | Lint Vue.js templates for accessibility violations at build time. |
| **Tailwind CSS** | Already in stack | Tailwind Plus components follow accessibility best practices, but custom components need manual attention. |
| **pa11y** | LGPL-3.0 | CLI accessibility testing tool. Can run against deployed pages in CI. |

**Recommendation:**

1. Add `eslint-plugin-vuejs-accessibility` to the Vue.js project for compile-time a11y linting.
2. Add `axe-core` to the end-to-end test suite (Playwright) for runtime accessibility testing.
3. Ensure the `<raven-chat>` widget follows WAI-ARIA authoring practices for chat widgets (role="log", aria-live="polite", keyboard navigation).
4. Test with screen readers (VoiceOver, NVDA) before GA.

### Priority

**NICE-TO-HAVE (future)** -- MVP can launch without full WCAG compliance, but the `<raven-chat>` widget should have basic keyboard navigation and ARIA roles from the start (since it runs on customer sites).

### Resource Impact

Zero runtime resources. Linting and testing happen in CI/development.

---

## Summary Matrix

| # | Gap | Recommended Tool | License | Priority | New Service? | Edge-Safe? |
|---|-----|-----------------|---------|----------|-------------|------------|
| 1 | Email / Notifications | External SMTP relay (SES/Resend) + `go-mail` | N/A (SaaS) | MUST-HAVE | No | N/A (cloud) |
| 2 | Payment / Billing | Stripe API + Go integration | N/A (SaaS) | MUST-HAVE | No | N/A (cloud) |
| 3 | Error Tracking | GlitchTip | MIT | SHOULD-HAVE | Yes | No |
| 4 | API Documentation | swaggo/swag + Scalar | MIT | SHOULD-HAVE | No | Yes |
| 5 | CDN | Cloudflare (free tier) | N/A (SaaS) | SHOULD-HAVE | No | N/A |
| 6 | SSL/TLS | Traefik ACME (Let's Encrypt) | Already in stack | MUST-HAVE | No | Yes |
| 7 | Backup Strategy | pgBackRest + Restic | MIT / BSD-2 | MUST-HAVE | No (batch jobs) | Partial |
| 8 | Rate Limiting | Traefik middleware + Go/Valkey | Already in stack | MUST-HAVE | No | Yes |
| 9 | Feature Flags | PostHog (already in stack) | -- | SHOULD-HAVE | No | Yes |
| 10 | Status Page | Upptime | MIT | SHOULD-HAVE | No | N/A |
| 11 | Documentation Site | VitePress | MIT | SHOULD-HAVE | No | N/A |
| 12 | Changelog | git-cliff + Conventional Commits | MIT | NICE-TO-HAVE | No | N/A |
| 13 | Legal Pages | cookieconsent + lawyer | MIT | MUST-HAVE | No | Yes |
| 14 | Webhook System | Custom Go implementation + Valkey | Already in stack | SHOULD-HAVE | No | Yes |
| 15 | API Versioning | URL-path versioning (design decision) | N/A | MUST-HAVE | No | Yes |
| 16 | Admin Search | PostgreSQL pg_trgm | Already in stack | SHOULD-HAVE | No | Yes |
| 17 | File Virus Scanning | ClamAV | GPL-2.0 | SHOULD-HAVE | Yes | No |
| 18 | CORS / Security Headers | Traefik + Gin middleware config | Already in stack | MUST-HAVE | No | Yes |
| 19 | Scheduled Jobs / Cron | Asynq | MIT | MUST-HAVE | No | Yes |
| 20 | Database Migrations | goose (already in stack) | MIT | -- (closed) | No | Yes |
| 21 | Secrets Management | Infisical (production) / SOPS (edge) | MIT / MPL-2.0 | SHOULD-HAVE | Yes | Partial |
| 22 | Load Testing | k6 | AGPL-3.0 (CLI tool, not deployed) | SHOULD-HAVE | No | N/A |
| 23 | i18n | Vue I18n | MIT | NICE-TO-HAVE | No | Yes |
| 24 | a11y | axe-core + eslint-plugin-vuejs-accessibility | MPL-2.0 / MIT | NICE-TO-HAVE | No | Yes |

---

## MUST-HAVE Items (MVP Blockers) -- Action Summary

These 7 items block MVP launch:

| # | Gap | Action | Effort Estimate |
|---|-----|--------|----------------|
| 1 | Email | Configure Keycloak SMTP. Build Go email service with `go-mail` and external relay. | 2-3 days |
| 2 | Billing | Integrate Stripe: customer/subscription model, Checkout, webhooks. Add billing columns to orgs table. | 1-2 weeks |
| 6 | SSL/TLS | Configure Traefik ACME resolver with Let's Encrypt and DNS-01 challenge. | 1 day |
| 7 | Backups | Deploy pgBackRest, configure backup schedule, test restore procedure. | 2-3 days |
| 8 | Rate Limiting | Implement Go middleware with Valkey sliding window counters. Configure Traefik rate limiter. | 2-3 days |
| 13 | Legal Pages | Engage lawyer for ToS/Privacy Policy. Add cookie consent banner. Consent records table. | 1-2 weeks (legal dependent) |
| 15 | API Versioning | Establish `/api/v1/` route group in Gin. Document versioning policy. | 1 day |
| 18 | CORS / Security Headers | Configure Gin CORS middleware with per-API-key domain allowlists. Add Traefik security headers. | 1-2 days |
| 19 | Scheduled Jobs | Integrate Asynq for job queue + cron. Migrate existing Valkey queue design. | 3-5 days |

**Total estimated effort for MUST-HAVE items: 4-6 weeks.**

---

## New Service Inventory (Deployment Impact)

Only 3 gaps require deploying a new service:

| Service | RAM | CPU | Storage | When |
|---------|-----|-----|---------|------|
| GlitchTip | ~512 MB | 1 core | PostgreSQL DB | v1.0 |
| ClamAV (clamd + freshclam) | ~1 GB | 1 core | ~300 MB signatures | v1.0 |
| Infisical | ~512 MB | 1 core | PostgreSQL DB | v1.0 |

Everything else is either configuration of existing tools, a build-time tool, a SaaS service, or a library integrated into the Go API server.

---

## Edge Deployment Considerations

For Raspberry Pi / edge deployment, the following items are NOT suitable:

| Component | Reason | Edge Alternative |
|-----------|--------|-----------------|
| GlitchTip | 512 MB RAM, Python/Django | Forward errors to cloud GlitchTip instance |
| ClamAV | 1 GB RAM for signature database | Skip scanning on edge; scan on cloud upload path |
| Infisical | 512 MB RAM, server process | Use SOPS with age encryption (zero runtime cost) |
| Lago (if adopted) | 1 GB RAM, PostgreSQL, Redis | Stripe direct integration (SaaS, no self-hosted component) |

The edge device should run only: Go API, PostgreSQL, Valkey, and optionally SeaweedFS/local FS. All heavy ancillary services run on the cloud component.
