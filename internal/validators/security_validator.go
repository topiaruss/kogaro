// Package validators provides security configuration validation functionality.
//
// This package implements validation of security configurations within
// a Kubernetes cluster, detecting security misconfigurations that could
// expose workloads to unnecessary risk. It validates SecurityContext settings,
// root privilege usage, ServiceAccount permissions, and NetworkPolicy coverage.
package validators

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SecurityConfig defines which security validation checks to perform
type SecurityConfig struct {
	EnableRootUserValidation       bool
	EnableSecurityContextValidation bool
	EnableServiceAccountValidation  bool
	EnableNetworkPolicyValidation   bool
	// Namespaces that require NetworkPolicies for security compliance
	SecuritySensitiveNamespaces []string
}

// SecurityValidator validates security configurations across workloads
type SecurityValidator struct {
	client client.Client
	log    logr.Logger
	config SecurityConfig
}

// NewSecurityValidator creates a new SecurityValidator with the given client, logger and config
func NewSecurityValidator(client client.Client, log logr.Logger, config SecurityConfig) *SecurityValidator {
	return &SecurityValidator{
		client: client,
		log:    log.WithName("security-validator"),
		config: config,
	}
}

// GetValidationType returns the validation type identifier for security validation
func (v *SecurityValidator) GetValidationType() string {
	return "security_validation"
}

// ValidateCluster performs comprehensive validation of security configurations across the entire cluster
func (v *SecurityValidator) ValidateCluster(ctx context.Context) error {
	var allErrors []ValidationError

	// Validate root user and SecurityContext configurations
	if v.config.EnableRootUserValidation || v.config.EnableSecurityContextValidation {
		deploymentErrors, err := v.validateDeploymentSecurity(ctx)
		if err != nil {
			return fmt.Errorf("failed to validate deployment security: %w", err)
		}
		allErrors = append(allErrors, deploymentErrors...)

		statefulSetErrors, err := v.validateStatefulSetSecurity(ctx)
		if err != nil {
			return fmt.Errorf("failed to validate statefulset security: %w", err)
		}
		allErrors = append(allErrors, statefulSetErrors...)

		daemonSetErrors, err := v.validateDaemonSetSecurity(ctx)
		if err != nil {
			return fmt.Errorf("failed to validate daemonset security: %w", err)
		}
		allErrors = append(allErrors, daemonSetErrors...)

		podErrors, err := v.validatePodSecurity(ctx)
		if err != nil {
			return fmt.Errorf("failed to validate pod security: %w", err)
		}
		allErrors = append(allErrors, podErrors...)
	}

	// Validate ServiceAccount permissions
	if v.config.EnableServiceAccountValidation {
		serviceAccountErrors, err := v.validateServiceAccountPermissions(ctx)
		if err != nil {
			return fmt.Errorf("failed to validate serviceaccount permissions: %w", err)
		}
		allErrors = append(allErrors, serviceAccountErrors...)
	}

	// Validate NetworkPolicy coverage
	if v.config.EnableNetworkPolicyValidation {
		networkPolicyErrors, err := v.validateNetworkPolicyCoverage(ctx)
		if err != nil {
			return fmt.Errorf("failed to validate networkpolicy coverage: %w", err)
		}
		allErrors = append(allErrors, networkPolicyErrors...)
	}

	// Log all validation errors and update metrics
	for _, validationErr := range allErrors {
		v.log.Info("security validation error found",
			"resource_type", validationErr.ResourceType,
			"resource_name", validationErr.ResourceName,
			"namespace", validationErr.Namespace,
			"validation_type", validationErr.ValidationType,
			"message", validationErr.Message,
		)

		validationErrors.WithLabelValues(
			validationErr.ResourceType,
			validationErr.ValidationType,
			validationErr.Namespace,
		).Inc()
	}

	v.log.Info("security validation completed", "total_errors", len(allErrors))
	return nil
}

func (v *SecurityValidator) validateDeploymentSecurity(ctx context.Context) ([]ValidationError, error) {
	var errors []ValidationError
	var deployments appsv1.DeploymentList

	if err := v.client.List(ctx, &deployments); err != nil {
		return nil, fmt.Errorf("failed to list deployments: %w", err)
	}

	for _, deployment := range deployments.Items {
		securityErrors := v.validatePodTemplateSecurity(deployment.Spec.Template, "Deployment", deployment.Name, deployment.Namespace)
		errors = append(errors, securityErrors...)
	}

	return errors, nil
}

