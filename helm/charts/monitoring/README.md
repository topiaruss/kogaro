# Kogaro Monitoring Stack

This Helm chart deploys a production-ready monitoring stack for Kogaro with Prometheus, Grafana, and Alertmanager.

## 🔒 Security-First Deployment

**IMPORTANT**: This repository is public. Never commit passwords or sensitive credentials to version control.

### Quick Start (Recommended)

1. **Set up environment variables**:
   ```bash
   cp env.example .env
   # Edit .env with your actual passwords and domain
   ```

2. **Deploy securely**:
   ```bash
   ./deploy.sh monitoring
   ```

### Manual Deployment (Not Recommended for Production)

If you must deploy manually, ensure you change the default passwords in `values.yaml`:

```bash
helm upgrade --install monitoring . -n monitoring
```

## Environment Variables

Copy `env.example` to `.env` and configure:

| Variable | Description | Default |
|----------|-------------|---------|
| `DOMAIN` | Your domain for ingress hosts | `ogaro.com` |
| `GRAFANA_ADMIN_USER` | Grafana admin username | `admin` |
| `GRAFANA_ADMIN_PASSWORD` | **REQUIRED** - Grafana admin password | - |
| `PROMETHEUS_BASIC_AUTH_USER` | Prometheus basic auth username | `prometheus` |
| `PROMETHEUS_BASIC_AUTH_PASSWORD` | **REQUIRED** - Prometheus basic auth password | - |
| `ALERTMANAGER_BASIC_AUTH_USER` | Alertmanager basic auth username | `alertmanager` |
| `ALERTMANAGER_BASIC_AUTH_PASSWORD` | **REQUIRED** - Alertmanager basic auth password | - |
| `TLS_SECRET_NAME` | TLS certificate secret name | `kogaro-monitoring-tls` |

## Components

### Prometheus
- **URL**: `https://prometheus.{DOMAIN}`
- **Authentication**: Basic auth
- **Storage**: 5Gi PVC with 15-day retention
- **Resources**: Configurable CPU/memory limits

### Grafana
- **URL**: `https://grafana.{DOMAIN}`
- **Authentication**: Admin credentials from environment
- **Dashboards**: Pre-configured Kogaro Temporal Intelligence dashboards
- **Resources**: Configurable CPU/memory limits

### Alertmanager
- **URL**: `https://alertmanager.{DOMAIN}`
- **Authentication**: Basic auth
- **Retention**: 120 hours
- **Resources**: Optimized for monitoring workloads

## Security Features

- **TLS/SSL**: All ingress endpoints use HTTPS
- **Security Headers**: HSTS, X-Frame-Options, XSS Protection
- **Basic Authentication**: Prometheus and Alertmanager protected
- **Environment Variables**: No secrets in version control
- **Resource Limits**: Prevent resource exhaustion

## Monitoring Features

### Temporal Intelligence
- **Error Age Classification**: New, Recent, Stable, Resolved
- **Workload Categories**: Application vs Infrastructure
- **Trend Analysis**: Historical validation patterns
- **Alerting**: Configurable thresholds for error trends

### Dashboards
- **Kogaro Overview**: High-level validation metrics
- **Temporal Intelligence**: Error age and category analysis
- **Validation Details**: Per-validator breakdowns
- **Cluster Health**: Resource usage and performance

## Troubleshooting

### Check Deployment Status
```bash
kubectl get pods -n monitoring
helm list -n monitoring
```

### View Logs
```bash
kubectl logs -n monitoring deployment/monitoring-prometheus
kubectl logs -n monitoring deployment/monitoring-grafana
kubectl logs -n monitoring deployment/monitoring-alertmanager
```

### Access Credentials
```bash
# Grafana admin password
kubectl get secret monitoring-grafana -n monitoring -o jsonpath='{.data.admin-password}' | base64 -d

# Basic auth passwords (if using manual deployment)
kubectl get secret prometheus-basic-auth -n monitoring -o jsonpath='{.data.auth}' | base64 -d
kubectl get secret alertmanager-basic-auth -n monitoring -o jsonpath='{.data.auth}' | base64 -d
```

## Upgrading

1. Update your `.env` file if needed
2. Run the deployment script:
   ```bash
   ./deploy.sh monitoring
   ```

## Uninstalling

```bash
helm uninstall monitoring -n monitoring
kubectl delete namespace monitoring
```

## Contributing

When adding new configuration options:
1. Add environment variable support in `generate-values.sh`
2. Update `env.example` with the new variable
3. Document the variable in this README
4. Ensure the variable has a secure default