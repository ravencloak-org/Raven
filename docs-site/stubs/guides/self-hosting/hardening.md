---
title: "Hardening"
---

# Hardening

Raven's default production posture is [OpenSSF Baseline Level 2](/trust/openssf-baseline)
attested and [SLSA Build Level 3](/trust/slsa-level-3) for every signed
release. CodeQL, Semgrep, Gitleaks and OpenSSF Scorecard run on every PR;
`golangci-lint`, `ruff`, and DCO sign-off are gating; container images and
release blobs are keyless-signed with cosign.

That gives you a hardened **build** pipeline. This page is the operator's
checklist for everything that requires deployment-time choices: TLS,
authentication, tenant isolation, secrets, rate limiting, container and
network exposure, audit logging, and the supply-chain controls you should
verify before going live.

If a control is not bundled today, this page says **"recommended, not bundled"**
rather than implying it is enforced. Read the linked source files to verify.

## TLS termination — Traefik + Let's Encrypt

Public traffic terminates TLS at Traefik v3 in front of every other service.
The shipped configuration lives in two files.

**`deploy/traefik/traefik.yml`** — static configuration:

- `entryPoints.web` (`:80`) issues a permanent 301 redirect to `websecure`,
  so plain HTTP is never served.
- `entryPoints.websecure` (`:443`) attaches the `security-headers@file`
  middleware to every route by default and resolves certificates via the
  `letsencrypt` resolver.
- `certificatesResolvers.letsencrypt.acme` performs a DNS-01 challenge
  (Cloudflare provider by default; override with `ACME_DNS_PROVIDER`) so
  certificates can be issued for hosts that are not directly reachable on
  port 80.
- `providers.docker.exposedByDefault: false` — services must opt in via
  `traefik.enable=true` labels; no container is exposed accidentally.

**`deploy/traefik/dynamic/tls.yml`** — TLS options and headers:

- `tls.options.default.minVersion: VersionTLS12`, `sniStrict: true`, and an
  explicit allow-list of TLS 1.3 / strong TLS 1.2 ciphers (`AES_*_GCM`,
  `CHACHA20_POLY1305`).
- The `security-headers` middleware sets HSTS (`stsSeconds: 63072000`,
  `stsPreload: true`, `forceSTSHeader: true`), `X-Content-Type-Options`,
  `X-Frame-Options: SAMEORIGIN`, `Referrer-Policy: strict-origin-when-cross-origin`,
  and strips `Server` / `X-Powered-By`.

The Go API also emits an HSTS-preload-eligible header itself
(`Strict-Transport-Security: max-age=31536000; includeSubDomains; preload`)
plus a `Content-Security-Policy` and `Permissions-Policy` from
`internal/middleware/security.go::SecurityHeadersMiddleware` — Traefik and the
app are belt-and-braces.

**Operator action:** set `ACME_EMAIL`, `RAVEN_DOMAIN`, and the DNS-01 provider
credentials (`CF_DNS_API_TOKEN` for Cloudflare) in your environment before the
first start.

## Authentication — SuperTokens session cookies

Authentication is delegated to SuperTokens. The Go API never issues its own
JWTs; it verifies opaque session cookies on every request via
`internal/auth/supertokens.go::SuperTokensProvider.VerifySession`, and
`internal/middleware/auth.go::SessionMiddleware` aborts with a structured 401
on any failure. Initialisation lives in
`internal/auth/supertokens_init.go::InitSuperTokens`:

- The `session.Init` recipe is configured with `CookieSameSite: "none"` so that
  `app.<domain>` and `api.<domain>` can share sessions across subdomains.
- `cookieDomain` is auto-set to `.ravencloak.org` in production and
  `localhost` in dev — set `RAVEN_API_DOMAIN` and `RAVEN_WEBSITE_DOMAIN` to
  match your deployment.
- Access-token and refresh-token lifetimes are managed by **SuperTokens Core**
  and not exposed in `internal/config/config.go::SuperTokensConfig`. Configure
  them in your SuperTokens instance — the Core defaults (1 h access token,
  100 day refresh token) are reasonable; tighten the access token to 15
  minutes for high-sensitivity tenants.
- Multi-factor authentication is supported by SuperTokens via its TOTP /
  email-OTP recipes. Enrolment is **not** enabled by default in
  `InitSuperTokens` and is recommended, not bundled — register the recipe
  client-side and the matching server-side init for production tenants.

For the **single-tenant desktop edition** (`RAVEN_SINGLE_USER=true`), the
session middleware is replaced with `SingleUserMiddleware` which injects a
synthetic `user_id=local`, `org_id=local` and skips SuperTokens entirely.
Never enable `RAVEN_SINGLE_USER` on a publicly reachable deployment.

