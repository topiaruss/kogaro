# Kogaro Deployment Guide

This guide provides comprehensive instructions for deploying and operating Kogaro in production environments.

## Overview

Kogaro is a Kubernetes validation agent that continuously monitors cluster resources for configuration hygiene issues. It detects dangling references that can cause silent failures and provides comprehensive observability through metrics and logging.

## Prerequisites

Before deploying Kogaro, ensure you have:

- **Kubernetes cluster** (v1.20+)
- **kubectl** configured with cluster access
- **Helm 3.x** installed
- **Container registry** access (if using private registry)
- **Cluster permissions** to create ClusterRoles and ClusterRoleBindings

## Deployment Methods

### Option 1: Helm Chart (Recommended)

The Helm chart provides the most flexible and production-ready deployment method.

#### Basic Installation

```bash
# Add the repository (if published to a Helm repo)
# helm repo add kogaro https://charts.kogaro.io

# Or install from local chart
git clone https://github.com/topiaruss/kogaro.git
cd kogaro

# Install with default configuration
helm install kogaro charts/kogaro --namespace kogaro-system --create-namespace
```

#### Production Installation with Custom Values

```bash
# Create custom values file
cat > kogaro-values.yaml << EOF
image:
  repository: registry.ogaro.com/kogaro
  tag: "0.1.1"
  pullPolicy: IfNotPresent

imagePullSecrets:
  - name: registry-credentials

validation:
  enableIngressValidation: true
  enableConfigMapValidation: true
  enableSecretValidation: true
  enablePVCValidation: true
  enableServiceAccountValidation: false  # Can be noisy
  scanInterval: "5m"

resources:
  limits:
    cpu: 500m
    memory: 256Mi
  requests:
    cpu: 100m
    memory: 128Mi

metrics:
  enabled: true
  serviceMonitor:
    enabled: true
    interval: 30s

nodeSelector:
  kubernetes.io/os: linux

tolerations:
  - key: node-role.kubernetes.io/control-plane
    effect: NoSchedule
EOF

# Install with custom values
helm install kogaro charts/kogaro \
  --namespace kogaro-system \
  --create-namespace \
  --values kogaro-values.yaml
```

### Option 2: Direct Kubernetes Manifests

For environments where Helm is not available:

```bash
# Render manifests
helm template kogaro charts/kogaro --namespace kogaro-system > kogaro-manifests.yaml

# Apply manifests
kubectl create namespace kogaro-system
kubectl apply -f kogaro-manifests.yaml
```

## Registry Configuration

### Using Private Container Registry

If using a private registry, create authentication credentials:

```bash
# Create registry secret
kubectl create secret docker-registry registry-credentials \
  --docker-server=registry.ogaro.com \
  --docker-username=your-username \
  --docker-password=your-password \
  --namespace kogaro-system

# Update Helm values to reference the secret
imagePullSecrets:
  - name: registry-credentials
```

### Using Public Registry

For public registries, simply specify the image repository:

```yaml
image:
  repository: ghcr.io/topiaruss/kogaro
  tag: "latest"
```

## Configuration Options

### Validation Settings

Configure which validations to enable based on your environment:

```yaml
validation:
  # Core validations (recommended to keep enabled)
  enableIngressValidation: true      # Validates Ingress → IngressClass, Service references
  enableConfigMapValidation: true    # Validates Pod → ConfigMap references
  enableSecretValidation: true       # Validates Pod → Secret, Ingress → TLS Secret references
  enablePVCValidation: true          # Validates Pod → PVC, PVC → StorageClass references
  
  # Optional validations
  enableServiceAccountValidation: false  # Can be noisy in dynamic environments
  
  # Scan frequency
  scanInterval: "5m"  # How often to run validation (1m, 5m, 15m, 1h)
```

### Resource Management

Set appropriate resource limits based on cluster size:

```yaml
resources:
  # For small clusters (< 100 nodes)
  limits:
    cpu: 200m
    memory: 128Mi
  requests:
    cpu: 50m
    memory: 64Mi
    
  # For large clusters (> 500 nodes)
  limits:
    cpu: 1000m
    memory: 512Mi
  requests:
    cpu: 200m
    memory: 256Mi
```

### Security Configuration

Kogaro follows security best practices by default:

