# Kogaro Helm Chart Repository

This is the Helm chart repository for Kogaro - Kubernetes Configuration Hygiene Agent.

## Usage

```bash
helm repo add kogaro https://topiaruss.github.io/kogaro
helm repo update
helm install kogaro kogaro/kogaro --namespace kogaro-system --create-namespace
```

## Charts

- **kogaro**: Kubernetes Configuration Hygiene Agent

For more information, visit: https://github.com/topiaruss/kogaro
