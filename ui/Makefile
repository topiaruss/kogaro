# Kogaro UI Makefile

# Variables
BINARY_NAME=kogaro-ui
BUILD_DIR=build/bin
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(shell date -u '+%Y-%m-%d %H:%M:%S UTC')
LDFLAGS=-ldflags "-X 'main.buildCommit=$(COMMIT)' -X 'main.buildTime=$(BUILD_TIME)'"
CGO_FLAGS=CGO_ENABLED=1 CGO_LDFLAGS="-framework UniformTypeIdentifiers"
BUILD_TAGS=-tags desktop,production

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOVET=$(GOCMD) vet

.PHONY: all build clean test test-v frontend run dev deps fmt vet help

# Default target
all: clean frontend build

# Build frontend (Svelte)
frontend:
	@echo "Building frontend..."
	cd frontend && npm install --silent && npm run build

# Build Go binary
build: frontend
	@mkdir -p $(BUILD_DIR)
	$(CGO_FLAGS) $(GOBUILD) $(BUILD_TAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME) ($(COMMIT) @ $(BUILD_TIME))"

# Build Go binary only (skip frontend)
build-go:
	@mkdir -p $(BUILD_DIR)
	$(CGO_FLAGS) $(GOBUILD) $(BUILD_TAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME) ($(COMMIT) @ $(BUILD_TIME))"

# Build and launch
run: build
	@pkill -f '$(BUILD_DIR)/$(BINARY_NAME)' 2>/dev/null || true
	@sleep 0.5
	./$(BUILD_DIR)/$(BINARY_NAME)

# Build and launch (alias)
dev: run

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)
	rm -rf frontend/dist

# Run all tests
test:
	$(GOTEST) -count=1 ./pkg/...

# Run tests verbose
test-v:
	$(GOTEST) -v -count=1 ./pkg/...

# Run tests with coverage
test-coverage:
	$(GOTEST) -v -coverprofile=coverage.out ./pkg/...
	$(GOCMD) tool cover -func=coverage.out

# Download dependencies
deps:
	$(GOCMD) mod tidy
	cd frontend && npm install

# Format code
fmt:
	gofmt -s -w .

# Vet code
vet:
	$(GOVET) ./...

# Kill any running instance
kill:
	@pkill -f '$(BUILD_DIR)/$(BINARY_NAME)' 2>/dev/null && echo "Stopped $(BINARY_NAME)" || echo "Not running"

# Help
help:
	@echo "Available targets:"
	@echo "  all           - Clean, build frontend, build binary (default)"
	@echo "  build         - Build frontend + Go binary"
	@echo "  build-go      - Build Go binary only (skip frontend)"
	@echo "  run / dev     - Build and launch the app"
	@echo "  clean         - Remove build artifacts"
	@echo "  test          - Run tests"
	@echo "  test-v        - Run tests (verbose)"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  deps          - Download Go and npm dependencies"
	@echo "  fmt           - Format Go code"
	@echo "  vet           - Vet Go code"
	@echo "  kill          - Stop running instance"
	@echo "  help          - Show this help"