## Tenant isolation — PostgreSQL row-level security

Every tenant-scoped table has Postgres row-level security enabled at table
creation time. Grep `migrations/` for `ROW LEVEL SECURITY` to see the full
list — the per-table policies are applied in:

- `migrations/00004_users.sql`
- `migrations/00005_workspaces.sql`
- `migrations/00006_workspace_members.sql`
- `migrations/00007_knowledge_bases.sql`
- `migrations/00008_documents.sql`
- `migrations/00009_sources.sql`
- `migrations/00010_chunks_and_embeddings.sql`
- `migrations/00011_llm_provider_configs.sql`
- `migrations/00012_api_keys.sql`
- `migrations/00013_chat.sql`
- `migrations/00014_processing_events.sql`
- `migrations/00022_security_rules.sql` (also `security_events`)
- `migrations/00032_billing_rls_and_payment_events.sql`

`migrations/00015_rls_policies.sql` is the audit marker that consolidates this
coverage and is intentionally a no-op — confirming, not introducing, the
policies.

Every policy follows the same shape:

```sql
CREATE POLICY tenant_isolation ON <table> FOR ALL
    USING       (org_id = current_setting('app.current_org_id')::uuid)
    WITH CHECK  (org_id = current_setting('app.current_org_id')::uuid);
CREATE POLICY admin_bypass ON <table> FOR ALL TO raven_admin USING (true);
```

The Go API enforces this by wrapping every tenant query in a transaction that
sets the GUC. See `internal/db/db.go::WithOrgID`:

```go
tx, _ := pool.Begin(ctx)
tx.Exec(ctx, "SELECT set_config('app.current_org_id', $1, true)", orgID)
```

The third argument `true` makes the setting **transaction-local** so it
cannot leak across pooled connections. The org ID is bound as a parameter,
not interpolated, so this is not a SQL-injection vector.

The DB role used by the API must **not** be a member of `raven_admin`; reserve
that role for migrations and out-of-band ops only. The `admin_bypass` policy
is the escape hatch — never use it from request handlers.

## Secrets management

