# Public Demo Deployment — Design

**Date:** 2026-05-12
**Status:** Approved (brainstorm + grill, 2026-05-12)
**Hostname:** `demo.raven.ravencloak.org`
**Audience:** Public CTA — anyone may sign up, data persists, GDPR/DPDP-aware.
**Voice:** Deferred. Demo ships text-only.
**Payments:** Sandbox only. Paid tiers UI shows "Coming soon".

---

## 1. Goals & Non-Goals

### Goals

- Stand up a publicly reachable, production-shaped instance of Raven at `demo.raven.ravencloak.org` so the landing-page CTA can convert a visitor into a signed-in user.
- Use existing repo infra (`deploy/ec2/`, `deploy/ansible/`, dotenvx, Hyperswitch wiring, SuperTokens, Cloudflared) without forking new patterns.
- Bound the steady-state cost at roughly $115–$130/mo and cap the runaway risk (LLM bill, abuse) with concrete fuses.
- Be defensible against a reasonable abuse case: scripted mass-signup, single bad actor, EU-jurisdiction DSAR request.

### Non-Goals

- Voice / LiveKit / WhatsApp surfaces. Services stay in compose so dependency chains hold; UI is hidden; no Cloudflared route. Voice is its own follow-up rollout.
- Live payments. Razorpay sandbox only until KYC + merchant compliance is complete.
- High availability. Single-box demo. Recovery is restore-from-backup, not failover.
- Full SOC2 control set. We build to it during operation, not as a launch gate.

---

## 2. Topology

### Compute

- **Instance:** EC2 `t4g.xlarge` (Graviton, ARM, 4 vCPU / 16 GB) in `ap-south-1`.
- **OS:** Amazon Linux 2023 ARM, latest.
- **Storage:** Single 100 GB `gp3` EBS volume mounted at `/var/lib/raven-data`. All stateful containers (PG, ClickHouse, OpenObserve, SeaweedFS if present) bind-mount under this path. One volume = one snapshot policy.
- **AWS account:** Same account as Beszel, so the existing Beszel agent can monitor host metrics for free.

### Network exposure

- **Cloudflare Tunnel only.** Cloudflared runs on the box, opens an outbound tunnel to Cloudflare. No inbound security-group ports. No Elastic IP.
- **Public hostnames:**
  - `demo.raven.ravencloak.org` → Traefik on `localhost:8080` / `:8180` (frontend + `/api`, existing path-prefix rules).
  - `observability.demo.raven.ravencloak.org` → OpenObserve UI on `localhost:5080`, **gated by Cloudflare Access** policy allowing only `jobinlawrance@gmail.com`.
- **TLS:** Cloudflare-terminated at the edge. Origin uses Tunnel (no public TLS on the box). The Traefik `certresolver=letsencrypt` configuration is unused on the demo overlay; can stay configured but inert.

### Services in scope on demo

`go-api`, `python-worker`, `frontend`, `postgres` (PG18 + pgvector + BM25 extension), `valkey`, `supertokens`, `clickhouse`, `openobserve`, `traefik`, `cloudflared`, `beszel-agent`.

**Kept-but-hidden:** `livekit-server`, STT/TTS sidecars — services run so the python-worker's `depends_on` chain stays satisfied, but no Cloudflared route is published and the frontend hides voice UI via a `RAVEN_VOICE_ENABLED=false` env flag.

**Open implementation question:** the admin dashboard side of voice may also need a gate analogous to the existing `voice-enabled` attribute on `RavenChat.ce.ts`. Decide during implementation plan.

---

## 3. Identity, Abuse Controls, and Cost Fuses

### Sign-in

- SuperTokens, **Google OAuth provider only**. No email/password recipe enabled on demo.
- OAuth redirect URI: `https://demo.raven.ravencloak.org/auth/callback/google`. Must be registered on the Google Cloud OAuth client.

### Abuse controls (layered)

