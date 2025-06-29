# Kogaro Testbed Chart

This Helm chart contains deliberately misconfigured resources designed to test Kogaro's validation capabilities. Each template represents a specific type of configuration error that Kogaro should detect.

## Test Cases Coverage

This testbed validates all validation errors that Kogaro detects across five validator types:

### Reference Validation (11 error types)

### 1. dangling_ingress_class
- **File**: `ingress-missing-ingressclass.yaml`
- **Test**: Ingress references non-existent IngressClass `nonexistent-class`
- **Expected Error**: `IngressClass 'nonexistent-class' does not exist`

### 2. dangling_service_reference  
- **File**: `ingress-orphan.yaml`
- **Test**: Ingress references non-existent Service `nonexistent-service`
- **Expected Error**: `Service 'nonexistent-service' referenced in Ingress does not exist`

### 3. dangling_configmap_volume
- **File**: `pod-missing-configmap-volume.yaml`
- **Test**: Pod volume references non-existent ConfigMap `missing-configmap`
- **Expected Error**: `ConfigMap 'missing-configmap' referenced in volume does not exist`

### 4. dangling_configmap_envfrom
- **File**: `deployment-missing-configmap.yaml`
- **Test**: Container envFrom references non-existent ConfigMap `nonexistent-config`
- **Expected Error**: `ConfigMap 'nonexistent-config' referenced in envFrom does not exist`

### 5. dangling_secret_volume
- **File**: `pod-missing-secret-volume.yaml`
- **Test**: Pod volume references non-existent Secret `missing-secret`
- **Expected Error**: `Secret 'missing-secret' referenced in volume does not exist`

### 6. dangling_secret_envfrom
- **File**: `pod-missing-secret-envfrom.yaml`
- **Test**: Container envFrom references non-existent Secret `missing-env-secret`
- **Expected Error**: `Secret 'missing-env-secret' referenced in envFrom does not exist`

### 7. dangling_secret_env
- **File**: `pod-missing-secret-env.yaml`
- **Test**: Container env var references non-existent Secret `missing-secret-key`
- **Expected Error**: `Secret 'missing-secret-key' referenced in env does not exist`

### 8. dangling_tls_secret
- **File**: `ingress-missing-tls-secret.yaml`
- **Test**: Ingress TLS references non-existent Secret `missing-tls-secret`
- **Expected Error**: `TLS Secret 'missing-tls-secret' referenced in Ingress does not exist`

### 9. dangling_storage_class
- **File**: `pvc-missing-storageclass.yaml`
- **Test**: PVC references non-existent StorageClass `nonexistent-storage`
- **Expected Error**: `StorageClass 'nonexistent-storage' does not exist`

### 10. dangling_pvc_reference
- **File**: `pod-missing-pvc.yaml`
- **Test**: Pod volume references non-existent PVC `missing-pvc`
- **Expected Error**: `PVC 'missing-pvc' referenced in volume does not exist`

### 11. dangling_service_account
- **File**: `pod-missing-serviceaccount.yaml`
- **Test**: Pod references non-existent ServiceAccount `missing-sa`
- **Expected Error**: `ServiceAccount 'missing-sa' does not exist`

### Resource Limits Validation (6 error types)

### 12. missing_resource_requests
- **File**: `deployment-missing-resources.yaml`, `statefulset-missing-resources.yaml`
- **Test**: Containers without resource requests defined
- **Expected Error**: `Container 'test-container' has no resource requests defined`

### 13. missing_resource_limits
- **File**: `deployment-missing-resources.yaml`, `deployment-insufficient-resources.yaml`, `statefulset-missing-resources.yaml`
- **Test**: Containers without resource limits defined
- **Expected Error**: `Container 'test-container' has no resource limits defined`

### 14. insufficient_cpu_request
- **File**: `deployment-insufficient-resources.yaml` (when using `--min-cpu-request=10m`)
- **Test**: Container CPU request below minimum threshold
- **Expected Error**: `Container 'test-container' CPU request 1m is below minimum 10m`

### 15. insufficient_memory_request
- **File**: `deployment-insufficient-resources.yaml` (when using `--min-memory-request=16Mi`)
- **Test**: Container memory request below minimum threshold
- **Expected Error**: `Container 'test-container' memory request 1Mi is below minimum 16Mi`

### 16. qos_class_issue (BestEffort)
- **File**: `deployment-missing-resources.yaml`, `statefulset-missing-resources.yaml`
- **Test**: Containers with no resource constraints (BestEffort QoS)
- **Expected Error**: `Container 'test-container': BestEffort QoS: no resource constraints, can be killed first under pressure`

### 17. qos_class_issue (Burstable)
- **File**: `deployment-burstable-qos.yaml`
- **Test**: Containers where requests != limits (Burstable QoS)
- **Expected Error**: `Container 'test-container': Burstable QoS: requests != limits, may face throttling under pressure`

