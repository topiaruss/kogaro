# Kogaro Monitoring Chart

This Helm chart provides a complete monitoring stack for Kogaro development and testing environments. It includes Prometheus, Grafana, and Alertmanager with pre-configured settings.

## Overview

The monitoring stack includes:
- Prometheus for metrics collection
- Grafana for visualization
- Alertmanager for alert handling
- Node Exporter for system metrics
- Kube State Metrics for Kubernetes metrics

## Usage

### Development/Testing Environment

To deploy the complete monitoring stack:

```bash
# Add the Prometheus Helm repository
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

# Create the monitoring namespace
kubectl create namespace monitoring

# Create basic auth secrets (required before installation)
echo "admin:$(openssl passwd -apr1 'your-password')" > auth
kubectl create secret generic prometheus-basic-auth --from-file=auth -n monitoring
kubectl create secret generic alertmanager-basic-auth --from-file=auth -n monitoring
rm auth

# Install the monitoring stack
helm install monitoring ./charts/monitoring
```

### Important Notes
- The basic auth secrets (`prometheus-basic-auth` and `alertmanager-basic-auth`) must be created before installing the chart
- Without these secrets, you will see 503 errors when trying to access Prometheus and Alertmanager
- If you see 503 errors after installation, check that the secrets exist:
  ```bash
  kubectl get secret -n monitoring | grep -E 'prometheus-basic-auth|alertmanager-basic-auth'
  ```

### Production Integration

For production environments, it's recommended to use your existing Prometheus setup. Here's how to integrate Kogaro with your existing monitoring:

1. **ServiceMonitor Configuration**
   Add the following ServiceMonitor to your Prometheus configuration:

   ```yaml
   apiVersion: monitoring.coreos.com/v1
   kind: ServiceMonitor
   metadata:
     name: kogaro
     namespace: monitoring
   spec:
     selector:
       matchLabels:
         app: kogaro
     endpoints:
     - port: metrics
       interval: 15s
       path: /metrics
   ```

2. **Grafana Dashboard**
   Import the Kogaro dashboard into your Grafana instance:
   - Dashboard ID: [TBD]
   - URL: [TBD]

3. **Alertmanager Rules**
   Add the following PrometheusRule to your configuration:

   ```yaml
   apiVersion: monitoring.coreos.com/v1
   kind: PrometheusRule
   metadata:
     name: kogaro-alerts
     namespace: monitoring
   spec:
     groups:
     - name: kogaro
       rules:
       - alert: KogaroValidationErrors
         expr: kogaro_validation_errors_total > 0
         for: 5m
         labels:
           severity: warning
         annotations:
           summary: "Kogaro validation errors detected"
           description: "Kogaro has detected {{ $value }} validation errors"
   ```

## Configuration

### Global Settings

```yaml
global:
  domain: "your-domain.com"
  tls:
    enabled: true
    secretName: "tls-secret"
```

### Component-Specific Settings

Each component (Prometheus, Grafana, Alertmanager) can be configured independently:

```yaml
prometheus:
  enabled: true
  ingress:
    enabled: true
    hostname: "prometheus.your-domain.com"

grafana:
  enabled: true
  ingress:
    enabled: true
    hostname: "grafana.your-domain.com"

alertmanager:
  enabled: true
  ingress:
    enabled: true
    hostname: "alertmanager.your-domain.com"
```

## Security Considerations

### Basic Authentication Setup
The monitoring stack uses basic authentication for Prometheus and Alertmanager. These secrets need to be created manually before accessing the services.

#### Required Secrets
You need to create two basic auth secrets:
1. `prometheus-basic-auth` for Prometheus
2. `alertmanager-basic-auth` for Alertmanager

To create these secrets, run:
```bash
# Create the auth file with your desired credentials
echo "admin:$(openssl passwd -apr1 'your-password')" > auth

# Create the secrets
kubectl create secret generic prometheus-basic-auth --from-file=auth -n monitoring
kubectl create secret generic alertmanager-basic-auth --from-file=auth -n monitoring

# Clean up
rm auth
```

#### Accessing the Services
After creating the secrets, you can access:
- Prometheus: https://prometheus.ogaro.com
- Alertmanager: https://alertmanager.ogaro.com

Both services will prompt for authentication with:
- Username: `admin`
- Password: `your-password`

Note: Browsers will cache these credentials. To test the authentication:
- Use an incognito/private window
- Clear your browser cache
- Or use a different browser

### TLS Configuration
1. **TLS**: All components are configured to use TLS by default
2. **Network Policies**: Consider adding network policies to restrict access

## Troubleshooting

1. **Metrics Not Showing**
   - Verify the ServiceMonitor is properly configured
   - Check that the metrics port is correctly exposed
   - Ensure Prometheus has the necessary RBAC permissions

2. **Alerts Not Firing**
   - Verify the PrometheusRule is properly configured
   - Check Alertmanager configuration
   - Ensure the alert expressions are correct

## Support

For issues or questions, please open an issue in the Kogaro repository. 