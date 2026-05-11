---
title: "Observability"
---

# Observability

Raven splits telemetry into two layers and uses a different tool for each:

- **Application telemetry** (traces, logs, metrics emitted by the Go API and the
  Python AI worker) lands in **OpenObserve**.
- **Host telemetry** (CPU, memory, disk, network, per-container resource usage
  for the box itself) lands in **Beszel**.

There is no Grafana in the stack. OpenObserve provides its own dashboarding,
SQL/PromQL query, and alerting; Beszel provides its own UI for host metrics.
Keeping them separate means a host-level issue (disk full, agent dead) can
still be diagnosed when the application stack is unhealthy.

## Stack overview

| Tool        | Covers                                          | Where it runs                                                                 | Default access                       |
| ----------- | ----------------------------------------------- | ----------------------------------------------------------------------------- | ------------------------------------ |
| OpenObserve | App traces, logs, metrics (OTLP/gRPC)           | `docker-compose.yml` service `openobserve`                                    | `http://localhost:5080`              |
| Beszel Hub  | Host CPU/RAM/disk/network, container resources  | `deploy/ansible/files/docker-compose.admin.yml` (separate admin compose file) | `http://<host>:8090` / `monitor.<your-domain>` via Cloudflare tunnel |
| Beszel Agent| Reports host stats to the hub                   | Standalone container, started by Ansible (`deploy/ansible/roles/admin-tools/tasks/main.yml`) | n/a — talks to the hub on `localhost:45876` |

Beszel is **not** bundled in the main Compose file. It lives in a separate
admin Compose file because it monitors the host running Raven; it must keep
running even when you tear the application stack down for maintenance.

## OpenObserve — app telemetry

### Default deployment (Compose)

