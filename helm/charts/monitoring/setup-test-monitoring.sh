#!/bin/bash

# Create test namespace
kubectl create namespace monitoring-test

# Create basic auth secret for Prometheus
htpasswd -c auth prometheus-test
kubectl create secret generic prometheus-test-basic-auth \
    --from-file=auth \
    --namespace monitoring-test

# Install the monitoring stack in test namespace
helm upgrade --install monitoring-test . \
    --namespace monitoring-test \
    --values values-test.yaml \
    --values prometheus-values.yaml

# Clean up
rm auth

echo "Test monitoring stack installation initiated. Please check the status with:"
echo "kubectl get pods -n monitoring-test"
echo "kubectl get ingress -n monitoring-test" 