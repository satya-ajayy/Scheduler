## ── variables ──────────────────────────────────────────────────────────────────

BINARY    := scheduler
CMD       := ./cmd/scheduler
BUILD_DIR := .bin
IMAGE     := scheduler

VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT     := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -ldflags "-s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.buildTime=$(BUILD_TIME)"

## ── phony targets ──────────────────────────────────────────────────────────────

.PHONY: all build run clean \
        fmt vet tidy lint \
        test test/cover \
        docker/build docker/run \
        compose/up compose/down compose/logs \
        help

## ── dev ────────────────────────────────────────────────────────────────────────

all: fmt vet build   ## Format, vet, then build

build:               ## Compile binary to bin/scheduler
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) $(CMD)
	@echo "built: $(BUILD_DIR)/$(BINARY)  ($(VERSION))"

run:                 ## Run the service directly (no build step)
	go run $(CMD)

run/config:          ## Run with a local config file  (CONFIG=path/to/config.yml)
	go run $(CMD) -c $(CONFIG)

clean:               ## Remove build artifacts and coverage reports
	rm -rf $(BUILD_DIR) bin coverage.out coverage.html

## ── code quality ───────────────────────────────────────────────────────────────

fmt:                 ## Format all Go source files
	go fmt ./...

vet:                 ## Run go vet across all packages
	go vet ./...

tidy:                ## Tidy and verify go.mod / go.sum
	go mod tidy
	go mod verify

lint:                ## Run golangci-lint (must be installed: https://golangci-lint.run)
	golangci-lint run ./...

## ── testing ────────────────────────────────────────────────────────────────────

test:                ## Run all tests with race detector
	go test -race ./...

test/cover:          ## Run tests with coverage; opens coverage.html
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "coverage report: coverage.html"

## ── docker ─────────────────────────────────────────────────────────────────────

docker/build:        ## Build the Docker image
	docker build \
		-f deployments/Dockerfile \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		-t $(IMAGE):$(VERSION) \
		-t $(IMAGE):latest \
		.

docker/run:          ## Run the Docker image (network=host)
	docker run --rm --network host $(IMAGE):latest

## ── compose ────────────────────────────────────────────────────────────────────

compose/up:          ## Start all services (MongoDB + scheduler) in the background
	docker compose -f deployments/docker-compose.yml up -d --build

compose/down:        ## Stop and remove compose services
	docker compose -f deployments/docker-compose.yml down

compose/logs:        ## Tail logs from all compose services
	docker compose -f deployments/docker-compose.yml logs -f

## ── help ───────────────────────────────────────────────────────────────────────

help:                ## Show this help message
	@echo "Usage: make <target>\n"
	@grep -E '^[a-zA-Z/_-]+:.*##' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*##"}; {printf "  %-22s %s\n", $$1, $$2}'
