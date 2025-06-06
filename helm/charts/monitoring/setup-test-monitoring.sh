#!/bin/bash

# Source environment variables
if [ -f .env ]; then
    source .env
else
    echo "Error: .env file not found"
    exit 1
fi

# Validate required environment variables
if [ -z "${MONITORING_USERNAME}" ] || [ -z "${MONITORING_PASSWORD}" ]; then
    echo "Error: Required environment variables are not set"
    echo "Please ensure MONITORING_USERNAME and MONITORING_PASSWORD are set in .env"
    exit 1
fi

# Create namespace
kubectl create namespace monitoring

# Create basic auth secrets
echo "Creating basic auth secrets..."
echo "${MONITORING_USERNAME}:$(openssl passwd -apr1 '${MONITORING_PASSWORD}')" > auth
kubectl create secret generic prometheus-basic-auth --from-file=auth -n monitoring
kubectl create secret generic alertmanager-basic-auth --from-file=auth -n monitoring
rm auth

# Add prometheus-community repo if not already added
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

# Install kube-prometheus-stack
echo "Installing monitoring stack..."
helm upgrade --install monitoring . \
  --namespace monitoring \
  -f values.yaml

# Wait for pods to be ready
echo "Waiting for pods to be ready..."
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=prometheus -n monitoring --timeout=300s
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=alertmanager -n monitoring --timeout=300s
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=grafana -n monitoring --timeout=300s

# Show status
echo "Monitoring stack installed. Check status with:"
echo "kubectl get pods -n monitoring"
echo "kubectl get ingress -n monitoring"
echo ""
echo "Access the services at:"
echo "- Prometheus: https://prometheus.ogaro.com (user: ${MONITORING_USERNAME}, pass: ${MONITORING_PASSWORD})"
echo "- Alertmanager: https://alertmanager.ogaro.com (user: ${MONITORING_USERNAME}, pass: ${MONITORING_PASSWORD})"
echo "- Grafana: https://grafana.ogaro.com" 