func (v *SecurityValidator) validateStatefulSetSecurity(ctx context.Context) ([]ValidationError, error) {
	var errors []ValidationError
	var statefulSets appsv1.StatefulSetList

	if err := v.client.List(ctx, &statefulSets); err != nil {
		return nil, fmt.Errorf("failed to list statefulsets: %w", err)
	}

	for _, statefulSet := range statefulSets.Items {
		securityErrors := v.validatePodTemplateSecurity(statefulSet.Spec.Template, "StatefulSet", statefulSet.Name, statefulSet.Namespace)
		errors = append(errors, securityErrors...)
	}

	return errors, nil
}

func (v *SecurityValidator) validateDaemonSetSecurity(ctx context.Context) ([]ValidationError, error) {
	var errors []ValidationError
	var daemonSets appsv1.DaemonSetList

	if err := v.client.List(ctx, &daemonSets); err != nil {
		return nil, fmt.Errorf("failed to list daemonsets: %w", err)
	}

	for _, daemonSet := range daemonSets.Items {
		securityErrors := v.validatePodTemplateSecurity(daemonSet.Spec.Template, "DaemonSet", daemonSet.Name, daemonSet.Namespace)
		errors = append(errors, securityErrors...)
	}

	return errors, nil
}

func (v *SecurityValidator) validatePodSecurity(ctx context.Context) ([]ValidationError, error) {
	var errors []ValidationError
	var pods corev1.PodList

	if err := v.client.List(ctx, &pods); err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	for _, pod := range pods.Items {
		// Skip pods managed by controllers (they're validated via their controllers)
		if len(pod.OwnerReferences) > 0 {
			continue
		}

		podTemplate := corev1.PodTemplateSpec{
			Spec: pod.Spec,
		}
		securityErrors := v.validatePodTemplateSecurity(podTemplate, "Pod", pod.Name, pod.Namespace)
		errors = append(errors, securityErrors...)
	}

	return errors, nil
}

func (v *SecurityValidator) validatePodTemplateSecurity(template corev1.PodTemplateSpec, resourceType, resourceName, namespace string) []ValidationError {
	var errors []ValidationError

	// Validate Pod-level SecurityContext
	if v.config.EnableSecurityContextValidation {
		if template.Spec.SecurityContext == nil {
			errors = append(errors, ValidationError{
				ResourceType:   resourceType,
				ResourceName:   resourceName,
				Namespace:      namespace,
				ValidationType: "missing_pod_security_context",
				Message:        "Pod has no SecurityContext defined",
			})
		} else {
			// Check for Pod-level security settings
			podSecurityErrors := v.validatePodSecurityContext(template.Spec.SecurityContext, resourceType, resourceName, namespace)
			errors = append(errors, podSecurityErrors...)
		}
	}

	// Validate Container-level security
	containerErrors := v.validateContainersSecurity(template.Spec.Containers, resourceType, resourceName, namespace, false)
	errors = append(errors, containerErrors...)

	initContainerErrors := v.validateContainersSecurity(template.Spec.InitContainers, resourceType, resourceName, namespace, true)
	errors = append(errors, initContainerErrors...)

	return errors
}

func (v *SecurityValidator) validatePodSecurityContext(securityContext *corev1.PodSecurityContext, resourceType, resourceName, namespace string) []ValidationError {
	var errors []ValidationError

	if v.config.EnableRootUserValidation {
		// Check if Pod is running as root user
		if securityContext.RunAsUser != nil && *securityContext.RunAsUser == 0 {
			errors = append(errors, ValidationError{
				ResourceType:   resourceType,
				ResourceName:   resourceName,
				Namespace:      namespace,
				ValidationType: "pod_running_as_root",
				Message:        "Pod SecurityContext specifies runAsUser: 0 (root)",
			})
		}

		// Check if Pod allows privilege escalation
		if securityContext.RunAsNonRoot == nil || !*securityContext.RunAsNonRoot {
			errors = append(errors, ValidationError{
				ResourceType:   resourceType,
				ResourceName:   resourceName,
				Namespace:      namespace,
				ValidationType: "pod_allows_root_user",
				Message:        "Pod SecurityContext does not enforce runAsNonRoot: true",
			})
		}
	}

	return errors
}

func (v *SecurityValidator) validateContainersSecurity(containers []corev1.Container, resourceType, resourceName, namespace string, isInitContainer bool) []ValidationError {
	var errors []ValidationError

	containerType := "container"
	if isInitContainer {
		containerType = "init container"
	}

	for _, container := range containers {
		if v.config.EnableSecurityContextValidation {
			if container.SecurityContext == nil {
				errors = append(errors, ValidationError{
					ResourceType:   resourceType,
					ResourceName:   resourceName,
					Namespace:      namespace,
					ValidationType: "missing_container_security_context",
					Message:        fmt.Sprintf("Container '%s' (%s) has no SecurityContext defined", container.Name, containerType),
				})
			} else {
				containerSecurityErrors := v.validateContainerSecurityContext(container.SecurityContext, container.Name, containerType, resourceType, resourceName, namespace)
				errors = append(errors, containerSecurityErrors...)
			}
		}
	}

	return errors
}

