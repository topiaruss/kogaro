# Kogaro - Kubernetes Configuration Hygiene Agent

[![CI](https://github.com/topiaruss/kogaro/workflows/CI/badge.svg)](https://github.com/topiaruss/kogaro/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/topiaruss/kogaro)](https://goreportcard.com/report/github.com/topiaruss/kogaro)
[![codecov](https://codecov.io/gh/topiaruss/kogaro/branch/main/graph/badge.svg)](https://codecov.io/gh/topiaruss/kogaro)
[![GoDoc](https://pkg.go.dev/badge/github.com/topiaruss/kogaro.svg)](https://pkg.go.dev/github.com/topiaruss/kogaro)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
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

### Comprehensive Kubernetes Validation (37+ validation types)

Kogaro provides four comprehensive validation categories covering all critical aspects of Kubernetes cluster hygiene:

#### 1. Reference Validation (11 validation types)
Detects dangling references to non-existent resources:

- **Ingress References** (`--enable-ingress-validation`)
  - `dangling_ingress_class`: Missing IngressClass references
  - `dangling_service_reference`: Missing Service references in ingress rules
  - `dangling_tls_secret`: Missing TLS Secrets in ingress

- **ConfigMap References** (`--enable-configmap-validation`)
  - `dangling_configmap_volume`: Missing ConfigMap volume references
  - `dangling_configmap_envfrom`: Missing ConfigMap envFrom references

- **Secret References** (`--enable-secret-validation`)
  - `dangling_secret_volume`: Missing Secret volume references
  - `dangling_secret_envfrom`: Missing Secret envFrom references
  - `dangling_secret_env`: Missing Secret env var references

- **Storage References** (`--enable-pvc-validation`)
  - `dangling_pvc_reference`: Missing PVC references
  - `dangling_storage_class`: Missing StorageClass references

- **ServiceAccount References** (`--enable-serviceaccount-validation`)
  - `dangling_service_account`: Missing ServiceAccount references

#### 2. Resource Limits Validation (6 validation types)
Ensures proper resource management and QoS:

- **Resource Constraints** (`--enable-resource-limits-validation`)
  - `missing_resource_requests`: Containers without CPU/memory requests
  - `missing_resource_limits`: Containers without CPU/memory limits
  - `insufficient_cpu_request`: CPU requests below minimum thresholds
  - `insufficient_memory_request`: Memory requests below minimum thresholds
  - `qos_class_issue` (BestEffort): Containers with no resource constraints
  - `qos_class_issue` (Burstable): Containers where requests ≠ limits

#### 3. Security Validation (11+ validation types)
Detects security misconfigurations and vulnerabilities:

- **Pod & Container Security** (`--enable-security-validation`)
  - `pod_running_as_root`: Pod SecurityContext specifies runAsUser: 0
  - `pod_allows_root_user`: Pod SecurityContext missing runAsNonRoot: true
  - `container_running_as_root`: Container SecurityContext specifies runAsUser: 0
  - `container_allows_privilege_escalation`: Container allows privilege escalation
  - `container_privileged_mode`: Container running in privileged mode
  - `container_writable_root_filesystem`: Container has writable root filesystem
  - `container_additional_capabilities`: Container adds Linux capabilities
  - `missing_pod_security_context`: Pod has no SecurityContext defined
  - `missing_container_security_context`: Container has no SecurityContext defined

- **ServiceAccount & RBAC Security** (`--enable-security-serviceaccount-validation`)
  - `serviceaccount_cluster_role_binding`: ServiceAccount with ClusterRoleBinding
  - `serviceaccount_excessive_permissions`: ServiceAccount with dangerous RoleBinding

#### 4. Networking Validation (9 validation types)
Validates service connectivity and network policies:

- **Service Connectivity** (`--enable-networking-validation`)
  - `service_selector_mismatch`: Service selectors that don't match any pods
  - `service_no_endpoints`: Services with no ready endpoints despite matching pods
  - `service_port_mismatch`: Service ports that don't match container ports
  - `pod_no_service`: Pods not exposed by any Service (warning when enabled)

- **NetworkPolicy Coverage** (`--networking-policy-validation`)
  - `network_policy_orphaned`: NetworkPolicy selectors that don't match any pods
  - `missing_network_policy_default_deny`: Namespaces with policies but no default deny
  - `missing_network_policy_required`: Required namespaces missing NetworkPolicies

- **Ingress Connectivity** (`--enable-networking-validation`)
  - `ingress_service_missing`: Ingress references to non-existent services
  - `ingress_service_port_mismatch`: Ingress references to non-existent service ports
  - `ingress_no_backend_pods`: Ingress services with no ready backend pods

### Observability

- **Prometheus Metrics**: Exports validation error counts and run statistics
- **Structured Logging**: Detailed logs of all validation issues found
- **Health Checks**: Kubernetes-native health and readiness probes

## Quick Start

For detailed deployment instructions, see the [Deployment Guide](docs/DEPLOYMENT-GUIDE.md).

### Prerequisites

- Go 1.21 or later
- Kubernetes cluster access
- kubectl configured

### Installation

#### Option 1: Helm Repository (Recommended)

```bash
# Add the Kogaro Helm repository
helm repo add kogaro https://topiaruss.github.io/kogaro
helm repo update

# Install Kogaro with default settings
helm install kogaro kogaro/kogaro \
  --namespace kogaro-system \
  --create-namespace

# Or install with custom configuration
helm install kogaro kogaro/kogaro \
  --namespace kogaro-system \
  --create-namespace \
  --set validation.enableServiceAccountValidation=true \
  --set validation.scanInterval=10m \
  --set resourceLimits.minCPURequest=50m \
  --set security.enableNetworkPolicyValidation=true

# Check deployment status
kubectl get pods -n kogaro-system

# View logs
kubectl logs -n kogaro-system -l app.kubernetes.io/name=kogaro -f
```

#### Option 2: Direct from Source

```bash
# Clone and install directly
git clone https://github.com/topiaruss/kogaro.git
cd kogaro
helm install kogaro charts/kogaro --namespace kogaro-system --create-namespace
```

#### Option 3: Docker Image

```bash
# Run directly with Docker (for testing)
docker run --rm topiaruss/kogaro:latest --help
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

# Run with specific validations enabled
go run main.go --enable-secret-validation=false --enable-security-validation=true --min-cpu-request=100m

# Or build and run
make build
./bin/kogaro --help
```

## Configuration

### Command Line Flags

#### Core Configuration Flags
- `--scan-interval`: Interval between cluster scans (default: 5m)
- `--metrics-bind-address`: Metrics server bind address (default: :8080)
- `--health-probe-bind-address`: Health probe bind address (default: :8081)
- `--leader-elect`: Enable leader election for HA deployments (default: false)

#### Reference Validation Flags
- `--enable-ingress-validation`: Enable Ingress references validation (default: true)
- `--enable-configmap-validation`: Enable ConfigMap references validation (default: true)
- `--enable-secret-validation`: Enable Secret references validation (default: true)
- `--enable-pvc-validation`: Enable PVC/StorageClass validation (default: true)
- `--enable-reference-serviceaccount-validation`: Enable ServiceAccount reference validation (default: false)

#### Resource Limits Validation Flags
- `--enable-resource-limits-validation`: Enable resource requests/limits validation (default: true)
- `--enable-missing-requests-validation`: Enable missing requests validation (default: true)
- `--enable-missing-limits-validation`: Enable missing limits validation (default: true)
- `--enable-qos-validation`: Enable QoS class analysis (default: true)
- `--min-cpu-request`: Minimum CPU request threshold (e.g., '10m')
- `--min-memory-request`: Minimum memory request threshold (e.g., '16Mi')

#### Security Validation Flags
- `--enable-security-validation`: Enable security configuration validation (default: true)
- `--enable-root-user-validation`: Enable root user validation (default: true)
- `--enable-security-context-validation`: Enable SecurityContext validation (default: true)
- `--enable-security-serviceaccount-validation`: Enable ServiceAccount permissions validation (default: true)
- `--enable-network-policy-validation`: Enable NetworkPolicy validation (default: true)
- `--security-required-namespaces`: Namespaces requiring NetworkPolicies for security validation

#### Networking Validation Flags
- `--enable-networking-validation`: Enable networking connectivity validation (default: true)
- `--enable-networking-service-validation`: Enable Service validation (default: true)
- `--enable-networking-ingress-validation`: Enable Ingress connectivity validation (default: true)
- `--enable-networking-policy-validation`: Enable NetworkPolicy coverage validation (default: true)
- `--networking-required-namespaces`: Namespaces requiring NetworkPolicies for networking validation
- `--warn-unexposed-pods`: Warn about pods not exposed by Services (default: false)

### Prometheus Metrics

Access metrics at `http://localhost:8080/metrics`:

```
# Total validation errors by type
kogaro_validation_errors_total{resource_type="Ingress",validation_type="dangling_ingress_class",namespace="default"}

# Total validation runs
kogaro_validation_runs_total
```

## Architecture

Kogaro uses a modular validator architecture built on controller-runtime:

1. **Validator Registry**: Manages four validator types (Reference, ResourceLimits, Security, Networking)
2. **Periodic Scanning**: Runs comprehensive validation every `scan-interval`
3. **Multi-Domain Validation**: 
   - Reference resolution for dangling references
   - Resource constraint analysis for proper limits/requests
   - Security configuration validation for misconfigurations
   - Networking connectivity validation for service health
4. **Centralized Metrics**: Thread-safe Prometheus metrics collection
5. **Error Reporting**: Structured logs and metrics for all validation issues
6. **Leader Election**: Supports HA deployments in multi-replica scenarios

## Extending Validations

The validator registry pattern supports easy extension. Add new validators by implementing the `Validator` interface:

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

## Documentation

- **[Deployment Guide](docs/DEPLOYMENT-GUIDE.md)** - Comprehensive deployment and configuration instructions
- **[Contributing Guide](CONTRIBUTING.md)** - Development setup and contribution guidelines
- **[Security Policy](SECURITY.md)** - Security considerations and vulnerability reporting

## Future Enhancements

- **HPA target validation** 
- **RBAC reference validation**
- **Custom resource validations**
- **Webhook for admission control**
- **Slack/email alerting**