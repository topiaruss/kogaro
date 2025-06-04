# Kogaro - Kubernetes Configuration Hygiene Agent

[![CI](https://github.com/topiaruss/kogaro/workflows/CI/badge.svg)](https://github.com/topiaruss/kogaro/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/topiaruss/kogaro)](https://goreportcard.com/report/github.com/topiaruss/kogaro)
[![codecov](https://codecov.io/gh/topiaruss/kogaro/branch/main/graph/badge.svg)](https://codecov.io/gh/topiaruss/kogaro)
[![GoDoc](https://pkg.go.dev/badge/github.com/topiaruss/kogaro.svg)](https://pkg.go.dev/github.com/topiaruss/kogaro)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![GitHub release](https://img.shields.io/github/release/topiaruss/kogaro.svg)](https://github.com/topiaruss/kogaro/releases)

Kogaro is a runtime Kubernetes agent that continuously validates resource references and identifies configuration hygiene issues that can cause silent failures.

## Problem Statement

Kubernetes resources often reference other resources (IngressClasses, ConfigMaps, Services, etc.), but these references can become "dangling" due to:

- **Late binding**: Resources deployed before their dependencies
- **Typos**: Incorrect resource names (e.g., `nginx` vs `ingress-nginx`) 
- **Manual changes**: Direct cluster modifications bypassing CI/CD
- **Cleanup issues**: Dependencies deleted while references remain

These issues often manifest as silent failures that are difficult to diagnose, or they are lost in the avalanche of logs.

## Features

### Current Validations

- **Ingress References** (`--enable-ingress-validation`)
  - Validates `ingressClassName` references to existing IngressClass resources
  - Validates Service references in Ingress rules

- **ConfigMap References** (`--enable-configmap-validation`)
  - Validates ConfigMap references in Pod volumes
  - Validates ConfigMap references in container `envFrom`

- **Secret References** (`--enable-secret-validation`)
  - Validates Secret references in Pod volumes
  - Validates Secret references in container `envFrom` and `env`
  - Validates TLS Secret references in Ingress resources

- **Storage References** (`--enable-pvc-validation`)
  - Validates PVC references in Pod volumes
  - Validates StorageClass references in PVCs

- **ServiceAccount References** (`--enable-serviceaccount-validation`, disabled by default)
  - Validates ServiceAccount references in Pods
  - Note: Can be noisy in environments with frequent Pod churn

### Observability

- **Prometheus Metrics**: Exports validation error counts and run statistics
- **Structured Logging**: Detailed logs of all validation issues found
- **Health Checks**: Kubernetes-native health and readiness probes

## Installation

### Prerequisites

- Go 1.21 or later
- Kubernetes cluster access
- kubectl configured

### Quick Start

#### Deploy to Cluster

```bash
# Apply the deployment manifests
kubectl apply -f config/deployment.yaml

# Check deployment status
kubectl get pods -n kogaro-system

# View logs
kubectl logs -n kogaro-system -l app=kogaro -f
```

#### Local Development

```bash
# Clone the repository
git clone https://github.com/topiaruss/kogaro.git
cd kogaro

# Install dependencies
go mod download

# Run locally against your current kubeconfig
go run main.go --scan-interval=30s

# Run with only specific validations enabled
go run main.go --enable-secret-validation=false --enable-serviceaccount-validation=true

# Or build and run
make build
./bin/kogaro --help
```

## Configuration

### Command Line Flags

#### Core Flags
- `--scan-interval`: How often to scan the cluster (default: 5m)
- `--metrics-bind-address`: Metrics server address (default: :8080)
- `--health-probe-bind-address`: Health probe address (default: :8081)
- `--leader-elect`: Enable leader election for HA deployments

#### Validation Control Flags
- `--enable-ingress-validation`: Enable Ingress reference validation (default: true)
- `--enable-configmap-validation`: Enable ConfigMap reference validation (default: true)
- `--enable-secret-validation`: Enable Secret reference validation (default: true)
- `--enable-pvc-validation`: Enable PVC/StorageClass validation (default: true)
- `--enable-serviceaccount-validation`: Enable ServiceAccount validation (default: false, may be noisy)

### Prometheus Metrics

Access metrics at `http://localhost:8080/metrics`:

```
# Total validation errors by type
kogaro_validation_errors_total{resource_type="Ingress",validation_type="dangling_ingress_class",namespace="default"}

# Total validation runs
kogaro_validation_runs_total
```

## Architecture

Kogaro uses the controller-runtime framework to:

1. **Periodic Scanning**: Runs validation every `scan-interval`
2. **Reference Resolution**: Checks that referenced resources exist
3. **Error Reporting**: Logs issues and exports metrics
4. **Leader Election**: Supports HA deployments

## Extending Validations

Add new validation types in `internal/validators/reference_validator.go`:

```go
func (v *ReferenceValidator) validateNewResourceType(ctx context.Context) ([]ValidationError, error) {
    // Your validation logic here
    return errors, nil
}
```

Then call it from `ValidateCluster()`.

## Example Issues Caught

### Dangling IngressClass Reference
```yaml
# Ingress with non-existent IngressClass
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-app
spec:
  ingressClassName: nginx  # ❌ Should be 'ingress-nginx'
```

**Kogaro Output:**
```
validation error found: resource_type=Ingress resource_name=my-app validation_type=dangling_ingress_class message="IngressClass 'nginx' does not exist"
```

### Missing ConfigMap Reference
```yaml
# Pod referencing non-existent ConfigMap
spec:
  containers:
  - name: app
    envFrom:
    - configMapRef:
        name: app-settings  # ❌ ConfigMap doesn't exist
```

**Kogaro Output:**
```
validation error found: resource_type=Pod resource_name=my-pod validation_type=dangling_configmap_envfrom message="ConfigMap 'app-settings' referenced in envFrom does not exist"
```

## Future Enhancements

- **Secret references validation**
- **HPA target validation** 
- **RBAC reference validation**
- **PVC/StorageClass validation**
- **Custom resource validations**
- **Webhook for admission control**
- **Slack/email alerting**