func (v *SecurityValidator) validateContainerSecurityContext(securityContext *corev1.SecurityContext, containerName, containerType, resourceType, resourceName, namespace string) []ValidationError {
	var errors []ValidationError

	if v.config.EnableRootUserValidation {
		// Check if container is running as root user
		if securityContext.RunAsUser != nil && *securityContext.RunAsUser == 0 {
			errors = append(errors, ValidationError{
				ResourceType:   resourceType,
				ResourceName:   resourceName,
				Namespace:      namespace,
				ValidationType: "container_running_as_root",
				Message:        fmt.Sprintf("Container '%s' (%s) SecurityContext specifies runAsUser: 0 (root)", containerName, containerType),
			})
		}

		// Check if container allows privilege escalation
		if securityContext.AllowPrivilegeEscalation == nil || *securityContext.AllowPrivilegeEscalation {
			errors = append(errors, ValidationError{
				ResourceType:   resourceType,
				ResourceName:   resourceName,
				Namespace:      namespace,
				ValidationType: "container_allows_privilege_escalation",
				Message:        fmt.Sprintf("Container '%s' (%s) SecurityContext does not set allowPrivilegeEscalation: false", containerName, containerType),
			})
		}

		// Check if container is running in privileged mode
		if securityContext.Privileged != nil && *securityContext.Privileged {
			errors = append(errors, ValidationError{
				ResourceType:   resourceType,
				ResourceName:   resourceName,
				Namespace:      namespace,
				ValidationType: "container_privileged_mode",
				Message:        fmt.Sprintf("Container '%s' (%s) SecurityContext specifies privileged: true", containerName, containerType),
			})
		}

		// Check if container has root filesystem read-only
		if securityContext.ReadOnlyRootFilesystem == nil || !*securityContext.ReadOnlyRootFilesystem {
			errors = append(errors, ValidationError{
				ResourceType:   resourceType,
				ResourceName:   resourceName,
				Namespace:      namespace,
				ValidationType: "container_writable_root_filesystem",
				Message:        fmt.Sprintf("Container '%s' (%s) SecurityContext does not set readOnlyRootFilesystem: true", containerName, containerType),
			})
		}
	}

	if v.config.EnableSecurityContextValidation {
		// Check for capabilities
		if securityContext.Capabilities != nil && len(securityContext.Capabilities.Add) > 0 {
			for _, capability := range securityContext.Capabilities.Add {
				errors = append(errors, ValidationError{
					ResourceType:   resourceType,
					ResourceName:   resourceName,
					Namespace:      namespace,
					ValidationType: "container_additional_capabilities",
					Message:        fmt.Sprintf("Container '%s' (%s) SecurityContext adds capability: %s", containerName, containerType, capability),
				})
			}
		}
	}

	return errors
}

