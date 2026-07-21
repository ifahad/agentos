GO ?= $(HOME)/.local/go/bin/go

.PHONY: test test-go test-python up down logs smoke fmt

test: test-go test-python

test-go:
	cd gateway && $(GO) vet ./... && $(GO) test ./...
	cd connectors/sql && $(GO) vet ./... && $(GO) test ./...

test-python:
	cd runtime && uv run ruff check . && uv run pytest

up:
	docker compose -f deploy/compose.yaml --env-file deploy/.env up -d --build

down:
	docker compose -f deploy/compose.yaml down -v

logs:
	docker compose -f deploy/compose.yaml logs -f

smoke:
	bash scripts/smoke.sh

fmt:
	cd gateway && $(HOME)/.local/go/bin/gofmt -w .
	cd connectors/sql && $(HOME)/.local/go/bin/gofmt -w .
	cd runtime && uv run ruff format .
