#!/bin/bash

# Create namespace
kubectl create namespace monitoring

# Add prometheus-community repo if not already added
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

# Install kube-prometheus-stack
helm upgrade --install monitoring . \
  --namespace monitoring \
  -f values.yaml \
  -f values-prod.yaml

# Wait for pods to be ready
echo "Waiting for pods to be ready..."
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=prometheus -n monitoring --timeout=300s
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=alertmanager -n monitoring --timeout=300s
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=grafana -n monitoring --timeout=300s

# Show status
echo "Monitoring stack installed. Check status with:"
echo "kubectl get pods -n monitoring"
echo "kubectl get ingress -n monitoring" 