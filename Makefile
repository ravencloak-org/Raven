.PHONY: build run dev test lint migrate-up migrate-down proto swagger

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

migrate-up:
	goose -dir migrations postgres "$$DATABASE_URL" up

migrate-down:
	goose -dir migrations postgres "$$DATABASE_URL" down

proto:
	@echo "protoc generation placeholder - will be configured when proto files are added"

swagger:
	swag init -g cmd/api/main.go --output docs/swagger --parseDependency --parseInternal
