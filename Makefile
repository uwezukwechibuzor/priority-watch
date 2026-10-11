.PHONY: setup up up-obs up-test down down-obs down-test logs logs-obs \
        build lint test test-integration fmt \
        migrate backfill follow \
        dbt-build dbt-docs \
        web-dev docs-serve docs-build \
        k6 release \
        smoke help

BINARY_DIR := bin
INGESTER   := $(BINARY_DIR)/pw-ingester
API        := $(BINARY_DIR)/pw-api

COMPOSE_BASE := docker-compose.yml
COMPOSE_DEV  := docker-compose.dev.yml
COMPOSE_OBS  := docker-compose.observability.yml
COMPOSE_TEST := docker-compose.test.yml

COMPOSE     := docker compose -f $(COMPOSE_BASE) -f $(COMPOSE_DEV)
COMPOSE_ALL := $(COMPOSE) -f $(COMPOSE_OBS)

## setup: one-time dev environment setup (install hooks, tools)
setup:
	@echo "→ enabling .githooks"
	git config core.hooksPath .githooks
	chmod +x .githooks/commit-msg .githooks/pre-commit .githooks/pre-push
	@echo "→ installing Go tools"
	go install mvdan.cc/gofumpt@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "✓ setup complete — run 'make build' to verify"

## up: start core services (ClickHouse, Postgres, Airflow)
up:
	$(COMPOSE) up -d --wait --remove-orphans

## up-obs: start core + observability (Prometheus, Grafana)
up-obs:
	$(COMPOSE_ALL) up -d --wait --remove-orphans

## up-test: start isolated test stack (ephemeral ports 8124/9001/5433)
up-test:
	docker compose -f $(COMPOSE_TEST) up -d --wait

## down: stop core + dev services, keep volumes
down:
	$(COMPOSE) down

## down-obs: stop all services including observability
down-obs:
	$(COMPOSE_ALL) down

## down-test: stop test stack and destroy ephemeral volumes
down-test:
	docker compose -f $(COMPOSE_TEST) down -v

## logs: tail all service logs (core + dev)
logs:
	$(COMPOSE) logs -f

## logs-obs: tail observability logs
logs-obs:
	docker compose -f $(COMPOSE_OBS) logs -f

## build: compile all Go binaries locally
build:
	mkdir -p $(BINARY_DIR)
	go build -o $(INGESTER) ./cmd/pw-ingester
	go build -o $(API) ./cmd/pw-api

## lint: run all linters (golangci-lint, ruff, sqlfluff, biome)
lint:
	golangci-lint run ./...
	@echo "✓ golangci-lint clean"

## test: run unit tests with race detector
test:
	go test -race -count=1 ./...

## test-integration: run integration tests (requires up-test)
test-integration:
	go test -race -count=1 -tags=integration ./...

## fmt: format Go source with gofumpt
fmt:
	gofumpt -l -w .

## smoke: wait for healthchecks then verify ClickHouse + Airflow
smoke:
	@bash scripts/smoke.sh

## migrate: run ClickHouse schema migrations
migrate: build
	$(INGESTER) migrate

## backfill: backfill blocks (usage: make backfill FROM=<block|date> TO=<block|date|head> [CHUNK=2000] [CONCURRENCY=4])
CHUNK       ?= 2000
CONCURRENCY ?= 4
backfill: build
	$(INGESTER) backfill --from=$(FROM) --to=$(TO) --chunk=$(CHUNK) --concurrency=$(CONCURRENCY)

## follow: start the live block follower (local, not via Docker)
follow: build
	$(INGESTER) follow

## dbt-build: run dbt incremental models
dbt-build:
	cd dbt && dbt build --select state:modified+

## dbt-docs: generate and serve dbt docs
dbt-docs:
	cd dbt && dbt docs generate && dbt docs serve

## web-dev: start the frontend Vite dev server
web-dev:
	cd apps/web && npm run dev

## docs-serve: serve MkDocs documentation
docs-serve:
	mkdocs serve

## docs-build: build MkDocs site to site/
docs-build:
	mkdocs build --strict

## k6: run k6 load test against the API (requires k6 + running API)
k6:
	k6 run tests/k6/api_load.js

## release: tag v1.0.0 and push
release:
	git tag -a v1.0.0 -m "chore: v1.0.0"
	git push origin v1.0.0

## help: list all targets with descriptions
help:
	@grep -E '^## ' Makefile | sed 's/^## //' | column -t -s ':'
