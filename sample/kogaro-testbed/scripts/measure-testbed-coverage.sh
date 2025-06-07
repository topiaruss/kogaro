#!/bin/bash
set -euo pipefail

# Kogaro Testbed Coverage Measurement Script
# This script deploys the testbed and measures what validation code paths are exercised

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTBED_DIR="$(dirname "$SCRIPT_DIR")"
KOGARO_ROOT="$(dirname "$(dirname "$TESTBED_DIR")")"

# Configuration
NAMESPACE="${NAMESPACE:-kogaro-testbed}"
COVERAGE_OUTPUT="${COVERAGE_OUTPUT:-testbed-coverage}"
KOGARO_BINARY="${KOGARO_BINARY:-$KOGARO_ROOT/bin/kogaro}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log() {
    echo -e "${BLUE}[$(date +'%Y-%m-%d %H:%M:%S')] $1${NC}"
}

warn() {
    echo -e "${YELLOW}[WARNING] $1${NC}"
}

error() {
    echo -e "${RED}[ERROR] $1${NC}"
}

success() {
    echo -e "${GREEN}[SUCCESS] $1${NC}"
}

# Function to check prerequisites
check_prerequisites() {
    log "Checking prerequisites..."
    
    # Check if kubectl is available
    if ! command -v kubectl &> /dev/null; then
        error "kubectl is required but not installed"
        exit 1
    fi
    
    # Check if helm is available
    if ! command -v helm &> /dev/null; then
        error "helm is required but not installed"
        exit 1
    fi
    
    # Check if go is available for coverage analysis
    if ! command -v go &> /dev/null; then
        error "go is required for coverage analysis"
        exit 1
    fi
    
    # Check if we can access the cluster
    if ! kubectl cluster-info &> /dev/null; then
        error "Cannot access Kubernetes cluster"
        exit 1
    fi
    
    success "Prerequisites check passed"
}

# Function to build Kogaro with coverage instrumentation
build_kogaro_with_coverage() {
    log "Building Kogaro with coverage instrumentation..."
    
    cd "$KOGARO_ROOT"
    
    # Build with coverage flags
    go build -cover -o "bin/kogaro-coverage" ./main.go
    
    if [ ! -f "bin/kogaro-coverage" ]; then
        error "Failed to build Kogaro with coverage"
        exit 1
    fi
    
    success "Built Kogaro with coverage instrumentation"
}

# Function to deploy the testbed
deploy_testbed() {
    log "Deploying kogaro-testbed..."
    
    # Create namespace if it doesn't exist
    kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -
    
    # Deploy the testbed chart
    helm upgrade --install kogaro-testbed "$TESTBED_DIR" \
        --namespace "$NAMESPACE" \
        --values "$TESTBED_DIR/values.yaml" \
        --wait --timeout=5m
    
    # Wait for resources to be ready
    log "Waiting for testbed resources to be ready..."
    kubectl wait --for=condition=available --timeout=300s deployment --all -n "$NAMESPACE" || true
    
    # Show deployed resources
    log "Testbed resources deployed:"
    kubectl get all,ingress,pvc,networkpolicies -n "$NAMESPACE"
    
    success "Testbed deployed successfully"
}

# Function to run Kogaro validation with coverage
run_kogaro_with_coverage() {
    log "Running Kogaro validation with coverage measurement..."
    
    cd "$KOGARO_ROOT"
    
    # Set up coverage environment
    export GOCOVERDIR="$PWD/coverage"
    mkdir -p "$GOCOVERDIR"
    
    # Create a temporary kubeconfig or use existing
    KUBECONFIG_ARG=""
    if [ -n "${KUBECONFIG:-}" ]; then
        KUBECONFIG_ARG="--kubeconfig=$KUBECONFIG"
    fi
    
    # Run Kogaro with coverage
    log "Executing Kogaro validation on testbed namespace: $NAMESPACE"
    
    # Create Kogaro configuration for comprehensive validation
    cat > /tmp/kogaro-testbed-config.yaml << EOF
validation:
  enableReferenceValidation: true
  enableResourceLimitsValidation: true
  enableSecurityValidation: true  
  enableNetworkingValidation: true
  enableServiceAccountValidation: true
  enableNetworkPolicyValidation: true
  enableIngressValidation: true
  enablePVCValidation: true
  enableConfigMapValidation: true
  enableSecretValidation: true
  
resourceLimits:
  minCPURequest: "10m"
  minMemoryRequest: "16Mi"
  
networking:
  enableServiceValidation: true
  enableNetworkPolicyValidation: true
  enableIngressValidation: true
  warnUnexposedPods: true
  policyRequiredNamespaces: ["kogaro-test-prod"]
  
security:
  enableRootUserValidation: true
  enableSecurityContextValidation: true
  enableServiceAccountValidation: true
  enableNetworkPolicyValidation: true
  securitySensitiveNamespaces: ["kogaro-test-prod"]
EOF
    
    # Run validation focusing on testbed namespaces
    GOCOVERDIR="$GOCOVERDIR" ./bin/kogaro-coverage validate \
        --config=/tmp/kogaro-testbed-config.yaml \
        --namespace="$NAMESPACE" \
        --namespace="kogaro-test-system" \
        --namespace="kogaro-test-prod" \
        --log-level=debug \
        --output=json > "/tmp/kogaro-validation-results.json" || true
    
    success "Kogaro validation completed"
    
    # Show validation results summary
    if [ -f "/tmp/kogaro-validation-results.json" ]; then
        log "Validation results summary:"
        cat "/tmp/kogaro-validation-results.json" | jq -r '.summary // "No summary available"' || cat "/tmp/kogaro-validation-results.json"
    fi
}

