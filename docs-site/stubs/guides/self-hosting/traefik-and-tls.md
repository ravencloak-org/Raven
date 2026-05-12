---
title: "Traefik & TLS"
---

# Traefik & TLS

Traefik is the single internet-facing service in a Raven deployment. It
terminates TLS, routes by hostname and path, and is the only container that
should publish ports `80` and `443` to the host. Every other Raven service —
the Go API, LiveKit, OpenObserve, SuperTokens — listens only on the internal
Docker network and is reached **through** Traefik.

This guide is a code-grounded operator reference for the Traefik layer as it
ships in this repository. For the broader hardening posture (HSTS preload,
rate limits, container-level controls) cross-link to
[/guides/self-hosting/hardening](/guides/self-hosting/hardening); for compose
topology see [/guides/self-hosting/docker-compose](/guides/self-hosting/docker-compose).

## Service overview

The `traefik` service is defined in
[`docker-compose.yml`](https://github.com/ravencloak-org/Raven/blob/main/docker-compose.yml)
and uses the pinned v3 image:

```yaml
traefik:
  image: traefik:v3.3
  ports:
    - "80:80"     # ACME HTTP-01 fallback + HTTP→HTTPS redirect
    - "443:443"   # TLS-terminated web traffic
    - "8082:8080" # dashboard (dev only — disable in prod)
  volumes:
    - /var/run/docker.sock:/var/run/docker.sock:ro
    - ./deploy/traefik/traefik.yml:/etc/traefik/traefik.yml:ro
    - ./deploy/traefik/dynamic:/etc/traefik/dynamic:ro
    - acme-data:/acme
  networks:
    - raven-internal
```

Key points:

- **Image**: `traefik:v3.3` (Traefik v3, static config schema).
- **Network**: `raven-internal` — every backend Traefik routes to must
  join the same network so the Docker provider can resolve container IPs.
- **ACME storage**: the named volume `acme-data` is mounted at `/acme` and
  holds `acme.json` (account key, issued certs, renewal state). Back this up.
- **Docker socket**: mounted read-only so Traefik can watch container start
  / stop events and rebuild its router table.

The edge variant (`docker-compose.edge.yml`) uses the same image
(`traefik:v3.3`, `platform: linux/arm64`) but passes static config via the
CLI flags `--providers.docker=true --providers.docker.network=raven-edge`
and skips the dashboard with `--api=false`.

## Static config — `deploy/traefik/traefik.yml`

The full static config lives at [`deploy/traefik/traefik.yml`](https://github.com/ravencloak-org/Raven/blob/main/deploy/traefik/traefik.yml).
It is mounted read-only at `/etc/traefik/traefik.yml`.

### Entrypoints

```yaml
entryPoints:
  web:
    address: ":80"
    http:
      redirections:
        entryPoint:
          to: websecure
          scheme: https
          permanent: true
  websecure:
    address: ":443"
    http:
      tls:
        certResolver: letsencrypt
      middlewares:
        - security-headers@file
```

- `web` (port 80) does nothing useful for application traffic — every
  request is 301'd to `websecure`. Port 80 stays open only for the ACME
  HTTP-01 fallback and for the redirect itself.
- `websecure` (port 443) is the real entry. **Every** request through it
  is run through the `security-headers@file` middleware (defined in the
  dynamic config below) before reaching a router.

### Providers

```yaml
providers:
  docker:
    endpoint: "unix:///var/run/docker.sock"
    exposedByDefault: false
    network: raven-internal
  file:
    directory: /etc/traefik/dynamic
    watch: true
```

- `exposedByDefault: false` — a container is invisible to Traefik until
  it sets `traefik.enable=true` in its labels. This is deliberate: backends
  that should never face the internet (PostgreSQL, Valkey, the AI worker)
  simply omit the label.
- `network: raven-internal` is the network Traefik will connect to when
  proxying — if a service is on a different network, Traefik can see the
  labels but cannot reach the container.
- `file.watch: true` means dynamic config changes (TLS options, headers,
  middlewares) reload without restarting Traefik.

### Dashboard / API

```yaml
api:
  dashboard: true
  insecure: true
```

This is the **dev default**. `insecure: true` exposes the dashboard on
the entrypoint without auth. See
[Disabling the dashboard in production](#disabling-the-dashboard-in-production)
below — this MUST be flipped before going to prod.

## Dynamic config — `deploy/traefik/dynamic/tls.yml`

Dynamic configuration is loaded from the file provider at
[`deploy/traefik/dynamic/`](https://github.com/ravencloak-org/Raven/tree/main/deploy/traefik/dynamic).
The single file (`tls.yml`) defines TLS options and shared middlewares.

### TLS options

```yaml
tls:
  options:
    default:
      minVersion: VersionTLS12
      cipherSuites:
        - TLS_AES_256_GCM_SHA384
        - TLS_AES_128_GCM_SHA256
        - TLS_CHACHA20_POLY1305_SHA256
        - TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384
        - TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384
        - TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256
        - TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256
        - TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256
        - TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256
      sniStrict: true
```

- `minVersion: VersionTLS12` — TLS 1.0/1.1 are refused.
- Cipher list is GCM and ChaCha20-Poly1305 only; no CBC, no RSA key
  exchange, no static keys.
- `sniStrict: true` — a TLS handshake without a matching SNI is
  terminated with `unrecognized_name`, not served the default cert. This
  blocks scanner traffic that hits the IP directly.

### Middlewares

```yaml
http:
  middlewares:
    security-headers:
      headers:
        stsSeconds: 63072000
        stsIncludeSubdomains: true
        stsPreload: true
        forceSTSHeader: true
        contentTypeNosniff: true
        browserXssFilter: true
        frameDeny: true
        customFrameOptionsValue: "SAMEORIGIN"
        referrerPolicy: "strict-origin-when-cross-origin"
        customResponseHeaders:
          X-Powered-By: ""
          Server: ""

    rate-limit:
      rateLimit:
        average: 100
        burst: 200
        period: 1s
```

Two named middlewares are exported:

- `security-headers@file` — attached globally on the `websecure` entrypoint
  (HSTS preload, anti-clickjacking, `X-Content-Type-Options`, redacted
  `Server` header). See
  [/guides/self-hosting/hardening#security-headers](/guides/self-hosting/hardening)
  for the rationale and OSPS-L2 evidence trail.
- `rate-limit@file` — 100 req/s average, 200 burst, per source. Not
  attached by default; opt-in on noisy routers via
  `traefik.http.routers.<name>.middlewares=rate-limit@file`.

## ACME / Let's Encrypt

```yaml
certificatesResolvers:
  letsencrypt:
    acme:
      email: "${ACME_EMAIL}"
      storage: /acme/acme.json
      caServer: "${ACME_CA_SERVER:-https://acme-v02.api.letsencrypt.org/directory}"
      dnsChallenge:
        provider: "${ACME_DNS_PROVIDER:-cloudflare}"
        delayBeforeCheck: 10
        resolvers:
          - "1.1.1.1:53"
          - "8.8.8.8:53"
```

- **CA**: defaults to Let's Encrypt production
  (`https://acme-v02.api.letsencrypt.org/directory`). Override
  `ACME_CA_SERVER` to point at the staging directory
  (`https://acme-staging-v02.api.letsencrypt.org/directory`) when
  iterating — staging has much higher rate limits and untrusted certs.
- **Storage**: `/acme/acme.json` inside the `acme-data` named volume.
  This file contains the account key and every issued cert — back it up,
  protect it (Traefik creates it as `0600`), and never commit it.
- **Challenge type**: **DNS-01** via the Cloudflare provider by default.
  Required environment variables in `.env`:
  - `ACME_EMAIL` — registration address for renewal notices.
  - `CF_API_EMAIL` — Cloudflare account email.
  - `CF_DNS_API_TOKEN` — scoped API token with `Zone:DNS:Edit` on the
    relevant zone.

  To switch providers, override `ACME_DNS_PROVIDER` and supply the env
  vars Traefik expects for that provider (see the
  [Traefik ACME providers reference](https://doc.traefik.io/traefik/https/acme/#providers)).
- **HTTP-01 fallback**: not configured. Switching from DNS-01 to HTTP-01
  requires editing `traefik.yml` to replace `dnsChallenge:` with
  `httpChallenge: { entryPoint: web }` and ensuring port 80 is reachable
  from the public internet.

DNS-01 is preferred because it works behind NAT, behind Cloudflare proxy,
and can issue wildcards (`*.raven.example.com`) — which matters for the
upcoming subdomain layout (see [Hostname routing](#hostname-routing)).

## Service discovery via Docker labels

Traefik discovers backends by reading container labels at startup and on
every Docker event. Here is the `go-api` service from `docker-compose.yml`
verbatim:

```yaml
go-api:
  labels:
    - "traefik.enable=true"
    - "traefik.http.routers.api.rule=Host(`${RAVEN_DOMAIN:-localhost}`) && PathPrefix(`/api`)"
    - "traefik.http.routers.api.entrypoints=websecure"
    - "traefik.http.routers.api.tls=true"
    - "traefik.http.routers.api.tls.certresolver=${TRAEFIK_CERT_RESOLVER:-letsencrypt}"
    - "traefik.http.routers.api.middlewares=security-headers@file"
    - "traefik.http.services.api.loadbalancer.server.port=8081"
```

Label by label:

| Label | Meaning |
|---|---|
| `traefik.enable=true` | Opt this container into discovery (the provider is `exposedByDefault: false`). |
| `traefik.http.routers.api.rule` | Match HTTPS requests to host `${RAVEN_DOMAIN}` with path prefix `/api`. |
| `traefik.http.routers.api.entrypoints=websecure` | Only attach to the `:443` entrypoint — bare HTTP is handled by the global redirect. |
| `traefik.http.routers.api.tls=true` | Negotiate TLS using the entrypoint's resolver. |
| `traefik.http.routers.api.tls.certresolver=letsencrypt` | Use the `letsencrypt` resolver defined in static config. |
| `traefik.http.routers.api.middlewares=security-headers@file` | Apply the HSTS / nosniff middleware before hitting the backend. |
| `traefik.http.services.api.loadbalancer.server.port=8081` | Backend listens on `:8081` inside the container. |

The router name (`api`) is local to the container — keep it unique per
service.

## Hostname routing

Today only two services own a public router in `docker-compose.yml`:

| Service | Router | Rule | Backend port |
|---|---|---|---|
| `go-api` | `api` | `Host(${RAVEN_DOMAIN}) && PathPrefix(/api)` | `:8081` |
| `livekit-server` | `livekit` | `Host(${RAVEN_DOMAIN}) && PathPrefix(/livekit)` | `:7880` |

LiveKit additionally has an inline path-strip middleware so the
backend sees `/` rather than `/livekit/...`:

```yaml
- "traefik.http.routers.livekit.middlewares=livekit-strip@docker"
- "traefik.http.middlewares.livekit-strip.stripprefix.prefixes=/livekit"
```

Other services (`postgres`, `valkey`, `supertokens`, `python-worker`,
`python-agent`, `openobserve`, `seaweedfs-*`, `clickhouse`) intentionally
omit Traefik labels — they are reachable only within `raven-internal`.

A `*.raven.ravencloak.org` subdomain split (e.g. `app.`, `auth.`, `logs.`,
`monitor.`) is tracked separately and will replace the path-prefix layout
once the wildcard ACME cert and DNS records are in place.

## Production gotchas

- **Port 80 must be reachable.** Even on a DNS-01-only setup, port 80
  serves the HTTP→HTTPS redirect users will hit first. Some ISPs and
  residential CGNAT routes block inbound 80; verify with
  `curl -v http://your-host/` from outside.
- **A and AAAA records first, ACME second.** Let's Encrypt validates
  DNS-01 by querying `_acme-challenge.<host>` from public resolvers; if
  your zone's NS records lag, validation fails with `propagation timeout`.
  The static config already sets `delayBeforeCheck: 10` and uses
  `1.1.1.1` / `8.8.8.8` as resolvers.
- **Test against staging first.** Production Let's Encrypt enforces
  [strict rate limits](https://letsencrypt.org/docs/rate-limits/) — 5
  failures per account per hour, 50 certs per registered domain per
  week. Export `ACME_CA_SERVER=https://acme-staging-v02.api.letsencrypt.org/directory`
  during a first deploy, delete the resulting `acme.json`, then re-issue
  against prod.
- **Back up `acme.json`.** Losing the named volume `acme-data` means
  re-issuing every certificate from scratch — which can trip the per-week
  rate limit if you have several hosts.

## Security headers

The dynamic config exports two middleware names:

- `security-headers@file` — HSTS preload, `X-Content-Type-Options`,
  `X-Frame-Options: SAMEORIGIN`, `Referrer-Policy`, and `Server` /
  `X-Powered-By` suppression.
- `rate-limit@file` — opt-in token bucket (100 rps average, 200 burst).

The full rationale, the OSPS-L2 evidence mapping, and per-service header
overrides live in [/guides/self-hosting/hardening](/guides/self-hosting/hardening) —
this guide intentionally does not duplicate them.

## mTLS between services

Traefik does **not** mTLS to backends today. The trust boundary is the
Docker network `raven-internal`: anything attached to it is trusted by
the other services on it. There is no client-cert auth between Traefik
and the Go API, between the Go API and the Python worker, or between the
Go API and PostgreSQL.

If you need cryptographic identity between containers (zero-trust /
SPIFFE-style), that is a workload-mesh concern (Linkerd, Istio, Consul
Connect) layered on top of Compose — Raven does not bake that in.

## Disabling the dashboard in production

The shipped `traefik.yml` is dev-friendly:

```yaml
api:
  dashboard: true
  insecure: true
```

`insecure: true` exposes the dashboard on the public entrypoint without
auth. **Flip both to `false` in production** — either edit
`traefik.yml` and redeploy, or override via CLI (the edge compose already
does this with `--api=false`):

```yaml
api:
  dashboard: false
  insecure: false
```

If you want the dashboard available to operators, attach it to a router
behind `Host(...)` and a `basicAuth` middleware, never on the
unauthenticated `:8080` port. Also stop publishing `8082:8080` to the
host in the compose file.

## Cloudflare in front

If Cloudflare proxies your hostname (orange-cloud), client IPs reaching
Traefik are Cloudflare edge IPs, and the real client IP is in the
`CF-Connecting-IP` / `X-Forwarded-For` headers. Traefik discards
forwarded headers by default unless the source is an explicitly trusted
proxy. To trust Cloudflare:

1. Extend the `websecure` entrypoint with a `forwardedHeaders.trustedIPs`
   list containing
   [Cloudflare's published IP ranges](https://www.cloudflare.com/ips/).
2. Mirror the same list under `proxyProtocol.trustedIPs` if you also
   enable Cloudflare's PROXY protocol (Spectrum).

Without this, every request appears to come from a Cloudflare node and
`rate-limit@file` becomes useless. Note that the shipped
`deploy/traefik/traefik.yml` does **not** ship a default trusted-IP list —
add it per deployment, because the trust set depends on whether you
front Traefik with Cloudflare at all.

## Troubleshooting

**ACME challenge fails.**

```
unable to generate a certificate for the domains [...] propagation: time limit exceeded
```

- Check that `CF_DNS_API_TOKEN` has `Zone:DNS:Edit` on the right zone.
- Verify the zone NS records actually point at Cloudflare.
- Confirm port 80 is reachable from outside the host
  (`curl -v http://<host>/`) — even DNS-01 keeps the redirect.
- Watch `docker logs traefik | grep -i acme` for the specific failure;
  retry against staging first.

**Routing returns 404.**

- `docker exec traefik traefik healthcheck` returns OK but a URL 404s →
  Traefik is up but no router matched. Common causes:
  - Typo in a `traefik.http.routers.<name>.rule` label.
  - Backend not on `raven-internal` — Traefik sees the label but can't
    reach the container.
  - `traefik.enable=true` missing.
- Open the dashboard (or run `traefik` with `--log.level=DEBUG`) and
  confirm the router shows in the **HTTP Routers** tab.

**TLS handshake errors.**

- `unrecognized_name` — `sniStrict: true` rejected the request because
  the client did not send SNI matching any router rule. Hit the actual
  hostname, not the IP.
- `no certificate found` on a sub-domain — the cert covers a different
  name. DNS-01 + wildcards solve this; HTTP-01 cannot issue wildcards.

**Dashboard not reachable.**

- Confirm `8082:8080` is published and `api.insecure: true` in
  `traefik.yml` — both required for the dev dashboard. If you disabled
  it for prod, that is intentional.

## See also

- [/guides/self-hosting/hardening](/guides/self-hosting/hardening) —
  full hardening posture, security-header rationale, OSPS-L2 evidence.
- [/guides/self-hosting/docker-compose](/guides/self-hosting/docker-compose) —
  compose variants, service map, common workflows.
- [/reference/configuration](/reference/configuration) — every
  `RAVEN_*` and `ACME_*` environment variable and default value.
