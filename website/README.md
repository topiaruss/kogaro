# Kogaro Website

This directory contains the source code for the Kogaro website (kogaro.com).

## Local Development with Docker Desktop

### Prerequisites
- Docker Desktop with Kubernetes enabled
- kubectl configured to use docker-desktop context

### Quick Start

1. **Enable Kubernetes in Docker Desktop**
   ```bash
   # Verify Kubernetes is running
   kubectl cluster-info
   kubectl config current-context  # Should show "docker-desktop"
   ```

2. **Build and deploy locally**
   ```bash
   # Navigate to website directory
   cd website/

   # Build the Docker image
   docker build -t kogaro-website:local .

   # Apply Kubernetes manifests
   kubectl apply -f k8s/

   # Check deployment status
   kubectl get pods -n kogaro-website
   kubectl get svc -n kogaro-website
   ```

3. **Access the website**
   ```bash
   # Port forward to access locally
   kubectl port-forward -n kogaro-website svc/kogaro-website 8080:80
   
   # Open in browser
   open http://localhost:8080
   ```

### Development Workflow

1. **Make changes to website files**
2. **Rebuild and update**
   ```bash
   # Rebuild image
   docker build -t kogaro-website:local .
   
   # Restart deployment to pick up new image
   kubectl rollout restart deployment/kogaro-website -n kogaro-website
   
   # Wait for rollout to complete
   kubectl rollout status deployment/kogaro-website -n kogaro-website
   ```

3. **View logs**
   ```bash
   kubectl logs -f deployment/kogaro-website -n kogaro-website
   ```

### Cleanup
```bash
# Remove everything
kubectl delete -f k8s/
```

## Production Deployment

For production deployment to your cluster, update the image reference in `k8s/deployment.yaml` to point to your container registry.

## File Structure

```
website/
├── public/           # Static website files
│   ├── index.html   # Main homepage
│   ├── css/         # Stylesheets
│   ├── js/          # JavaScript files
│   └── images/      # Static images
├── k8s/             # Kubernetes manifests
├── nginx.conf       # Nginx configuration
├── Dockerfile       # Container build instructions
└── README.md        # This file
```# Test comment
