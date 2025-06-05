# Kogaro Installation Guide

This guide covers multiple ways to install and use Kogaro in your Kubernetes cluster.

## Prerequisites

- Kubernetes cluster (1.24+)
- kubectl configured with cluster access
- Helm 3.x (for Helm installation)

## Installation Methods

### 1. Helm Repository (Recommended)

The easiest way to install Kogaro is via our Helm repository:

```bash
# Add the Kogaro Helm repository
helm repo add kogaro https://topiaruss.github.io/kogaro
helm repo update

# Install with default settings
helm install kogaro kogaro/kogaro \
  --namespace kogaro-system \
  --create-namespace

# Install with custom configuration
helm install kogaro kogaro/kogaro \
  --namespace kogaro-system \
  --create-namespace \
  --set validation.enableServiceAccountValidation=true \
  --set validation.scanInterval=10m
```

### 2. Docker Hub Image

Kogaro images are available on Docker Hub at `topiaruss/kogaro`:

```bash
# Pull the latest image
docker pull topiaruss/kogaro:latest

# Run locally for testing (requires kubeconfig)
docker run --rm \
  -v ~/.kube:/root/.kube:ro \
  topiaruss/kogaro:latest \
  --scan-interval=30s
```

### 3. Direct from Source

Clone and install directly from the source repository:

```bash
git clone https://github.com/topiaruss/kogaro.git
cd kogaro

# Install with Helm
helm install kogaro charts/kogaro \
  --namespace kogaro-system \
  --create-namespace

# Or build and run locally
make build
./bin/kogaro --help
```

## Configuration Options

### Helm Values

Key configuration options in `values.yaml`:

```yaml
# Image configuration
image:
  repository: topiaruss/kogaro
  tag: "latest"
  pullPolicy: IfNotPresent

# Validation settings
validation:
  enableIngressValidation: true
  enableConfigMapValidation: true
  enableSecretValidation: true
  enablePVCValidation: true
  enableServiceAccountValidation: false  # Can be noisy
  scanInterval: "5m"

# Resource limits
resources:
  limits:
    cpu: 500m
    memory: 128Mi
  requests:
    cpu: 100m
    memory: 64Mi

# Metrics and monitoring
metrics:
  enabled: true
  serviceMonitor:
    enabled: false  # Set to true if using Prometheus Operator
    interval: 30s
```

### Command Line Options

When running directly:

```bash
# Core options
--scan-interval=5m                           # How often to scan
--metrics-bind-address=:8080                # Metrics server address
--health-probe-bind-address=:8081           # Health probe address

# Validation toggles
--enable-ingress-validation=true
--enable-configmap-validation=true
--enable-secret-validation=true
--enable-pvc-validation=true
--enable-serviceaccount-validation=false
```

## Verification

After installation, verify Kogaro is working:

```bash
# Check pod status
kubectl get pods -n kogaro-system

# View logs
kubectl logs -n kogaro-system -l app.kubernetes.io/name=kogaro -f

# Check metrics (if metrics are enabled)
kubectl port-forward -n kogaro-system svc/kogaro 8080:8080
curl http://localhost:8080/metrics | grep kogaro
```

## Upgrading

### Helm Repository

```bash
# Update repository
helm repo update

# Upgrade installation
helm upgrade kogaro kogaro/kogaro -n kogaro-system
```

### Direct from Source

```bash
# Pull latest changes
git pull origin main

# Upgrade with Helm
helm upgrade kogaro charts/kogaro -n kogaro-system
```

## Uninstalling

```bash
# Remove Kogaro
helm uninstall kogaro -n kogaro-system

# Remove namespace (optional)
kubectl delete namespace kogaro-system
```

## Troubleshooting

### Common Issues

1. **RBAC Permissions**: Ensure the service account has proper cluster-wide read permissions
2. **Resource Limits**: Increase memory limits if validation fails on large clusters
3. **Metrics Not Available**: Check that the metrics port (8080) is accessible

### Getting Help

- Check logs: `kubectl logs -n kogaro-system -l app.kubernetes.io/name=kogaro`
- Verify RBAC: `kubectl auth can-i get pods --as=system:serviceaccount:kogaro-system:kogaro`
- Health check: `kubectl get pods -n kogaro-system` (should show Running status)

For more detailed deployment information, see the [Deployment Guide](DEPLOYMENT-GUIDE.md).