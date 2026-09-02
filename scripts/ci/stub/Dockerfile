# Multi-stage build: compile the CI stub server, then run on a minimal image.
# Used by docker-compose.ci.yml as the unified external-service mock
# (magiclink + xendit) for integration testing.

FROM golang:1.26-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
# No external deps needed — stdlib only. But copy module files for
# future-proofing if deps are added later.
RUN go mod download 2>/dev/null || true

COPY scripts/ci/stub/ ./scripts/ci/stub/
RUN CGO_ENABLED=0 go build -o /ci-stub ./scripts/ci/stub/

# ── Runtime ──────────────────────────────────────────────────────────────
FROM alpine:3.20

RUN apk add --no-cache ca-certificates
COPY --from=builder /ci-stub /ci-stub

EXPOSE 8081
ENTRYPOINT ["/ci-stub"]
