# ── build stage ────────────────────────────────────────────────────────────────
FROM golang:1.26.1-alpine AS builder

WORKDIR /build

ENV CGO_ENABLED=0

RUN apk add --no-cache git

# Download dependencies first for layer caching
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

# Accept explicit build args (from `make docker/build`).
# Fall back to computing from git when running via `docker compose up`.
ARG COMMIT
ARG BUILD_TIME

RUN _COMMIT=${COMMIT:-$(git rev-parse HEAD 2>/dev/null || echo "unknown")} && \
    _BUILD_TIME=${BUILD_TIME:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")} && \
    go build \
        -ldflags "-s -w \
                  -X scheduler/utils/version.Version=${_COMMIT} \
                  -X scheduler/utils/version.BuildTime=${_BUILD_TIME}" \
        -o /usr/local/bin/scheduler \
        ./cmd/scheduler

# ── runtime stage ──────────────────────────────────────────────────────────────
FROM alpine:3.21.0

RUN apk add --no-cache tzdata curl

WORKDIR /app
COPY --from=builder /usr/local/bin/scheduler ./scheduler

EXPOSE 4202

ENTRYPOINT ["/app/scheduler"]
