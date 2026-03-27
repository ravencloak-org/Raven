.PHONY: build run dev test lint lint-fix check setup-hooks migrate-up migrate-down proto

build:
	go build -o bin/api ./cmd/api

run:
	go run ./cmd/api

dev:
	air

test:
	go test ./...

lint:
	golangci-lint run

lint-fix:
	golangci-lint run --fix

check: lint
	go vet ./...
	go test -short ./...

setup-hooks:
	git config core.hooksPath .githooks
	@echo "Git hooks installed. Pre-push lint check is now active."

migrate-up:
	goose -dir migrations postgres "$$DATABASE_URL" up

migrate-down:
	goose -dir migrations postgres "$$DATABASE_URL" down

proto:
	@echo "protoc generation placeholder - will be configured when proto files are added"
