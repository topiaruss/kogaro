# Website Makefile

# Variables
WEBSITE_IMAGE=registry.ogaro.com/kogaro-website
VERSION?=$(shell git describe --tags --always --dirty)
WEBSITE_DIR=/Users/russ/dev/int/kogaro/website

# Kubernetes variables (for website)
PRODUCTION_CONTEXT=23af57eb-2479-4a7a-b9fc-d788a2a475e4/default
WEBSITE_CLUSTER=kogaro-website
WEBSITE_CONTEXT=kind-$(WEBSITE_CLUSTER)
WEBSITE_NAMESPACE=kogaro-website

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

# Deploy website to remote production cluster
website-deploy:
	@echo "Deploying website to REMOTE production cluster..."
	kubectl config use-context $(PRODUCTION_CONTEXT)
	@echo "Current context: $$(kubectl config current-context)"
	@read -p "Are you sure you want to deploy to PRODUCTION? (y/N): " confirm && [ "$$confirm" = "y" ]
	helm upgrade kogaro-website . --namespace $(WEBSITE_NAMESPACE) --create-namespace
	@echo "Restarting deployment to pull latest image..."
	kubectl rollout restart deployment/kogaro-website -n $(WEBSITE_NAMESPACE)

# Full build, push, and deploy pipeline
website-all: website-push website-deploy
	@echo "Website build and deployment complete!"

# Local kind testing
website-kind:
	@echo "Building website image for kind..."
	docker build -t $(WEBSITE_IMAGE):$(VERSION) $(WEBSITE_DIR)
	@echo "Setting up website in kind cluster..."
	@if ! kind get clusters | grep -q "$(WEBSITE_CLUSTER)"; then \
		echo "Creating kind cluster for website testing..."; \
		kind create cluster --name $(WEBSITE_CLUSTER); \
		echo "Waiting for cluster to be ready..."; \
		sleep 5; \
	fi
	@echo "Switching to website context: $(WEBSITE_CONTEXT)"
	kubectl config use-context $(WEBSITE_CONTEXT)
	@echo "Loading website image into kind..."
	kind load docker-image $(WEBSITE_IMAGE):$(VERSION) --name $(WEBSITE_CLUSTER)
	@echo "Deploying website to kind cluster..."
	helm upgrade --install kogaro-website . \
		--namespace $(WEBSITE_NAMESPACE) --create-namespace \
		--set image.repository=$(WEBSITE_IMAGE) \
		--set image.tag=$(VERSION) \
		--set image.pullPolicy=Never
	@echo "Website deployed! Access via port-forward:"
	@echo "kubectl port-forward -n $(WEBSITE_NAMESPACE) svc/kogaro-website 8080:80"
	@echo "Then visit: http://localhost:8080"

# Help
help:
	@echo "Available targets:"
	@echo ""
	@echo "Remote Production Deployment:"
	@echo "  website-build   - Build multi-arch website Docker image"
	@echo "  website-push    - Build and push multi-arch website image"
	@echo "  website-helm    - Package Helm chart"
	@echo "  website-deploy  - Deploy website to REMOTE production cluster"
	@echo "  website-all     - Full pipeline: build, push, deploy to PRODUCTION"
	@echo ""
	@echo "Local Development:"
	@echo "  website-kind    - Build and deploy website in LOCAL kind cluster"
	@echo ""
	@echo "  help           - Show this help"