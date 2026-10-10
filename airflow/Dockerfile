# syntax=docker/dockerfile:1
# Multi-stage: stage 1 builds pw-ingester as a static binary;
# stage 2 is the official Airflow image with dbt + Cosmos + the binary on PATH.

# ── Stage 1: Go builder ────────────────────────────────────────────────────────
FROM golang:1.26.1-bookworm AS go-builder

WORKDIR /src

# Download dependencies first so this layer is cached unless go.mod changes.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy source (only what pw-ingester needs).
COPY cmd/pw-ingester/ ./cmd/pw-ingester/
COPY internal/           ./internal/

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags "-s -w" \
    -o /out/pw-ingester ./cmd/pw-ingester

# ── Stage 2: Airflow + dbt + Cosmos ────────────────────────────────────────────
FROM apache/airflow:2.10.4-python3.12

USER root

# Install pw-ingester so DAGs can shell out to it.
COPY --from=go-builder /out/pw-ingester /usr/local/bin/pw-ingester
RUN chmod +x /usr/local/bin/pw-ingester

# dbt project lives in /opt/airflow/dbt (mounted at runtime in dev).
RUN mkdir -p /opt/airflow/dbt && chown airflow: /opt/airflow/dbt

USER airflow

COPY airflow/requirements.txt /tmp/pw-requirements.txt
RUN pip install --no-cache-dir -r /tmp/pw-requirements.txt
