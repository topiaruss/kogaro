// Package validators provides resource limits validation functionality.
//
// This package implements validation of resource requests and limits for
// Kubernetes workloads, detecting pods without proper resource constraints
// which can lead to resource contention and cluster instability.
package validators

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ResourceLimitsConfig defines which resource limit validations to perform
type ResourceLimitsConfig struct {
	EnableMissingRequestsValidation bool
	EnableMissingLimitsValidation   bool
	EnableQoSValidation             bool
	// Minimum resource thresholds for validation
	MinCPURequest    *resource.Quantity
	MinMemoryRequest *resource.Quantity
}

// ResourceLimitsValidator validates resource requests and limits across workloads
type ResourceLimitsValidator struct {
	client client.Client
	log    logr.Logger
	config ResourceLimitsConfig
}

// NewResourceLimitsValidator creates a new ResourceLimitsValidator with the given client, logger and config
func NewResourceLimitsValidator(client client.Client, log logr.Logger, config ResourceLimitsConfig) *ResourceLimitsValidator {
	return &ResourceLimitsValidator{
		client: client,
		log:    log.WithName("resource-limits-validator"),
		config: config,
	}
}

// GetValidationType returns the validation type identifier for resource limits validation
func (v *ResourceLimitsValidator) GetValidationType() string {
	return "resource_limits_validation"
}

