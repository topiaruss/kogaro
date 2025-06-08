#!/bin/bash

set -e

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

# Show current context
CURRENT_CONTEXT=$(kubectl config current-context 2>/dev/null || echo "none")
echo "📋 Current kubectl context: $CURRENT_CONTEXT"
echo

# Confirm production deployment
read -p "🔥 Deploy to PRODUCTION cluster? This will be live at kogaro.com (y/N): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "❌ Deployment cancelled"
    exit 1
fi

# Build and push the Docker image
echo "🔨 Building Docker image..."
docker build -t kogaro-website:latest . --quiet

echo "📤 Tagging and pushing to registry..."
docker tag kogaro-website:latest registry.ogaro.com/kogaro-website:latest
docker push registry.ogaro.com/kogaro-website:latest

# Create namespace and registry secret
echo "📦 Creating namespace and registry secret..."
./scripts/create-registry-secret.sh

# Deploy using Helm
echo "⚙️  Deploying with Helm..."
helm upgrade --install kogaro-website . \
    --namespace kogaro-website \
    --wait \
    --timeout=300s

# Show status
echo "✅ Deployment complete!"
echo
echo "📊 Status:"
kubectl get pods,svc,ingress -n kogaro-website
echo

echo "🌐 Website should be available at:"
echo "   https://kogaro.com"
echo "   https://www.kogaro.com"
echo
echo "📝 To view logs:"
echo "   kubectl logs -f deployment/kogaro-website -n kogaro-website"
echo
echo "🔍 To check certificate status:"
echo "   kubectl get certificate -n kogaro-website"
echo "   kubectl describe certificate kogaro-website-tls -n kogaro-website"
echo
echo "🧹 To cleanup:"
echo "   helm uninstall kogaro-website -n kogaro-website"
echo "   kubectl delete namespace kogaro-website"