Yes, you're absolutely right. The Helm charts didn't fully cover several critical steps we had to perform manually:

Basic Auth Secrets: We had to manually create the basic auth secrets for both Prometheus and Alertmanager. This should ideally be handled by the Helm chart, either through:
A pre-install hook to create the secrets
Or a documented step in the values.yaml/README

Certificate Management: While the chart had the Certificate resource defined, we had to:
Clean up conflicting certificates
Manually create a new certificate for all domains
Remove the cert-manager.io/cluster-issuer annotations from the ingresses

Ingress Configuration: We had to manually patch the ingress configurations to:
Remove deprecated annotations
Ensure proper TLS configuration
Set up basic auth correctly
Would you like me to:
Update the Helm chart to handle these steps automatically?
Or at minimum, document these manual steps in the README for future reference?
This would make the deployment process more reliable and repeatable for future installations.

---



I'm trying to understand. how did  cert-manager.io/cluster-issuer annotations get there? My doing?

Let me check the Helm chart's ingress templates to see where these annotations might be coming from.
Ah, I found it! The cert-manager.io/cluster-issuer annotation is coming from the values.yaml file in the global ingress annotations section:
Apply to grist.md
.
This is actually a conflict in our setup because:
We have a dedicated Certificate resource in certificate.yaml that handles the TLS certificate issuance
We also have the cert-manager.io/cluster-issuer annotation in the global ingress settings
This is why we had to remove the annotation from the ingresses - it was causing cert-manager to try to manage the certificates in two different ways:
Through the Certificate resource (which is what we want)
Through the ingress annotations (which was causing conflicts)
To fix this properly, we should:
Remove the cert-manager.io/cluster-issuer annotation from the global ingress settings in values.yaml
Keep the Certificate resource as the single source of truth for certificate management
Would you like me to make this change to prevent this issue in future deployments?