### Security Validation (9 error types)

### 18. pod_running_as_root
- **File**: `deployment-root-user.yaml`
- **Test**: Pod SecurityContext specifies runAsUser: 0 (root)
- **Expected Error**: `Pod SecurityContext specifies runAsUser: 0 (root)`

### 19. pod_allows_root_user
- **File**: `deployment-root-user.yaml`
- **Test**: Pod SecurityContext does not enforce runAsNonRoot: true
- **Expected Error**: `Pod SecurityContext does not enforce runAsNonRoot: true`

### 20. container_running_as_root
- **File**: `deployment-root-user.yaml`
- **Test**: Container SecurityContext specifies runAsUser: 0 (root)
- **Expected Error**: `Container 'root-container' (container) SecurityContext specifies runAsUser: 0 (root)`

### 21. container_allows_privilege_escalation
- **File**: `deployment-root-user.yaml`, `deployment-privileged-container.yaml`
- **Test**: Container SecurityContext does not set allowPrivilegeEscalation: false
- **Expected Error**: `Container 'root-container' (container) SecurityContext does not set allowPrivilegeEscalation: false`

### 22. container_privileged_mode
- **File**: `deployment-privileged-container.yaml`
- **Test**: Container SecurityContext specifies privileged: true
- **Expected Error**: `Container 'privileged-container' (container) SecurityContext specifies privileged: true`

### 23. container_writable_root_filesystem
- **File**: `deployment-privileged-container.yaml`
- **Test**: Container SecurityContext does not set readOnlyRootFilesystem: true
- **Expected Error**: `Container 'privileged-container' (container) SecurityContext does not set readOnlyRootFilesystem: true`

### 24. container_additional_capabilities
- **File**: `deployment-privileged-container.yaml`
- **Test**: Container SecurityContext adds capabilities
- **Expected Error**: `Container 'privileged-container' (container) SecurityContext adds capability: NET_ADMIN`

### 25. missing_pod_security_context
- **File**: `deployment-missing-security-context.yaml`
- **Test**: Pod has no SecurityContext defined
- **Expected Error**: `Pod has no SecurityContext defined`

### 26. missing_container_security_context
- **File**: `deployment-missing-security-context.yaml`
- **Test**: Container has no SecurityContext defined
- **Expected Error**: `Container 'no-security-container' (container) has no SecurityContext defined`

### Image Validation (5 validation types when image validation enabled)

### 27. invalid_image_reference
- **File**: `deployment-invalid-image-reference.yaml`
- **Test**: Container with malformed image reference containing invalid characters
- **Expected Error**: `Container 'invalid-ref-container' has invalid image reference format`

### 28. missing_image
- **File**: `deployment-missing-image.yaml`
- **Test**: Container image that doesn't exist in registry
- **Expected Error**: `Container 'missing-image-container' references non-existent image: registry.example.com/nonexistent/image:v1.0.0`

### 29. missing_image_warning
- **File**: `deployment-missing-image.yaml` (when using `--allow-missing-images`)
- **Test**: Missing image treated as warning instead of error
- **Expected Error**: `Container 'missing-image-container' references potentially missing image (warning)`

### 30. architecture_mismatch
- **File**: `deployment-architecture-mismatch.yaml`
- **Test**: Container images with architecture incompatible with cluster nodes (ARM64 on AMD64 cluster)
- **Expected Error**: `Container 'wrong-arch-container' image architecture (arm64) incompatible with cluster nodes (amd64)`

### 31. architecture_mismatch_warning
- **File**: `deployment-architecture-mismatch.yaml` (when using `--allow-architecture-mismatch`)
- **Test**: Architecture mismatch treated as warning instead of error
- **Expected Error**: `Container 'wrong-arch-container' image architecture mismatch (warning)`

### ServiceAccount Security (2 additional validation types when SA validation enabled)

### 32. serviceaccount_cluster_role_binding
- **File**: `serviceaccount-excessive-permissions.yaml`
- **Test**: ServiceAccount has ClusterRoleBinding with cluster-admin role
- **Expected Error**: `ServiceAccount has ClusterRoleBinding 'admin-service-account-binding' with role 'cluster-admin'`

### 33. serviceaccount_excessive_permissions
- **File**: `serviceaccount-excessive-permissions.yaml`
- **Test**: ServiceAccount has potentially excessive RoleBinding with admin role
- **Expected Error**: `ServiceAccount has potentially excessive RoleBinding 'admin-role-binding' with role 'admin'`

### Security Best Practices Example
- **File**: `deployment-secure-example.yaml`
- **Test**: Demonstrates secure configuration with proper SecurityContext settings
- **Expected**: No validation errors (secure configuration example)

