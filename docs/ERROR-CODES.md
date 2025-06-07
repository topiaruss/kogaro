# Kogaro Error Codes Reference

Kogaro uses structured error codes to categorize and identify validation issues systematically. Each error follows the format `KOGARO-CCC-XXX` where:

- `CCC` = Category (REF, RES, SEC, NET)
- `XXX` = Sequential number within category

## Error Code Categories

### Reference Validation (REF)
Validates references between Kubernetes resources to detect dangling references.

| Error Code | Validation Type | Entity | Description |
|------------|----------------|--------|-------------|
| KOGARO-REF-001 | `dangling_ingress_class` | Ingress | IngressClass referenced but does not exist |
| KOGARO-REF-002 | `dangling_service_reference` | Ingress | Service referenced in Ingress does not exist |
| KOGARO-REF-003 | `dangling_configmap_volume` | Pod | ConfigMap referenced in volume does not exist |
| KOGARO-REF-004 | `dangling_configmap_envfrom` | Pod | ConfigMap referenced in envFrom does not exist |
| KOGARO-REF-005 | `dangling_secret_volume` | Pod | Secret referenced in volume does not exist |
| KOGARO-REF-006 | `dangling_secret_envfrom` | Pod | Secret referenced in envFrom does not exist |
| KOGARO-REF-007 | `dangling_secret_env` | Pod | Secret referenced in env does not exist |
| KOGARO-REF-008 | `dangling_tls_secret` | Ingress | TLS Secret referenced in Ingress does not exist |
| KOGARO-REF-009 | `dangling_storage_class` | PVC | StorageClass referenced but does not exist |
| KOGARO-REF-010 | `dangling_pvc_reference` | Pod | PVC referenced in volume does not exist |
| KOGARO-REF-011 | `dangling_service_account` | Pod | ServiceAccount referenced but does not exist |

### Resource Limits Validation (RES)
Validates resource requests, limits, and QoS configurations.

| Error Code | Validation Type | Entity | Description |
|------------|----------------|--------|-------------|
| KOGARO-RES-001 | `missing_resource_requests` | Deployment | Container has no resource requests defined |
| KOGARO-RES-002 | `missing_resource_requests` | StatefulSet | Container has no resource requests defined |
| KOGARO-RES-003 | `missing_resource_limits` | Deployment | Container has no resource limits (no requests either) |
| KOGARO-RES-004 | `missing_resource_limits` | Deployment | Container has no resource limits (has requests) |
| KOGARO-RES-005 | `missing_resource_limits` | StatefulSet | Container has no resource limits defined |
| KOGARO-RES-006 | `insufficient_cpu_request` | Deployment | Container CPU request below minimum threshold |
| KOGARO-RES-007 | `insufficient_memory_request` | Deployment | Container memory request below minimum threshold |
| KOGARO-RES-008 | `qos_class_issue` | Deployment | BestEffort QoS: no resource constraints |
| KOGARO-RES-009 | `qos_class_issue` | StatefulSet | BestEffort QoS: no resource constraints |
| KOGARO-RES-010 | `qos_class_issue` | Deployment | Burstable QoS: requests != limits |

### Security Validation (SEC)
Validates security contexts, permissions, and compliance.

