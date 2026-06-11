.PHONY: build build-cover run dev test test-integration bench-integration coverage coverage-unit coverage-integration coverage-html coverage-clean lint migrate-up migrate-down proto swagger compose compose-down sql-lint-local schema-inspect schema-diff

build:
	go build -o bin/api ./cmd/api

# Build an instrumented `./cmd/api` binary that writes coverage counters to
# $$GOCOVERDIR at runtime. Point GOCOVERDIR at .coverdata/binary/ (or any dir)
# before invoking the resulting binary in your harness; `make coverage` will
# pick up anything it finds there automatically.
build-cover:
	mkdir -p bin .coverdata/binary
	go build -cover \
		-coverpkg=github.com/ravencloak-org/Raven/internal/...,github.com/ravencloak-org/Raven/pkg/...,github.com/ravencloak-org/Raven/cmd/... \
		-o bin/api-cover ./cmd/api

run:
	dotenvx run -- go run ./cmd/api

dev:
	dotenvx run -- air

test:
	dotenvx run -- go test ./...

test-integration:
	go test -tags=integration ./internal/integration/ -v -timeout 5m -count=1

bench-integration:
	go test -tags=integration ./internal/integration/ -bench=. -benchmem -timeout 10m

# Produce a merged unit + integration (+ instrumented-binary if present)
# coverage report. See scripts/coverage.sh for the full pipeline.
coverage:
	./scripts/coverage.sh

coverage-unit:
	SKIP_INTEGRATION=1 ./scripts/coverage.sh

coverage-integration:
	SKIP_UNIT=1 ./scripts/coverage.sh

coverage-html: coverage
	@printf 'open %s/coverage/coverage.html in a browser\n' "$$PWD"

coverage-clean:
	rm -rf .coverdata coverage

lint:
	golangci-lint run

migrate-up:
	dotenvx run -- goose -dir migrations postgres "$$DATABASE_URL" up

migrate-down:
	dotenvx run -- goose -dir migrations postgres "$$DATABASE_URL" down

proto:
	@echo "protoc generation placeholder - will be configured when proto files are added"

swagger:
	swag init -g cmd/api/main.go --output docs/swagger --parseDependency --parseInternal

compose:
	@if [ -f ./.env.keys ]; then set -a; . ./.env.keys; set +a; fi; \
	dotenvx run -- docker compose up --build

compose-down:
	@if [ -f ./.env.keys ]; then set -a; . ./.env.keys; set +a; fi; \
	docker compose down

sql-lint-local:
	@base=$$(git merge-base HEAD origin/main 2>/dev/null || echo HEAD~); \
	files=$$(git diff --diff-filter=d --name-only $$base...HEAD -- 'migrations/*.sql' | tr '\n' ' '); \
	if [ -z "$$files" ]; then \
		echo "No migrations modified vs origin/main — nothing to lint."; \
	else \
		echo "Linting modified migrations: $$files"; \
		npx --yes squawk-cli@latest $$files; \
	fi

# --- Atlas schema version-control (runs ALONGSIDE goose; goose stays the applier) ---
# `schema-inspect` regenerates db/schema.sql from a throwaway pgvector DB that has
# had ALL goose migrations applied. `schema-diff` reports drift between db/schema.sql
# and that goose-migrated DB. Both spin an ephemeral pgvector/pgvector:pg18 container
# via scripts/atlas-dev-db.sh and tear it down on exit. Requires the `atlas` CLI
# (brew install ariga/tap/atlas) and a container runtime (podman locally, docker in CI).
# See docs/adr/0010-atlas-schema-version-control.md.

schema-inspect:
	@set -e; \
	urls=$$(scripts/atlas-dev-db.sh); \
	trap '$${ATLAS_DEV_RUNTIME:-podman} rm -f $${ATLAS_DEV_CONTAINER:-raven-atlas-dev} >/dev/null 2>&1 || true' EXIT; \
	goose_url=$$(printf '%s\n' "$$urls" | sed -n 1p); \
	goose -dir migrations postgres "$$goose_url" up >&2; \
	printf '%s\n' \
	  '-- ============================================================================' \
	  '-- Atlas-managed canonical schema for Raven (schema-as-code).' \
	  '--' \
	  '-- DO NOT EDIT BY HAND. Regenerate with:  make schema-inspect' \
	  '--' \
	  '-- This file is the declarative, version-controlled snapshot of the Postgres' \
	  '-- schema, produced verbatim by `atlas schema inspect` against a database that' \
	  '-- has had ALL goose migrations in migrations/ applied. It is the Dolt-like' \
	  '-- artifact: `git diff` / `git blame` / branch / merge the schema shape here.' \
	  '--' \
	  '-- goose REMAINS the applier of record (Makefile `migrate-up`). Atlas does NOT' \
	  '-- apply migrations at runtime; it only provides schema version-control review.' \
	  '-- See docs/adr/0010-atlas-schema-version-control.md.' \
	  '--' \
	  '-- EXTENSIONS are owned by migrations/00001_extensions_and_types.sql, which runs:' \
	  '--     CREATE EXTENSION "uuid-ossp";' \
	  '--     CREATE EXTENSION "vector";   -- pgvector, enables vector(768) + HNSW below' \
	  '--     CREATE EXTENSION "pg_trgm";' \
	  '-- They are intentionally NOT re-emitted as executable DDL in this file: the' \
	  '-- free-tier `atlas` binary rejects CREATE EXTENSION in a declarative schema' \
	  '-- source ("extensions are available to logged-in users only"), and re-emitting' \
	  '-- them would break `make schema-diff`. The pgvector round-trip is proven below' \
	  '-- by the vector(768) columns and the `USING HNSW (... vector_cosine_ops)`' \
	  '-- indexes, which Atlas inspected and reproduces faithfully. The Atlas dev-url' \
	  '-- used by `make schema-inspect` / `make schema-diff` pre-installs these three' \
	  '-- extensions (and the raven_app / raven_admin roles) so the schema materialises.' \
	  '-- ============================================================================' \
	  '' > db/schema.sql; \
	atlas schema inspect --env local --url "$$goose_url" --format '{{ sql . }}' >> db/schema.sql; \
	echo "Regenerated db/schema.sql" >&2

schema-diff:
	@set -e; \
	urls=$$(scripts/atlas-dev-db.sh); \
	trap '$${ATLAS_DEV_RUNTIME:-podman} rm -f $${ATLAS_DEV_CONTAINER:-raven-atlas-dev} >/dev/null 2>&1 || true' EXIT; \
	goose_url=$$(printf '%s\n' "$$urls" | sed -n 1p); \
	atlas_dev=$$(printf '%s\n' "$$urls" | sed -n 2p); \
	goose -dir migrations postgres "$$goose_url" up >&2; \
	ATLAS_DEV_URL="$$atlas_dev" atlas schema diff --env local --from "$$goose_url" --to file://db/schema.sql