### Networking Validation (8 error types)

### 34. service_selector_mismatch
- **File**: `service-no-endpoints.yaml`
- **Test**: Service selector does not match any pods
- **Expected Error**: `Service selector {app:nonexistent-app} does not match any pods`

### 35. service_no_endpoints
- **File**: `service-no-endpoints.yaml`
- **Test**: Service has no ready endpoints despite matching pods
- **Expected Error**: `Service has no ready endpoints despite matching pods`

### 36. service_port_mismatch
- **File**: `service-port-mismatch.yaml`
- **Test**: Service targetPort does not match any container ports in matching pods
- **Expected Error**: `Service port (target: 9999) does not match any container ports in matching pods`

### 37. pod_no_service
- **File**: `pod-unexposed.yaml`
- **Test**: Pod is not exposed by any Service (warning when enabled)
- **Expected Error**: `Pod is not exposed by any Service (consider if this is intentional)`

### 38. network_policy_orphaned
- **File**: `networkpolicy-orphaned.yaml`
- **Test**: NetworkPolicy selector does not match any pods in namespace
- **Expected Error**: `NetworkPolicy selector does not match any pods in namespace`

### 39. missing_network_policy_default_deny
- **File**: `networkpolicy-missing-default-deny.yaml`
- **Test**: Namespace has NetworkPolicies but no default deny policy
- **Expected Error**: `Namespace has NetworkPolicies but no default deny policy`

### 40. ingress_service_missing
- **File**: `ingress-missing-backend-service.yaml`
- **Test**: Ingress references non-existent service
- **Expected Error**: `Ingress references non-existent service 'nonexistent-service'`

### 41. ingress_service_port_mismatch
- **File**: `ingress-port-mismatch.yaml`
- **Test**: Ingress references service port that doesn't exist
- **Expected Error**: `Ingress references service 'ingress-backend-service' port that doesn't exist`

### 42. ingress_no_backend_pods
- **File**: `ingress-no-backend-pods.yaml`
- **Test**: Ingress service has no ready backend pods
- **Expected Error**: `Ingress service 'empty-backend-service' has no ready backend pods`

## Enhanced SharedConfig Test Cases (New)

### 43. SharedConfig Custom Values Test
- **File**: `deployment-custom-config-test.yaml`
- **Test**: Deployment using custom user IDs and resource values different from SharedConfig defaults
- **Expected**: Should validate against configurable thresholds, not hardcoded values
- **Purpose**: Tests that SharedConfig values are properly used for validation recommendations

### 44. Context-Specific Namespace Exclusions Test  
- **File**: `deployment-namespace-exclusion-test.yaml`
- **Test**: Insecure deployment that should be excluded from security validation in system namespaces but still validated for networking
- **Expected**: Security violations ignored in system namespaces, networking issues still detected
- **Purpose**: Tests different namespace exclusion sets for security vs networking validation

### 45. Enhanced ValidationError Details Test
- **File**: `deployment-enhanced-errors-test.yaml`
- **Test**: Multiple validation errors that should generate rich error details with severity levels, remediation hints, and related resources
- **Expected**: Comprehensive error reporting with actionable guidance
- **Purpose**: Tests enhanced ValidationError API with detailed error context

### 46. Production Namespace Patterns Test
- **File**: `deployment-production-patterns-test.yaml`  
- **Test**: Production-like workload without NetworkPolicies in namespace matching production patterns
- **Expected**: Security warnings about missing NetworkPolicies for production workloads
- **Purpose**: Tests SharedConfig production namespace detection patterns

### 47. Batch Workload Patterns Test
- **File**: `job-batch-patterns-test.yaml`
- **Test**: Job, CronJob, and migration pods that should be excluded from "unexposed pod" warnings
- **Expected**: No pod_no_service warnings for batch workloads
- **Purpose**: Tests SharedConfig batch workload exclusion patterns

## Additional Files (Legacy/Other Tests)

- `deployment-missing-volume.yaml` - VolumeMount references missing volume (Kubernetes validation catches this)
- `service-invalid-selector.yaml` - Service with non-matching selector (not currently detected by Kogaro)
- `hpa-mismatch.yaml` - HPA with target mismatch (not currently detected by Kogaro)
- `service-good.yaml` - Valid service and deployment for other tests to reference

## Deployment

To deploy the testbed to a Kubernetes cluster:

```bash
# Create a dedicated namespace
kubectl create namespace kogaro-testbed

# Deploy the testbed chart
helm install kogaro-testbed ./sample/sample/kogaro-testbed -n kogaro-testbed

# Verify resources are created
kubectl get all,ingress,pvc -n kogaro-testbed
```

## Testing with Kogaro

