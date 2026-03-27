# Analytics & Observability Platform Research

> Research date: 2026-03-27
> Purpose: Evaluate PostHog and OpenObserve for the Raven platform

---

## 1. PostHog - Product Analytics Platform

**Repository:** https://github.com/PostHog/posthog
**Stars:** ~32,200
**Primary Language:** Python (Django backend), TypeScript (frontend)
**Latest Version:** PostHog uses continuous deployment (rolling `posthog-live-*` tags). The last numbered release was `1.38.1`. The Docker image `posthog/posthog:latest` tracks the current production build.

### What is it?

PostHog is an all-in-one, open-source product analytics platform. It replaces a patchwork of tools (Amplitude, LaunchDarkly, Hotjar, Optimizely) with a single integrated platform.

### License

| Component | License |
|-----------|---------|
| Core (`/` excluding `ee/`) | **MIT** (permissive, SaaS-safe) |
| Enterprise (`ee/` directory) | **PostHog Enterprise License** (proprietary - requires subscription for production use) |
| Fully FOSS build | Available at [posthog-foss](https://github.com/PostHog/posthog-foss) (MIT only, enterprise features stripped) |

**SaaS-safe verdict:** Yes. The MIT-licensed core is fully permissive. The `ee/` directory features (advanced RBAC, SSO, project permissions) require a PostHog subscription if used in production, but you can modify/test them freely. The `posthog-foss` repo is 100% MIT if strict FOSS compliance is needed.

### Cloud Free Tier Limits (PostHog Cloud)

| Product | Monthly Free Allowance |
|---------|----------------------|
| Product Analytics | 1 million events |
| Session Replay | 5,000 recordings |
| Feature Flags | 1 million API requests |
| Error Tracking | 100,000 exceptions |
| Surveys | 1,500 responses |
| Data Warehouse | Included |
| LLM Analytics | Included |

Pay-as-you-go beyond the free tier. No credit card required to start. Pricing is transparent and usage-based.

### Key Features

- **Product Analytics:** Autocapture + manual event tracking, funnels, trends, retention, paths, SQL querying
- **Web Analytics:** GA-like dashboard for web traffic, conversions, web vitals, revenue
- **Session Replay:** Watch real user sessions (web + mobile), DOM-based recording with network/console capture
- **Feature Flags:** Boolean and multivariate flags, percentage rollouts, targeting by user properties/cohorts
- **A/B Testing (Experiments):** Statistical significance testing, Bayesian analysis, goal metrics, no-code setup option
- **Error Tracking:** Exception autocapture, alerting, resolution workflow
- **Surveys:** No-code survey builder with templates, in-app and link surveys
- **Data Warehouse:** Sync data from Stripe, HubSpot, S3, etc. Query alongside product data
- **Data Pipelines (CDP):** Real-time transformations, 25+ export destinations, batch exports
- **LLM Analytics:** Trace generations, latency, cost for LLM-powered features
- **Workflows:** Automated actions triggered by events

### SDKs Available

| Frontend | Mobile | Backend |
|----------|--------|---------|
| JavaScript, React, Next.js, Vue, Angular | React Native, Android, iOS, Flutter | **Python**, **Go**, Node, PHP, Ruby, .NET/C# |

Both **Go** and **Python** SDKs are available, which is relevant for Raven's stack.

### Self-Hosting

- **Docker deployment:** One-line install script available
  ```bash
  /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/posthog/posthog/HEAD/bin/deploy-hobby)"
  ```
- **Recommended memory:** **4 GB RAM minimum** for the hobby/self-hosted deploy
- **Scale limit:** Open-source self-hosted deployments scale to ~100,000 events/month. Beyond that, PostHog recommends migrating to PostHog Cloud.
- **Infrastructure dependencies:** PostgreSQL, Redis, ClickHouse, Kafka (all included in the Docker Compose setup)
- **No official Kubernetes Helm chart** for the hobby deploy; the full production-grade deploy is Cloud-only
- **No customer support or guarantees** for self-hosted deployments

### Memory Footprint Assessment

**4 GB minimum is heavy for edge deployment.** The stack includes ClickHouse (analytical DB), Kafka (event streaming), PostgreSQL, and Redis alongside the PostHog application. This is a multi-container deployment, not a single lightweight binary.

**Recommendation for Raven:** Use PostHog Cloud (generous free tier) rather than self-hosting. Self-hosting only makes sense if data sovereignty is a hard requirement and you have at least 4-8 GB RAM to spare.

---

## 2. OpenObserve - Observability Platform (Logs, Metrics, Traces)

**Repository:** https://github.com/openobserve/openobserve
**Stars:** ~18,400
**Primary Language:** Rust (backend), TypeScript (frontend)
**Latest Stable Version:** **v0.70.1** (latest RC: v0.80.0-rc1)

> Note: The user referenced "OpenObservability" -- the correct name is **OpenObserve** (formerly ZincObserve), by the organization `openobserve`.

### What is it?

OpenObserve is a unified observability platform for logs, metrics, traces, and frontend monitoring (RUM). It positions itself as a cost-effective alternative to Datadog, Splunk, and Elasticsearch, claiming 140x lower storage costs through Parquet columnar storage and S3-native architecture.

### License

| Edition | License |
|---------|---------|
| Open Source | **AGPL-3.0** |
| Enterprise | Commercial Enterprise License Agreement (not AGPL) |

**SaaS-safe verdict: CAUTION -- AGPL-3.0 has significant implications.**

The AGPL requires that if you modify OpenObserve and provide it as a network service (which is exactly what self-hosting for Raven would be), you must release your modifications under AGPL. Key considerations:

- **If using unmodified:** You can self-host OpenObserve without releasing Raven's source code, as long as you don't modify OpenObserve itself.
- **If modifying OpenObserve:** Any modifications to OpenObserve's code must be made available under AGPL to users interacting with it over a network.
- **Raven's own code is NOT affected** by AGPL as long as it communicates with OpenObserve via standard APIs/protocols (HTTP, OTLP). AGPL does not "infect" separate programs communicating over a network.
- **Enterprise Edition eliminates AGPL concerns** entirely (commercial license).

Previously, OpenObserve was Apache-2.0 licensed. They moved to AGPL to prevent cloud providers from offering it as a managed service without contributing back. See their [blog post on the license change](https://openobserve.ai/blog/what-are-apache-gpl-and-agpl-licenses-and-why-openobserve-moved-from-apache-to-agpl/).

**Practical assessment for Raven:** If Raven uses OpenObserve as an unmodified infrastructure component (deployed via Docker, data sent via OTLP/HTTP), the AGPL is not a problem. Raven's application code remains under whatever license you choose. However, if strict "no AGPL anywhere in the stack" policy is required, the Enterprise Edition or an alternative like Grafana stack would be needed.

### Cloud/Enterprise Free Tier

| Tier | Limit |
|------|-------|
| Enterprise Free Tier | Up to **200 GB/day** ingestion (~6 TB/month) |
| Registration required | At 100 GB/day |

This is an exceptionally generous free tier for an observability platform.

### Key Features

#### Logs Management
- Full-text search with SQL queries
- Log parsing, enrichment, and transformation via pipelines
- High cardinality support (unlike Loki)

#### Distributed Tracing
- OpenTelemetry-native OTLP ingestion
- Flamegraphs and Gantt charts for trace visualization
- Service graph and span details
- Golden metrics derived from traces

#### Metrics & Dashboards
- PromQL and SQL query support
- Pre-built and custom dashboards with drag-and-drop builder
- Prometheus remote-write compatible

#### Frontend Monitoring (RUM)
- Real User Monitoring for web applications
- Performance analytics, error tracking

#### Alerts
- Scheduled and real-time alerting
- Multi-window alert conditions
- Integrations: Slack, PagerDuty, Telegram, webhooks

#### Pipelines
- Data transformation during ingestion
- VRL (Vector Remap Language) functions for parsing/enrichment

### OpenTelemetry Compatibility

OpenObserve has **native OTLP support** -- it is built on the OpenTelemetry standard:

- **OTLP/HTTP and OTLP/gRPC** endpoints for logs, metrics, and traces
- Works with the **OpenTelemetry Collector** as well as direct SDK instrumentation
- **Go integration:** Use `go.opentelemetry.io/otel` SDK to export traces, metrics, and logs directly to OpenObserve's OTLP endpoints. Configure the OTLP exporter to point at OpenObserve's ingest URL.
- **Python integration:** Use `opentelemetry-sdk` + `opentelemetry-exporter-otlp` packages. OpenObserve also provides an **OpenObserve Python SDK** for direct integration.
- **No proprietary agents required** -- standard OTel collectors/SDKs work natively.

### Self-Hosting & Docker Deployment

Single-container deployment:
```bash
docker run -d \
    --name openobserve \
    -v $PWD/data:/data \
    -p 5080:5080 \
    -e ZO_ROOT_USER_EMAIL="root@example.com" \
    -e ZO_ROOT_USER_PASSWORD="Complexpass#123" \
    public.ecr.aws/zinclabs/openobserve:latest
```

- **Single binary** -- no external dependencies (no separate DB, no Kafka, no Redis)
- **High Availability mode** available for production with Kubernetes (Helm charts provided)
- Local disk or S3-compatible object storage for data

### Memory Footprint

This is where OpenObserve truly excels for Raven's edge deployment needs:

- **Single binary built in Rust** -- extremely memory efficient
- Claims to run on **1/4 the hardware** compared to Elasticsearch
- No external dependencies (PostgreSQL, Redis, Kafka, etc.) unlike PostHog
- The single Docker container approach means minimal overhead
- **Estimated minimum: ~256 MB - 512 MB RAM** for a lightweight single-node deployment (based on Rust binary + Parquet engine). Production workloads with significant ingestion will need more, but the baseline is very low.
- Scales from single binary (small edge deployments) to petabyte-scale HA clusters

**This is an excellent fit for edge/resource-constrained deployments.**

### Comparison to Alternatives

#### vs. Grafana + Loki + Prometheus + Tempo Stack

| Aspect | OpenObserve | Grafana Stack |
|--------|-------------|---------------|
| Components | Single platform/binary | 4+ separate tools to deploy and manage |
| Management | One deployment | Multiple deployments, configs, upgrades |
| High cardinality | Full support | Loki struggles with high cardinality |
| Query performance | Fast on large volumes | Loki slow on large datasets |
| Query language | SQL + PromQL | LogQL + PromQL |
| Storage | Parquet + S3 (140x cheaper) | Chunks + object storage |

#### vs. ELK Stack (Elasticsearch + Logstash + Kibana)

| Aspect | OpenObserve | ELK Stack |
|--------|-------------|-----------|
| Storage cost | 140x lower | High (inverted index is storage-heavy) |
| Setup complexity | Single binary | Complex cluster management |
| Query language | SQL | Lucene/KQL |
| Hardware requirements | 1/4 the resources | High memory/CPU demands |
| License | AGPL-3.0 | SSPL (Elasticsearch) / AGPL (OpenSearch) |

#### vs. Datadog

| Aspect | OpenObserve | Datadog |
|--------|-------------|---------|
| Deployment | Self-hosted or Cloud | SaaS only |
| Pricing | Per-GB (free up to 200 GB/day) | Per-host + per-GB (expensive) |
| Open source | Yes (AGPL) | No |
| OpenTelemetry | Native OTLP | Supported |
| Vendor lock-in | None | High |

---

## Comparative Summary for Raven

| Criterion | PostHog | OpenObserve |
|-----------|---------|-------------|
| **Purpose** | Product analytics, feature flags, A/B tests | Logs, metrics, traces, RUM |
| **They solve different problems** | User behavior & product decisions | System observability & debugging |
| **License (core)** | MIT (permissive) | AGPL-3.0 (copyleft, network clause) |
| **Enterprise license** | Proprietary (ee/ dir) | Commercial (eliminates AGPL) |
| **SaaS-safe?** | Yes (MIT core) | Yes if unmodified; caution if modified |
| **Self-host memory** | 4 GB minimum (multi-container) | ~256-512 MB minimum (single binary) |
| **Edge-deployable?** | No (too heavy) | Yes (excellent fit) |
| **Cloud free tier** | 1M events, 5K replays, 1M flag requests/mo | 200 GB/day ingestion |
| **OpenTelemetry** | Not native (event-based model) | Native OTLP (built on OTel) |
| **Go SDK** | Yes | Yes (via OTel SDK + OTLP export) |
| **Python SDK** | Yes | Yes (OTel SDK + dedicated Python SDK) |
| **Docker deploy** | Yes (Docker Compose, multi-container) | Yes (single container) |
| **Latest version** | Rolling releases (`posthog-live-*`) | v0.70.1 stable, v0.80.0-rc1 |
| **GitHub stars** | ~32,200 | ~18,400 |

### Recommendation

These tools are **complementary, not competing**:

1. **PostHog** for product analytics: Use **PostHog Cloud** (free tier is generous for early-stage). The self-hosted option is too heavy for edge deployment and lacks support guarantees. The MIT license makes it worry-free.

2. **OpenObserve** for observability: **Self-host via Docker** for edge deployments where resource efficiency matters. Its single-binary Rust architecture, native OpenTelemetry support, and minimal memory footprint make it ideal for Raven's edge nodes. Use the AGPL open-source edition without modification to avoid license complications, or evaluate the Enterprise edition if AGPL is a policy concern.

3. **Integration pattern:**
   - Raven Go services -> `go.opentelemetry.io/otel` SDK -> OpenObserve (OTLP) for traces/metrics/logs
   - Raven Python services -> `opentelemetry-sdk` -> OpenObserve (OTLP) for traces/metrics/logs
   - Raven frontend/product -> PostHog JS SDK -> PostHog Cloud for product analytics
   - Feature flags -> PostHog feature flags API -> Raven services

---

## License Risk Summary

| Tool | License | Risk Level | Notes |
|------|---------|------------|-------|
| PostHog (core) | MIT | **None** | Fully permissive |
| PostHog (ee/) | Proprietary | **Low** | Only if using enterprise features without subscription |
| OpenObserve (OSS) | AGPL-3.0 | **Low-Medium** | Safe if used unmodified as infrastructure; modifications must be AGPL |
| OpenObserve (Enterprise) | Commercial | **None** | Standard commercial license |

**Neither tool uses SSPL.** PostHog's MIT core is the cleanest license. OpenObserve's AGPL is manageable for infrastructure use but requires awareness of the modification disclosure requirement.
