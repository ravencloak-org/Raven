---
title: "Upgrades"
---

# Upgrades

Upgrading a Raven instance is a four-step ritual: read the CHANGELOG, take
a backup, pull new images, run migrations. The order matters. Skipping the
backup or skipping the migration step has lost data for operators of
similar stacks; do not improvise.

::: danger Pre-1.0 means breaking changes can land in any minor release
Raven is currently pre-1.0. The [release process](/contributing/release-process)
page is explicit about this:

> While the project is pre-1.0, **minor version bumps may include
> breaking changes**; patch releases (`vX.Y.Z` → `vX.Y.Z+1`) are reserved
> for bug-fix and security backports.

Always read every CHANGELOG entry between your current version and the
target before you upgrade. Schema, config-file shape, and API surfaces
can all change without a deprecation cycle until `v1.0.0` ships.
:::

## Versioning

Raven follows [Semantic Versioning](https://semver.org/). The pre-1.0
contract differs from the post-1.0 contract on **minor** releases:

| Bump    | Pre-1.0 (`v0.x.y`)                                    | Post-1.0 (`v1.x.y` and later)                  |
|---------|-------------------------------------------------------|------------------------------------------------|
| `major` | Reserved — `v1.0.0` itself is the first major bump.   | Breaking changes allowed; documented.          |
| `minor` | **May include breaking changes.** Read CHANGELOG.     | Backwards-compatible features only.            |
| `patch` | Always safe: bug fixes and security backports only.   | Always safe: bug fixes and security backports. |

The release tag format is `vX.Y.Z`. Release candidates use `-rc` suffixes
(`v0.4.0-rc1`); these do not get the `latest` Docker tag and should not
be pulled in production.

See [Release process](/contributing/release-process) for the full
mechanics of how releases are cut, signed, and attested.

## Read the changelog

Every release entry is generated from conventional commits via `git-cliff`
(`cliff.toml` at the repo root) and grouped into these sections:

- **Features** (`feat:`) — new behaviour; check for opt-in flags.
- **Bug Fixes** (`fix:`) — usually safe; read in case the fix changes
  observable behaviour you relied on.
- **Performance** (`perf:`), **Refactors** (`refactor:`) — typically
  internal, but read anyway.
- **Documentation** (`docs:`), **Tests** (`test:`), **CI / Build**
  (`ci:`), **Chores** (`chore:`), **Dependencies** (`deps:`) — rarely
  operator-affecting.
- **Security** (`security:` / `sec:`) — treat as mandatory upgrades.
- **Reverts** (`revert:`) — note carefully; a feature you depended on
  may be gone.

The full changelog is rendered inline on the docs site at
[/reference/changelog](/reference/changelog); it is the same
`CHANGELOG.md` shipped at the root of the repository, included verbatim
via VitePress's `@include` directive. Read it from the version you are
on through the version you are targeting — not just the latest entry.

## Backup before upgrading

**Do not skip this.** Migrations are forward-only (see below) and image
rollback while the database is on a newer schema is not guaranteed to
work.

1. **PostgreSQL full backup via pgBackRest.** The `pgbackrest` sidecar
   in `docker-compose.yml` is the wired path:

   ```bash
   docker compose exec pgbackrest pgbackrest --stanza=raven backup --type=full
   ```

2. **SeaweedFS volume snapshot.** No application-level backup is wired;
   either `docker compose stop seaweedfs-volume` and `tar -czf
   seaweedfs-$(date +%F).tar.gz` the `seaweedfs-data` volume contents,
   or use `weed volume.copy` to a remote target. Full procedure in
   [Backups](/guides/self-hosting/backups).

3. **Capture your `.env`** (or the `dotenvx`-encrypted `.env.keys`)
   somewhere outside the host. Config drift across upgrades is a common
   regression source.

4. **Note your current commit / image tag.** `git rev-parse HEAD` if
   running from source, or `docker compose images go-api` for the
   currently-running image digest. You will need this if you have to
   roll back.

If you do not have a verified restore path, see the
[Backup verification](/guides/self-hosting/backups#backup-verification)
section before touching production.

## Upgrade procedure (cloud / full compose)

This is the path for the full `docker-compose.yml` stack — Go API,
Python AI worker, Python agent, frontend, Postgres, Valkey, SuperTokens,
OpenObserve, LiveKit, SeaweedFS, Traefik, and pgBackRest.

```bash
# 1. Update the source tree (if running from source).
git fetch --tags
git checkout v0.4.0          # the target tag

# 2. Pull the new container images.
docker compose pull

# 3. Apply pending database migrations BEFORE starting the new API.
#    Migrations are NOT auto-run on go-api boot (see "Database
#    migrations" below).
make migrate-up

# 4. Clean stop the running stack. Named volumes are preserved.
docker compose down

# 5. Start the new stack.
docker compose up -d

# 6. Tail logs until everything reports healthy.
docker compose logs -f go-api python-worker python-agent
```

Then smoke-test:

```bash
curl -fsS http://localhost:8080/healthz       # expect: HTTP 200
# Run a search query through the UI.
# Run a chat turn through the UI.
# If voice is in use, start a LiveKit session and speak.
```

If `/healthz` is green and a search round-trips, the upgrade landed.

::: tip Order matters
Apply migrations *before* `docker compose up -d` of the new API image.
A new API talking to an old schema fails at startup; an old API talking
to a new schema may silently corrupt data. The order above puts you in
the safe state at every step.
:::

## Upgrade procedure (edge)

For `docker-compose.edge.yml` deployments (Raspberry Pi and similar —
only Go API, Postgres, and Traefik run on the node; the Python AI worker
lives off-host and is reached over gRPC at `GRPC_AI_WORKER_ADDR`):

```bash
git fetch --tags && git checkout v0.4.0
docker compose -f docker-compose.edge.yml --env-file .env.edge pull
make migrate-up
docker compose -f docker-compose.edge.yml --env-file .env.edge down
docker compose -f docker-compose.edge.yml --env-file .env.edge up -d
```

The edge image is `linux/arm64` only. If you build locally instead of
pulling, use `make -f Makefile.edge docker-arm64` to produce the
matching image before `docker compose up`.

Edge nodes typically do not run pgBackRest in-stack. Take a Postgres
dump (`pg_dump`) to a remote host before upgrading, or skip the
in-stack backup step and rely on your separate off-host backup job.

## Database migrations

Migrations are managed by [pressly/goose](https://github.com/pressly/goose)
v3, files in `migrations/` numbered `00001_*.sql` through `00039_*.sql`
(at time of writing — count grows with each release). The in-DB state
table is `goose_db_version`.

**Migrations are NOT applied automatically on `go-api` startup.** The
production binary (`/app/api` in the container) does not import goose at
all — `github.com/pressly/goose/v3` only appears under
`internal/testutil/db.go` and other test-only paths. The wired operator
path is the `Makefile` target:

```bash
# Apply all pending migrations.
make migrate-up

# Roll back the most recent migration (rarely safe — see "Rollback").
make migrate-down
```

Both targets delegate to:

```bash
dotenvx run -- goose -dir migrations postgres "$DATABASE_URL" up
```

Inspect migration state at any time:

```bash
# Via goose itself (needs DATABASE_URL).
goose -dir migrations postgres "$DATABASE_URL" status

# Or directly in the DB.
psql "$DATABASE_URL" -c \
  "SELECT MAX(version_id) FROM goose_db_version WHERE is_applied = true;"
```

Migration files are forward-only. A `-- +goose Down` block may exist
for development convenience, but **production should not rely on goose
`down` for rollback** — destructive schema changes are not always
reversible from the down-script alone. Use the
[pgBackRest restore path](/guides/self-hosting/backups#restore-procedure)
instead.

## Image tag strategy

The release workflow (`.github/workflows/release.yml`) produces three
image-tag flavours via `docker/metadata-action`:

| Tag pattern         | Example                | When to use                                                      |
|---------------------|------------------------|------------------------------------------------------------------|
| `vX.Y.Z`            | `v0.4.0`               | **Recommended for production.** Pinned, immutable, signed.       |
| `X.Y` (major.minor) | `0.4`                  | Auto-tracks patch releases inside a minor line. Use for staging. |
| `latest`            | `latest`               | **Do not use in production.** Tracks the newest non-RC tag.      |
| `sha-<short>`       | `sha-c3091421`         | Pinned to a specific commit. Useful for bisecting regressions.   |

Set the tag explicitly in `.env`:

```bash
RAVEN_API_IMAGE=ghcr.io/ravencloak-org/go-api:v0.4.0
```

Release-candidate tags (`v0.4.0-rc1`) deliberately do **not** receive
`latest`; the workflow has
`enable=${{ !contains(needs.preflight.outputs.tag, '-rc') }}` on that
tag rule. Pulling `latest` will never silently give you an RC.

All multi-arch manifests (`linux/amd64` + `linux/arm64`) are cosign-signed
with SLSA build provenance and SPDX SBOM attestations. Verify before
deploying — see [SLSA Build Level 3](/trust/slsa-level-3).

## Rollback

Container rollback is one command away; **database rollback is not.**

### Container images

```bash
# Edit .env to pin the previous tag, then:
docker compose pull go-api
docker compose up -d go-api
```

This works **only if the database schema is still compatible**. If the
failed upgrade applied new migrations, the old API may refuse to start
or — worse — start and write data the new schema expected.

### Database

Goose `down` migrations are not a supported production rollback path.
The correct procedure is a pgBackRest point-in-time restore to a moment
just before you ran `make migrate-up`:

```bash
docker compose stop go-api
docker compose exec pgbackrest pgbackrest --stanza=raven \
  --type=time --target="2026-05-12 14:00:00+00" restore
docker compose start postgres
docker compose start go-api
```

Full restore procedure in
[Backups → Restore procedure](/guides/self-hosting/backups#restore-procedure).
You will lose any writes made after the target time; coordinate with
users before doing this in production.

## Breaking changes

The CHANGELOG follows the [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
convention. Breaking changes are surfaced in two places:

- A **`Notes`** section at the bottom of an affected release (visible
  in the rendered changelog) calls out config-shape changes, removed
  endpoints, and required manual steps.
- The commit subject for a breaking change uses the conventional-commits
  `!` marker (`feat!: …`, `refactor!: …`). `cliff.toml` has
  `protect_breaking_commits = false`, so these are not auto-promoted to
  a separate section yet — read the **Notes** block carefully.

Typical breaking changes to look for in `Notes`:

- **Config-shape changes** — renamed or removed `RAVEN_*` environment
  variables. Re-read `.env.example` after each minor bump.
- **Removed endpoints** — the API surface has not stabilised; older
  routes may be removed without a deprecation window pre-1.0.
- **Schema changes that require data migration** — occasionally a
  migration runs a non-trivial backfill. The release notes will say so;
  budget extra downtime.

## Frontend, docs, and landing

These three sites deploy via **Cloudflare Pages** on every push to
`main` (`pages.yml`, `landing.yml`, `docs.yml`). They are **not** part
of the tagged release artefact set. Upgrading them is implicit — merge
to `main` and Cloudflare ships in minutes.

Rollback is via the **Cloudflare Pages dashboard** → project →
Deployments → previous deployment → "Rollback to this deployment". A
revert PR on `main` will also re-deploy; whichever is faster.

This is the same flow documented in
[Release process → Rollback → Cloudflare Pages sites](/contributing/release-process#cloudflare-pages-sites-frontend-landing-docs).

## Voice / LiveKit upgrades

The reference compose pins `livekit/livekit-server:latest` in
`docker-compose.yml`. A LiveKit-server image bump can change the
WebRTC negotiation surface, which means:

- **Browser clients may need a hard refresh** to pick up the matching
  client SDK shipped in the new frontend bundle. After an upgrade that
  touches LiveKit, advise users to reload the tab if voice fails to
  connect once.
- **Old desktop clients** (Tauri) bundled with a prior SDK may need to
  be re-installed. Check the release notes for the desktop bundle.
- **TURN / SFU port range** (`50000-60000/udp`) and `7882/tcp` are
  unchanged across LiveKit minor versions to date — no firewall changes
  expected — but verify in the release notes if voice connection drops
  after upgrade.

For production, pin `livekit/livekit-server` to a tag rather than
`latest` so voice upgrades are deliberate rather than incidental.

## Pre-1.0 caveats

Until `v1.0.0` ships, treat the following as possible in any minor
release:

- **Schema changes without a deprecation cycle.** A column can be
  renamed or dropped; a table can be merged or split. Read every
  migration file added between your version and the target.
- **API shape changes.** Request and response bodies under `/api/v1`
  can change between minor releases. If you have built integrations,
  pin to a `vX.Y.Z` Docker tag and upgrade deliberately.
- **Config-file format changes.** `.env` variables are renamed or
  removed; the YAML overlays in `docker-compose.*.yml` are restructured.
  Diff `.env.example` against your `.env` after every minor bump.
- **Default-value changes.** A feature flag that defaulted to `false`
  may become `true` (or vice versa). The release notes will say so;
  read them.

Once `v1.0.0` ships, standard SemVer applies and minor releases become
strictly additive.

## Post-upgrade checklist

After every upgrade, confirm:

- [ ] `docker compose ps` shows every service in the `healthy` state.
- [ ] `curl -fsS http://localhost:8080/healthz` returns HTTP 200.
- [ ] `psql "$DATABASE_URL" -c "SELECT MAX(version_id) FROM goose_db_version WHERE is_applied = true;"` matches the highest filename in `migrations/`.
- [ ] A search query through the UI returns results.
- [ ] A chat turn through the UI completes (RAG + LLM round-trip).
- [ ] If voice is in use, a LiveKit session connects and audio flows.
- [ ] `docker compose exec pgbackrest pgbackrest --stanza=raven check` succeeds (backups still running).
- [ ] Telemetry — OpenObserve shows traces and logs from the new build
  tag; Beszel shows host metrics steady-state.

If any item fails, do not declare the upgrade complete. Open an issue
with the failing check and the previous + current image tags.

## Cross-references

- [Release process](/contributing/release-process) — how Raven cuts and
  signs releases.
- [Changelog](/reference/changelog) — every release entry, rendered
  inline.
- [Backups](/guides/self-hosting/backups) — pgBackRest, restore, PITR.
- [SLSA Build Level 3](/trust/slsa-level-3) — provenance attached to
  each release.
