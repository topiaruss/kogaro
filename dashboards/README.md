# Kogaro Temporal Intelligence Dashboards

This directory contains Grafana dashboards for monitoring Kogaro's Temporal Intelligence features.

## Current Dashboard

### `kogaro-temporal-dashboard-enhanced.json` ⭐ **RECOMMENDED**
- **Purpose**: Enhanced Temporal Intelligence dashboard with filtering capabilities
- **Features**:
  - **Temporal Intelligence Panels**: URGENT (< 1h), Recent (1h-24h), Stable (> 24h), Total Issues
  - **Filtering Dropdowns**: Namespace, Severity, Workload Category filters
  - **Real-time Charts**: Validation errors by workload category, severity, and namespace
  - **Issue Age Tracking**: Temporal visualization of validation error ages
  - **Label Consistency**: Works with new consistent Prometheus metric labels
  - **Graceful Legacy Handling**: Ignores old inconsistent data gracefully

## Import Instructions

1. **Access Grafana**: Navigate to http://localhost:3000
2. **Login**: Use `admin` / `[password from .env file]`
3. **Import Dashboard**:
   - Click the "+" icon → "Import"
   - Copy the JSON content from `kogaro-temporal-dashboard-enhanced.json`
   - Paste into the import dialog
   - Click "Load" then "Import"

## Metrics Structure

### Key Metrics
- `kogaro_validation_errors_total`: Total validation errors with consistent labels
- `kogaro_validation_first_seen_timestamp`: When validation errors were first detected

### Labels (Consistent)
- `exported_namespace`: Kubernetes namespace
- `resource_type`: Type of Kubernetes resource
- `resource_name`: Name of the resource
- `validation_type`: Type of validation error
- `severity`: Error or warning
- `workload_category`: Application or infrastructure
- `expected_pattern`: Boolean indicating expected vs unexpected patterns

## Current Status

### ✅ Working Features
- **Real-time filtering** by namespace, severity, and workload category
- **Temporal intelligence** tracking for new validation errors
- **Consistent label joins** for proper temporal analysis
- **Graceful handling** of legacy inconsistent data
- **Responsive dashboard** with 30-second refresh

### 📊 Expected Behavior
- **Temporal panels** will populate as new validation errors emerge
- **Issue age tracking** shows age distribution of existing issues
- **Filtering** allows focused analysis of specific namespaces/workloads
- **Real-time updates** as validation state changes

## Usage Examples

### Focus on Testbed
1. Set Namespace filter to "kogaro-testbed"
2. Observe validation patterns specific to test environment
3. Monitor temporal progression of testbed issues

### Monitor Application vs Infrastructure
1. Use Workload Category filter to compare application vs infrastructure issues
2. Analyze severity distribution across workload types
3. Track temporal patterns by workload category

### Track New Issues
1. Watch URGENT panel for issues detected in last hour
2. Monitor Recent panel for issues 1-24 hours old
3. Observe Stable panel for long-standing patterns

## Management Script

Use `import-dashboards.sh` for automated dashboard management:

```bash
# List available dashboards
./dashboards/import-dashboards.sh list

# Show dashboard information
./dashboards/import-dashboards.sh info kogaro-temporal-dashboard-enhanced.json

# Import the enhanced dashboard
./dashboards/import-dashboards.sh import kogaro-temporal-dashboard-enhanced.json
```

## Maintenance Notes

- **Label Consistency**: Dashboard works with new consistent metric labels
- **Legacy Data**: Old inconsistent data is gracefully ignored
- **Temporal Intelligence**: New validation errors are properly tracked
- **Filtering**: All panels respect filter selections for focused analysis 