func (v *SecurityValidator) validateServiceAccountPermissions(ctx context.Context) ([]ValidationError, error) {
	var errors []ValidationError

	// Get all ServiceAccounts
	var serviceAccounts corev1.ServiceAccountList
	if err := v.client.List(ctx, &serviceAccounts); err != nil {
		return nil, fmt.Errorf("failed to list serviceaccounts: %w", err)
	}

	// Get all RoleBindings and ClusterRoleBindings
	var roleBindings rbacv1.RoleBindingList
	if err := v.client.List(ctx, &roleBindings); err != nil {
		return nil, fmt.Errorf("failed to list rolebindings: %w", err)
	}

	var clusterRoleBindings rbacv1.ClusterRoleBindingList
	if err := v.client.List(ctx, &clusterRoleBindings); err != nil {
		return nil, fmt.Errorf("failed to list clusterrolebindings: %w", err)
	}

	// Check for ServiceAccounts with potentially excessive permissions
	for _, sa := range serviceAccounts.Items {
		// Skip default and system ServiceAccounts for some checks
		if sa.Name == "default" && sa.Namespace == "default" {
			continue
		}

		// Check for ClusterRoleBindings that give this ServiceAccount cluster-wide permissions
		for _, crb := range clusterRoleBindings.Items {
			for _, subject := range crb.Subjects {
				if subject.Kind == "ServiceAccount" && subject.Name == sa.Name && subject.Namespace == sa.Namespace {
					errors = append(errors, ValidationError{
						ResourceType:   "ServiceAccount",
						ResourceName:   sa.Name,
						Namespace:      sa.Namespace,
						ValidationType: "serviceaccount_cluster_role_binding",
						Message:        fmt.Sprintf("ServiceAccount has ClusterRoleBinding '%s' with role '%s'", crb.Name, crb.RoleRef.Name),
					})
				}
			}
		}

		// Check for potentially dangerous RoleBindings
		for _, rb := range roleBindings.Items {
			if rb.Namespace != sa.Namespace {
				continue
			}

			for _, subject := range rb.Subjects {
				if subject.Kind == "ServiceAccount" && subject.Name == sa.Name {
					// Flag some potentially dangerous role names
					if v.isDangerousRole(rb.RoleRef.Name) {
						errors = append(errors, ValidationError{
							ResourceType:   "ServiceAccount",
							ResourceName:   sa.Name,
							Namespace:      sa.Namespace,
							ValidationType: "serviceaccount_excessive_permissions",
							Message:        fmt.Sprintf("ServiceAccount has potentially excessive RoleBinding '%s' with role '%s'", rb.Name, rb.RoleRef.Name),
						})
					}
				}
			}
		}
	}

	return errors, nil
}

func (v *SecurityValidator) isDangerousRole(roleName string) bool {
	dangerousRoles := []string{
		"admin",
		"cluster-admin",
		"edit",
		"system:admin",
	}

	for _, dangerous := range dangerousRoles {
		if roleName == dangerous {
			return true
		}
	}
	return false
}

func (v *SecurityValidator) validateNetworkPolicyCoverage(ctx context.Context) ([]ValidationError, error) {
	var errors []ValidationError

	// Get all NetworkPolicies
	var networkPolicies networkingv1.NetworkPolicyList
	if err := v.client.List(ctx, &networkPolicies); err != nil {
		return nil, fmt.Errorf("failed to list networkpolicies: %w", err)
	}

	// Create a map of namespaces that have NetworkPolicies
	namespacesWithPolicies := make(map[string]bool)
	for _, np := range networkPolicies.Items {
		namespacesWithPolicies[np.Namespace] = true
	}

	// Check if security-sensitive namespaces have NetworkPolicies
	for _, sensitiveNamespace := range v.config.SecuritySensitiveNamespaces {
		if !namespacesWithPolicies[sensitiveNamespace] {
			errors = append(errors, ValidationError{
				ResourceType:   "Namespace",
				ResourceName:   sensitiveNamespace,
				Namespace:      sensitiveNamespace,
				ValidationType: "missing_network_policy_security_sensitive",
				Message:        fmt.Sprintf("Security-sensitive namespace '%s' has no NetworkPolicies defined", sensitiveNamespace),
			})
		}
	}

	// Get all namespaces and check for production-like namespaces without policies
	var namespaces corev1.NamespaceList
	if err := v.client.List(ctx, &namespaces); err != nil {
		return nil, fmt.Errorf("failed to list namespaces: %w", err)
	}

	for _, ns := range namespaces.Items {
		// Skip system namespaces
		if v.isSystemNamespace(ns.Name) {
			continue
		}

		// Check if this looks like a production namespace without NetworkPolicies
		if v.isProductionLikeNamespace(ns.Name) && !namespacesWithPolicies[ns.Name] {
			errors = append(errors, ValidationError{
				ResourceType:   "Namespace",
				ResourceName:   ns.Name,
				Namespace:      ns.Name,
				ValidationType: "missing_network_policy_production",
				Message:        fmt.Sprintf("Production-like namespace '%s' has no NetworkPolicies defined", ns.Name),
			})
		}
	}

	return errors, nil
}

func (v *SecurityValidator) isSystemNamespace(namespace string) bool {
	systemNamespaces := []string{
		"kube-system",
		"kube-public",
		"kube-node-lease",
		"default",
	}

	for _, systemNS := range systemNamespaces {
		if namespace == systemNS {
			return true
		}
	}
	return false
}

func (v *SecurityValidator) isProductionLikeNamespace(namespace string) bool {
	productionIndicators := []string{
		"prod",
		"production",
		"live",
		"api",
		"app",
		"web",
		"service",
	}

	for _, indicator := range productionIndicators {
		if namespace == indicator || len(namespace) > len(indicator) && 
			(namespace[:len(indicator)] == indicator || namespace[len(namespace)-len(indicator):] == indicator) {
			return true
		}
	}
	return false
}