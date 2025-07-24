# Kogaro Grafana Dashboards

This directory contains Grafana dashboards for Kogaro Temporal Intelligence monitoring and analysis.

## Available Dashboards

### 1. **Kogaro Temporal Intelligence - Error Code Enhanced** ⭐ **NEW**
**File**: `kogaro-temporal-dashboard-error-codes.json`  
**Features**:
- **Error Code Filtering**: Filter by specific error codes (KOGARO-IMG-001, KOGARO-RES-001, etc.)
- **Heatmap Visualization**: 2D matrix showing error types vs namespaces
- **Enhanced Filtering**: Namespace, Severity, Workload Category, and Error Code filters
- **Temporal Intelligence**: Issue age tracking with error codes
- **Top Error Codes Table**: Most frequent error codes with details

**Use Cases**:
- Identify security issues constrained to specific namespaces
- Analyze cross-namespace error patterns
- Track error code distribution and trends
- Plan remediation efforts based on error code frequency

### 2. **Kogaro Temporal Intelligence - Enhanced**
**File**: `kogaro-temporal-dashboard-enhanced.json`  
**Features**:
- Filtering dropdowns for Namespace, Severity, and Workload Category
- Temporal intelligence with issue age tracking
- Validation error breakdowns by category
- Improved temporal queries with consistent labels

### 3. **Kogaro Temporal Intelligence - Simple**
**File**: `kogaro-temporal-dashboard-simple.json`  
**Features**:
- Basic temporal intelligence metrics
- Simplified queries for immediate data visibility
- Good starting point for understanding the data

### 4. **Kogaro Temporal Intelligence - Working**
**File**: `kogaro-temporal-dashboard-working.json`  
**Features**:
- Experimental version with attempted fixes
- Reference implementation for troubleshooting

## Import Instructions

### Using the Import Script
```bash
# Import the new error code enhanced dashboard
./dashboards/import-dashboards.sh import kogaro-temporal-dashboard-error-codes.json

# Import all dashboards
./dashboards/import-dashboards.sh import-all

# List available dashboards
./dashboards/import-dashboards.sh list
```

### Manual Import
1. Open Grafana (http://localhost:3000)
2. Go to Dashboards → Import
3. Upload the JSON file or paste the content
4. Select Prometheus as the data source
5. Click Import

## Metrics Structure

### Core Metrics
- `kogaro_validation_errors_total`: Total validation errors with labels
- `kogaro_validation_first_seen_timestamp`: When errors were first detected
- `kogaro_validation_last_seen_timestamp`: When errors were last seen
- `kogaro_validation_age_hours`: Age of validation errors in hours

### Labels Available
- `resource_type`: Type of Kubernetes resource (Pod, Deployment, Service, etc.)
- `validation_type`: Type of validation (missing_resource_requests, pod_running_as_root, etc.)
- `exported_namespace`: Kubernetes namespace
- `resource_name`: Name of the specific resource
- `severity`: Error severity (error, warning, info)
- `workload_category`: Workload classification (application, infrastructure)
- `expected_pattern`: Whether this matches expected patterns
- `error_code`: Specific error code (KOGARO-IMG-001, KOGARO-RES-001, etc.)

### Error Code Categories
- **KOGARO-IMG-XXX**: Image validation errors
- **KOGARO-RES-XXX**: Resource limits and requests errors
- **KOGARO-NET-XXX**: Networking validation errors
- **KOGARO-SEC-XXX**: Security validation errors
- **KOGARO-REF-XXX**: Reference validation errors

## Current Status

### ✅ Working Features
- All dashboards import successfully
- Temporal intelligence metrics collecting
- Error code filtering operational
- Heatmap visualization functional
- Cross-namespace pattern analysis

### 🔄 In Development
- Advanced temporal state classification (New/Recent/Stable/Resolved)
- Error code class analysis
- Automated alerting based on error patterns

## Usage Examples

### Security Analysis
1. Use the **Error Code Enhanced** dashboard
2. Filter by `error_code` starting with "KOGARO-SEC"
3. Use the heatmap to see which namespaces have security issues
4. Identify if security problems are isolated or widespread

### Resource Planning
1. Filter by `error_code` starting with "KOGARO-RES"
2. Use the heatmap to identify namespaces with resource issues
3. Prioritize remediation based on error frequency

### Cross-Namespace Analysis
1. Use the heatmap to identify error types that span multiple namespaces
2. Look for patterns that indicate systemic issues
3. Plan organization-wide fixes for widespread problems

## Maintenance Notes

### Dashboard Updates
- All dashboards are compatible with Grafana 12.0.2
- Error codes are automatically populated from Prometheus metrics
- Filtering variables are dynamically populated from available data

### Troubleshooting
- If error codes don't appear, ensure Kogaro is running with error code integration
- If heatmap is empty, check that validation errors are being generated
- If filters don't populate, verify Prometheus is scraping Kogaro metrics

### Performance
- Dashboards refresh every 5 seconds
- Heatmap uses exponential color scaling for better visualization
- Large datasets may require longer load times 