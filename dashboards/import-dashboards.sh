#!/bin/bash

# Kogaro Temporal Intelligence Dashboard Import Script
# This script helps import Grafana dashboards for Kogaro monitoring

set -e

# Configuration
GRAFANA_URL="http://localhost:3000"
GRAFANA_USER="admin"
GRAFANA_PASSWORD="${GRAFANA_PASSWORD:-$(grep GRAFANA_ADMIN_PASSWORD ../../helm/charts/monitoring/.env | cut -d'=' -f2 2>/dev/null || echo '')}"
DASHBOARDS_DIR="$(dirname "$0")"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to check if Grafana is accessible
check_grafana() {
    print_status "Checking Grafana accessibility..."
    
    if curl -s -f "$GRAFANA_URL/api/health" > /dev/null; then
        print_success "Grafana is accessible at $GRAFANA_URL"
        return 0
    else
        print_error "Grafana is not accessible at $GRAFANA_URL"
        print_warning "Make sure Grafana is running and port-forward is active:"
        print_warning "kubectl port-forward -n monitoring svc/monitoring-grafana 3000:80"
        return 1
    fi
}

# Function to import a dashboard
import_dashboard() {
    local dashboard_file="$1"
    local dashboard_name="$2"
    
    if [[ ! -f "$dashboard_file" ]]; then
        print_error "Dashboard file not found: $dashboard_file"
        return 1
    fi
    
    print_status "Importing dashboard: $dashboard_name"
    
    # Import dashboard using Grafana API
    response=$(curl -s -w "%{http_code}" -X POST \
        -H "Content-Type: application/json" \
        -d @"$dashboard_file" \
        "$GRAFANA_URL/api/dashboards/db" \
        -u "$GRAFANA_USER:$GRAFANA_PASSWORD")
    
    http_code="${response: -3}"
    response_body="${response%???}"
    
    if [[ "$http_code" == "200" ]]; then
        print_success "Successfully imported: $dashboard_name"
        echo "$response_body" | jq -r '.url' | sed "s|http://localhost:3000|$GRAFANA_URL|g"
    else
        print_error "Failed to import $dashboard_name (HTTP $http_code)"
        echo "$response_body" | jq -r '.message // "Unknown error"'
        return 1
    fi
}

# Function to list available dashboards
list_dashboards() {
    print_status "Available dashboards:"
    echo
    for file in "$DASHBOARDS_DIR"/*.json; do
        if [[ -f "$file" ]]; then
            filename=$(basename "$file")
            echo "  📊 $filename"
        fi
    done
    echo
}

# Function to show dashboard info
show_dashboard_info() {
    local dashboard_file="$1"
    
    if [[ ! -f "$dashboard_file" ]]; then
        print_error "Dashboard file not found: $dashboard_file"
        return 1
    fi
    
    print_status "Dashboard information for: $(basename "$dashboard_file")"
    echo
    
    # Extract dashboard info from JSON
    title=$(jq -r '.title // "Unknown"' "$dashboard_file")
    uid=$(jq -r '.uid // "Unknown"' "$dashboard_file")
    version=$(jq -r '.version // "Unknown"' "$dashboard_file")
    tags=$(jq -r '.tags[]? // empty' "$dashboard_file" | tr '\n' ', ' | sed 's/,$//')
    
    echo "  Title: $title"
    echo "  UID: $uid"
    echo "  Version: $version"
    echo "  Tags: ${tags:-None}"
    echo
}

# Main script logic
main() {
    echo "🚀 Kogaro Temporal Intelligence Dashboard Manager"
    echo "=================================================="
    echo
    
    # Check if Grafana is accessible
    if ! check_grafana; then
        exit 1
    fi
    
    # Parse command line arguments
    case "${1:-help}" in
        "list")
            list_dashboards
            ;;
        "info")
            if [[ -z "$2" ]]; then
                print_error "Please specify a dashboard file"
                echo "Usage: $0 info <dashboard-file.json>"
                exit 1
            fi
            show_dashboard_info "$2"
            ;;
        "import")
            if [[ -z "$2" ]]; then
                print_error "Please specify a dashboard file"
                echo "Usage: $0 import <dashboard-file.json>"
                exit 1
            fi
            import_dashboard "$2" "$(basename "$2")"
            ;;
        "import-all")
            print_status "Importing all available dashboards..."
            for file in "$DASHBOARDS_DIR"/*.json; do
                if [[ -f "$file" ]]; then
                    import_dashboard "$file" "$(basename "$file")"
                    echo
                fi
            done
            print_success "All dashboards imported!"
            ;;
        "help"|*)
            echo "Usage: $0 <command> [options]"
            echo
            echo "Commands:"
            echo "  list                    List all available dashboards"
            echo "  info <dashboard.json>   Show information about a dashboard"
            echo "  import <dashboard.json> Import a specific dashboard"
            echo "  import-all              Import all available dashboards"
            echo "  help                    Show this help message"
            echo
            echo "Examples:"
            echo "  $0 list"
            echo "  $0 info kogaro-temporal-dashboard-simple.json"
            echo "  $0 import kogaro-temporal-dashboard-simple.json"
            echo "  $0 import-all"
            echo
            echo "Configuration:"
            echo "  Grafana URL: $GRAFANA_URL"
            echo "  Dashboards directory: $DASHBOARDS_DIR"
            ;;
    esac
}

# Run main function
main "$@" 