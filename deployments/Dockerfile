# ── build stage ────────────────────────────────────────────────────────────────
FROM golang:1.25.0-alpine AS builder

WORKDIR /build

ENV CGO_ENABLED=0

# Download dependencies first for layer caching
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown

RUN go build \
    -ldflags "-s -w \
              -X main.version=${VERSION} \
              -X main.commit=${COMMIT} \
              -X main.buildTime=${BUILD_TIME}" \
    -o /usr/local/bin/scheduler \
    ./cmd/scheduler

# ── runtime stage ──────────────────────────────────────────────────────────────
FROM alpine:3.21.0

RUN apk add --no-cache tzdata curl

WORKDIR /app
COPY --from=builder /usr/local/bin/scheduler ./scheduler

EXPOSE 4202

ENTRYPOINT ["/app/scheduler"]