Once deployed, Kogaro should detect **60+ validation errors** across all five validator types:
- **15 reference validation errors** (includes new ServiceAccount and system namespace tests)
- **8 resource limits validation errors** (includes minimum threshold tests)
- **12 security validation errors** (includes enhanced SecurityContext and RBAC tests)
- **5 image validation errors** (requires `--enable-image-validation=true`)
- **10 networking validation errors** (includes system namespace exclusion tests) 
- **10+ enhanced SharedConfig validation errors** (tests for configurable thresholds and exclusion patterns)

```bash
# Check Kogaro logs for validation errors
kubectl logs -l app.kubernetes.io/name=kogaro -n kogaro-system --tail=100

# To test image validation, redeploy Kogaro with image validation enabled
helm upgrade kogaro charts/kogaro --namespace kogaro-system \
  --set validation.enableImageValidation=true

# Check Prometheus metrics
kubectl port-forward -n kogaro-system svc/kogaro 8080:8080
curl http://localhost:8080/metrics | grep kogaro_validation_errors_total
```

## Coverage Measurement

The testbed includes comprehensive coverage measurement capabilities for marketing and development purposes.

### Automated Coverage Measurement

```bash
# Run the automated coverage measurement script
./scripts/measure-testbed-coverage.sh

# Custom namespace and output
NAMESPACE=my-testbed COVERAGE_OUTPUT=my-coverage ./scripts/measure-testbed-coverage.sh

# Keep testbed deployed after measurement
KEEP_TESTBED=true ./scripts/measure-testbed-coverage.sh
```

The script will:
1. **Build Kogaro with coverage instrumentation**
2. **Deploy the comprehensive testbed**
3. **Run Kogaro validation with coverage tracking**
4. **Generate detailed coverage reports** (HTML and text)
5. **Create marketing-ready coverage documentation**
6. **Analyze coverage gaps** and improvement opportunities

### Coverage Reports Generated

- **`testbed-coverage.html`** - Interactive HTML coverage report
- **`testbed-coverage-functions.txt`** - Function-level coverage statistics  
- **`testbed-coverage-marketing-report.md`** - Marketing-ready coverage summary

### Coverage Targets

The testbed is designed to achieve:
- **95%+ coverage** of validation logic functions
- **100% coverage** of critical security validation paths
- **100% coverage** of reference validation scenarios
- **90%+ coverage** of networking validation features
- **100% coverage** of SharedConfig functionality

### Marketing Value

The coverage measurement demonstrates:
- **Comprehensive validation capabilities** - 55+ distinct validation scenarios
- **Production-ready reliability** - High test coverage ensures robustness
- **Configurable validation rules** - Tests all SharedConfig patterns
- **Enterprise security features** - Complete security validation coverage

## Expected Prometheus Metrics

After Kogaro scans the testbed namespace, you should see metrics like:

```
kogaro_validation_errors_total{resource_type="Ingress",validation_type="dangling_ingress_class",namespace="kogaro-testbed"} 1
kogaro_validation_errors_total{resource_type="Ingress",validation_type="dangling_service_reference",namespace="kogaro-testbed"} 1
kogaro_validation_errors_total{resource_type="Pod",validation_type="dangling_configmap_volume",namespace="kogaro-testbed"} 1
kogaro_validation_errors_total{resource_type="Pod",validation_type="dangling_configmap_envfrom",namespace="kogaro-testbed"} 1
kogaro_validation_errors_total{resource_type="Pod",validation_type="dangling_secret_volume",namespace="kogaro-testbed"} 1
kogaro_validation_errors_total{resource_type="Pod",validation_type="dangling_secret_envfrom",namespace="kogaro-testbed"} 1
kogaro_validation_errors_total{resource_type="Pod",validation_type="dangling_secret_env",namespace="kogaro-testbed"} 1
kogaro_validation_errors_total{resource_type="Ingress",validation_type="dangling_tls_secret",namespace="kogaro-testbed"} 1
kogaro_validation_errors_total{resource_type="PersistentVolumeClaim",validation_type="dangling_storage_class",namespace="kogaro-testbed"} 1
kogaro_validation_errors_total{resource_type="Pod",validation_type="dangling_pvc_reference",namespace="kogaro-testbed"} 1
kogaro_validation_errors_total{resource_type="Pod",validation_type="dangling_service_account",namespace="kogaro-testbed"} 1
```

## Cleanup

```bash
# Remove the testbed
helm uninstall kogaro-testbed -n kogaro-testbed
kubectl delete namespace kogaro-testbed
```

## Safety Notes

- All containers use `busybox:1.35` or `nginx:1.21-alpine` with minimal resource usage
- Pods are designed to sleep rather than perform active work
- All misconfigurations are designed to be non-functional but safe
- Resources are clearly labeled for easy identification and cleanup