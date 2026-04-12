.PHONY: build run dev test lint migrate-up migrate-down proto swagger compose compose-down

build:
	go build -o bin/api ./cmd/api

run:
	dotenvx run -- go run ./cmd/api

dev:
	dotenvx run -- air

test:
	dotenvx run -- go test ./...

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
	dotenvx run -- docker compose up --build

compose-down:
	docker compose down
