# Kogaro Monitoring Guide

This guide covers the complete monitoring setup for Kogaro, including Prometheus metrics, Grafana dashboards, and the new **Temporal Intelligence** features that help reduce alert fatigue and prioritize validation issues.

## Overview

Kogaro provides rich observability through:
- **Prometheus metrics** for monitoring validation trends
- **Temporal Intelligence** for classifying validation errors by age and severity
- **Grafana dashboards** for visualization and alerting
- **Alertmanager rules** for intelligent notification routing

## Quick Start

### 1. Deploy Monitoring Stack (Secure)

**IMPORTANT**: This repository is public. Use the secure deployment approach to avoid committing passwords.

```bash
# Navigate to the monitoring chart directory
cd helm/charts/monitoring

# Set up environment variables
cp env.example .env
# Edit .env with your actual passwords and domain

# Deploy securely
./deploy.sh monitoring
```

### Alternative: Manual Deployment

If you must deploy manually, ensure you change the default passwords:

```bash
# Install the monitoring stack with Kogaro-specific configurations
helm install monitoring helm/charts/monitoring \
  --namespace monitoring \
  --create-namespace \
  --values helm/charts/monitoring/values.yaml
```

### 2. Deploy Kogaro with Monitoring

```bash
# Install Kogaro with monitoring enabled
helm install kogaro helm/charts/kogaro \
  --namespace kogaro-system \
  --create-namespace \
  --set metrics.enabled=true \
  --set metrics.serviceMonitor.enabled=true
```

### 3. Access Dashboards

- **Grafana**: https://grafana.ogaro.com (credentials from your .env file)
- **Prometheus**: https://prometheus.ogaro.com (basic auth)
- **Alertmanager**: https://alertmanager.ogaro.com (basic auth)

**Note**: If you used the secure deployment, the credentials are stored in your `.env` file and not committed to version control.

## Prometheus Metrics

### Core Metrics

Kogaro exposes the following Prometheus metrics:

#### Validation Errors (`kogaro_validation_errors_total`)
```promql
# Total validation errors by type
kogaro_validation_errors_total{validation_type="dangling_service_reference"}

# Errors by severity
kogaro_validation_errors_total{severity="error"}
kogaro_validation_errors_total{severity="warning"}

# Errors by workload category
kogaro_validation_errors_total{workload_category="application"}
kogaro_validation_errors_total{workload_category="infrastructure"}
```

#### Temporal Intelligence Metrics

**First Seen Timestamp** (`kogaro_validation_first_seen_timestamp`)
```promql
# When validation errors were first detected
kogaro_validation_first_seen_timestamp{namespace="production"}
```

**Last Seen Timestamp** (`kogaro_validation_last_seen_timestamp`)
```promql
# When validation errors were last observed
kogaro_validation_last_seen_timestamp{validation_type="missing_configmap"}
```

**Error Age** (`kogaro_validation_age_hours`)
```promql
# Age of validation errors in hours
kogaro_validation_age_hours{namespace="production"}
```

**State Changes** (`kogaro_validation_state_changes_total`)
```promql
# Number of state transitions for validation errors
kogaro_validation_state_changes_total{state="new"}
```

**Resolved Errors** (`kogaro_validation_resolved_total`)
```promql
# Number of resolved validation errors
kogaro_validation_resolved_total{namespace="production"}
```

### Temporal State Classification

Kogaro automatically classifies validation errors into temporal states:

- **New** (< 1 hour): Requires immediate attention
- **Recent** (1-24 hours): Monitor for resolution
- **Stable** (> 24 hours): Pattern analysis, potential systemic issues
- **Resolved**: Tracked for resolution patterns

### Workload Classification

Errors are automatically categorized by workload type:

- **Application**: User-facing services (default, production, staging namespaces)
- **Infrastructure**: System services (kube-system, monitoring, ingress-nginx namespaces)

## Grafana Dashboards

### Kogaro Temporal Intelligence Dashboard

The main dashboard provides comprehensive temporal analysis:

#### Key Panels

1. **URGENT: New Issues**
   - Critical new validation errors requiring immediate attention
   - Filtered by severity and workload category

2. **Recent Issues**
   - Issues persisting 1-24 hours
   - Helps identify problems that aren't resolving quickly

3. **Stable Patterns**
   - Long-standing issues (>24 hours)
   - Useful for identifying systemic configuration problems

4. **Resolved Today**
   - Recently resolved issues
   - Tracks resolution success rates

5. **Temporal State Timeline**
   - Historical view of issue state transitions
   - Helps understand validation error lifecycle

6. **New Issues by Namespace**
   - Namespace-specific new issues
   - Helps prioritize by business impact