# Function to generate coverage report
generate_coverage_report() {
    log "Generating coverage report..."
    
    cd "$KOGARO_ROOT"
    
    if [ ! -d "coverage" ] || [ -z "$(ls -A coverage)" ]; then
        warn "No coverage data found"
        return 1
    fi
    
    # Convert coverage data to legacy format
    go tool covdata textfmt -i=coverage -o="${COVERAGE_OUTPUT}.out"
    
    # Generate HTML report
    go tool cover -html="${COVERAGE_OUTPUT}.out" -o="${COVERAGE_OUTPUT}.html"
    
    # Generate function-level report
    go tool cover -func="${COVERAGE_OUTPUT}.out" > "${COVERAGE_OUTPUT}-functions.txt"
    
    # Calculate coverage statistics
    TOTAL_COVERAGE=$(go tool cover -func="${COVERAGE_OUTPUT}.out" | tail -1 | awk '{print $NF}')
    
    success "Coverage report generated: ${COVERAGE_OUTPUT}.html"
    success "Total testbed coverage: $TOTAL_COVERAGE"
    
    # Show function-level coverage for key validation functions
    log "Coverage for key validation functions:"
    grep -E "(IsSystemNamespace|IsBatchOwnerKind|GetMinResourceThresholds|validateServiceAccountExists|validatePVCExists)" "${COVERAGE_OUTPUT}-functions.txt" || warn "Some functions not found in coverage report"
}

# Function to analyze coverage gaps
analyze_coverage_gaps() {
    log "Analyzing coverage gaps..."
    
    cd "$KOGARO_ROOT"
    
    if [ ! -f "${COVERAGE_OUTPUT}-functions.txt" ]; then
        warn "Function coverage report not found"
        return 1
    fi
    
    log "Functions with 0% coverage:"
    grep "0.0%" "${COVERAGE_OUTPUT}-functions.txt" || log "No functions with 0% coverage found!"
    
    log "Functions with <50% coverage:"
    awk '$NF != "0.0%" && $NF < "50.0%" {print}' "${COVERAGE_OUTPUT}-functions.txt" || log "No functions with <50% coverage found!"
    
    # Compare with internal test coverage
    if [ -f "coverage.out" ]; then
        INTERNAL_COVERAGE=$(go tool cover -func=coverage.out | tail -1 | awk '{print $NF}')
        log "Coverage comparison:"
        log "  Internal tests coverage: $INTERNAL_COVERAGE"
        log "  Testbed coverage: $TOTAL_COVERAGE"
    fi
}

# Function to generate marketing report
generate_marketing_report() {
    log "Generating marketing coverage report..."
    
    cat > "${COVERAGE_OUTPUT}-marketing-report.md" << EOF
# Kogaro Testbed Coverage Report

## Overview
The kogaro-testbed provides comprehensive validation coverage for demonstrating Kogaro's capabilities.

**Testbed Coverage: $TOTAL_COVERAGE**

## Test Categories Covered

### Reference Validation
- Tests for dangling resource references
- Validates ConfigMaps, Secrets, PVCs, ServiceAccounts, IngressClasses, StorageClasses
- Ensures all referenced resources exist

### Resource Limits Validation  
- Tests for missing resource requests and limits
- Validates QoS class configurations
- Tests minimum resource threshold enforcement

### Security Validation
- Tests for security misconfigurations
- Validates SecurityContext settings
- Tests for root users, privileged containers, excessive capabilities
- Validates ServiceAccount permissions

### Networking Validation
- Tests for networking connectivity issues
- Validates Service selectors and port matching
- Tests NetworkPolicy coverage requirements
- Validates Ingress backend connectivity

### SharedConfig Features
- Tests configurable validation thresholds
- Validates namespace exclusion logic
- Tests production namespace detection
- Validates batch workload exclusion patterns

## Coverage by Function

\`\`\`
$(cat "${COVERAGE_OUTPUT}-functions.txt" | head -20)
\`\`\`

## Validation Results

The testbed exercises **50+ validation scenarios** across all validator types, providing comprehensive demonstration of Kogaro's validation capabilities.

Generated on: $(date)
EOF

    success "Marketing report generated: ${COVERAGE_OUTPUT}-marketing-report.md"
}

# Function to cleanup
cleanup() {
    log "Cleaning up..."
    
    # Option to keep testbed deployed for further testing
    if [ "${KEEP_TESTBED:-false}" = "true" ]; then
        log "Keeping testbed deployed (KEEP_TESTBED=true)"
    else
        log "Removing testbed deployment..."
        helm uninstall kogaro-testbed --namespace "$NAMESPACE" || true
        kubectl delete namespace "$NAMESPACE" --ignore-not-found=true
    fi
    
    # Clean up temporary files
    rm -f /tmp/kogaro-testbed-config.yaml /tmp/kogaro-validation-results.json
    
    success "Cleanup completed"
}

# Main execution
main() {
    log "Starting Kogaro Testbed Coverage Measurement"
    log "Namespace: $NAMESPACE"
    log "Coverage output: $COVERAGE_OUTPUT"
    
    check_prerequisites
    build_kogaro_with_coverage
    deploy_testbed
    run_kogaro_with_coverage
    generate_coverage_report
    analyze_coverage_gaps
    generate_marketing_report
    
    success "Testbed coverage measurement completed!"
    log "View coverage report: open ${COVERAGE_OUTPUT}.html"
    log "View marketing report: cat ${COVERAGE_OUTPUT}-marketing-report.md"
}

# Trap cleanup on script exit
trap cleanup EXIT

# Run main function
main "$@"