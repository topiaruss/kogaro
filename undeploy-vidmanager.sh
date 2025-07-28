#!/bin/bash

# Undeploy vidmanager namespace and all its resources
# This script safely removes the vidmanager namespace which contains only test/debug resources

set -e  # Exit on any error

echo "🔍 Analyzing vidmanager namespace before undeployment..."

# Check if namespace exists
if ! kubectl get namespace vidmanager >/dev/null 2>&1; then
    echo "❌ Namespace 'vidmanager' does not exist. Nothing to undeploy."
    exit 0
fi

echo "📋 Current resources in vidmanager namespace:"
kubectl get all -n vidmanager

echo ""
echo "🔐 Secrets and ConfigMaps:"
kubectl get secrets,configmaps -n vidmanager

echo ""
echo "🌐 Ingress resources:"
kubectl get ingress -n vidmanager

echo ""
echo "💾 Storage resources:"
kubectl get pvc,pv -n vidmanager

echo ""
echo "⚠️  WARNING: This will permanently delete the vidmanager namespace and all its resources."
echo "   This includes:"
echo "   - debug-pod (netshoot container)"
echo "   - test-pod (failed container)"
echo "   - Completed jobs (cleanup-images, node-debugger)"
echo "   - app-ingress (broken ingress pointing to non-existent service)"
echo "   - TLS secret (vidmanager-tls)"
echo "   - Docker registry secret (regcred - local copy only)"
echo ""
echo "   No persistent storage or cross-namespace dependencies will be affected."

read -p "🤔 Are you sure you want to proceed? (y/N): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "❌ Undeployment cancelled."
    exit 1
fi

echo ""
echo "🗑️  Starting undeployment..."

# Delete ingress first (to avoid any routing issues)
echo "   Removing ingress..."
kubectl delete ingress app-ingress -n vidmanager --ignore-not-found=true

# Delete pods
echo "   Removing pods..."
kubectl delete pod debug-pod -n vidmanager --ignore-not-found=true
kubectl delete pod test-pod -n vidmanager --ignore-not-found=true

# Delete jobs
echo "   Removing jobs..."
kubectl delete job cleanup-images -n vidmanager --ignore-not-found=true
kubectl delete job node-debugger-artistic-shad-707058344 -n vidmanager --ignore-not-found=true

# Delete secrets and configmaps
echo "   Removing secrets and configmaps..."
kubectl delete secret regcred -n vidmanager --ignore-not-found=true
kubectl delete secret vidmanager-tls -n vidmanager --ignore-not-found=true
kubectl delete configmap kube-root-ca.crt -n vidmanager --ignore-not-found=true

# Finally, delete the namespace
echo "   Removing namespace..."
kubectl delete namespace vidmanager --ignore-not-found=true

echo ""
echo "✅ Undeployment completed successfully!"
echo ""
echo "🎯 Benefits:"
echo "   - Removed 2 Kogaro validation errors (KOGARO-SEC-002, KOGARO-SEC-010)"
echo "   - Cleaned up cluster resources"
echo "   - Improved security posture"
echo "   - Reduced dashboard noise"
echo ""
echo "📊 You can now check your Kogaro dashboard for cleaner results." 