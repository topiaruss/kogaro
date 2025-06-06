# Kogaro Monitoring Chart

This Helm chart provides a complete monitoring stack for Kogaro development and testing environments. It includes Prometheus, Grafana, and Alertmanager with pre-configured settings.

## Overview

The monitoring stack includes:
- Prometheus for metrics collection
- Grafana for visualization
- Alertmanager for alert handling
- Node Exporter for system metrics
- Kube State Metrics for Kubernetes metrics

## Prerequisites

1. Create a `.env` file in the chart directory with the following variables:
```bash
MONITORING_USERNAME=your-username
MONITORING_PASSWORD=your-password
```

## Installation

### Quick Start

The easiest way to deploy the monitoring stack is using the provided setup script:

```bash
# Make the script executable
chmod +x setup-test-monitoring.sh

# Run the setup script
./setup-test-monitoring.sh
```

The script will:
1. Create the monitoring namespace
2. Set up basic authentication secrets
3. Install the monitoring stack
4. Wait for all components to be ready

### Manual Installation

If you prefer to install manually:

```bash
# Create the monitoring namespace
kubectl create namespace monitoring

# Create basic auth secrets
echo "${MONITORING_USERNAME}:$(openssl passwd -apr1 '${MONITORING_PASSWORD}')" > auth
kubectl create secret generic prometheus-basic-auth --from-file=auth -n monitoring
kubectl create secret generic alertmanager-basic-auth --from-file=auth -n monitoring
rm auth

# Install the monitoring stack
helm install monitoring . -n monitoring
```

## Accessing the Services

After installation, you can access:
- Prometheus: https://prometheus.ogaro.com
- Alertmanager: https://alertmanager.ogaro.com
- Grafana: https://grafana.ogaro.com

Both Prometheus and Alertmanager require authentication using the credentials from your `.env` file.

Note: Browsers will cache these credentials. To test the authentication:
- Use an incognito/private window
- Clear your browser cache
- Or use a different browser

## Security Considerations

### Basic Authentication
The monitoring stack uses basic authentication for Prometheus and Alertmanager. The credentials are managed through:
1. A `.env` file containing the username and password
2. Kubernetes secrets created during installation

If you need to update the credentials:
1. Update the `.env` file
2. Delete the existing secrets:
   ```bash
   kubectl delete secret prometheus-basic-auth alertmanager-basic-auth -n monitoring
   ```
3. Recreate the secrets using the setup script or manual commands

### TLS Configuration
All components are configured to use TLS by default with certificates managed by cert-manager.

## Troubleshooting

### 503 Errors
If you see 503 errors when accessing Prometheus or Alertmanager:
1. Check that the basic auth secrets exist:
   ```bash
   kubectl get secret -n monitoring | grep -E 'prometheus-basic-auth|alertmanager-basic-auth'
   ```
2. Verify the `.env` file exists and contains valid credentials
3. Recreate the secrets if needed

### Pod Issues
Check pod status and logs:
```bash
kubectl get pods -n monitoring
kubectl logs -n monitoring <pod-name>
```

## Configuration

### Global Settings

```yaml
global:
  domain: "ogaro.com"
  tls:
    enabled: true
    secretName: "kogaro-monitoring-tls"
    hosts:
      - alertmanager.ogaro.com
      - grafana.ogaro.com
      - prometheus.ogaro.com
  ingress:
    annotations:
      cert-manager.io/cluster-issuer: letsencrypt-prod
    className: ingress-nginx
```

### Component-Specific Settings

Each component (Prometheus, Grafana, Alertmanager) can be configured independently in `values.yaml`. See the file for detailed configuration options.

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

## Support

For issues or questions, please open an issue in the Kogaro repository. 