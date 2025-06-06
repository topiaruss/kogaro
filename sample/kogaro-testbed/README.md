# Kogaro Testbed Chart

This Helm chart contains deliberately misconfigured resources designed to test Kogaro's validation capabilities. Each template represents a specific type of configuration error that Kogaro should detect.

## Test Cases Coverage

This testbed validates all 11 error types that Kogaro currently detects:

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

Once deployed, Kogaro should detect exactly **11 validation errors** (one for each test case):

```bash
# Check Kogaro logs for validation errors
kubectl logs -l app.kubernetes.io/name=kogaro -n kogaro-system --tail=100

# Check Prometheus metrics
kubectl port-forward -n kogaro-system svc/kogaro 8080:8080
curl http://localhost:8080/metrics | grep kogaro_validation_errors_total
```

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