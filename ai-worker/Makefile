.PHONY: proto run test lint format install

PROTO_DIR := ../proto
GEN_DIR   := raven_worker/generated

proto: ## Generate Python gRPC stubs from proto definitions
	./scripts/generate_proto.sh

run: ## Start the gRPC server
	python -m raven_worker

test: ## Run the test suite
	pytest

lint: ## Run ruff linter
	ruff check .

format: ## Format code with ruff
	ruff format .

install: ## Install package in editable mode with dev dependencies
	pip install -e ".[dev]"