```yaml
securityContext:
  allowPrivilegeEscalation: false
  capabilities:
    drop: ["ALL"]
  readOnlyRootFilesystem: true
  runAsNonRoot: true
  runAsUser: 65534

podSecurityContext:
  runAsNonRoot: true
  runAsUser: 65534
  fsGroup: 65534
```

## RBAC Requirements

Kogaro requires cluster-wide read access to the following resources:

```yaml
rules:
- apiGroups: [""]
  resources: ["pods", "services", "configmaps", "secrets", "serviceaccounts", "persistentvolumeclaims"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["networking.k8s.io"]
  resources: ["ingresses", "ingressclasses"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["storage.k8s.io"]
  resources: ["storageclasses"]
  verbs: ["get", "list", "watch"]
```

**Note**: Kogaro requires only **read** permissions and cannot modify cluster resources.

## Monitoring and Observability

### Prometheus Metrics

Kogaro exposes metrics on port 8080 at `/metrics`:

| Metric | Type | Description | Labels |
|--------|------|-------------|---------|
| `kogaro_validation_runs_total` | Counter | Total validation runs completed | none |
| `kogaro_validation_errors_total` | Counter | Total validation errors found | `resource_type`, `validation_type`, `namespace` |

### ServiceMonitor for Prometheus Operator

Enable automatic metrics collection:

```yaml
metrics:
  enabled: true
  serviceMonitor:
    enabled: true
    interval: 30s
    path: /metrics
```

### Grafana Dashboard

Example queries for monitoring:

```promql
# Validation error rate
rate(kogaro_validation_errors_total[5m])

# Validation runs per hour
increase(kogaro_validation_runs_total[1h])

# Errors by type
sum by (validation_type) (kogaro_validation_errors_total)

# Errors by namespace
sum by (namespace) (kogaro_validation_errors_total)
```

### Log Analysis

Kogaro uses structured logging. Key log entries:

```json
// Successful validation
{"level":"info","msg":"cluster validation completed","total_errors":0}

// Validation error found
{"level":"info","msg":"validation error found","resource_type":"Pod","resource_name":"my-pod","namespace":"default","validation_type":"dangling_configmap_volume","message":"ConfigMap 'missing-config' referenced in volume does not exist"}
```

## Operational Procedures

### Health Checks

Kogaro provides health and readiness endpoints:

```bash
# Check health
kubectl get pods -n kogaro-system
kubectl exec -n kogaro-system deployment/kogaro -- wget -q -O- http://localhost:8081/healthz

# Check readiness
kubectl exec -n kogaro-system deployment/kogaro -- wget -q -O- http://localhost:8081/readyz
```

### Viewing Validation Results

```bash
# View recent logs
kubectl logs -n kogaro-system -l app.kubernetes.io/name=kogaro --tail=50

# Follow logs in real-time
kubectl logs -n kogaro-system -l app.kubernetes.io/name=kogaro -f

# Query specific validation errors
kubectl logs -n kogaro-system -l app.kubernetes.io/name=kogaro | grep "validation error found"
```

### Accessing Metrics

```bash
# Port forward to metrics endpoint
kubectl port-forward -n kogaro-system svc/kogaro 8080:8080

# Query metrics
curl http://localhost:8080/metrics | grep kogaro
```

### Upgrading

```bash
# Update Helm chart
helm upgrade kogaro charts/kogaro \
  --namespace kogaro-system \
  --values kogaro-values.yaml

# Check rollout status
kubectl rollout status deployment/kogaro -n kogaro-system
```

### Scaling for High Availability

For critical environments, consider running multiple replicas:

```yaml
replicaCount: 2

affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
    - weight: 100
      podAffinityTerm:
        labelSelector:
          matchExpressions:
          - key: app.kubernetes.io/name
            operator: In
            values:
            - kogaro
        topologyKey: kubernetes.io/hostname
```

**Note**: Multiple replicas provide fault tolerance through leader election. Only the leader performs validation, while other replicas remain idle but ready to take over if the leader fails.

## Troubleshooting

### Common Issues

#### 1. ImagePullBackOff

```bash
# Check image pull secrets
kubectl describe pod -n kogaro-system

# Verify registry credentials
kubectl get secret registry-credentials -n kogaro-system -o yaml
```

#### 2. RBAC Permission Errors

```bash
# Test service account permissions
kubectl auth can-i list pods --as=system:serviceaccount:kogaro-system:kogaro
kubectl auth can-i list ingresses --as=system:serviceaccount:kogaro-system:kogaro
```

