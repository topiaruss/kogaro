// Copyright 2025 Russell Ferriday
// Licensed under the Apache License, Version 2.0
//
// Kogaro - Kubernetes Configuration Hygiene Agent

// Package validators provides resource limits validation functionality.
//
// This package implements validation of resource requests and limits for
// Kubernetes workloads, detecting pods without proper resource constraints
// which can lead to resource contention and cluster instability.
package validators

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/topiaruss/kogaro/internal/metrics"
)

const (
	// DeploymentType represents a Kubernetes Deployment resource type
	DeploymentType = "Deployment"
	// StatefulSetType represents a Kubernetes StatefulSet resource type
	StatefulSetType = "StatefulSet"
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
	client               client.Client
	log                  logr.Logger
	config               ResourceLimitsConfig
	sharedConfig         SharedConfig
	lastValidationErrors []ValidationError
	logReceiver          LogReceiver
}

// NewResourceLimitsValidator creates a new ResourceLimitsValidator with the given client, logger and config
func NewResourceLimitsValidator(client client.Client, log logr.Logger, config ResourceLimitsConfig) *ResourceLimitsValidator {
	return &ResourceLimitsValidator{
		client:       client,
		log:          log.WithName("resource-limits-validator"),
		config:       config,
		sharedConfig: DefaultSharedConfig(),
	}
}

// SetClient updates the client used by the validator
func (v *ResourceLimitsValidator) SetClient(c client.Client) {
	v.client = c
}

// SetLogReceiver updates the log receiver used by the validator
func (v *ResourceLimitsValidator) SetLogReceiver(lr LogReceiver) {
	v.logReceiver = lr
}

// GetLastValidationErrors returns the errors from the last validation run
func (v *ResourceLimitsValidator) GetLastValidationErrors() []ValidationError {
	return v.lastValidationErrors
}

// GetValidationType returns the validation type identifier for resource limits validation
func (v *ResourceLimitsValidator) GetValidationType() string {
	return "resource_limits_validation"
}

