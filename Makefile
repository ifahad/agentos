GO ?= $(HOME)/.local/go/bin/go

.PHONY: test test-go test-python test-console up down logs smoke smoke2 fmt

test: test-go test-python test-console

test-go:
	cd gateway && $(GO) vet ./... && $(GO) test ./...
	cd connectors/sql && $(GO) vet ./... && $(GO) test ./...

test-python:
	cd runtime && uv run ruff check . && uv run pytest

test-console:
	cd console && npm test -- --run && npm run build

up:
	docker compose -f deploy/compose.yaml --env-file deploy/.env up -d --build

down:
	docker compose -f deploy/compose.yaml down -v

logs:
	docker compose -f deploy/compose.yaml logs -f

smoke:
	bash scripts/smoke.sh

smoke2:
	bash scripts/smoke2.sh

smoke3:
	bash scripts/smoke3.sh

smoke4:
	bash scripts/smoke4.sh

smoke5:
	bash scripts/smoke5.sh

test-rust:
	cd sandbox && $(HOME)/.cargo/bin/cargo fmt --check && $(HOME)/.cargo/bin/cargo clippy --all-targets -- -D warnings && $(HOME)/.cargo/bin/cargo test

fmt:
	cd gateway && $(HOME)/.local/go/bin/gofmt -w .
	cd connectors/sql && $(HOME)/.local/go/bin/gofmt -w .
	cd runtime && uv run ruff format .
