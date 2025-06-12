# Kogaro Makefile

# Variables
BINARY_NAME=kogaro
BUILD_DIR=bin
DOCKER_IMAGE=topiaruss/kogaro
VERSION?=$(shell git describe --tags --always --dirty)
LDFLAGS=-ldflags "-X main.Version=$(VERSION)"

# Kubernetes variables (for main Kogaro application)
KOGARO_CLUSTER=kogaro-test
KOGARO_CONTEXT=kind-$(KOGARO_CLUSTER)
NAMESPACE=kogaro-system

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOVET=$(GOCMD) vet

# Version management
VERSION_NUMBER := $(shell echo $(VERSION) | sed 's/^v//')

.PHONY: all build clean test deps fmt vet docker run help release check-version check-clean

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
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

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
	golangci-lint run internal/...
	golangci-lint run *.go 

# Build Docker image
docker:
	docker build -t $(DOCKER_IMAGE):$(VERSION_NUMBER) .
	docker tag $(DOCKER_IMAGE):$(VERSION_NUMBER) $(DOCKER_IMAGE):latest

# Build Docker image for multiple platforms
docker-buildx:
	docker buildx build --platform linux/amd64,linux/arm64 -t $(DOCKER_IMAGE):$(VERSION_NUMBER) .

# Build and push Docker image
docker-push:
	docker buildx build --platform linux/amd64,linux/arm64 -t $(DOCKER_IMAGE):$(VERSION_NUMBER) --push .

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
release: check-version check-clean
	@echo "Starting release process for version $(VERSION)"
	@echo "1. Updating Helm chart version..."
	@sed -i '' 's/^version: .*/version: $(VERSION_NUMBER)/' charts/kogaro/Chart.yaml
	@sed -i '' 's/^appVersion: .*/appVersion: "$(VERSION_NUMBER)"/' charts/kogaro/Chart.yaml
	@sed -i '' 's/^  tag: .*/  tag: "$(VERSION_NUMBER)"/' charts/kogaro/values.yaml
	@echo "2. Creating git tag..."
	@git add charts/kogaro/Chart.yaml charts/kogaro/values.yaml
	@git commit -m "chore: bump version to $(VERSION_NUMBER)"
	@git tag -a $(VERSION) -m "Release $(VERSION)"
	@echo "3. Pushing changes..."
	@git push origin main
	@git push origin $(VERSION)
	@echo "Release $(VERSION) initiated. GitHub Actions will handle the rest!"

# Prerequisites
.PHONY: check-version
check-version:
	@if [ "$(VERSION)" = "v0.0.0" ]; then \
		echo "Error: No version specified. Use VERSION=vX.Y.Z"; \
		exit 1; \
	fi

.PHONY: check-clean
check-clean:
	@if [ -n "$(shell git status --porcelain)" ]; then \
		echo "Error: Working directory is not clean"; \
		git status; \
		exit 1; \
	fi

# Kubernetes targets for Kogaro application
kind-setup:
	@echo "Setting up kind cluster for Kogaro..."
	@if ! kind get clusters | grep -q "$(KOGARO_CLUSTER)"; then \
		echo "Creating kind cluster: $(KOGARO_CLUSTER)"; \
		kind create cluster --name $(KOGARO_CLUSTER); \
	fi
	@echo "Switching to Kogaro context: $(KOGARO_CONTEXT)"
	kubectl config use-context $(KOGARO_CONTEXT)

kind-deploy: docker kind-setup
	@echo "Deploying Kogaro to kind cluster..."
	@echo "Loading Kogaro image into kind..."
	kind load docker-image $(DOCKER_IMAGE):$(VERSION_NUMBER) --name $(KOGARO_CLUSTER)
	@echo "Installing Kogaro via Helm..."
	helm upgrade --install kogaro ./charts/kogaro \
		--namespace $(NAMESPACE) --create-namespace \
		--set image.repository=$(DOCKER_IMAGE) \
		--set image.tag=$(VERSION_NUMBER) \
		--set image.pullPolicy=Never \
		--set metrics.serviceMonitor.enabled=false
	@echo "Kogaro deployed! Check status:"
	@echo "kubectl get pods -n $(NAMESPACE)"

k8s-logs:
	@echo "Switching to Kogaro context: $(KOGARO_CONTEXT)"
	kubectl config use-context $(KOGARO_CONTEXT)
	kubectl logs -n $(NAMESPACE) -l app.kubernetes.io/name=kogaro -f

k8s-status:
	@echo "Switching to Kogaro context: $(KOGARO_CONTEXT)"
	kubectl config use-context $(KOGARO_CONTEXT)
	kubectl get pods -n $(NAMESPACE)

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
	@echo ""
	@echo "Kubernetes targets:"
	@echo "  kind-setup    - Create and setup kogaro-test kind cluster"
	@echo "  kind-deploy   - Build, load, and deploy Kogaro to kind cluster"
	@echo "  k8s-logs      - Follow Kogaro logs (switches to kogaro-test context)"
	@echo "  k8s-status    - Check Kogaro pod status (switches to kogaro-test context)"
	@echo ""
	@echo "  help          - Show this help"