1. **Cloudflare Turnstile** on the signup button. Kills script-driven mass signup.
2. **Existing per-org rate limiting on Valkey** (#205) applies to free tier.
3. **Existing subscription enforcement** (#244) gates AI features on the free plan's limits. The "demo" *is* the free tier.
4. **New: global daily $-fuse** on the python-worker's LLM client. Env var (e.g. `RAVEN_LLM_DAILY_USD_CAP`). When exceeded, AI endpoints return a "demo limit reached, try tomorrow" error. ~50 LOC, single-process counter in Valkey.

No demo-specific quota tables, no demo-specific feature flags beyond `RAVEN_VOICE_ENABLED` and `RAVEN_LLM_DAILY_USD_CAP`.

---

## 4. Provisioning

### Terraform

A new module under `deploy/terraform/demo/`:

- VPC with one public subnet (Cloudflared dials out; no inbound).
- Security group: egress-only, no inbound rules.
- IAM role with:
  - `AmazonSSMManagedInstanceCore` (for SSM session + Run Command).
  - Inline policy: read `ssm:/raven/demo/*` parameters.
  - Inline policy: write to `s3://raven-demo-backups/*`.
  - Inline policy: AWS Backup operations on the data volume.
- 100 GB gp3 EBS volume, encrypted.
- EC2 t4g.xlarge with user-data cloud-init.
- AWS Backup plan: nightly EBS snapshot, 14-day retention.
- S3 bucket `raven-demo-backups` with 30-day lifecycle expiration and versioning.

**TF state:** S3 backend (`raven-tf-state` bucket) with DynamoDB lock table. Same pattern used elsewhere or to-be-created.

### Cloud-init (in user-data)

1. Install git, ansible, docker, docker-compose.
2. Mount the gp3 volume at `/var/lib/raven-data` (formatted on first boot).
3. Clone the repo at the deploy tag.
4. Fetch SSM SecureString params under `/raven/demo/*`, write to `/etc/raven/env` (mode 0600, owner root).
5. Run `ansible-playbook deploy/ansible/playbook.yml -i 'localhost,' -c local`.

### Secrets layout

**In repo (dotenvx-encrypted `.env`):** app-level config, TMDB key, OTel endpoint, default flags.

**In SSM SecureString under `/raven/demo/`:**

- `DOTENV_PRIVATE_KEY`
- `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`
- `RAVEN_ENCRYPTION_AES_KEY`
- `SUPERTOKENS_API_KEY`
- `RAZORPAY_KEY_ID`, `RAZORPAY_KEY_SECRET` (sandbox)
- `HYPERSWITCH_API_KEY`
- `RESEND_API_KEY`
- `CLOUDFLARE_TUNNEL_TOKEN`
- `TURNSTILE_SITE_KEY`, `TURNSTILE_SECRET_KEY`
- `RAVEN_LLM_DAILY_USD_CAP`

Cloud-init reads these via the IAM role and writes a single `/etc/raven/env` file consumed by all compose services.

---

## 5. CI/CD

### Image build

- Existing GitHub Actions workflows already build images. New requirement: **multi-arch (amd64 + arm64) GHCR pushes** on tag.
- Tag format: `v0.x.y`. Tagging `main` triggers the demo-deploy workflow.

### Deploy

- New workflow `.github/workflows/demo-deploy.yml`:
  - Triggers on `v*` tag.
  - Assumes IAM role via **GitHub OIDC** (no long-lived AWS keys in GH Secrets).
  - Calls `aws ssm send-command --document-name AWS-RunShellScript --instance-ids <demo-instance> --parameters 'commands=["/opt/raven/update.sh <tag>"]'`.
  - Polls for command completion; fails the workflow on non-zero exit.
- `deploy/ec2/update.sh` (existing) is the canonical update path: docker compose pull, docker compose up -d, prune, healthcheck.
- No SSH from CI. No deploy-only Linux user. CloudTrail records every deploy.

---

## 6. Data Lifecycle

### Backups (two independent layers)

- **Logical (per-service):** nightly cron in the host (not a container) runs:
  - `pg_dump -Fc` → `s3://raven-demo-backups/postgres/YYYY-MM-DD.dump`
  - `clickhouse-backup create_remote` → S3 remote backend
  - Lifecycle: 30-day expiration
- **Volume:** AWS Backup nightly EBS snapshot, 14-day retention. Faster restore if the whole disk is gone.

A monthly restore drill is part of the operational definition of done (not the launch gate).

### Retention purge

- Nightly cron (host) calls a new admin API endpoint that:
  1. Finds accounts with `last_active_at < now() - 23d` and emails a 7-day warning via Resend.
  2. Sets an in-app banner flag for the same users.
  3. Hard-deletes accounts inactive for 30+ days (cascade to all owned data; SuperTokens user delete).
- Same code path serves the user-initiated DSAR delete (`/account/delete`).

### DSAR endpoints

- `GET /account/export` → on-demand zip of the user's PG rows + ClickHouse vectors + audit log. Streamed.
- `POST /account/delete` → confirms via email, schedules hard delete in 24h (grace window for accidental clicks). Sends final confirmation email on completion.

---

## 7. Payments

- Hyperswitch in **test mode**. Razorpay **sandbox** keys only.
- Frontend renders the existing billing UI (#250). The "Upgrade" button on paid plans is replaced with a "Join waitlist" form (email captured to PG, no payment flow).
- "Free" plan is the only actually-usable plan. All AI/feature gates apply at the free-tier limit defined in the existing subscription enforcement.

---

## 8. Email

- **Resend** free tier (3000/mo).
- From: `noreply@ravencloak.org`. SPF + DKIM + DMARC records added to the Cloudflare DNS zone for `ravencloak.org`.
- Used for: retention warning emails, DSAR confirmations, DSAR delete final receipt. Not for OTP (SuperTokens Google OAuth doesn't need it).
- Provider abstracted behind a `MailSender` interface in the API so we can swap to AWS SES later (#257) without code changes outside the adapter.

---

## 9. Observability

- **OpenObserve** UI at `observability.demo.raven.ravencloak.org`, Cloudflare Access policy: allow `jobinlawrance@gmail.com` only. Ingestion (`:5081`) stays on the docker network — not exposed externally.
- **Beszel** existing AWS instance monitors the demo box for host metrics.
- **BetterStack** external uptime monitor:
  - Heartbeat hits `https://demo.raven.ravencloak.org/healthz` every 60s.
  - Below-threshold or non-200 = email + phone push to `jobinlawrance@gmail.com`.
- No PagerDuty / on-call rotation. Single operator.

---

## 10. Compliance

### At launch

- **Privacy policy** and **Terms of Service** pages on the landing site, linked from the demo's footer and signup screen.
- **Cookie consent banner** — essential-only. No analytics cookies set until consent is granted (and PostHog stays off entirely on the demo until M9 ships).
- **DSAR endpoints** functional (see §6).
- Public `security@ravencloak.org` mailbox forwarded to the operator.

### Built during operation (not launch gates)

- SOC2 audit log of admin actions.
- Formal vendor list / data-flow diagram.
- DPA template for design-partner accounts.

---

## 11. First-Run UX

- Every Google signup triggers a **pre-seeded sample workspace** so the product is non-empty from second 1.
- Reuses the existing Keycloak realm onboarding wizard wiring shape (#251) — adapted to the SuperTokens flow.
- Seed content is fixture-defined under `migrations/seed/demo/`. Idempotent. Re-applied on user creation, not on every login.

**Open implementation question (deferred to plan):** exact fixture contents — what's in a "sample workspace" depends on the product surface we want to highlight. Owner: the implementation plan brainstorm.

---

## 12. Launch Sequencing

Three phases, each gated by 48h of clean monitoring (no `/healthz` flaps, no quota exhaustion, no error spikes in OpenObserve).

### Phase 1 — Internal

- Box up, Cloudflared route active.
- Cloudflare Access policy in front of **whole hostname**: allow `jobinlawrance@gmail.com` only.
- You are the only signup. Smoke-test full flow.

### Phase 2 — Closed beta

- Remove Cloudflare Access from the public hostname (`observability.*` stays gated).
- Add an invite-code field to the signup form (alongside Google OAuth).
- Distribute 10–20 codes to named friends and design partners.
- Turnstile is live on the signup button.

### Phase 3 — Public CTA

- Remove invite-code field.
- Landing page CTA points to `https://demo.raven.ravencloak.org`.
- Soft announcement first (your network, Twitter). HN/Reddit only after another 48h of clean.

### Definition of done (per phase)

- Tag deployed cleanly via the GHCR + SSM path.
- E2E Playwright suite (#201, #243) passes on the deployed image.
- `/healthz` green for 48 contiguous hours.
- BetterStack monitor active.
- One successful restore drill against staging backups (Phase 1 only; not re-required for 2 → 3).
- DSAR endpoints manually verified end-to-end.
- Privacy policy / ToS / cookie banner live.
- `RAVEN_LLM_DAILY_USD_CAP` set to a non-zero value.

---

## 13. Cost Envelope

| Item | Monthly estimate |
|---|---|
| EC2 t4g.xlarge | ~$110 |
| 100 GB gp3 | ~$10 |
| EBS snapshots (~50 GB delta) | ~$2 |
| S3 backup storage (~10 GB) | ~$0.30 |
| Cloudflare (Tunnel + Access + Turnstile + DNS) | $0 (free tier) |
| Resend | $0 (free tier) |
| BetterStack | $0 (free tier) |
| LLM API spend (capped) | ≤ `RAVEN_LLM_DAILY_USD_CAP` × 30 |

**Steady-state:** ~$125/mo + LLM budget. The fuse means LLM cannot exceed the configured cap.

---

## 14. Risks & Mitigations

| Risk | Mitigation |
|---|---|
| Single box dies | EBS snapshot restore + Terraform re-apply. Documented in a runbook before Phase 2. |
| LLM bill runaway | Per-account free-tier quota + global daily $-fuse. Two independent fuses. |
| Scripted signup flood | Turnstile + Phase 2 invite gate. |
| Cloudflared outage | Demo is unreachable. No fallback path. Acceptable for a demo; document in status page. |
| GHCR rate limit on deploy | Use authenticated pulls (deploy IAM role has no relevance here; use a GHCR PAT in `/etc/raven/env`). |
| Voice services consume RAM but yield no value | Accepted (~300–500MB). Cheaper than refactoring `depends_on` for a demo. |
| EU user signs up, files DSAR | `/account/export` and `/account/delete` already wired. |

---

## 15. Open Implementation Questions (for the plan)

These are decisions deferred to the implementation plan, not blockers on this spec:

1. Exact contents of the seeded sample workspace fixtures.
2. Admin-dashboard voice-UI gating mechanism (env flag, build flag, or runtime feature flag).
3. Whether the global LLM $-fuse counter lives in Valkey (shared) or in-process (per-replica) — Valkey is the obvious answer for future multi-replica but adds a hop.
4. Exact CloudWatch/SSM document for `update.sh` invocation (output capture, timeout, retries).
5. Whether the host-side backup cron runs as a systemd timer or as a sidecar container.
