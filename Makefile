.PHONY: build build-cover run dev test test-integration bench-integration coverage coverage-unit coverage-integration coverage-html coverage-clean lint migrate-up migrate-down proto swagger compose compose-down

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