All secrets are env-driven. The repo uses [`dotenvx`](https://dotenvx.com/) at
runtime — see `Makefile`, `Dockerfile`, `ai-worker/Dockerfile` — and the
container `CMD` is wrapped in `dotenvx run --` so a Sigstore-signed `.env`
file can be decrypted from `DOTENV_PRIVATE_KEY` at boot.

**Required, fail-fast secrets** (the API refuses to start without them; see
the `:?` markers in `docker-compose.yml`):

- `POSTGRES_PASSWORD`
- `CLICKHOUSE_PASSWORD`
- `RAVEN_ENCRYPTION_AES_KEY` — 32-byte AES-256 key used by
  `internal/crypto/aes.go` to seal LLM provider API keys at rest in
  `llm_provider_configs`.
- `ZO_ROOT_USER_PASSWORD` (OpenObserve)

**Recommended secrets to rotate per deployment:**

- `SUPERTOKENS_API_KEY` — auth Core admin key.
- `RAVEN_RATELIMIT_APIKEY_HASH_SECRET` — HMAC key for hashing API-key bucket
  identifiers in Valkey. If unset,
  `internal/middleware/ratelimit.go::NewRateLimiter` logs a warning and uses
  a zero-length HMAC key (acceptable only for dev).
- `RAVEN_SEED_KEY` — gate for the `/admin/seed-demo` endpoint
  (`cmd/api/main.go` line 869). Leave **empty** in production to disable the
  seed route entirely.
- `LIVEKIT_API_KEY`, `LIVEKIT_API_SECRET`, `CF_DNS_API_TOKEN`, OAuth client
  secrets, etc.

**Recommended, not bundled:** front the deployment with a real secrets store
— HashiCorp Vault, AWS Secrets Manager, Doppler, 1Password CLI, or k8s
sealed-secrets. Treat committed `.env*` files as templates only; the actual
production values should be injected at deploy time. Encrypted dotenvx files
are an acceptable middle ground for small edge deployments.

## Rate limiting

Rate limiting is implemented in `internal/middleware/ratelimit.go` against
Valkey and is multi-scope:

- **Algorithm:** atomic sliding window via a Lua script over a Valkey sorted
  set (see `slidingWindowLua`). Window is fixed at 60 s.
- **Failure mode:** **fail-open** — if Valkey is unreachable within the
  500 ms `callCtx`, the request is admitted with a logged warning. Operators
  must monitor for sustained Valkey errors; relying on rate-limit + Valkey
  outage as a denial-of-service prevention layer is not safe.
- **Scopes:** `ByAPIKey`, `ByUserID`, `ByOrgID`, and tier-aware `ByOrgTier`
  (route groups: `general`, `completion`, `widget`).
- **API key bucket keys** are HMAC-SHA-256 of the raw key, keyed with
  `RAVEN_RATELIMIT_APIKEY_HASH_SECRET` — raw API keys never reach Valkey.
- **Per-tier defaults** from `DefaultTierConfig()` in
  `internal/middleware/ratelimit.go`:

  | Tier        | General RPM | Completion RPM | Widget RPM |
  |-------------|------------:|---------------:|-----------:|
  | Free        | 60          | 10             | 30         |
  | Pro         | 600         | 120            | 30         |
  | Enterprise  | 6000        | unlimited (-1) | 30         |

Override per-tier limits via `RAVEN_RATELIMIT_*` env vars
(`internal/config/config.go::RateLimitConfig`).

**Recommended, not bundled:** add an edge-tier rate limit at Traefik
(`traefik.http.middlewares.rate-limit.rateLimit.average` — see
`deploy/traefik/dynamic/tls.yml`) or a WAF/CDN in front for L7 DDoS.

## Container hardening

All three runtime images run as a non-root UID:

| Image                     | Base                                       | USER         |
|---------------------------|--------------------------------------------|--------------|
| Go API (`Dockerfile`)     | `alpine:3.23` (digest-pinned)              | `raven` (1000) — line 42 |
| AI worker (`ai-worker/Dockerfile`) | `python:3.14-slim` (digest-pinned) | `raven` (1000) — line 53 |
| Frontend (`frontend/Dockerfile`)   | `nginxinc/nginx-unprivileged:1.27-alpine` (digest-pinned) | already non-root by image |

Every base image reference is **digest-pinned** (e.g.
`alpine:3.23@sha256:5b10f432...`) so image rebuilds are reproducible.

Build pipelines are multi-stage: the Go binary is statically linked
(`-extldflags '-static'`) into a minimal Alpine runtime; the Python build
uses `pip install --require-hashes` against a `uv pip compile`d lockfile
with full per-archive hashes; the frontend builds Vue with `npm ci` (lockfile
required) and serves the dist via the unprivileged nginx image.

**Bundled, applied to every service in `docker-compose.yml`:**

```yaml
security_opt:
  - "no-new-privileges:true"
cap_drop:
  - ALL
```

`security_opt: no-new-privileges:true` is universal — it prevents any setuid
binary from elevating privileges and has zero functional impact. `cap_drop:
[ALL]` removes the full Linux capability set, then each service re-adds only
the capabilities it actually needs via `cap_add`:

| Service | Re-added capabilities | Reason |
|---------|-----------------------|--------|
| `go-api`              | — (none)                                            | Static Go binary, runs as non-root `raven` UID. eBPF overlay adds `CAP_BPF` + `CAP_NET_ADMIN`. |
| `python-worker`       | — (none)                                            | Python service, non-root. |
| `python-agent`        | — (none)                                            | Python LiveKit agent, non-root. |
| `livekit-server`      | — (none)                                            | High-port SFU. |
| `postgres`            | `CHOWN`, `DAC_OVERRIDE`, `FOWNER`, `SETGID`, `SETUID` | `docker-entrypoint.sh` fixes `PGDATA` ownership then drops to `postgres` via gosu. |
| `clickhouse`          | `CHOWN`, `DAC_OVERRIDE`, `FOWNER`, `SETGID`, `SETUID`, `IPC_LOCK`, `SYS_NICE` | Same ownership/gosu handoff plus mlock for caches and io_uring scheduling. |
| `valkey`              | `CHOWN`, `DAC_OVERRIDE`, `SETGID`, `SETUID`         | Entrypoint chowns `/data` then drops to `valkey` via su-exec. |
| `supertokens`         | `CHOWN`, `DAC_OVERRIDE`, `SETGID`, `SETUID`         | JVM entrypoint drops to `supertokens` user. |
| `seaweedfs-master`    | `SETGID`, `SETUID`, `SETPCAP`                       | Image uses su-exec; needs `SETPCAP` to shrink the bounding set on re-exec. |
| `seaweedfs-volume`    | `SETGID`, `SETUID`, `SETPCAP`                       | Same as master. |
| `seaweedfs-filer`     | `SETGID`, `SETUID`, `SETPCAP`                       | Same as master. |
| `traefik`             | `NET_BIND_SERVICE`                                  | Binds privileged ports 80 and 443. |
| `openobserve`         | — (none)                                            | Rust binary on high ports. |
| `pgbackrest`          | `CHOWN`, `DAC_OVERRIDE`, `FOWNER`, `SETGID`, `SETUID` | Reads `PGDATA` as uid 999 (postgres) and writes the repo as uid 1001 (pgbackrest). |

`docker-compose.edge.yml` applies the same defaults to its three services
(`go-api`, `postgres`, `traefik`).

**Not bundled (deferred to a follow-up):**

```yaml
read_only: true
tmpfs:
  - /tmp
```

`read_only: true` requires per-service tmpfs additions for `/tmp`,
`/var/run`, and any other write paths the image expects, and breaks several
images that write outside the data volume (Postgres lock dirs, OpenObserve
work dirs). Add this in a `docker-compose.override.yml` once you have
identified the writable paths each service genuinely needs.

## Network exposure

The bundled `docker-compose.yml` puts all services on the `raven-internal`
bridge network and exposes ports through Traefik. **However**, several
services also publish ports directly on the host for development convenience:

- `go-api`: `8081:8081`
- `livekit-server`: `7882/tcp` and `50000-60000/udp`
- `clickhouse`: `127.0.0.1:9000`, `127.0.0.1:8123` (loopback only)
- `seaweedfs-filer`: `8888:8888` (S3 API)
- `openobserve`: `5080:5080`, `5081:5081`
- `traefik`: `80:80`, `443:443`, `8082:8080` (dashboard)

**For production, remove or override every `ports:` block except Traefik's
`80` and `443`** (LiveKit's UDP range is the necessary exception — WebRTC
media cannot be reverse-proxied). The OpenObserve UI on `5080` and the
Traefik dashboard on `8082` are particularly sensitive — never expose them
publicly. Use SSH tunnelling or a VPN to reach them.

`providers.docker.network: raven-internal` in `traefik.yml` ensures Traefik
only routes through the internal network even if other networks exist.

## Supply chain

Every signed Raven release ships:

- **SLSA v1 Build Level 3 provenance** for the Go binary checksums and for
  every container image — verifiable with `cosign verify-blob` /
  `cosign verify` and the recipe in `docs/security/slsa-verification.md`
  (rendered at [/trust/slsa-level-3](/trust/slsa-level-3)).
- **Sigstore keyless signatures** with a workflow-identity certificate that
  ties the artifact to the exact GitHub Actions run that produced it.
- **SPDX SBOMs** as image attestations.

**Inputs to those builds are pinned at the same level:**

- Every action in `.github/workflows/*.yml` is pinned to a 40-character
  commit SHA with the human-readable tag in a comment — e.g.
  `actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd  # v6.0.2` in
  `.github/workflows/codeql.yml`.
- Python builds pass `pip install --require-hashes` against
  `requirements*.txt` generated by `uv pip compile --generate-hashes`
  (`.github/workflows/python.yml`, line 41 and 94; AI-worker `Dockerfile`,
  line 11).
- `pgBackRest` repository checksums protect WAL and full backups; verify on
  restore as documented on the [Backups](/guides/self-hosting/backups)
  documentation page.
- `dotenvx` itself is installed from a release URL with a verified
  `sha256sum -c` against the `DOTENVX_INSTALLER_SHA256` ARG in the
  Dockerfiles — bumps require updating the hash in lockstep.

Verify before deploying:

```bash
cosign verify ghcr.io/ravencloak-org/go-api:<tag> \
  --certificate-identity-regexp '^https://github.com/ravencloak-org/Raven/' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com'
```

## Audit logging

The OSS distribution records security-relevant events in the Postgres
`security_events` table (defined in `migrations/00022_security_rules.sql`).
Each row carries `org_id`, `rule_id`, `event_type` (`blocked`, `throttled`,
`alert`, `suspicious`), `ip_address`, `country_code`, `request_path`,
`request_method`, `user_agent`, and a freeform `details` JSONB. RLS is
enforced on this table the same way as on tenant data — `tenant_isolation`
plus an `admin_bypass` for `raven_admin`.

OpenTelemetry traces, metrics, and structured logs from every Go and Python
service are shipped to OpenObserve via OTLP gRPC on `5081` (configured by
`RAVEN_OTEL_ENABLED`, `RAVEN_OTEL_ENDPOINT`, `RAVEN_OTEL_SERVICE_NAME` in
`docker-compose.yml`). The Beszel agent runs alongside on the host for
process and resource metrics. See the
[Observability](/guides/self-hosting/observability) page for retention and
forwarding.

**Recommended, not bundled:** ship `security_events` and the application log
stream off-box to immutable storage (S3 with object lock, or a SIEM) for
forensic retention beyond the local Postgres / OpenObserve window.

A richer EE audit subsystem lives in `internal/ee/audit/audit.go` for
GDPR/SOC2 compliance reporting — that is part of the Enterprise edition and
out of scope for this self-hosting page.

## Demo seed — turn off in production

The Go API ships a `/admin/seed-demo` route that scaffolds a demo TMDB
knowledge base. It is gated by **two** env vars (see `cmd/api/main.go`
around line 865 and `internal/service/seed.go::SeedDemo`):

- `TMDB_API_KEY` — the seed service refuses to run without it (returns
  `"TMDB client not configured"`).
- `RAVEN_SEED_KEY` — the route is mounted only inside an `admin` group whose
  middleware checks the `X-Seed-Key` header against `cfg.Seed.Key`. **If
  `RAVEN_SEED_KEY` is empty, every request to `/admin/seed-demo` returns 401**
  — the route is effectively disabled.

For production, **leave both env vars unset.** If you must seed a demo
tenant, set both, run the call once, then unset and restart the API.

## Disable optional services

Not every deployment needs every service. The Compose file is modular —
remove what you do not run:

- **LiveKit + Python voice agent** — voice/WhatsApp calling. Drop the
  `livekit-server` and `python-agent` services if you are not exposing the
  voice features. This also frees the UDP `50000-60000` range.
- **SeaweedFS** — only needed if you ingest binary documents that need
  block-storage. Direct S3 (with `RAVEN_S3_*` env) is supported as an
  alternative.
- **ClickHouse** — only required for the EE vector path; OSS uses Postgres
  + pgvector.
- **Hyperswitch** — payments orchestrator for the billing layer. The OSS
  build does not enable the Hyperswitch worker by default; only deploy it if
  you operate metered billing.
- **eBPF observability** — `docker-compose.ebpf.yml` is an opt-in overlay
  that adds `CAP_BPF` and `CAP_NET_ADMIN` to `go-api`. Only enable it on
  hosts where you understand the kernel-level visibility implications.

The recommended pattern is a `docker-compose.override.yml` that comments out
unused services rather than editing the shipped file — that way the upstream
file can keep diff-clean.

## Recommended additions

The following are not bundled and not enforced by Raven, but are advisable
in any self-hosted production deployment:

- **Web Application Firewall** in front of Traefik — Cloudflare, AWS WAF,
  or self-hosted Coraza/ModSecurity. The shipped Traefik config has hooks
  (`security-headers@file`) but no WAF middleware.
- **OS-level CIS benchmark** for the host (Debian/Ubuntu CIS, AWS-EC2-CIS,
  or your distribution's STIG). At minimum: enable unattended security
  upgrades, disable password SSH (key-only), and run `lynis audit system`
  quarterly.
- **fail2ban** or sshguard for SSH brute-force protection on the host
  itself (Traefik's rate limiter does not cover SSH).
- **Host-level firewall** — `ufw` / `nftables` rules that allow only `22`
  (or your moved SSH port), `80`, `443`, and the LiveKit UDP range.
- **`read_only: true` filesystems with explicit `tmpfs:`** per service in a
  Compose override (`cap_drop: ALL` + `no-new-privileges` are already bundled —
  see [Container hardening](#container-hardening)).
- **Out-of-band log shipping** of `security_events` and OpenObserve to an
  immutable sink — see [Audit logging](#audit-logging).
- **Quarterly dependency review** — Dependabot is enabled and CodeRabbit
  reviews PRs, but a human review of the major-version bumps before merging
  is still warranted.

## See also

- [/trust/openssf-baseline](/trust/openssf-baseline) — full OSPS-L2
  control evidence map.
- [/trust/slsa-level-3](/trust/slsa-level-3) — SLSA verification recipe
  for binaries and container images.
- [/reference/security-policy](/reference/security-policy) — vulnerability
  disclosure path and 72 h / 7 d / 90 d response SLAs.
- [/reference/configuration](/reference/configuration) — exhaustive list
  of `RAVEN_*` environment variables and their defaults.
- [/guides/self-hosting/traefik-and-tls](/guides/self-hosting/traefik-and-tls)
  — deeper Traefik and ACME walk-through.
- [/guides/self-hosting/observability](/guides/self-hosting/observability)
  — OpenObserve and Beszel deployment.
- [/guides/self-hosting/backups](/guides/self-hosting/backups) —
  pgBackRest configuration and restore drills.
