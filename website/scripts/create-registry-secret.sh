#!/bin/bash

set -e

# Load environment variables from .env file if it exists
if [[ -f ".env" ]]; then
    source .env
    echo "✅ Loaded credentials from .env file"
else
    echo "❌ .env file not found. Please create one based on .env.example"
    exit 1
fi

# Validate required environment variables
if [[ -z "$REGISTRY_SERVER" || -z "$REGISTRY_USERNAME" || -z "$REGISTRY_PASSWORD" ]]; then
    echo "❌ Missing required environment variables:"
    echo "   REGISTRY_SERVER, REGISTRY_USERNAME, REGISTRY_PASSWORD"
    echo "   Please check your .env file"
    exit 1
fi

# Create namespace if it doesn't exist
kubectl create namespace kogaro-website --dry-run=client -o yaml | kubectl apply -f -

# Create or update the registry secret
echo "🔐 Creating registry secret..."
kubectl create secret docker-registry regcred \
    --docker-server="$REGISTRY_SERVER" \
    --docker-username="$REGISTRY_USERNAME" \
    --docker-password="$REGISTRY_PASSWORD" \
    --namespace kogaro-website \
    --dry-run=client -o yaml | kubectl apply -f -

echo "✅ Registry secret created/updated successfully"
echo "🔒 Credentials are now stored securely in Kubernetes secret 'regcred'"