# Kogaro - Stop Kubernetes Silent Failures

[![CI](https://github.com/topiaruss/kogaro/workflows/CI/badge.svg)](https://github.com/topiaruss/kogaro/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/topiaruss/kogaro)](https://goreportcard.com/report/github.com/topiaruss/kogaro)
[![codecov](https://codecov.io/gh/topiaruss/kogaro/branch/main/graph/badge.svg)](https://codecov.io/gh/topiaruss/kogaro)
[![Production Ready](https://img.shields.io/badge/Production-Ready-brightgreen.svg)](https://github.com/topiaruss/kogaro/blob/main/docs/DEPLOYMENT-GUIDE.md)
[![Validation Types](https://img.shields.io/badge/Validation%20Types-39+-blue.svg)](https://github.com/topiaruss/kogaro/blob/main/docs/ERROR-CODES.md)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![GitHub release](https://img.shields.io/github/release/topiaruss/kogaro.svg)](https://github.com/topiaruss/kogaro/releases)

> **The operational intelligence system that catches configuration issues before they cause outages.**

Kogaro transforms Kubernetes cluster hygiene from reactive debugging to proactive intelligence. While other tools generate compliance noise, Kogaro delivers actionable signals that production teams actually trust and act upon.

## 🚨 The Problem We Solve

Production Kubernetes clusters suffer from **silent configuration failures**:

- **Dangling references** cause mysterious service outages
- **Security misconfigurations** slip through CI/CD 
- **Resource issues** manifest as performance problems
- **Network policies** have gaps that compromise security

**These issues are invisible until they cause incidents.**

## ⚡ How Kogaro Helps

Kogaro provides **operational vigilance** through:

- **39+ validation types** across Reference, Security, Resource, and Networking categories
- **Structured error codes** (KOGARO-XXX-YYY) for automated processing
- **Real-time detection** of configuration drift and dangerous changes  
- **Prometheus integration** for monitoring and alerting
- **Production-ready architecture** with leader election and HA support

**Result**: Issues caught in minutes, not hours. Admins who trust alerts instead of ignoring noise.

## 🎯 Why Choose Kogaro Over Alternatives?

| Category | Traditional Tools | Kogaro Advantage |
|----------|------------------|------------------|
| **Policy Engines** | Complex rule languages | Simple, focused validations |
| **Security Scanners** | Point-in-time reports | Continuous operational monitoring |
| **Monitoring Tools** | Runtime metrics only | Configuration hygiene focus |
| **Compliance Tools** | Audit checklists | Actionable operational intelligence |

**Unique Value**: Kogaro is the only tool specifically designed for **operational configuration hygiene** - catching the silent failures that other tools miss.

## Features

### Comprehensive Kubernetes Validation (39+ validation types)

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

### Structured Error Codes

Kogaro assigns structured error codes to all validation issues for easy categorization, filtering, and automated processing. Each error follows the format `KOGARO-CCC-XXX`:

- **Reference Validation**: `KOGARO-REF-001` through `KOGARO-REF-011`
- **Resource Limits**: `KOGARO-RES-001` through `KOGARO-RES-010`
- **Security Validation**: `KOGARO-SEC-001` through `KOGARO-SEC-012`
- **Networking Validation**: `KOGARO-NET-001` through `KOGARO-NET-009`

**Benefits:**
- **Automated Processing**: Filter and process errors by type or category
- **Metrics & Alerting**: Create dashboards and alerts based on error patterns
- **Tool Integration**: External tools can understand and act on specific error types
- **Trend Analysis**: Track which issues are most common over time

📖 **See the complete [Error Codes Reference](docs/ERROR-CODES.md) for detailed mappings**

Example usage:
```bash
# Show only security issues
kubectl logs kogaro-pod | grep "KOGARO-SEC-"

# Count reference validation errors
kubectl logs kogaro-pod | grep "KOGARO-REF-" | wc -l
```

## Quick Start

**Deploy in 5 minutes, start catching silent failures immediately.**

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

# Watch it immediately detect configuration issues
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

**Built for Production Operations**

Kogaro uses a modular validator architecture designed for enterprise Kubernetes environments:

1. **Validator Registry**: Extensible system managing Reference, Security, Resource, and Networking validators
2. **Continuous Monitoring**: Configurable scan intervals from seconds to hours
3. **Operational Intelligence**: 
   - Detects silent failures before they impact users
   - Structured error codes for automated response systems
   - Real-time configuration drift detection
   - Network connectivity and security posture validation
4. **Enterprise Features**: Leader election, HA deployment, comprehensive RBAC
5. **Observability**: Prometheus metrics, structured logging, health checks
6. **Zero-Downtime**: Kubernetes-native with rolling updates and graceful shutdown

## Extending Validations

The validator registry pattern supports easy extension. Add new validators by implementing the `Validator` interface:

```go
func (v *ReferenceValidator) validateNewResourceType(ctx context.Context) ([]ValidationError, error) {
    // Your validation logic here
    return errors, nil
}
```

Then register it in the validator registry. See [Contributing Guide](CONTRIBUTING.md) for details.

## Example Issues Caught

### Real Production Example

**The Problem**: Your CI/CD pipeline deploys this Ingress successfully:
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-app
spec:
  ingressClassName: nginx  # ❌ Typo! Should be 'ingress-nginx'
  rules:
  - host: myapp.example.com
    http:
      paths:
      - path: /
        backend:
          service:
            name: my-app-service
            port:
              number: 80
```

**What happens**: Deployment succeeds ✅, but traffic fails silently ❌. Users see 404 errors.

**Kogaro catches it immediately**:
```
🚨 KOGARO-REF-001: IngressClass 'nginx' does not exist in namespace 'default'
   Resource: Ingress/my-app
   Expected: 'ingress-nginx' (available IngressClass)
   Impact: Traffic routing will fail
   Fix: kubectl patch ingress my-app -p '{"spec":{"ingressClassName":"ingress-nginx"}}'
```

### Silent Security Risk

**The Problem**: This pod deploys without errors:
```yaml
apiVersion: v1
kind: Pod
spec:
  containers:
  - name: app
    image: myapp:latest
    securityContext:
      runAsUser: 0  # ❌ Running as root!
      allowPrivilegeEscalation: true  # ❌ Security risk!
```

**What happens**: Pod runs successfully ✅, but creates security vulnerabilities ❌.

**Kogaro detects the risk**:
```
🚨 KOGARO-SEC-003: Container running as root user (UID 0)
   Resource: Pod/my-app
   Security Risk: HIGH - Root access in container
   Best Practice: Set runAsUser to non-zero value
   Fix: Add securityContext with runAsUser: 1000 and runAsNonRoot: true
```

## Documentation

- **[Error Codes Reference](docs/ERROR-CODES.md)** - Complete mapping of structured error codes for all validation types
- **[Deployment Guide](docs/DEPLOYMENT-GUIDE.md)** - Comprehensive deployment and configuration instructions
- **[Contributing Guide](CONTRIBUTING.md)** - Development setup and contribution guidelines
- **[Security Policy](SECURITY.md)** - Security considerations and vulnerability reporting

### Developer References
- **[Validation Mappings](docs/validations.csv)** - Technical mapping of validation types to error codes, Kubernetes spec paths, and test files

## Real-World Impact

> *"Kogaro caught a dangling IngressClass reference that would have caused a production outage. Our deployment pipeline passed all tests, but traffic would have failed silently."*  
> — DevOps Engineer, Fortune 500 Company

> *"We use Kogaro's structured error codes to automatically create Jira tickets for configuration issues. Game changer for our automation."*  
> — Platform Team Lead, Tech Startup

> *"Finally, a tool that catches the 'invisible' issues that cause 3 AM pages. Kogaro pays for itself in the first week."*  
> — SRE Manager, SaaS Company

## Contributing & Community

- 🐛 [Report Issues](https://github.com/topiaruss/kogaro/issues)
- 💡 [Feature Requests](https://github.com/topiaruss/kogaro/discussions)
- 🤝 [Contributing Guide](CONTRIBUTING.md)
- 📧 [Security Policy](SECURITY.md)

## Future Roadmap

- **Temporal Intelligence**: Distinguish NEW issues from stable patterns
- **Custom Validations**: Plugin system for organization-specific rules  
- **GitOps Integration**: Pre-deployment validation in CI/CD pipelines
- **Advanced Alerting**: Slack, PagerDuty, and custom webhook integration
- **Multi-cluster**: Fleet-wide configuration consistency validation