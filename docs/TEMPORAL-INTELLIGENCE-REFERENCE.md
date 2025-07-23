# Temporal Intelligence Quick Reference

This reference covers Kogaro's **Temporal Intelligence** features that automatically classify validation errors by age and severity to reduce alert fatigue.

## Temporal States

| State | Age Range | Description | Alert Priority |
|-------|-----------|-------------|----------------|
| **New** | < 1 hour | Requires immediate attention | 🔴 Critical |
| **Recent** | 1-24 hours | Monitor for resolution | 🟡 Warning |
| **Stable** | > 24 hours | Pattern analysis needed | 🔵 Info |
| **Resolved** | N/A | Recently fixed | ✅ Success |

## Workload Categories

| Category | Namespaces | Description | Alert Routing |
|----------|------------|-------------|---------------|
| **Application** | `default`, `production`, `staging`, `app-*` | User-facing services | PagerDuty/Slack |
| **Infrastructure** | `kube-system`, `monitoring`, `ingress-nginx` | System services | Slack only |

## Key Metrics

### Core Metrics
```promql
# Total validation errors
kogaro_validation_errors_total

# Error age in hours
kogaro_validation_age_hours

# State changes
kogaro_validation_state_changes_total

# Resolved errors
kogaro_validation_resolved_total
```

### Useful Queries
```promql
# New critical application errors
kogaro_validation_errors_total{severity="error", workload_category="application"}
  and kogaro_validation_age_hours < 1

# Long-standing issues
kogaro_validation_age_hours > 24

# Resolution rate
rate(kogaro_validation_resolved_total[1h])
```

## Alert Rules

### Critical New Issues
```yaml
# Triggers for new application errors
expr: |
  increase(kogaro_validation_errors_total{severity="error", workload_category="application"}[1h]) > 0
  and kogaro_validation_age_hours{severity="error", workload_category="application"} < 1
```

### Infrastructure Issues
```yaml
# Triggers for new infrastructure errors
expr: |
  increase(kogaro_validation_errors_total{workload_category="infrastructure"}[1h]) > 0
  and kogaro_validation_age_hours{workload_category="infrastructure"} < 1
```

### Recent Issues
```yaml
# Issues persisting 1-24 hours
expr: |
  kogaro_validation_age_hours >= 1 
  and kogaro_validation_age_hours < 24
```

## Dashboard Panels

### URGENT: New Issues
- **Query**: `kogaro_validation_age_hours < 1`
- **Purpose**: Immediate attention required
- **Action**: Page on-call engineer

### Recent Issues  
- **Query**: `kogaro_validation_age_hours >= 1 and kogaro_validation_age_hours < 24`
- **Purpose**: Monitor resolution progress
- **Action**: Follow up with team lead

### Stable Patterns
- **Query**: `kogaro_validation_age_hours >= 24`
- **Purpose**: Identify systemic issues
- **Action**: Schedule investigation

### Resolved Today
- **Query**: `increase(kogaro_validation_resolved_total[24h])`
- **Purpose**: Track success rates
- **Action**: Celebrate improvements

## Configuration Examples

### Alertmanager Routing
```yaml
route:
  routes:
    - match:
        severity: critical
        workload_category: application
      receiver: 'pager-duty-critical'
    - match:
        severity: warning
        workload_category: infrastructure  
      receiver: 'slack-infrastructure'
    - match:
        severity: info
      receiver: 'slack-notifications'
```

### Grafana Variables
```json
{
  "name": "namespace",
  "type": "query",
  "query": "label_values(kogaro_validation_errors_total, namespace)"
}
```

### Dashboard Refresh
- **New Issues**: 30s (real-time)
- **Recent Issues**: 1m (near real-time)
- **Stable Patterns**: 5m (trend analysis)
- **Resolved**: 1m (success tracking)

## Best Practices

### 1. Alert Tuning
- Start with conservative thresholds
- Use temporal states to reduce noise
- Route by business impact, not just severity

### 2. Dashboard Usage
- Focus on "New Issues" for immediate response
- Use "Stable Patterns" for weekly reviews
- Monitor "Resolved Today" for team metrics

### 3. Team Workflows
- **New Issues**: Immediate investigation
- **Recent Issues**: Daily standup review
- **Stable Patterns**: Weekly retrospective
- **Resolved**: Monthly success review

## Troubleshooting

### Metrics Not Updating
```bash
# Check state tracker
kubectl logs -n kogaro-system deployment/kogaro | grep "state"

# Check Prometheus targets
kubectl port-forward svc/prometheus-operated 9090:9090 -n monitoring
```

### Alerts Not Firing
```bash
# Check alert rules
kubectl get prometheusrule -n monitoring kogaro-temporal-rules

# Test alert expression
# Visit Prometheus UI and test the expression
```

### Dashboard Issues
```bash
# Check Grafana data source
kubectl port-forward svc/grafana 3000:3000 -n monitoring
# Visit http://localhost:3000/datasources
```

## Quick Commands

### Check Current Issues
```bash
# New issues
kubectl port-forward svc/prometheus-operated 9090:9090 -n monitoring
# Then query: kogaro_validation_age_hours < 1

# Stable patterns  
# Query: kogaro_validation_age_hours > 24
```

### Monitor Resolution
```bash
# Resolution rate
# Query: rate(kogaro_validation_resolved_total[1h])

# Top error types
# Query: topk(5, sum by (validation_type) (kogaro_validation_errors_total))
```

### Export Dashboard
```bash
# Get dashboard JSON
kubectl get configmap -n monitoring kogaro-temporal-dashboard -o jsonpath='{.data.dashboard\.json}'
```

## Integration Examples

### Slack Notifications
```yaml
slack_configs:
  - channel: '#kogaro-alerts'
    title: '{{ if eq .GroupLabels.severity "critical" }}🚨{{ else }}⚠️{{ end }} {{ .GroupLabels.validation_type }}'
    text: |
      *Namespace:* {{ .GroupLabels.namespace }}
      *Resource:* {{ .GroupLabels.resource_name }}
      *Age:* {{ $value }} hours
      *State:* {{ .GroupLabels.state }}
```

### PagerDuty Integration
```yaml
pagerduty_configs:
  - routing_key: 'YOUR_ROUTING_KEY'
    description: 'Kogaro {{ .GroupLabels.severity }}: {{ .GroupLabels.validation_type }} in {{ .GroupLabels.namespace }}'
    severity: '{{ if eq .GroupLabels.severity "critical" }}critical{{ else }}warning{{ end }}'
```

For detailed configuration, see the [Monitoring Guide](MONITORING-GUIDE.md). 