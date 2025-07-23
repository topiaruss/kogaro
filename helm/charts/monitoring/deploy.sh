#!/bin/bash

# Deploy monitoring stack with environment-based configuration
# Usage: ./deploy.sh [namespace]

set -e

NAMESPACE=${1:-"monitoring"}
CHART_NAME="monitoring"
RELEASE_NAME="monitoring"

echo "🚀 Deploying Kogaro Monitoring Stack to namespace: $NAMESPACE"

# Check if .env file exists
if [ ! -f ".env" ]; then
    echo "❌ Error: .env file not found!"
    echo "Please copy env.example to .env and configure your values:"
    echo "  cp env.example .env"
    echo "  # Edit .env with your actual passwords and domain"
    exit 1
fi

# Generate values.yaml from environment variables
echo "📝 Generating Helm values from environment variables..."
./generate-values.sh > values.yaml

# Create namespace if it doesn't exist
echo "📦 Creating namespace if it doesn't exist..."
kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -

# Create basic auth secrets for Prometheus and Alertmanager
echo "🔐 Creating basic auth secrets..."
source .env

# Create Prometheus basic auth secret
kubectl create secret generic prometheus-basic-auth \
    --from-literal=auth="$PROMETHEUS_BASIC_AUTH_USER:$(openssl passwd -apr1 $PROMETHEUS_BASIC_AUTH_PASSWORD)" \
    -n "$NAMESPACE" \
    --dry-run=client -o yaml | kubectl apply -f -

# Create Alertmanager basic auth secret
kubectl create secret generic alertmanager-basic-auth \
    --from-literal=auth="$ALERTMANAGER_BASIC_AUTH_USER:$(openssl passwd -apr1 $ALERTMANAGER_BASIC_AUTH_PASSWORD)" \
    -n "$NAMESPACE" \
    --dry-run=client -o yaml | kubectl apply -f -

# Deploy the monitoring stack
echo "🎯 Deploying monitoring stack with Helm..."
helm upgrade --install "$RELEASE_NAME" . \
    --namespace "$NAMESPACE" \
    --values values.yaml \
    --wait \
    --timeout 10m

# Clean up generated values file
rm -f values.yaml

echo "✅ Monitoring stack deployed successfully!"
echo ""
echo "📊 Access URLs:"
echo "  Grafana: https://grafana.$DOMAIN"
echo "  Prometheus: https://prometheus.$DOMAIN"
echo "  Alertmanager: https://alertmanager.$DOMAIN"
echo ""
echo "🔑 Credentials:"
echo "  Grafana: $GRAFANA_ADMIN_USER / [password from .env]"
echo "  Prometheus: $PROMETHEUS_BASIC_AUTH_USER / [password from .env]"
echo "  Alertmanager: $ALERTMANAGER_BASIC_AUTH_USER / [password from .env]"
echo ""
echo "📋 To check deployment status:"
echo "  kubectl get pods -n $NAMESPACE"
echo "  helm list -n $NAMESPACE" 