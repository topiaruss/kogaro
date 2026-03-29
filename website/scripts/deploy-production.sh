#!/bin/bash

set -euo pipefail

PRODUCTION_CONTEXT="${PRODUCTION_CONTEXT:-23af57eb-2479-4a7a-b9fc-d788a2a475e4/default}"
WEBSITE_NAMESPACE="${WEBSITE_NAMESPACE:-kogaro-website}"
IMAGE_REPO="${IMAGE_REPO:-registry.ogaro.com/kogaro-website}"
GIT_SHA="$(git rev-parse --short HEAD 2>/dev/null || echo "nogit")"
BUILD_TS="$(date -u '+%Y%m%d%H%M%S')"
IMAGE_TAG="${IMAGE_TAG:-${GIT_SHA}-${BUILD_TS}}"

echo "🚀 Deploying Kogaro website to production cluster..."

# Check if we're in the right directory
if [[ ! -f "Chart.yaml" ]]; then
    echo "❌ Please run this script from the website/ directory"
    exit 1
fi

# Check if helm is available
if ! command -v helm &> /dev/null; then
    echo "❌ helm is not installed or not in PATH"
    exit 1
fi

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    echo "❌ kubectl is not installed or not in PATH"
    exit 1
fi

# Show current + target context
CURRENT_CONTEXT="$(kubectl config current-context 2>/dev/null || echo "none")"
echo "📋 Current kubectl context: $CURRENT_CONTEXT"
echo "🎯 Target production context: $PRODUCTION_CONTEXT"
echo "🏷️  Deploy image tag: $IMAGE_TAG"
echo

# Confirm production deployment
read -p "🔥 Deploy to PRODUCTION cluster? This will be live at kogaro.com (y/N): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "❌ Deployment cancelled"
    exit 1
fi

# Switch to the production context
echo "🔀 Switching kubectl context..."
kubectl config use-context "$PRODUCTION_CONTEXT"
echo "📋 Active context: $(kubectl config current-context)"

# Build and push multi-arch image with immutable tag and latest
echo "🔨 Building and pushing multi-arch Docker image..."
docker buildx build \
    --platform linux/amd64,linux/arm64 \
    --no-cache \
    -t "${IMAGE_REPO}:${IMAGE_TAG}" \
    -t "${IMAGE_REPO}:latest" \
    . \
    --push

# Verify image platforms before rollout
echo "🧪 Verifying image platforms..."
./scripts/check-image-arch.sh "${IMAGE_REPO}:${IMAGE_TAG}" linux/amd64 linux/arm64

# Create namespace and registry secret
echo "📦 Creating namespace and registry secret..."
./scripts/create-registry-secret.sh

# Deploy using Helm
echo "⚙️  Deploying with Helm..."
helm upgrade --install kogaro-website . \
    --namespace "$WEBSITE_NAMESPACE" \
    --set image.repository="$IMAGE_REPO" \
    --set image.tag="$IMAGE_TAG" \
    --set image.pullPolicy=Always \
    --set-string ingress.annotations.nginx\\.ingress\\.kubernetes\\.io/ssl-redirect=true \
    --set-string ingress.annotations.nginx\\.ingress\\.kubernetes\\.io/force-ssl-redirect=true \
    --wait \
    --timeout=300s

echo "⏳ Waiting for rollout..."
kubectl rollout status deployment/kogaro-website -n "$WEBSITE_NAMESPACE" --timeout=300s

# Show status
echo "✅ Deployment complete!"
echo
echo "📊 Status:"
kubectl get pods,svc,ingress -n "$WEBSITE_NAMESPACE"
echo

echo "🌐 Website should be available at:"
echo "   https://kogaro.com"
echo "   https://www.kogaro.com"
echo
echo "📝 To view logs:"
echo "   kubectl logs -f deployment/kogaro-website -n $WEBSITE_NAMESPACE"
echo
echo "🔍 To check certificate status:"
echo "   kubectl get certificate -n $WEBSITE_NAMESPACE"
echo "   kubectl describe certificate kogaro-website-tls -n $WEBSITE_NAMESPACE"
echo
echo "🧹 To cleanup:"
echo "   helm uninstall kogaro-website -n $WEBSITE_NAMESPACE"
echo "   kubectl delete namespace $WEBSITE_NAMESPACE"