#!/bin/bash

set -e

echo "🚀 Deploying Kogaro website to Docker Desktop Kubernetes..."

# Check if we're in the right directory
if [[ ! -f "Dockerfile" ]]; then
    echo "❌ Please run this script from the website/ directory"
    exit 1
fi

# Check if kubectl is available and pointing to docker-desktop
CURRENT_CONTEXT=$(kubectl config current-context 2>/dev/null || echo "none")
if [[ "$CURRENT_CONTEXT" != "docker-desktop" ]]; then
    echo "⚠️  Current kubectl context is '$CURRENT_CONTEXT'"
    echo "   Consider switching to docker-desktop: kubectl config use-context docker-desktop"
    read -p "   Continue anyway? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# Build and push the Docker image
echo "🔨 Building Docker image..."
docker build -t kogaro-website:latest . --quiet

echo "📤 Tagging and pushing to registry..."
docker tag kogaro-website:latest registry.ogaro.com/kogaro-website:latest
docker push registry.ogaro.com/kogaro-website:latest

# Create namespace first
echo "📦 Creating namespace..."
kubectl apply -f k8s/namespace.yaml

# Apply remaining Kubernetes manifests
echo "⚙️  Applying Kubernetes manifests..."
kubectl apply -f k8s/

# Wait for deployment to be ready
echo "⏳ Waiting for deployment to be ready..."
kubectl wait --for=condition=available --timeout=60s deployment/kogaro-website -n kogaro-website

# Show status
echo "✅ Deployment complete!"
echo
echo "📊 Status:"
kubectl get pods,svc -n kogaro-website

echo
echo "🌐 To access the website:"
echo "   kubectl port-forward -n kogaro-website svc/kogaro-website 8080:80"
echo "   Then open: http://localhost:8080"
echo
echo "📝 To view logs:"
echo "   kubectl logs -f deployment/kogaro-website -n kogaro-website"
echo
echo "🧹 To cleanup:"
echo "   kubectl delete -f k8s/"