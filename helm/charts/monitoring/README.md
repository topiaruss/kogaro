# Kogaro Monitoring Chart

This Helm chart provides a complete monitoring stack for Kogaro development and testing environments. It includes Prometheus, Grafana, and Alertmanager with pre-configured settings.

## Overview

The monitoring stack includes:
- Prometheus for metrics collection
- Grafana for visualization
- Alertmanager for alert handling
- Node Exporter for system metrics
- Kube State Metrics for Kubernetes metrics

## Development Tools

### Pre-commit Hooks

The repository includes pre-commit hooks to ensure code quality and prevent common issues. These hooks run automatically before each commit.

#### Required Tools

Install the required tools:

```bash
# Install Helm (if not already installed)
brew install helm

# Install kubeconform for Kubernetes manifest validation
brew install kubeconform

# Install yamllint for YAML file linting
brew install yamllint
```

#### What the Hooks Check

The pre-commit hooks perform the following checks:

1. **Sensitive Data Detection**
   - Scans for potential secrets, passwords, tokens, and keys
   - Prevents accidental commit of sensitive information
   - Checks for patterns like `password:`, `secret:`, `token:`, etc.

2. **Helm Chart Validation**
   - Runs `helm lint` on all charts
   - Ensures charts follow Helm best practices
   - Validates chart structure and dependencies

3. **Kubernetes Manifest Validation**
   - Uses `kubeconform` to validate rendered manifests
   - Ensures compatibility with Kubernetes API
   - Validates against the correct Kubernetes version

4. **YAML Linting**
   - Enforces consistent YAML formatting
   - Checks for syntax errors
   - Validates against YAML best practices

#### Troubleshooting Pre-commit Hooks

If a pre-commit hook fails:

1. **Sensitive Data Found**
   - Review the file for actual sensitive data
   - If it's a false positive, consider adding the pattern to `.gitignore`
   - If it's real sensitive data, remove it and use secrets management

2. **Helm Lint Failures**
   - Check the error message for specific issues
   - Common issues include:
     - Missing required fields
     - Invalid template syntax
     - Dependency issues

3. **Kubernetes Validation Failures**
   - Review the `kubeconform` output
   - Check for API version mismatches
   - Verify resource specifications

4. **YAML Lint Errors**
   - Fix formatting issues
   - Ensure consistent indentation
   - Check for syntax errors

#### Bypassing Hooks (Not Recommended)

In rare cases, you might need to bypass the hooks:

```bash
git commit -m "your message" --no-verify
```

⚠️ **Warning**: Only bypass hooks if you're absolutely sure it's necessary. The hooks are there to prevent common issues and maintain code quality.

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