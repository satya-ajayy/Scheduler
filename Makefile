## ── variables ──────────────────────────────────────────────────────────────────

IMAGE     := scheduler
CMD       := ./cmd/scheduler
BUILD_DIR := .bin
BIN       := $(BUILD_DIR)/$(IMAGE)
PORT      := 4202
CONFIG    ?= config.yml

COMMIT     := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -ldflags "-s -w \
	-X scheduler/internal/version.Version=$(COMMIT) \
	-X scheduler/internal/version.BuildTime=$(BUILD_TIME)"

## ── phony targets ──────────────────────────────────────────────────────────────

.PHONY: all build run run/config clean \
        fmt vet tidy lint \
        test test/cover \
        docker/build docker/run docker/all \
        compose/up compose/down compose/logs \
        help

## ── dev ────────────────────────────────────────────────────────────────────────

all: fmt vet build   ## Format, vet, then build

build:               ## Compile binary → .bin/scheduler
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BIN) $(CMD)
	@echo "built: $(BIN)  ($(COMMIT))"

run:                 ## Run the service with built-in defaults (no config file)
	go run $(LDFLAGS) $(CMD)

run/config:          ## Run with a config file  →  make run/config CONFIG=config.yml
	go run $(LDFLAGS) $(CMD) -c $(CONFIG)

clean:               ## Remove build artifacts and coverage reports
	rm -rf $(BUILD_DIR) coverage.out coverage.html

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

docker/build:        ## Build Docker image  →  $(IMAGE):$(COMMIT)
	docker build \
		-f deployments/Dockerfile \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		-t $(IMAGE):$(COMMIT) \
		-t $(IMAGE):latest \
		.

docker/run:          ## Run image; mounts CONFIG and port-forwards $(PORT) → localhost
	docker run --rm \
		-p $(PORT):$(PORT) \
		-v $(PWD)/$(CONFIG):/etc/scheduler/config.yml:ro \
		$(IMAGE):$(COMMIT) \
		-c /etc/scheduler/config.yml

docker/all:          ## Build image, run it, and forward port — all in one step
	$(MAKE) docker/build
	$(MAKE) docker/run

## ── help ───────────────────────────────────────────────────────────────────────

help:                ## Show this help message
	@echo "Usage: make <target>\n"
	@grep -E '^[a-zA-Z/_-]+:.*##' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*##"}; {printf "  %-24s %s\n", $$1, $$2}'
