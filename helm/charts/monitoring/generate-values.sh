#!/bin/bash

# Script to generate Helm values from environment variables
# Usage: ./generate-values.sh > values.yaml

set -e

# Load environment variables from .env file if it exists
if [ -f ".env" ]; then
    echo "Loading environment variables from .env file..." >&2
    export $(cat .env | grep -v '^#' | xargs)
fi

# Set defaults if not provided
DOMAIN=${DOMAIN:-"ogaro.com"}
GRAFANA_ADMIN_USER=${GRAFANA_ADMIN_USER:-"admin"}
GRAFANA_ADMIN_PASSWORD=${GRAFANA_ADMIN_PASSWORD:-"your-secure-password-here"}
PROMETHEUS_BASIC_AUTH_USER=${PROMETHEUS_BASIC_AUTH_USER:-"prometheus"}
PROMETHEUS_BASIC_AUTH_PASSWORD=${PROMETHEUS_BASIC_AUTH_PASSWORD:-"your-prometheus-password"}
ALERTMANAGER_BASIC_AUTH_USER=${ALERTMANAGER_BASIC_AUTH_USER:-"alertmanager"}
ALERTMANAGER_BASIC_AUTH_PASSWORD=${ALERTMANAGER_BASIC_AUTH_PASSWORD:-"your-alertmanager-password"}
TLS_SECRET_NAME=${TLS_SECRET_NAME:-"kogaro-monitoring-tls"}

# Resource limits with defaults
PROMETHEUS_CPU_LIMIT=${PROMETHEUS_CPU_LIMIT:-"125m"}
PROMETHEUS_MEMORY_LIMIT=${PROMETHEUS_MEMORY_LIMIT:-"256Mi"}
GRAFANA_CPU_LIMIT=${GRAFANA_CPU_LIMIT:-"100m"}
GRAFANA_MEMORY_LIMIT=${GRAFANA_MEMORY_LIMIT:-"200Mi"}

# Validate required variables
if [ "$GRAFANA_ADMIN_PASSWORD" = "your-secure-password-here" ]; then
    echo "ERROR: Please set GRAFANA_ADMIN_PASSWORD in your .env file" >&2
    exit 1
fi

if [ "$PROMETHEUS_BASIC_AUTH_PASSWORD" = "your-prometheus-password" ]; then
    echo "ERROR: Please set PROMETHEUS_BASIC_AUTH_PASSWORD in your .env file" >&2
    exit 1
fi

if [ "$ALERTMANAGER_BASIC_AUTH_PASSWORD" = "your-alertmanager-password" ]; then
    echo "ERROR: Please set ALERTMANAGER_BASIC_AUTH_PASSWORD in your .env file" >&2
    exit 1
fi

# Generate the values.yaml content
cat << EOF
# Global settings
# Monitoring stack configuration for production environment.
global:
  domain: "${DOMAIN}"
  tls:
    enabled: true
    secretName: "${TLS_SECRET_NAME}"
    hosts:
      - alertmanager.${DOMAIN}
      - grafana.${DOMAIN}
      - prometheus.${DOMAIN}
  ingress:
    annotations:
      # SSL/TLS Configuration
      nginx.ingress.kubernetes.io/ssl-redirect: "true"
      nginx.ingress.kubernetes.io/force-ssl-redirect: "true"
      nginx.ingress.kubernetes.io/backend-protocol: "HTTP"
      
      # Security Headers (commented out - requires nginx ingress configuration-snippet enabled)
      # nginx.ingress.kubernetes.io/configuration-snippet: |
      #   more_set_headers "Strict-Transport-Security: max-age=31536000; includeSubDomains" always;
      #   more_set_headers "X-Content-Type-Options: nosniff" always;
      #   more_set_headers "X-Frame-Options: DENY" always;
      #   more_set_headers "X-XSS-Protection: 1; mode=block" always;
      
      # Proxy Settings
      nginx.ingress.kubernetes.io/proxy-body-size: "50m"
      nginx.ingress.kubernetes.io/proxy-read-timeout: "600"
      nginx.ingress.kubernetes.io/proxy-send-timeout: "600"
    className: ingress-nginx

# Prometheus configuration
prometheus:
  enabled: true
  ingress:
    enabled: true
    hosts:
      - prometheus.${DOMAIN}
    tls:
      - secretName: ${TLS_SECRET_NAME}
        hosts:
          - prometheus.${DOMAIN}
    annotations:
      nginx.ingress.kubernetes.io/auth-realm: "Authentication Required - Prometheus"
      nginx.ingress.kubernetes.io/auth-secret: "prometheus-basic-auth"
      nginx.ingress.kubernetes.io/auth-type: "basic"
  prometheusSpec:
    resources:
      limits:
        cpu: ${PROMETHEUS_CPU_LIMIT}
        memory: ${PROMETHEUS_MEMORY_LIMIT}
      requests:
        cpu: 25m
        memory: 100Mi
    retention: 15d
    storageSpec:
      volumeClaimTemplate:
        spec:
          accessModes:
            - ReadWriteOnce
          resources:
            requests:
              storage: 5Gi

# Grafana configuration
grafana:
  enabled: true
  # Admin credentials - loaded from environment variables
  adminUser: ${GRAFANA_ADMIN_USER}
  adminPassword: "${GRAFANA_ADMIN_PASSWORD}"
  service:
    type: ClusterIP
    port: 3000
    targetPort: 3000
  ingress:
    enabled: true
    hosts:
      - grafana.${DOMAIN}
    tls:
      - secretName: ${TLS_SECRET_NAME}
        hosts:
          - grafana.${DOMAIN}
  resources:
    limits:
      cpu: ${GRAFANA_CPU_LIMIT}
      memory: ${GRAFANA_MEMORY_LIMIT}
    requests:
      cpu: 50m
      memory: 100Mi

# Alertmanager configuration
alertmanager:
  enabled: true
  ingress:
    enabled: true
    hosts:
      - alertmanager.${DOMAIN}
    tls:
      - secretName: ${TLS_SECRET_NAME}
        hosts:
          - alertmanager.${DOMAIN}
    annotations:
      nginx.ingress.kubernetes.io/auth-realm: "Authentication Required - Alertmanager"
      nginx.ingress.kubernetes.io/auth-secret: "alertmanager-basic-auth"
      nginx.ingress.kubernetes.io/auth-type: "basic"
  alertmanagerSpec:
    resources:
      limits:
        cpu: 25m
        memory: 50Mi
      requests:
        cpu: 12m
        memory: 25Mi
    retention: 120h 