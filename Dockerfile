# syntax=docker/dockerfile:1
# Shared Go binary builder for pw-api and pw-follower.
# Build with: docker build --build-arg SERVICE=pw-api .
# Used by docker-compose services: api (task 14), follower (task 6).

ARG GO_VERSION=1.26.1
ARG SERVICE=pw-api

# ── Stage 1: Build ─────────────────────────────────────────────────────────────
FROM golang:${GO_VERSION}-bookworm AS builder
ARG SERVICE

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux \
    go build -trimpath -ldflags "-s -w" \
    -o /out/service ./cmd/${SERVICE}

# ── Stage 2: Minimal runtime ────────────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:debug-nonroot

COPY --from=builder /out/service /service

EXPOSE 8080

ENTRYPOINT ["/service"]
