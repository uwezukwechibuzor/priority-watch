.PHONY: setup up down logs build lint test fmt migrate backfill follow dbt-build dbt-docs web-dev docs-serve smoke help

BINARY_DIR := bin
INGESTER    := $(BINARY_DIR)/pw-ingester
API         := $(BINARY_DIR)/pw-api

## setup: one-time dev environment setup (install hooks, tools)
setup:
	@echo "→ enabling .githooks"
	git config core.hooksPath .githooks
	chmod +x .githooks/commit-msg .githooks/pre-commit .githooks/pre-push
	@echo "→ installing Go tools"
	go install mvdan.cc/gofumpt@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "✓ setup complete — run 'make build' to verify"

## up: start all services via Docker Compose
up:
	docker compose up -d

## down: stop all services
down:
	docker compose down

## logs: tail all service logs
logs:
	docker compose logs -f

## build: compile all Go binaries
build:
	mkdir -p $(BINARY_DIR)
	go build -o $(INGESTER) ./cmd/pw-ingester
	go build -o $(API) ./cmd/pw-api

## lint: run all linters (golangci-lint, ruff, sqlfluff, biome)
lint:
	golangci-lint run ./...
	@echo "✓ golangci-lint clean"

## test: run all tests with race detector
test:
	go test -race -count=1 ./...

## test-integration: run integration tests (requires running services)
test-integration:
	go test -race -count=1 -tags=integration ./...

## fmt: format Go and frontend code
fmt:
	gofumpt -l -w .

## migrate: run ClickHouse schema migrations
migrate:
	$(INGESTER) migrate

## backfill: backfill blocks (usage: make backfill FROM=<block> TO=<block>)
backfill:
	$(INGESTER) backfill --from=$(FROM) --to=$(TO)

## follow: start the live block follower
follow:
	$(INGESTER) follow

## dbt-build: run dbt incremental models
dbt-build:
	cd dbt && dbt build --select state:modified+

## dbt-docs: generate and serve dbt docs
dbt-docs:
	cd dbt && dbt docs generate && dbt docs serve

## web-dev: start the frontend dev server
web-dev:
	cd apps/web && npm run dev

## docs-serve: serve MkDocs documentation
docs-serve:
	mkdocs serve

## smoke: quick smoke test against a running stack
smoke:
	curl -sf http://localhost:$(or $(API_PORT),8080)/healthz | grep -q '"status":"ok"'
	@echo "✓ smoke test passed"

## help: list all targets
help:
	@grep -E '^## ' Makefile | sed 's/^## //' | column -t -s ':'