#### 3. High Memory Usage

```bash
# Check resource usage
kubectl top pod -n kogaro-system

# Increase memory limits if needed
resources:
  limits:
    memory: 512Mi
```

#### 4. Missing Metrics

```bash
# Verify metrics endpoint
kubectl port-forward -n kogaro-system svc/kogaro 8080:8080
curl http://localhost:8080/metrics | grep kogaro

# Check ServiceMonitor if using Prometheus Operator
kubectl get servicemonitor -n kogaro-system
```

### Debug Mode

Enable verbose logging for troubleshooting:

```yaml
# Add to deployment args
args:
  - --zap-log-level=debug
  - --zap-development=true
```

## Performance Considerations

### Cluster Size Impact

| Cluster Size | Recommended Resources | Scan Interval |
|--------------|----------------------|---------------|
| Small (< 100 nodes) | 100m CPU, 128Mi RAM | 5m |
| Medium (100-500 nodes) | 200m CPU, 256Mi RAM | 5m |
| Large (> 500 nodes) | 500m CPU, 512Mi RAM | 10m |

### Network Considerations

Kogaro generates API calls proportional to cluster resource count:

- ~1 API call per resource type per validation run
- Uses Kubernetes client caching to minimize API load
- Watch events reduce full list operations

### Tuning for Noisy Validations

Some validations may generate noise in dynamic environments:

```yaml
validation:
  # Disable in environments with frequent Pod creation/deletion
  enableServiceAccountValidation: false
  
  # Increase interval in busy clusters
  scanInterval: "15m"
```

## Security Considerations

### Principle of Least Privilege

Kogaro follows security best practices:

- Read-only cluster access
- Non-root container execution
- No secret value logging
- Minimal attack surface with distroless images

### Network Policies

Restrict Kogaro's network access if required:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: kogaro-netpol
  namespace: kogaro-system
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: kogaro
  policyTypes:
  - Egress
  egress:
  - to: []  # Kubernetes API server
    ports:
    - protocol: TCP
      port: 443
```

### Audit Logging

Kogaro operations appear in Kubernetes audit logs:

```bash
# Monitor Kogaro API access
kubectl get events --field-selector involvedObject.name=kogaro
```

## Integration Examples

### Alertmanager Rules

```yaml
groups:
- name: kogaro
  rules:
  - alert: KogaroValidationErrors
    expr: increase(kogaro_validation_errors_total[5m]) > 0
    for: 0m
    labels:
      severity: warning
    annotations:
      summary: "Kogaro detected configuration issues"
      description: "{{ $value }} validation errors detected in {{ $labels.namespace }}"
      
  - alert: KogaroDown
    expr: up{job="kogaro"} == 0
    for: 5m
    labels:
      severity: critical
    annotations:
      summary: "Kogaro is down"
      description: "Kogaro validation service is not responding"
```

### CI/CD Integration

Use Kogaro metrics to gate deployments:

```bash
#!/bin/bash
# Check for validation errors before proceeding
ERRORS=$(curl -s http://kogaro.kogaro-system.svc.cluster.local:8080/metrics | \
  grep kogaro_validation_errors_total | \
  awk '{sum += $2} END {print sum}')

if [ "$ERRORS" -gt 0 ]; then
  echo "❌ Deployment blocked: $ERRORS validation errors detected"
  exit 1
fi

echo "✅ No validation errors detected, proceeding with deployment"
```

## Support and Maintenance

### Log Retention

Configure log retention based on requirements:

```bash
# View log volume
kubectl logs -n kogaro-system -l app.kubernetes.io/name=kogaro --tail=1000 | wc -l

# Rotate logs if needed (handled by Kubernetes)
kubectl rollout restart deployment/kogaro -n kogaro-system
```

### Backup and Recovery

Kogaro is stateless - no backup required. Recovery involves redeployment:

```bash
# Disaster recovery
helm uninstall kogaro -n kogaro-system
helm install kogaro charts/kogaro -n kogaro-system --values kogaro-values.yaml
```

### Updates and Maintenance

```bash
# Check for updates
helm repo update  # if using helm repo

# Review changelog before updating
# Update image tag in values.yaml
# Test in staging environment first
helm upgrade kogaro charts/kogaro -n kogaro-system --values kogaro-values.yaml
```

For additional support, see the [main README](../README.md) and [CONTRIBUTING](../CONTRIBUTING.md) guides.