The service block in [`docker-compose.yml`](https://github.com/ravencloak-org/Raven/blob/main/docker-compose.yml):

```yaml
openobserve:
  image: public.ecr.aws/zinclabs/openobserve:latest
  ports:
    - "5080:5080"   # UI + HTTP API
    - "5081:5081"   # gRPC OTLP receiver (traces, metrics, logs)
  environment:
    ZO_ROOT_USER_EMAIL: ${ZO_ROOT_USER_EMAIL:-admin@raven.local}
    ZO_ROOT_USER_PASSWORD: ${ZO_ROOT_USER_PASSWORD:?ZO_ROOT_USER_PASSWORD is required}
    ZO_GRPC_PORT: "5081"
  volumes:
    - openobserve-data:/data
  networks:
    - raven-internal
  restart: unless-stopped
```

| Setting       | Default                                          | Purpose                                            |
| ------------- | ------------------------------------------------ | -------------------------------------------------- |
| Image         | `public.ecr.aws/zinclabs/openobserve:latest`     | Pin to a digest in production.                     |
| HTTP/UI port  | `5080`                                           | Web UI, REST API, query.                           |
| OTLP gRPC port| `5081`                                           | Where Raven services push traces/logs/metrics.     |
| Data volume   | `openobserve-data` → `/data`                     | All ingested data, dashboards, users.              |
| Admin email   | `${ZO_ROOT_USER_EMAIL:-admin@raven.local}`       | Initial root user; first-login credentials.        |
| Admin password| `${ZO_ROOT_USER_PASSWORD}`                       | Required at boot — Compose fails fast without it.  |

Open the UI at `http://localhost:5080` and log in with the email and password
above (configured via `.env`; see [`.env.example`](https://github.com/ravencloak-org/Raven/blob/main/.env.example)).

### What gets emitted

#### Go API (`raven-api`)

Telemetry is wired in [`internal/telemetry/telemetry.go`](https://github.com/ravencloak-org/Raven/blob/main/internal/telemetry/telemetry.go)
and applied via the Gin middleware in [`internal/middleware/otel.go`](https://github.com/ravencloak-org/Raven/blob/main/internal/middleware/otel.go).

- **Traces** — one span per HTTP request (server kind), named `<METHOD> <route>`,
  with `http.request.method` and `url.path` attributes. The middleware sets an
  `X-Trace-ID` response header so a trace can be looked up from any user-facing
  error.
- **Logs** — `slog` is wrapped with the `otelslog` bridge: every structured log
  goes to stderr **and** to OpenObserve as an OTLP log record, automatically
  decorated with `trace_id` / `span_id` from the request context.
- **Metrics** — pre-registered instruments in
  [`internal/telemetry/metrics.go`](https://github.com/ravencloak-org/Raven/blob/main/internal/telemetry/metrics.go):

  | Metric                          | Type            | Unit          |
  | ------------------------------- | --------------- | ------------- |
  | `raven.http.requests.total`     | counter         | `{request}`   |
  | `http.server.request.duration`  | histogram       | `s`           |
  | `raven.chat.completions.total`  | counter         | `{completion}`|
  | `raven.chat.tokens.total`       | counter         | `{token}`     |
  | `raven.chat.completion.latency` | histogram       | `ms`          |
  | `raven.voice.sessions.created`  | counter         | `{session}`   |
  | `raven.voice.sessions.active`   | up-down counter | `{session}`   |
  | `raven.voice.turns.total`       | counter         | `{turn}`      |
  | `raven.whatsapp.calls.total`    | counter         | `{call}`      |
  | `raven.whatsapp.messages.total` | counter         | `{message}`   |

When OTLP is not configured, all of these become no-ops with zero overhead.

#### Python AI worker (`raven-ai-worker`)

Configured in [`ai-worker/raven_worker/telemetry.py`](https://github.com/ravencloak-org/Raven/blob/main/ai-worker/raven_worker/telemetry.py)
using `opentelemetry-sdk`, `opentelemetry-exporter-otlp`, and
`opentelemetry-instrumentation-grpc` (versions pinned in
[`ai-worker/pyproject.toml`](https://github.com/ravencloak-org/Raven/blob/main/ai-worker/pyproject.toml)).

The worker emits **traces** for gRPC calls — one server-side span per RPC,
covering ingest, embedding, retrieval, and chat completion. Logs go through
`structlog` to stdout (collected by Docker); the worker does not currently
push OTLP logs or custom metrics to OpenObserve.

### OTLP wiring

Two ways to point a Raven service at OpenObserve.

**Go API — Raven-specific vars**

| Variable                  | Default (Compose)         | Notes                                       |
| ------------------------- | ------------------------- | ------------------------------------------- |
| `RAVEN_OTEL_ENABLED`      | `true` (in `docker-compose.yml`) | When unset/false, telemetry is a no-op. |
| `RAVEN_OTEL_ENDPOINT`     | `openobserve:5081`        | gRPC endpoint, host:port (no scheme).       |
| `RAVEN_OTEL_SERVICE_NAME` | `raven-api`               | Becomes `service.name` resource attribute.  |

**Python worker — standard OTel var**

| Variable                       | Default                       |
| ------------------------------ | ----------------------------- |
| `OTEL_EXPORTER_OTLP_ENDPOINT`  | `http://openobserve:5081`     |

When connecting from outside the Compose network, replace `openobserve` with
the host's address. TLS is **not** enabled by default (the exporter uses
`WithInsecure()`); put OpenObserve behind your reverse proxy or a Cloudflare
tunnel for external exposure.

See [`/reference/configuration/`](/reference/configuration) for the full set
of env vars consumed by the API.

### Retention

Retention is **not** preconfigured in this repo. `deploy/observability/` ships
dashboards only — no retention or storage policy YAMLs. OpenObserve defaults
apply (per-stream retention is set in the OpenObserve UI under
**Streams → \<stream\> → Settings**). Recommended starting points for a
single-node deployment:

- Traces: 7 days
- Logs: 14 days
- Metrics: 30 days

Tighten or loosen based on the disk allocated to the `openobserve-data`
volume.

### Querying

OpenObserve supports SQL for logs and PromQL for metrics; consult its
[official docs](https://openobserve.ai/docs/) for full syntax. Two examples
that work against Raven's emitted telemetry:

```sql
-- Logs: every error from the API in the last 15 minutes
SELECT _timestamp, body, trace_id, attributes_org_id
FROM "default"
WHERE service_name = 'raven-api'
  AND severity_text = 'ERROR'
ORDER BY _timestamp DESC
LIMIT 200
```

```promql
# Metrics: P95 HTTP server latency, per route, over 5-minute windows
histogram_quantile(
  0.95,
  sum by (le, http_route) (rate(http_server_request_duration_bucket[5m]))
)
```

## Beszel — host telemetry

### What it covers

CPU usage, memory, disk usage and I/O, network throughput, system load, plus
per-container CPU/memory for every Docker container running on the host. It
does **not** see application metrics — that's OpenObserve's job.

### Default deployment

Beszel is split into two pieces and deployed by Ansible, separately from the
main application stack:

- **Beszel Hub** — `henrygd/beszel:0.18.7`, defined in
  [`deploy/ansible/files/docker-compose.admin.yml`](https://github.com/ravencloak-org/Raven/blob/main/deploy/ansible/files/docker-compose.admin.yml).
  Runs with `network_mode: host` because the agent talks to it over SSH on
  `localhost:45876`.
- **Beszel Agent** — `henrygd/beszel-agent:0.18.7`, started as a standalone
  container by [`deploy/ansible/roles/admin-tools/tasks/main.yml`](https://github.com/ravencloak-org/Raven/blob/main/deploy/ansible/roles/admin-tools/tasks/main.yml).
  Mounts `/var/run/docker.sock` read-only so it can report container stats.
  Requires a `KEY` obtained from the Hub UI after adding the system; the key
  is stored in Ansible vault as `beszel_agent_key`.

The agent is intentionally **not** in `docker-compose.yml`: it must survive
`docker compose down` of the application stack so you can still see what is
happening to the host while you bring services back up.

### Default URL

In production deployments wired through Cloudflare tunnels (see
`deploy/ansible/roles/cloudflared/templates/config.yml.j2`) the hub is
exposed at `monitor.<your-domain>` — for the reference deployment that is
`https://monitor.ravencloak.org`. On a local host without a tunnel, it is
served on port `8090` of the host network.

## Application logs vs host logs

| You're investigating…                              | Look at      |
| -------------------------------------------------- | ------------ |
| Why a chat completion returned 500                 | OpenObserve (logs + traces, filter by `trace_id`) |
| Why a request took 8 seconds                       | OpenObserve (`http.server.request.duration` histogram) |
| Why the box is OOM-killing containers              | Beszel (memory + per-container) |
| Why a container is restarting on a loop            | Beszel (container state) **then** OpenObserve (logs from before the restart) |
| Why disk pressure is alerting                      | Beszel (disk usage) — OpenObserve will likely also be unhappy |

## Alerting

OpenObserve has built-in alert rules with webhook actions. Raven ships **alert
rule definitions but not active alerts** — they live as JSON in
[`deploy/observability/dashboards/alerts.json`](https://github.com/ravencloak-org/Raven/blob/main/deploy/observability/dashboards/alerts.json)
and must be imported (or recreated) manually in the OpenObserve UI before
they fire. The bundled rules cover:

| Rule                       | Condition                                                         | Severity |
| -------------------------- | ----------------------------------------------------------------- | -------- |
| `high_error_rate`          | 5xx rate > 5% over 5 min                                          | critical |
| `high_p95_latency`         | P95 `http.server.request.duration` > 2 s over 5 min               | warning  |
| `chat_completion_failures` | 5xx rate on `/api/v1/chat/.../completions` > 5% over 5 min        | critical |

Each rule has a `${ALERT_WEBHOOK_URL}` placeholder; point it at Slack,
PagerDuty, or any other receiver before enabling.

Beszel has its own alerting (CPU/memory/disk thresholds, SSH liveness)
configured per-system in the Hub UI.

## Sample dashboards

Four dashboards ship in [`deploy/observability/dashboards/`](https://github.com/ravencloak-org/Raven/tree/main/deploy/observability/dashboards):

| File                     | Purpose                                                                  |
| ------------------------ | ------------------------------------------------------------------------ |
| `api-overview.json`      | Request rate, error rate, P50/P95/P99 latency, status code distribution  |
| `chat-completions.json`  | Completion volume, token throughput, latency by org/KB/model             |
| `voice-sessions.json`    | Sessions created, active gauge, turns processed                          |
| `whatsapp.json`          | Calls and messages by org, direction, type                               |

These are JSON definitions, **not** OpenObserve native exports — you will
need to recreate them in the UI (PromQL queries are reusable as-is) or import
via the OpenObserve API. Treat them as a known-good starting set; build
deployment-specific dashboards on top.

## See also

- [Configuration reference](/reference/configuration) — full env-var list,
  including all `RAVEN_OTEL_*` and `OTEL_EXPORTER_OTLP_*` variables.
- [Docker Compose guide](/guides/self-hosting/docker-compose) — bringing the
  stack up, including the `openobserve` service.
- [Hardening guide](/guides/self-hosting/hardening) — putting OpenObserve
  behind TLS and restricting access.
