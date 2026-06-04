# ============================================================================
# VARIABLES
# ============================================================================
BINARY          := .bin/scheduler
SOURCE          := ./cmd/scheduler

PROD_CONFIG     := config/prod.yaml

# ============================================================================
# PHONY TARGETS
# ============================================================================
.PHONY: build run run-prod fmt vet lint deps tidy clean info help

# ============================================================================
# BUILD
# ============================================================================
build:
	@echo "Building $(BINARY)..."
	go build -o $(BINARY) $(SOURCE)

# ============================================================================
# RUN
# ============================================================================
run: build
	./$(BINARY)

run-prod: build
	./$(BINARY) -c $(PROD_CONFIG)

# ============================================================================
# CODE QUALITY
# ============================================================================
fmt:
	go fmt ./...

vet:
	@echo "Running go vet..."
	go vet ./...

lint: vet
	@echo "Checking code formatting..."
	@gofmt -l . | grep -q . && (echo "Unformatted files:" && gofmt -l . && exit 1) || echo "Code is clean"

# ============================================================================
# DEPENDENCIES
# ============================================================================
deps:
	go mod download

tidy:
	go mod tidy

# ============================================================================
# CLEANUP
# ============================================================================
clean:
	rm -rf .bin
	@echo "Cleanup complete"

# ============================================================================
# INFO / HELP
# ============================================================================
info:
	@echo "Binary:   $(BINARY)"
	@echo "Prod cfg: $(PROD_CONFIG)"

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Build & Run:"
	@echo "  build            Build the binary"
	@echo "  run              Run locally with embedded dev config"
	@echo "  run-prod         Run locally with prod config"
	@echo ""
	@echo "Code Quality:"
	@echo "  fmt              Format Go code"
	@echo "  vet              Run go vet"
	@echo "  lint             Run vet + check formatting"
	@echo "  deps             Download dependencies"
	@echo "  tidy             Tidy go.mod"
	@echo ""
	@echo "Cleanup:"
	@echo "  clean            Remove build artifacts"
	@echo "  info             Show current configuration"