7. **Issue Age Distribution**
   - Age analysis of validation errors
   - Identifies clusters with chronic vs. acute issues

8. **Recent Issues Details**
   - Detailed view of recent issues with full context
   - Includes error codes and remediation hints

### Dashboard Configuration

#### Import the Dashboard

1. Navigate to Grafana (https://grafana.ogaro.com)
2. Go to **Dashboards** → **Import**
3. Copy the JSON from `helm/charts/monitoring/templates/kogaro-temporal-dashboard.yaml`
4. Set the data source to your Prometheus instance
5. Import the dashboard

#### Customize for Your Environment

```yaml
# Example dashboard customization
dashboard:
  title: "Kogaro Validation Intelligence - Production"
  tags: ["kogaro", "validation", "production"]
  refresh: "30s"
  time:
    from: "now-24h"
    to: "now"
```

## Alerting Rules

### Pre-configured Alert Rules

Kogaro includes intelligent alerting rules that reduce noise:

#### Critical New Issues
```yaml
# KogaroValidationNewCritical
# Triggers for new application issues requiring immediate attention
expr: |
  increase(kogaro_validation_errors_total{severity="error", workload_category="application"}[1h]) > 0
  and kogaro_validation_age_hours{severity="error", workload_category="application"} < 1
```

#### Infrastructure Issues
```yaml
# KogaroValidationNewInfrastructure  
# Triggers for new infrastructure issues
expr: |
  increase(kogaro_validation_errors_total{workload_category="infrastructure"}[1h]) > 0
  and kogaro_validation_age_hours{workload_category="infrastructure"} < 1
```

#### Recent Issues
```yaml
# KogaroValidationRecent
# Issues persisting 1-24 hours
expr: |
  kogaro_validation_age_hours >= 1 
  and kogaro_validation_age_hours < 24
```

#### Stable Patterns
```yaml
# KogaroValidationStable
# Long-standing issues for pattern analysis
expr: |
  kogaro_validation_age_hours >= 24
```

### Alert Configuration

#### Alert Severity Levels

- **Critical**: New application errors (< 1 hour)
- **Warning**: New infrastructure errors (< 1 hour)
- **Info**: Recent issues (1-24 hours)
- **Debug**: Stable patterns (> 24 hours)

#### Alert Annotations

```yaml
annotations:
  summary: "{{ $labels.severity | title }} validation error in {{ $labels.namespace }}"
  description: |
    {{ $labels.validation_type }} error detected in {{ $labels.resource_type }} {{ $labels.resource_name }}
    Namespace: {{ $labels.namespace }}
    Age: {{ $value }} hours
    Error Code: {{ $labels.error_code }}
  runbook_url: "https://kogaro.com/docs/error-codes/{{ $labels.error_code }}"
```

## Alertmanager Configuration

### Routing Strategy

Configure Alertmanager to route alerts based on severity and workload:

```yaml
route:
  group_by: ['alertname', 'namespace', 'workload_category']
  group_wait: 30s
  group_interval: 5m
  repeat_interval: 4h
  receiver: 'slack-notifications'
  routes:
    - match:
        severity: critical
        workload_category: application
      receiver: 'pager-duty-critical'
      continue: true
    - match:
        severity: warning
        workload_category: infrastructure
      receiver: 'slack-infrastructure'
      continue: true
    - match:
        severity: info
      receiver: 'slack-notifications'
```

### Notification Channels

#### Slack Integration
```yaml
receivers:
  - name: 'slack-notifications'
    slack_configs:
      - api_url: 'https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK'
        channel: '#kogaro-alerts'
        title: '{{ template "slack.kogaro.title" . }}'
        text: '{{ template "slack.kogaro.text" . }}'
        actions:
          - type: button
            text: 'View Dashboard'
            url: '{{ template "slack.kogaro.dashboardURL" . }}'
```

#### PagerDuty Integration
```yaml
receivers:
  - name: 'pager-duty-critical'
    pagerduty_configs:
      - routing_key: 'YOUR_PAGERDUTY_ROUTING_KEY'
        description: '{{ template "pagerduty.kogaro.description" . }}'
        severity: '{{ if eq .GroupLabels.severity "critical" }}critical{{ else }}warning{{ end }}'
```

## Production Configuration

### High Availability Setup

```yaml
# values.yaml
replicaCount: 3
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

resources:
  limits:
    cpu: 500m
    memory: 512Mi
  requests:
    cpu: 100m
    memory: 256Mi
```

### Monitoring Stack Resources

```yaml
# monitoring/values.yaml
prometheus:
  prometheusSpec:
    resources:
      limits:
        cpu: 1
        memory: 2Gi
      requests:
        cpu: 500m
        memory: 1Gi
    retention: 30d
    storageSpec:
      volumeClaimTemplate:
        spec:
          accessModes:
            - ReadWriteOnce
          resources:
            requests:
              storage: 50Gi

grafana:
  resources:
    limits:
      cpu: 200m
      memory: 512Mi
    requests:
      cpu: 100m
      memory: 256Mi
```

## Troubleshooting

### Common Issues

#### Metrics Not Appearing
```bash
# Check if ServiceMonitor is working
kubectl get servicemonitor -n monitoring

# Check Prometheus targets
kubectl port-forward svc/prometheus-operated 9090:9090 -n monitoring
# Then visit http://localhost:9090/targets
```

#### Dashboard Not Loading
```bash
# Check Grafana data source
kubectl port-forward svc/grafana 3000:3000 -n monitoring
# Then visit http://localhost:3000/datasources
```

#### Alerts Not Firing
```bash
# Check alert rules
kubectl get prometheusrule -n monitoring

# Check Alertmanager configuration
kubectl port-forward svc/alertmanager-operated 9093:9093 -n monitoring
# Then visit http://localhost:9093
```

### Useful Queries

#### Validation Error Trends
```promql
# Error rate over time
rate(kogaro_validation_errors_total[5m])

# Top error types
topk(5, sum by (validation_type) (kogaro_validation_errors_total))
```

#### Temporal Analysis
```promql
# New vs stable errors
sum by (state) (kogaro_validation_state_changes_total)

# Resolution rate
rate(kogaro_validation_resolved_total[1h])
```

#### Workload Impact
```promql
# Errors by workload category
sum by (workload_category) (kogaro_validation_errors_total)

# Critical application errors
kogaro_validation_errors_total{severity="error", workload_category="application"}
```

## Best Practices

### 1. Alert Tuning
- Start with conservative thresholds
- Use temporal intelligence to reduce noise
- Route alerts based on business impact

### 2. Dashboard Organization
- Create environment-specific dashboards
- Use variables for namespace filtering
- Set appropriate refresh intervals

### 3. Metric Retention
- Configure appropriate retention periods
- Monitor storage usage
- Archive historical data if needed

### 4. Security
- **Environment Variables**: Never commit passwords to version control
- **Basic Authentication**: Use strong passwords for Prometheus and Alertmanager
- **TLS/SSL**: All endpoints use HTTPS with security headers
- **RBAC**: Use Kubernetes RBAC for dashboard access
- **Secret Management**: Store credentials in Kubernetes secrets, not in Helm values

## Security Best Practices

### 🔒 Credential Management

**Never commit passwords to version control!** This repository is public, so use the secure deployment approach:

1. **Use Environment Variables**:
   ```bash
   cp helm/charts/monitoring/env.example .env
   # Edit .env with your actual passwords
   ```

2. **Secure Deployment Script**:
   ```bash
   cd helm/charts/monitoring
   ./deploy.sh monitoring
   ```

3. **Password Requirements**:
   - Use strong, unique passwords for each service
   - Consider using a password manager
   - Rotate passwords regularly

### 🔐 Access Control

1. **Grafana Access**:
   - Use admin credentials from environment variables
   - Consider setting up LDAP/SSO for production
   - Implement user roles and permissions

2. **Prometheus/Alertmanager**:
   - Basic authentication with strong passwords
   - Consider IP whitelisting for production
   - Monitor access logs

3. **Kubernetes RBAC**:
   - Limit access to monitoring namespace
   - Use service accounts with minimal permissions
   - Audit access regularly

### 🛡️ Network Security

1. **TLS Configuration**:
   - All ingress endpoints use HTTPS
   - Security headers enabled (HSTS, X-Frame-Options, etc.)
   - Valid SSL certificates required

2. **Network Policies**:
   - Restrict pod-to-pod communication
   - Limit external access to monitoring endpoints
   - Use ingress controllers with security features

### 📊 Monitoring Security

1. **Audit Logging**:
   - Monitor access to sensitive endpoints
   - Log authentication attempts
   - Track configuration changes

2. **Alerting on Security Events**:
   - Failed login attempts
   - Unusual access patterns
   - Configuration drift

## Next Steps

With monitoring configured, you can:

1. **Analyze Patterns**: Use temporal intelligence to identify systemic issues
2. **Optimize Alerts**: Fine-tune alerting based on your team's response patterns
3. **Track Improvements**: Monitor resolution rates and validation error trends
4. **Scale Monitoring**: Add custom dashboards for specific use cases

For advanced configuration, see the [Deployment Guide](DEPLOYMENT-GUIDE.md) and [Error Codes Reference](ERROR-CODES.md). 