| Error Code | Validation Type | Entity | Description |
|------------|----------------|--------|-------------|
| KOGARO-SEC-001 | `pod_running_as_root` | Pod | Pod SecurityContext specifies runAsUser: 0 (root) |
| KOGARO-SEC-002 | `pod_allows_root_user` | Pod | Pod SecurityContext does not enforce runAsNonRoot: true |
| KOGARO-SEC-003 | `container_running_as_root` | Container | Container SecurityContext specifies runAsUser: 0 (root) |
| KOGARO-SEC-004 | `container_allows_privilege_escalation` | Container | Container does not set allowPrivilegeEscalation: false |
| KOGARO-SEC-005 | `container_allows_privilege_escalation` | Container | Privileged container does not set allowPrivilegeEscalation: false |
| KOGARO-SEC-006 | `container_privileged_mode` | Container | Container SecurityContext specifies privileged: true |
| KOGARO-SEC-007 | `container_writable_root_filesystem` | Container | Container does not set readOnlyRootFilesystem: true |
| KOGARO-SEC-008 | `container_additional_capabilities` | Container | Container SecurityContext adds dangerous capabilities |
| KOGARO-SEC-009 | `missing_pod_security_context` | Pod | Pod has no SecurityContext defined |
| KOGARO-SEC-010 | `missing_container_security_context` | Container | Container has no SecurityContext defined |
| KOGARO-SEC-011 | `serviceaccount_cluster_role_binding` | ServiceAccount | ServiceAccount has excessive ClusterRoleBinding |
| KOGARO-SEC-012 | `serviceaccount_excessive_permissions` | ServiceAccount | ServiceAccount has potentially excessive RoleBinding |

### Networking Validation (NET)
Validates service connectivity, network policies, and ingress configurations.

| Error Code | Validation Type | Entity | Description |
|------------|----------------|--------|-------------|
| KOGARO-NET-001 | `service_selector_mismatch` | Service | Service selector does not match any pods |
| KOGARO-NET-002 | `service_no_endpoints` | Service | Service has no ready endpoints |
| KOGARO-NET-003 | `service_port_mismatch` | Service | Service port does not match container ports |
| KOGARO-NET-004 | `pod_no_service` | Pod | Pod is not exposed by any Service |
| KOGARO-NET-005 | `network_policy_orphaned` | NetworkPolicy | NetworkPolicy selector does not match any pods |
| KOGARO-NET-006 | `missing_network_policy_default_deny` | Namespace | Namespace has NetworkPolicies but no default deny |
| KOGARO-NET-007 | `ingress_service_missing` | Ingress | Ingress references non-existent service |
| KOGARO-NET-008 | `ingress_service_port_mismatch` | Ingress | Ingress references service port that doesn't exist |
| KOGARO-NET-009 | `ingress_no_backend_pods` | Ingress | Ingress service has no ready backend pods |

## Usage in API/Logs

When Kogaro detects validation issues, each `ValidationError` includes:

```go
type ValidationError struct {
    ResourceType   string // e.g., "Deployment", "Service"
    ResourceName   string // e.g., "my-app"
    Namespace      string // e.g., "production" 
    ValidationType string // e.g., "missing_resource_limits"
    ErrorCode      string // e.g., "KOGARO-RES-004"
    Message        string // Human-readable description
    Severity       string // "Error", "Warning", "Info"
    // ... additional fields
}
```

## Error Code Benefits

1. **Automated Processing**: Tools can filter, count, and process errors by category or specific type
2. **Metrics & Alerting**: Create dashboards and alerts based on error code patterns
3. **Documentation Linking**: Each code can link to specific remediation documentation
4. **Trend Analysis**: Track which types of issues are most common over time
5. **Tool Integration**: External tools can understand and act on specific error types

## Examples

### Filtering by Category
```bash
# Show only security issues
kubectl logs kogaro-pod | grep "KOGARO-SEC-"

# Show only reference validation issues
kubectl logs kogaro-pod | grep "KOGARO-REF-"
```

### Prometheus Metrics
```promql
# Count of errors by category
kogaro_validation_errors_total{error_code=~"KOGARO-SEC-.*"}

# Specific error type trends
increase(kogaro_validation_errors_total{error_code="KOGARO-RES-003"}[1h])
```

### Automated Remediation
Tools can trigger specific actions based on error codes:
- `KOGARO-REF-*`: Check resource deployment order
- `KOGARO-RES-*`: Review resource quotas and limits
- `KOGARO-SEC-*`: Security policy enforcement
- `KOGARO-NET-*`: Network connectivity troubleshooting