// ValidateCluster performs comprehensive validation of resource limits across the entire cluster
func (v *ResourceLimitsValidator) ValidateCluster(ctx context.Context) error {
	var allErrors []ValidationError

	// Validate Deployments
	if v.config.EnableMissingRequestsValidation || v.config.EnableMissingLimitsValidation || v.config.EnableQoSValidation {
		deploymentErrors, err := v.validateDeploymentResources(ctx)
		if err != nil {
			return fmt.Errorf("failed to validate deployment resources: %w", err)
		}
		allErrors = append(allErrors, deploymentErrors...)

		// Validate StatefulSets
		statefulSetErrors, err := v.validateStatefulSetResources(ctx)
		if err != nil {
			return fmt.Errorf("failed to validate statefulset resources: %w", err)
		}
		allErrors = append(allErrors, statefulSetErrors...)

		// Validate DaemonSets
		daemonSetErrors, err := v.validateDaemonSetResources(ctx)
		if err != nil {
			return fmt.Errorf("failed to validate daemonset resources: %w", err)
		}
		allErrors = append(allErrors, daemonSetErrors...)

		// Validate standalone Pods
		podErrors, err := v.validatePodResources(ctx)
		if err != nil {
			return fmt.Errorf("failed to validate pod resources: %w", err)
		}
		allErrors = append(allErrors, podErrors...)
	}

	// Log all validation errors and update metrics
	for _, validationErr := range allErrors {
		v.log.Info("resource limits validation error found",
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

	v.log.Info("resource limits validation completed", "total_errors", len(allErrors))
	return nil
}

func (v *ResourceLimitsValidator) validateDeploymentResources(ctx context.Context) ([]ValidationError, error) {
	var errors []ValidationError
	var deployments appsv1.DeploymentList

	if err := v.client.List(ctx, &deployments); err != nil {
		return nil, fmt.Errorf("failed to list deployments: %w", err)
	}

	for _, deployment := range deployments.Items {
		containerErrors := v.validateContainerResources(deployment.Spec.Template.Spec.Containers, "Deployment", deployment.Name, deployment.Namespace)
		errors = append(errors, containerErrors...)

		initContainerErrors := v.validateContainerResources(deployment.Spec.Template.Spec.InitContainers, "Deployment", deployment.Name, deployment.Namespace)
		errors = append(errors, initContainerErrors...)
	}

	return errors, nil
}

func (v *ResourceLimitsValidator) validateStatefulSetResources(ctx context.Context) ([]ValidationError, error) {
	var errors []ValidationError
	var statefulSets appsv1.StatefulSetList

	if err := v.client.List(ctx, &statefulSets); err != nil {
		return nil, fmt.Errorf("failed to list statefulsets: %w", err)
	}

	for _, statefulSet := range statefulSets.Items {
		containerErrors := v.validateContainerResources(statefulSet.Spec.Template.Spec.Containers, "StatefulSet", statefulSet.Name, statefulSet.Namespace)
		errors = append(errors, containerErrors...)

		initContainerErrors := v.validateContainerResources(statefulSet.Spec.Template.Spec.InitContainers, "StatefulSet", statefulSet.Name, statefulSet.Namespace)
		errors = append(errors, initContainerErrors...)
	}

	return errors, nil
}

func (v *ResourceLimitsValidator) validateDaemonSetResources(ctx context.Context) ([]ValidationError, error) {
	var errors []ValidationError
	var daemonSets appsv1.DaemonSetList

	if err := v.client.List(ctx, &daemonSets); err != nil {
		return nil, fmt.Errorf("failed to list daemonsets: %w", err)
	}

	for _, daemonSet := range daemonSets.Items {
		containerErrors := v.validateContainerResources(daemonSet.Spec.Template.Spec.Containers, "DaemonSet", daemonSet.Name, daemonSet.Namespace)
		errors = append(errors, containerErrors...)

		initContainerErrors := v.validateContainerResources(daemonSet.Spec.Template.Spec.InitContainers, "DaemonSet", daemonSet.Name, daemonSet.Namespace)
		errors = append(errors, initContainerErrors...)
	}

	return errors, nil
}

func (v *ResourceLimitsValidator) validatePodResources(ctx context.Context) ([]ValidationError, error) {
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

		containerErrors := v.validateContainerResources(pod.Spec.Containers, "Pod", pod.Name, pod.Namespace)
		errors = append(errors, containerErrors...)

		initContainerErrors := v.validateContainerResources(pod.Spec.InitContainers, "Pod", pod.Name, pod.Namespace)
		errors = append(errors, initContainerErrors...)
	}

	return errors, nil
}

func (v *ResourceLimitsValidator) validateContainerResources(containers []corev1.Container, resourceType, resourceName, namespace string) []ValidationError {
	var errors []ValidationError

	for _, container := range containers {
		// Check for missing resource requests
		if v.config.EnableMissingRequestsValidation {
			if container.Resources.Requests == nil ||
				(container.Resources.Requests.Cpu().IsZero() && container.Resources.Requests.Memory().IsZero()) {
				errors = append(errors, ValidationError{
					ResourceType:   resourceType,
					ResourceName:   resourceName,
					Namespace:      namespace,
					ValidationType: "missing_resource_requests",
					Message:        fmt.Sprintf("Container '%s' has no resource requests defined", container.Name),
				})
			} else {
				// Check minimum CPU request
				if v.config.MinCPURequest != nil && container.Resources.Requests.Cpu().Cmp(*v.config.MinCPURequest) < 0 {
					errors = append(errors, ValidationError{
						ResourceType:   resourceType,
						ResourceName:   resourceName,
						Namespace:      namespace,
						ValidationType: "insufficient_cpu_request",
						Message:        fmt.Sprintf("Container '%s' CPU request %s is below minimum %s", container.Name, container.Resources.Requests.Cpu().String(), v.config.MinCPURequest.String()),
					})
				}

				// Check minimum memory request
				if v.config.MinMemoryRequest != nil && container.Resources.Requests.Memory().Cmp(*v.config.MinMemoryRequest) < 0 {
					errors = append(errors, ValidationError{
						ResourceType:   resourceType,
						ResourceName:   resourceName,
						Namespace:      namespace,
						ValidationType: "insufficient_memory_request",
						Message:        fmt.Sprintf("Container '%s' memory request %s is below minimum %s", container.Name, container.Resources.Requests.Memory().String(), v.config.MinMemoryRequest.String()),
					})
				}
			}
		}

		// Check for missing resource limits
		if v.config.EnableMissingLimitsValidation {
			if container.Resources.Limits == nil ||
				(container.Resources.Limits.Cpu().IsZero() && container.Resources.Limits.Memory().IsZero()) {
				errors = append(errors, ValidationError{
					ResourceType:   resourceType,
					ResourceName:   resourceName,
					Namespace:      namespace,
					ValidationType: "missing_resource_limits",
					Message:        fmt.Sprintf("Container '%s' has no resource limits defined", container.Name),
				})
			}
		}

		// Check QoS class implications
		if v.config.EnableQoSValidation {
			qosIssues := v.analyzeQoSClass(container)
			for _, issue := range qosIssues {
				errors = append(errors, ValidationError{
					ResourceType:   resourceType,
					ResourceName:   resourceName,
					Namespace:      namespace,
					ValidationType: "qos_class_issue",
					Message:        fmt.Sprintf("Container '%s': %s", container.Name, issue),
				})
			}
		}
	}

	return errors
}

func (v *ResourceLimitsValidator) analyzeQoSClass(container corev1.Container) []string {
	var issues []string

	hasRequests := container.Resources.Requests != nil && (!container.Resources.Requests.Cpu().IsZero() || !container.Resources.Requests.Memory().IsZero())
	hasLimits := container.Resources.Limits != nil && (!container.Resources.Limits.Cpu().IsZero() || !container.Resources.Limits.Memory().IsZero())

	if !hasRequests && !hasLimits {
		issues = append(issues, "BestEffort QoS: no resource constraints, can be killed first under pressure")
	} else if hasRequests && hasLimits {
		// Check if requests equal limits (Guaranteed QoS)
		if container.Resources.Requests != nil && container.Resources.Limits != nil {
			cpuRequestsEqualLimits := container.Resources.Requests.Cpu().Equal(*container.Resources.Limits.Cpu())
			memoryRequestsEqualLimits := container.Resources.Requests.Memory().Equal(*container.Resources.Limits.Memory())

			if !cpuRequestsEqualLimits || !memoryRequestsEqualLimits {
				issues = append(issues, "Burstable QoS: requests != limits, may face throttling under pressure")
			}
		}
	} else if hasRequests && !hasLimits {
		issues = append(issues, "Burstable QoS: has requests but no limits, may consume unlimited resources")
	} else if !hasRequests && hasLimits {
		issues = append(issues, "Burstable QoS: has limits but no requests, requests will default to limits")
	}

	return issues
}