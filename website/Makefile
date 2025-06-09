# Website Makefile

# Variables
WEBSITE_IMAGE=registry.ogaro.com/kogaro-website
VERSION?=$(shell git describe --tags --always --dirty)
WEBSITE_DIR=/Users/russ/dev/int/kogaro/website

.PHONY: website-build website-push website-deploy website-all help

# Default target
all: website-all

# Build multi-arch website image
website-build:
	@echo "Building multi-arch website image..."
	docker buildx build --platform linux/amd64,linux/arm64 -t $(WEBSITE_IMAGE):$(VERSION) -t $(WEBSITE_IMAGE):latest $(WEBSITE_DIR)

# Push website image
website-push:
	@echo "Building and pushing multi-arch website image..."
	docker buildx build --platform linux/amd64,linux/arm64 -t $(WEBSITE_IMAGE):$(VERSION) -t $(WEBSITE_IMAGE):latest $(WEBSITE_DIR) --push

# Package and push Helm chart
website-helm:
	@echo "Packaging and updating Helm chart..."
	helm package . --destination /tmp
	@echo "Chart packaged to /tmp"

# Deploy website via Helm
website-deploy:
	@echo "Deploying website..."
	helm upgrade kogaro-website . --namespace kogaro-website --create-namespace

# Full build, push, and deploy pipeline
website-all: website-push website-deploy
	@echo "Website build and deployment complete!"

# Help
help:
	@echo "Available targets:"
	@echo "  website-build   - Build multi-arch website Docker image"
	@echo "  website-push    - Build and push multi-arch website image"
	@echo "  website-helm    - Package Helm chart"
	@echo "  website-deploy  - Deploy website via Helm"
	@echo "  website-all     - Full pipeline: build, push, deploy"
	@echo "  help           - Show this help"