// ValidateCluster performs comprehensive validation of resource limits across the entire cluster
func (v *ResourceLimitsValidator) ValidateCluster(ctx context.Context) error {
	metrics.ValidationRuns.Inc()

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
		// Always use LogReceiver for consistent dependency injection
		v.logReceiver.LogValidationError(
			"resource_limits",
			validationErr.ResourceType,
			validationErr.ResourceName,
			validationErr.Namespace,
			validationErr.ValidationType,
			validationErr.Message,
		)

		// Use new temporal-aware metrics recording
		metrics.RecordValidationErrorWithState(
			validationErr.ResourceType,
			validationErr.ResourceName,
			validationErr.Namespace,
			validationErr.ValidationType,
			string(validationErr.Severity),
			false, // expectedPattern - false for actual errors
		)
	}

	v.log.Info("validation completed", "validator_type", "resource_limits", "total_errors", len(allErrors))

	// Store errors for CLI reporting
	v.lastValidationErrors = allErrors
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
				errorCode := v.getResourceLimitsErrorCode("missing_resource_requests", resourceType, "", false)
				errors = append(errors, NewValidationErrorWithCode(resourceType, resourceName, namespace, "missing_resource_requests", errorCode, fmt.Sprintf("Container '%s' has no resource requests defined", container.Name)).
					WithSeverity(SeverityError).
					WithRemediationHint(fmt.Sprintf("Add resource requests to prevent resource contention (e.g., cpu: %s, memory: %s)", v.sharedConfig.DefaultResourceRecommendations.DefaultCPURequest, v.sharedConfig.DefaultResourceRecommendations.DefaultMemoryRequest)).
					WithRelatedResources(fmt.Sprintf("Container/%s", container.Name)).
					WithDetail("container_name", container.Name).
					WithDetail("recommended_cpu", v.sharedConfig.DefaultResourceRecommendations.DefaultCPURequest).
					WithDetail("recommended_memory", v.sharedConfig.DefaultResourceRecommendations.DefaultMemoryRequest))
			} else {
				// Check minimum CPU request
				if v.config.MinCPURequest != nil && container.Resources.Requests.Cpu().Cmp(*v.config.MinCPURequest) < 0 {
					errorCode := v.getResourceLimitsErrorCode("insufficient_cpu_request", resourceType, "", true)
					errors = append(errors, NewValidationErrorWithCode(resourceType, resourceName, namespace, "insufficient_cpu_request", errorCode, fmt.Sprintf("Container '%s' CPU request %s is below minimum %s", container.Name, container.Resources.Requests.Cpu().String(), v.config.MinCPURequest.String())).
						WithSeverity(SeverityError).
						WithRemediationHint(fmt.Sprintf("Increase CPU request to at least %s to meet minimum requirements", v.config.MinCPURequest.String())).
						WithRelatedResources(fmt.Sprintf("Container/%s", container.Name)).
						WithDetail("container_name", container.Name).
						WithDetail("current_cpu_request", container.Resources.Requests.Cpu().String()).
						WithDetail("minimum_cpu_request", v.config.MinCPURequest.String()))
				}

				// Check minimum memory request
				if v.config.MinMemoryRequest != nil && container.Resources.Requests.Memory().Cmp(*v.config.MinMemoryRequest) < 0 {
					errorCode := v.getResourceLimitsErrorCode("insufficient_memory_request", resourceType, "", true)
					errors = append(errors, NewValidationErrorWithCode(resourceType, resourceName, namespace, "insufficient_memory_request", errorCode, fmt.Sprintf("Container '%s' memory request %s is below minimum %s", container.Name, container.Resources.Requests.Memory().String(), v.config.MinMemoryRequest.String())).
						WithSeverity(SeverityError).
						WithRemediationHint(fmt.Sprintf("Increase memory request to at least %s to meet minimum requirements", v.config.MinMemoryRequest.String())).
						WithRelatedResources(fmt.Sprintf("Container/%s", container.Name)).
						WithDetail("container_name", container.Name).
						WithDetail("current_memory_request", container.Resources.Requests.Memory().String()).
						WithDetail("minimum_memory_request", v.config.MinMemoryRequest.String()))
				}
			}
		}

		// Check for missing resource limits
		if v.config.EnableMissingLimitsValidation {
			if container.Resources.Limits == nil ||
				(container.Resources.Limits.Cpu().IsZero() && container.Resources.Limits.Memory().IsZero()) {
				// Check if container has resource requests to determine the error code context
				hasRequests := container.Resources.Requests != nil &&
					(!container.Resources.Requests.Cpu().IsZero() || !container.Resources.Requests.Memory().IsZero())
				errorCode := v.getResourceLimitsErrorCode("missing_resource_limits", resourceType, "", hasRequests)
				errors = append(errors, NewValidationErrorWithCode(resourceType, resourceName, namespace, "missing_resource_limits", errorCode, fmt.Sprintf("Container '%s' has no resource limits defined", container.Name)).
					WithSeverity(SeverityError).
					WithRemediationHint(fmt.Sprintf("Add resource limits to prevent resource overconsumption (e.g., cpu: %s, memory: %s)", v.sharedConfig.DefaultResourceRecommendations.DefaultCPULimit, v.sharedConfig.DefaultResourceRecommendations.DefaultMemoryLimit)).
					WithRelatedResources(fmt.Sprintf("Container/%s", container.Name)).
					WithDetail("container_name", container.Name).
					WithDetail("recommended_cpu_limit", v.sharedConfig.DefaultResourceRecommendations.DefaultCPULimit).
					WithDetail("recommended_memory_limit", v.sharedConfig.DefaultResourceRecommendations.DefaultMemoryLimit))
			}
		}

		// Check QoS class implications
		if v.config.EnableQoSValidation {
			qosIssues := v.analyzeQoSClass(container)
			for _, issue := range qosIssues {
				severity := SeverityWarning
				var remediationHint string

				if strings.Contains(issue, "BestEffort") {
					severity = SeverityError
					remediationHint = "Add both resource requests and limits for predictable scheduling and resource management"
				} else if strings.Contains(issue, "requests != limits") {
					severity = SeverityWarning
					remediationHint = "Consider setting requests equal to limits for Guaranteed QoS, or optimize resource allocation"
				} else if strings.Contains(issue, "no limits") {
					severity = SeverityWarning
					remediationHint = "Add resource limits to prevent unlimited resource consumption"
				} else {
					remediationHint = "Review resource configuration for optimal QoS class assignment"
				}

				errorCode := v.getResourceLimitsErrorCode("qos_class_issue", resourceType, issue, false)
				errors = append(errors, NewValidationErrorWithCode(resourceType, resourceName, namespace, "qos_class_issue", errorCode, fmt.Sprintf("Container '%s': %s", container.Name, issue)).
					WithSeverity(severity).
					WithRemediationHint(remediationHint).
					WithRelatedResources(fmt.Sprintf("Container/%s", container.Name)).
					WithDetail("container_name", container.Name).
					WithDetail("qos_issue_type", issue))
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

// getResourceLimitsErrorCode returns the appropriate error code for resource limits validations
func (v *ResourceLimitsValidator) getResourceLimitsErrorCode(validationType, resourceType, issueDetail string, hasRequests bool) string {
	switch validationType {
	case "missing_resource_requests":
		switch resourceType {
		case DeploymentType:
			return "KOGARO-RES-001"
		case StatefulSetType:
			return "KOGARO-RES-002"
		}
	case "missing_resource_limits":
		switch resourceType {
		case DeploymentType:
			if hasRequests {
				// Has requests but missing limits
				return "KOGARO-RES-004"
			}
			// Complete absence of resource constraints
			return "KOGARO-RES-003"
		case StatefulSetType:
			return "KOGARO-RES-005"
		}
	case "insufficient_cpu_request":
		return "KOGARO-RES-006"
	case "insufficient_memory_request":
		return "KOGARO-RES-007"
	case "qos_class_issue":
		// Determine QoS issue type from the issue detail
		if strings.Contains(issueDetail, "BestEffort") {
			switch resourceType {
			case DeploymentType:
				return "KOGARO-RES-008"
			case StatefulSetType:
				return "KOGARO-RES-009"
			}
		} else if strings.Contains(issueDetail, "Burstable") && strings.Contains(issueDetail, "requests != limits") {
			return "KOGARO-RES-010"
		}
	}
	// Fallback - should not happen if mapping is complete
	return "KOGARO-RES-UNKNOWN"
}
