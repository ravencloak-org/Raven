---
title: "CLI Reference"
---

# CLI Reference

Raven does not ship a standalone end-user CLI today. The binaries under
[`cmd/`](https://github.com/ravencloak-org/Raven/tree/main/cmd) are long-running
**server processes**, not user-facing tools. There is no `raven` or `ravenctl`
command for operators to drive the platform from a terminal.

Operators interact with Raven through four surfaces, in roughly this order:

- The REST API at `https://api.<your-host>/api/v1/...` — see
  [API overview](/api/overview).
- The Vue dashboard at `https://app.<your-host>` — see the
  [first-knowledge-base guide](/get-started/first-knowledge-base).
- `docker compose` (or `docker compose -f docker-compose.edge.yml`) against
  the running stack — see
  [Self-hosting with Docker Compose](/guides/self-hosting/docker-compose).
- `wrangler` for Cloudflare Pages deploys of the frontend, landing, and docs
  sites — managed in the respective `package.json` scripts, not by Raven core.

This page enumerates what actually exists in the repo today.

## Binaries shipped

| Binary | Purpose | How to run |
|---|---|---|
| [`cmd/api`](https://github.com/ravencloak-org/Raven/tree/main/cmd/api) | The Go HTTP/REST API server (Gin). Listens on `RAVEN_SERVER_PORT`, default `8081`. | `docker compose up go-api`, or `make build && ./bin/api`. |
| [`cmd/worker`](https://github.com/ravencloak-org/Raven/tree/main/cmd/worker) | The Asynq background-job worker. Consumes the same Valkey queue the API enqueues onto; runs document processing, URL scraping, KB reindexing, webhook delivery, and the post-session email summary job. | `docker compose up worker` (the worker is a **separate** Compose service from `go-api`), or `go run ./cmd/worker`. |

That is the entire user-runnable Go surface. There is no `cmd/migrate`,
no `cmd/seed`, and no `cmd/update-check`. The Python AI worker (`ai-worker/`)
is a separate gRPC service launched by Compose, not a Go binary, and is
documented under [AI Worker Design](/concepts/ai-worker-design).

## Server flags

**Neither binary uses `flag.*` or positional sub-commands.** Configuration
is loaded entirely from environment variables (and an optional `.env` file)
by `internal/config.Load()` using Viper with the `RAVEN_` prefix. The full
variable list lives in [Configuration](/reference/configuration).

Concretely, running either binary with `--help`, `-h`, `migrate`, or
`seed-demo` is a no-op — the argument is ignored and the binary boots in
its single normal mode. To change behaviour you change the environment.

The only argv-like control is the standard process signals:

- `SIGINT` / `SIGTERM` → graceful shutdown. Both binaries register handlers
  in their `main()` and drain in-flight work before exiting.

## Helper scripts

These live in [`scripts/`](https://github.com/ravencloak-org/Raven/tree/main/scripts)
and are wrapped by `make` targets where useful.

| Script | Purpose |
|---|---|
| `coverage.sh` | Merge unit, integration, and instrumented-binary Go coverage into one report under `coverage/`. Invoked by `make coverage`. |
| `test-compose.sh` | Validate `docker-compose.yml`, `.env.example` completeness, and Dockerfile healthcheck declarations. |
| `test-ci-workflows.sh` | Lint and dry-run GitHub Actions workflow files locally before pushing CI changes. |
| `validate-migrations.sh` | Check `migrations/*.sql` for `+goose Up`/`+goose Down` blocks, sequential numbering, and no duplicates. |
| `chartdb-query.sh` | Emit `chartdb-query.sql` results as JSON for pasting into the ChartDB schema viewer. |
| `dev-agent.py` | Local-development helper for the AI worker (not for production). |
| `test-keycloak-realm.py` | Legacy realm validator from the pre-SuperTokens era; retained for reference. |

The relevant `make` targets that wrap these (and the binaries) are:

| Target | What it does |
|---|---|
| `make build` | `go build -o bin/api ./cmd/api`. |
| `make run` / `make dev` | Run `cmd/api` directly against the local env. |
| `make migrate-up` / `make migrate-down` | Run goose migrations against `$DATABASE_URL` via `dotenvx`. |
| `make compose` / `make compose-down` | Bring the full Compose stack up or down with `dotenvx`-decrypted secrets. |
| `make lint` | `golangci-lint run`. |
| `make swagger` | Regenerate OpenAPI from `cmd/api/main.go` annotations. |
| `make -f Makefile.edge build-arm64` | Cross-compile static binaries for Raspberry Pi / ARM64 edge nodes. |

## Common operations

| Task | Command |
|---|---|
| Run database migrations | `make migrate-up` (wraps `goose -dir migrations postgres "$DATABASE_URL" up`). |
| Seed demo data | `curl -X POST -H "X-Seed-Key: $RAVEN_SEED_KEY" https://api.<host>/api/v1/admin/seed-demo` — the seed endpoint is **HTTP**, not a CLI sub-command. |
| Tail service logs | `docker compose logs -f <service>` (e.g. `go-api`, `worker`, `ai-worker`). |
| Open a `psql` shell | `docker compose exec postgres psql -U raven` |
| Inspect the queue | `docker compose exec valkey valkey-cli` |
| Verify a signed release | `cosign verify ...` — see [Security Policy](/reference/security-policy) for the full verification recipe. |
| Cross-compile for edge | `make -f Makefile.edge build-arm64` (see [Edge & Raspberry Pi](/guides/self-hosting/edge-and-raspberry-pi)). |

## Where the CLI would go

A dedicated `ravenctl` binary — wrapping the most common operator flows
(tenant creation, key rotation, queue inspection, demo seeding) into a single
typed CLI — is a credible candidate for a future milestone, but it is not on
the current roadmap. Treat this as a tracking item rather than a promise; if
you have a strong use-case, open an issue and reference the
[architecture-decisions process](/contributing/architecture-decisions) so the
shape of the tool gets nailed down before any code lands.

## See also

- [Configuration](/reference/configuration) — every environment variable the
  binaries read.
- [Self-hosting with Docker Compose](/guides/self-hosting/docker-compose) —
  the canonical operator surface today.
- [API overview](/api/overview) — the REST surface that replaces a CLI.
- [Security Policy](/reference/security-policy) — release verification with
  cosign.
