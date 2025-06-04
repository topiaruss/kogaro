# Kogaro Makefile

# Variables
BINARY_NAME=kogaro
BUILD_DIR=bin
DOCKER_IMAGE=kogaro
VERSION?=$(shell git describe --tags --always --dirty)
LDFLAGS=-ldflags "-X main.Version=$(VERSION)"

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOVET=$(GOCMD) vet

.PHONY: all build clean test deps fmt vet docker run help

# Default target
all: clean fmt fmt-imports vet lint test build

# Build the binary
build:
	mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) main.go

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)

# Run tests
test:
	$(GOTEST) -v ./...

# Run tests with coverage
test-coverage:
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Run tests with race detection
test-race:
	$(GOTEST) -v -race ./...

# Run short tests only
test-short:
	$(GOTEST) -v -short ./...

# Run tests and show coverage percentage
test-coverage-report:
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -func=coverage.out

# Download dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Format code
fmt:
	$(GOFMT) -s -w .

# Format imports (requires goimports)
fmt-imports:
	goimports -w .

# Vet code
vet:
	$(GOVET) ./...

# Lint code (requires golangci-lint)
lint:
	golangci-lint run

# Build Docker image
docker:
	docker build -t $(DOCKER_IMAGE):$(VERSION) .
	docker tag $(DOCKER_IMAGE):$(VERSION) $(DOCKER_IMAGE):latest

# Run locally
run:
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) main.go
	./$(BUILD_DIR)/$(BINARY_NAME)

# Run with specific flags
run-dev:
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) main.go
	./$(BUILD_DIR)/$(BINARY_NAME) --scan-interval=30s

# Install binary to GOPATH/bin
install:
	$(GOCMD) install $(LDFLAGS) .

# Check for security issues (requires gosec)
security:
	gosec ./...

# Generate code (if needed)
generate:
	$(GOCMD) generate ./...

# Release build (optimized)
release:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -a -installsuffix cgo -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 main.go
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -a -installsuffix cgo -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 main.go
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -a -installsuffix cgo -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe main.go

# Help
help:
	@echo "Available targets:"
	@echo "  build         - Build the binary"
	@echo "  clean         - Clean build artifacts"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  test-race     - Run tests with race detection"
	@echo "  test-short    - Run short tests only"
	@echo "  test-coverage-report - Run tests and show coverage percentage"
	@echo "  deps          - Download and tidy dependencies"
	@echo "  fmt           - Format code"
	@echo "  fmt-imports   - Format imports (requires goimports)"
	@echo "  vet           - Vet code"
	@echo "  lint          - Lint code (requires golangci-lint)"
	@echo "  docker        - Build Docker image"
	@echo "  run           - Build and run locally"
	@echo "  run-dev       - Build and run with dev flags"
	@echo "  install       - Install binary to GOPATH/bin"
	@echo "  security      - Run security checks (requires gosec)"
	@echo "  generate      - Generate code"
	@echo "  release       - Build release binaries for multiple platforms"
	@echo "  help          - Show this help"