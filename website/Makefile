# Website Makefile

# Variables
WEBSITE_IMAGE=registry.ogaro.com/kogaro-website
VERSION?=$(shell git describe --tags --always --dirty)
WEBSITE_DIR=/Users/russ/dev/int/kogaro/website

.PHONY: website-build website-push website-deploy website-all website-kind help

# Default target
all: website-all

# Build multi-arch website image
website-build:
	@echo "Building multi-arch website image..."
	docker buildx build --platform linux/amd64,linux/arm64 --no-cache -t $(WEBSITE_IMAGE):$(VERSION) -t $(WEBSITE_IMAGE):latest $(WEBSITE_DIR)

# Push website image
website-push:
	@echo "Building and pushing multi-arch website image..."
	docker buildx build --platform linux/amd64,linux/arm64 --no-cache -t $(WEBSITE_IMAGE):$(VERSION) -t $(WEBSITE_IMAGE):latest $(WEBSITE_DIR) --push

# Package and push Helm chart
website-helm:
	@echo "Packaging and updating Helm chart..."
	helm package . --destination /tmp
	@echo "Chart packaged to /tmp"

# Deploy website via Helm
website-deploy:
	@echo "Deploying website..."
	helm upgrade kogaro-website . --namespace kogaro-website --create-namespace
	@echo "Restarting deployment to pull latest image..."
	kubectl rollout restart deployment/kogaro-website -n kogaro-website

# Full build, push, and deploy pipeline
website-all: website-push website-deploy
	@echo "Website build and deployment complete!"

# Local kind testing
website-kind:
	@echo "Building website image for kind..."
	docker build -t $(WEBSITE_IMAGE):$(VERSION) $(WEBSITE_DIR)
	@echo "Setting up website in kind cluster..."
	@if ! kind get clusters | grep -q "kogaro-website"; then \
		echo "Creating kind cluster for website testing..."; \
		kind create cluster --name kogaro-website; \
	fi
	@echo "Loading website image into kind..."
	kind load docker-image $(WEBSITE_IMAGE):$(VERSION) --name kogaro-website
	@echo "Deploying website to kind cluster..."
	helm upgrade --install kogaro-website . \
		--namespace kogaro-website --create-namespace \
		--set image.repository=$(WEBSITE_IMAGE) \
		--set image.tag=$(VERSION) \
		--set image.pullPolicy=Never
	@echo "Website deployed! Access via port-forward:"
	@echo "kubectl port-forward -n kogaro-website svc/kogaro-website 8080:80"
	@echo "Then visit: http://localhost:8080"

# Help
help:
	@echo "Available targets:"
	@echo "  website-build   - Build multi-arch website Docker image"
	@echo "  website-push    - Build and push multi-arch website image"
	@echo "  website-helm    - Package Helm chart"
	@echo "  website-deploy  - Deploy website via Helm"
	@echo "  website-all     - Full pipeline: build, push, deploy"
	@echo "  website-kind    - Build and deploy website in local kind cluster"
	@echo "  help           - Show this help"