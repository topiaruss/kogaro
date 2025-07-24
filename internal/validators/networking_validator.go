// Copyright 2025 Russell Ferriday
// Licensed under the Apache License, Version 2.0
//
// Kogaro - Kubernetes Configuration Hygiene Agent

// Package validators provides networking configuration validation functionality.
//
// This package implements validation of networking configurations within
// a Kubernetes cluster, detecting network connectivity issues that could
// cause service disruptions. It validates Service selectors, NetworkPolicy
// coverage, and Ingress connectivity to ensure proper network communication.
package validators

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/topiaruss/kogaro/internal/metrics"
)

// NetworkingConfig defines which networking validation checks to perform
type NetworkingConfig struct {
	EnableServiceValidation       bool
	EnableNetworkPolicyValidation bool
	EnableIngressValidation       bool
	// Namespaces that require NetworkPolicy coverage
	PolicyRequiredNamespaces []string
	// Enable warnings for pods not exposed by services
	WarnUnexposedPods bool
}

// NetworkingValidator validates networking configurations across workloads
type NetworkingValidator struct {
	client               client.Client
	log                  logr.Logger
	config               NetworkingConfig
	sharedConfig         SharedConfig
	lastValidationErrors []ValidationError
	logReceiver          LogReceiver
}

// NewNetworkingValidator creates a new NetworkingValidator with the given client, logger and config
func NewNetworkingValidator(client client.Client, log logr.Logger, config NetworkingConfig) *NetworkingValidator {
	return &NetworkingValidator{
		client:       client,
		log:          log.WithName("networking-validator"),
		config:       config,
		sharedConfig: DefaultSharedConfig(),
	}
}

// SetClient updates the client used by the validator
func (v *NetworkingValidator) SetClient(c client.Client) {
	v.client = c
}

// SetLogReceiver sets the log receiver for validation errors
func (v *NetworkingValidator) SetLogReceiver(lr LogReceiver) {
	v.logReceiver = lr
}

// GetLastValidationErrors returns the errors from the last validation run
func (v *NetworkingValidator) GetLastValidationErrors() []ValidationError {
	return v.lastValidationErrors
}

// GetValidationType returns the validation type identifier for networking validation
func (v *NetworkingValidator) GetValidationType() string {
	return "networking_validation"
}

// ValidateCluster performs comprehensive validation of networking configurations across the entire cluster
func (v *NetworkingValidator) ValidateCluster(ctx context.Context) error {
	metrics.ValidationRuns.Inc()

	var allErrors []ValidationError

	// Validate Service connectivity
	if v.config.EnableServiceValidation {
		serviceErrors, err := v.validateServiceConnectivity(ctx)
		if err != nil {
			return fmt.Errorf("failed to validate service connectivity: %w", err)
		}
		allErrors = append(allErrors, serviceErrors...)
	}

	// Validate NetworkPolicy coverage
	if v.config.EnableNetworkPolicyValidation {
		networkPolicyErrors, err := v.validateNetworkPolicyCoverage(ctx)
		if err != nil {
			return fmt.Errorf("failed to validate networkpolicy coverage: %w", err)
		}
		allErrors = append(allErrors, networkPolicyErrors...)
	}

	// Validate Ingress connectivity
	if v.config.EnableIngressValidation {
		ingressErrors, err := v.validateIngressConnectivity(ctx)
		if err != nil {
			return fmt.Errorf("failed to validate ingress connectivity: %w", err)
		}
		allErrors = append(allErrors, ingressErrors...)
	}

	// Log all validation errors and update metrics
	for _, validationErr := range allErrors {
		// Always use LogReceiver for consistent dependency injection
		v.logReceiver.LogValidationError("networking", validationErr)

		// Use new temporal-aware metrics recording
		metrics.RecordValidationErrorWithState(
			validationErr.ResourceType,
			validationErr.ResourceName,
			validationErr.Namespace,
			validationErr.ValidationType,
			string(validationErr.Severity),
			validationErr.ErrorCode,
			false, // expectedPattern - false for actual errors
		)
	}

	v.log.Info("validation completed", "validator_type", "networking", "total_errors", len(allErrors))

	// Store errors for CLI reporting
	v.lastValidationErrors = allErrors
	return nil
}

func (v *NetworkingValidator) validateServiceConnectivity(ctx context.Context) ([]ValidationError, error) {
	var errors []ValidationError

	// Get all Services
	var services corev1.ServiceList
	if err := v.client.List(ctx, &services); err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	// Get all Pods
	var pods corev1.PodList
	if err := v.client.List(ctx, &pods); err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	// Get all Endpoints
	var endpoints corev1.EndpointsList
	if err := v.client.List(ctx, &endpoints); err != nil {
		return nil, fmt.Errorf("failed to list endpoints: %w", err)
	}

	// Create maps for efficient lookup
	podsByNamespace := make(map[string][]corev1.Pod)
	// TODO: Migrate from deprecated corev1.Endpoints to discoveryv1.EndpointSlice
	endpointsByName := make(map[string]corev1.Endpoints) //nolint:staticcheck

	for _, pod := range pods.Items {
		podsByNamespace[pod.Namespace] = append(podsByNamespace[pod.Namespace], pod)
	}

	for _, ep := range endpoints.Items {
		key := fmt.Sprintf("%s/%s", ep.Namespace, ep.Name)
		endpointsByName[key] = ep
	}

	// Validate each service
	for _, service := range services.Items {
		// Skip headless services and special services
		if v.isSpecialService(service) {
			continue
		}

		serviceErrors := v.validateService(service, podsByNamespace[service.Namespace], endpointsByName)
		errors = append(errors, serviceErrors...)
	}

	// Optionally warn about unexposed pods
	if v.config.WarnUnexposedPods {
		unexposedErrors := v.findUnexposedPods(pods.Items, services.Items)
		errors = append(errors, unexposedErrors...)
	}

	return errors, nil
}

func (v *NetworkingValidator) validateService(service corev1.Service, namespacePods []corev1.Pod, endpointsMap map[string]corev1.Endpoints) []ValidationError { //nolint:staticcheck
	var errors []ValidationError

	// Check if service selector matches any pods
	if len(service.Spec.Selector) > 0 {
		matchingPods := v.findMatchingPods(service.Spec.Selector, namespacePods)

		if len(matchingPods) == 0 {
			errorCode := v.getNetworkingErrorCode("service_selector_mismatch")
			errors = append(errors, NewValidationErrorWithCode("Service", service.Name, service.Namespace, "service_selector_mismatch", errorCode, fmt.Sprintf("Service selector %v does not match any pods", service.Spec.Selector)).
				WithSeverity(SeverityWarning).
				WithRemediationHint("Update service selector to match existing pod labels or deploy pods with matching labels").
				WithRelatedResources(fmt.Sprintf("Service/%s", service.Name)).
				WithDetail("service_selector", fmt.Sprintf("%v", service.Spec.Selector)).
				WithDetail("namespace_pod_count", fmt.Sprintf("%d", len(namespacePods))))
		}

		// Check if service has endpoints
		endpointsKey := fmt.Sprintf("%s/%s", service.Namespace, service.Name)
		if endpoints, exists := endpointsMap[endpointsKey]; exists {
			if v.hasNoReadyEndpoints(endpoints) {
				errorCode := v.getNetworkingErrorCode("service_no_endpoints")
				errors = append(errors, NewValidationErrorWithCode("Service", service.Name, service.Namespace, "service_no_endpoints", errorCode, "Service has no ready endpoints despite matching pods").
					WithSeverity(SeverityError).
					WithRemediationHint("Check pod readiness probes and ensure pods are in Ready state").
					WithRelatedResources(fmt.Sprintf("Service/%s", service.Name), fmt.Sprintf("Endpoints/%s", service.Name)).
					WithDetail("matching_pods_count", fmt.Sprintf("%d", len(matchingPods))).
					WithDetail("endpoints_subset_count", fmt.Sprintf("%d", len(endpoints.Subsets))))
			}
		} else {
			errorCode := v.getNetworkingErrorCode("service_no_endpoints")
			errors = append(errors, NewValidationErrorWithCode("Service", service.Name, service.Namespace, "service_no_endpoints", errorCode, "Service has no endpoints object").
				WithSeverity(SeverityError).
				WithRemediationHint("Verify service selector matches pod labels and pods are ready").
				WithRelatedResources(fmt.Sprintf("Service/%s", service.Name)).
				WithDetail("endpoints_missing", "true").
				WithDetail("matching_pods_count", fmt.Sprintf("%d", len(matchingPods))))
		}

		// Validate port matching between service and pods
		portErrors := v.validateServicePorts(service, matchingPods)
		errors = append(errors, portErrors...)
	}

	return errors
}

func (v *NetworkingValidator) validateServicePorts(service corev1.Service, matchingPods []corev1.Pod) []ValidationError {
	var errors []ValidationError

	if len(matchingPods) == 0 {
		return errors
	}

	// Check if service ports match container ports in pods
	for _, servicePort := range service.Spec.Ports {
		if servicePort.TargetPort.IntVal == 0 && servicePort.TargetPort.StrVal == "" {
			// TargetPort defaults to Port if not specified
			continue
		}

		portFound := false
		for _, pod := range matchingPods {
			if v.podHasPort(pod, servicePort.TargetPort) {
				portFound = true
				break
			}
		}

		if !portFound {
			errorCode := v.getNetworkingErrorCode("service_port_mismatch")
			errors = append(errors, NewValidationErrorWithCode("Service", service.Name, service.Namespace, "service_port_mismatch", errorCode, fmt.Sprintf("Service port %s (target: %s) does not match any container ports in matching pods", servicePort.Name, servicePort.TargetPort.String())).
				WithSeverity(SeverityError).
				WithRemediationHint("Update service targetPort to match container ports or add the missing port to container specifications").
				WithRelatedResources(fmt.Sprintf("Service/%s", service.Name)).
				WithDetail("service_port_name", servicePort.Name).
				WithDetail("target_port", servicePort.TargetPort.String()).
				WithDetail("service_port_number", fmt.Sprintf("%d", servicePort.Port)))
		}
	}

	return errors
}

func (v *NetworkingValidator) podHasPort(pod corev1.Pod, targetPort intstr.IntOrString) bool {
	for _, container := range pod.Spec.Containers {
		for _, port := range container.Ports {
			if targetPort.Type == intstr.Int {
				if port.ContainerPort == targetPort.IntVal {
					return true
				}
			} else {
				if port.Name == targetPort.StrVal {
					return true
				}
			}
		}
	}
	return false
}

func (v *NetworkingValidator) findMatchingPods(selector map[string]string, pods []corev1.Pod) []corev1.Pod {
	var matchingPods []corev1.Pod

	selectorLabels := labels.Set(selector)

	for _, pod := range pods {
		if selectorLabels.AsSelector().Matches(labels.Set(pod.Labels)) {
			matchingPods = append(matchingPods, pod)
		}
	}

	return matchingPods
}

func (v *NetworkingValidator) hasNoReadyEndpoints(endpoints corev1.Endpoints) bool { //nolint:staticcheck
	for _, subset := range endpoints.Subsets {
		if len(subset.Addresses) > 0 {
			return false
		}
	}
	return true
}

func (v *NetworkingValidator) isSpecialService(service corev1.Service) bool {
	// Skip headless services
	if service.Spec.ClusterIP == "None" {
		return true
	}

	// Skip services without selectors (external services)
	if len(service.Spec.Selector) == 0 {
		return true
	}

	// Skip system services
	if v.sharedConfig.IsNetworkingExcludedNamespace(service.Namespace) {
		return true
	}

	return false
}

func (v *NetworkingValidator) findUnexposedPods(pods []corev1.Pod, services []corev1.Service) []ValidationError {
	var errors []ValidationError

	// Create map of service selectors by namespace
	servicesByNamespace := make(map[string][]corev1.Service)
	for _, service := range services {
		if !v.isSpecialService(service) {
			servicesByNamespace[service.Namespace] = append(servicesByNamespace[service.Namespace], service)
		}
	}

	for _, pod := range pods {
		// Skip system pods and pods owned by controllers that typically don't need services
		if v.isSystemPod(pod) || v.isPodTypicallyUnexposed(pod) {
			continue
		}

		// Check if pod is matched by any service
		isExposed := false
		for _, service := range servicesByNamespace[pod.Namespace] {
			if v.findMatchingPods(service.Spec.Selector, []corev1.Pod{pod}) != nil {
				isExposed = true
				break
			}
		}

		if !isExposed {
			errorCode := v.getNetworkingErrorCode("pod_no_service")
			errors = append(errors, NewValidationErrorWithCode("Pod", pod.Name, pod.Namespace, "pod_no_service", errorCode, "Pod is not exposed by any Service (consider if this is intentional)").
				WithSeverity(SeverityInfo).
				WithRemediationHint("Create a Service to expose this pod or verify this is intentional for batch/worker pods").
				WithRelatedResources(fmt.Sprintf("Pod/%s", pod.Name)).
				WithDetail("pod_labels", fmt.Sprintf("%v", pod.Labels)).
				WithDetail("namespace_services_count", fmt.Sprintf("%d", len(servicesByNamespace[pod.Namespace]))))
		}
	}

	return errors
}

func (v *NetworkingValidator) isSystemPod(pod corev1.Pod) bool {
	return v.sharedConfig.IsNetworkingExcludedNamespace(pod.Namespace)
}

func (v *NetworkingValidator) isPodTypicallyUnexposed(pod corev1.Pod) bool {
	// Check if pod is owned by a batch workload (typically don't need services)
	for _, owner := range pod.OwnerReferences {
		if v.sharedConfig.IsBatchOwnerKind(owner.Kind) {
			return true
		}
	}

	// Check for common patterns in pod names that suggest they don't need services
	if v.sharedConfig.IsUnexposedPodPattern(pod.Name) {
		return true
	}

	return false
}

func (v *NetworkingValidator) validateNetworkPolicyCoverage(ctx context.Context) ([]ValidationError, error) {
	var errors []ValidationError

	// Get all NetworkPolicies
	var networkPolicies networkingv1.NetworkPolicyList
	if err := v.client.List(ctx, &networkPolicies); err != nil {
		return nil, fmt.Errorf("failed to list networkpolicies: %w", err)
	}

	// Get all Pods
	var pods corev1.PodList
	if err := v.client.List(ctx, &pods); err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	// Get all Namespaces
	var namespaces corev1.NamespaceList
	if err := v.client.List(ctx, &namespaces); err != nil {
		return nil, fmt.Errorf("failed to list namespaces: %w", err)
	}

	// Validate NetworkPolicy coverage
	coverageErrors := v.validatePolicyRequired(namespaces.Items, networkPolicies.Items)
	errors = append(errors, coverageErrors...)

	// Validate NetworkPolicy selectors
	selectorErrors := v.validateNetworkPolicySelectors(networkPolicies.Items, pods.Items)
	errors = append(errors, selectorErrors...)

	return errors, nil
}

func (v *NetworkingValidator) validatePolicyRequired(namespaces []corev1.Namespace, policies []networkingv1.NetworkPolicy) []ValidationError {
	var errors []ValidationError

	// Create map of namespaces with policies
	namespacesWithPolicies := make(map[string]bool)
	for _, policy := range policies {
		namespacesWithPolicies[policy.Namespace] = true
	}

	// Check policy-required namespaces
	for _, requiredNS := range v.config.PolicyRequiredNamespaces {
		if !namespacesWithPolicies[requiredNS] {
			// Note: This validation type is not in the CSV - using UNKNOWN error code
			errorCode := v.getNetworkingErrorCode("missing_network_policy_required")
			errors = append(errors, NewValidationErrorWithCode("Namespace", requiredNS, requiredNS, "missing_network_policy_required", errorCode, fmt.Sprintf("Policy-required namespace '%s' has no NetworkPolicies", requiredNS)).
				WithSeverity(SeverityError).
				WithRemediationHint("Create NetworkPolicies with default-deny ingress/egress rules and explicit allow rules for required traffic").
				WithRelatedResources("NetworkPolicy/default-deny-all").
				WithDetail("namespace_type", "policy_required").
				WithDetail("security_risk", "unrestricted_network_access"))
		}
	}

	// Check for missing default deny policies in namespaces with policies
	for _, ns := range namespaces {
		if v.isSystemNamespace(ns.Name) {
			continue
		}

		if namespacesWithPolicies[ns.Name] {
			hasDefaultDeny := v.hasDefaultDenyPolicy(policies, ns.Name)
			if !hasDefaultDeny {
				existingPolicies := v.getPoliciesInNamespace(policies, ns.Name)
				errorCode := v.getNetworkingErrorCode("missing_network_policy_default_deny")
				errors = append(errors, NewValidationErrorWithCode("Namespace", ns.Name, ns.Name, "missing_network_policy_default_deny", errorCode, "Namespace has NetworkPolicies but no default deny policy").
					WithSeverity(SeverityWarning).
					WithRemediationHint("Add a default deny NetworkPolicy to deny all ingress/egress traffic by default, then create specific allow policies").
					WithRelatedResources("NetworkPolicy/default-deny-all").
					WithDetail("existing_policies_count", fmt.Sprintf("%d", len(existingPolicies))).
					WithDetail("security_best_practice", "default_deny_principle"))
			}
		}
	}

	return errors
}

func (v *NetworkingValidator) hasDefaultDenyPolicy(policies []networkingv1.NetworkPolicy, namespace string) bool {
	for _, policy := range policies {
		if policy.Namespace != namespace {
			continue
		}

		// Check if it's a default deny policy (empty selectors with no ingress/egress rules)
		if v.isDefaultDenyPolicy(policy) {
			return true
		}
	}
	return false
}

func (v *NetworkingValidator) isDefaultDenyPolicy(policy networkingv1.NetworkPolicy) bool {
	// A default deny policy typically:
	// 1. Selects all pods (empty podSelector)
	// 2. Has empty ingress and/or egress rules

	if len(policy.Spec.PodSelector.MatchLabels) > 0 {
		return false
	}

	if len(policy.Spec.PodSelector.MatchExpressions) > 0 {
		return false
	}

	// Check if it denies ingress (has ingress policy type but no ingress rules)
	hasIngressDeny := false
	hasEgressDeny := false

	for _, policyType := range policy.Spec.PolicyTypes {
		if policyType == networkingv1.PolicyTypeIngress && len(policy.Spec.Ingress) == 0 {
			hasIngressDeny = true
		}
		if policyType == networkingv1.PolicyTypeEgress && len(policy.Spec.Egress) == 0 {
			hasEgressDeny = true
		}
	}

	return hasIngressDeny || hasEgressDeny
}

func (v *NetworkingValidator) validateNetworkPolicySelectors(policies []networkingv1.NetworkPolicy, pods []corev1.Pod) []ValidationError {
	var errors []ValidationError

	for _, policy := range policies {
		// Check if policy selector matches any pods in its namespace
		namespacePods := v.getPodsInNamespace(pods, policy.Namespace)
		matchingPods := v.findPodsMatchingPolicy(policy, namespacePods)

		if len(matchingPods) == 0 {
			errorCode := v.getNetworkingErrorCode("network_policy_orphaned")
			errors = append(errors, NewValidationErrorWithCode("NetworkPolicy", policy.Name, policy.Namespace, "network_policy_orphaned", errorCode, "NetworkPolicy selector does not match any pods in namespace").
				WithSeverity(SeverityWarning).
				WithRemediationHint("Update NetworkPolicy selector to match existing pods or deploy pods with matching labels").
				WithRelatedResources(fmt.Sprintf("NetworkPolicy/%s", policy.Name)).
				WithDetail("policy_selector", fmt.Sprintf("%v", policy.Spec.PodSelector)).
				WithDetail("namespace_pods_count", fmt.Sprintf("%d", len(namespacePods))))
		}
	}

	return errors
}

func (v *NetworkingValidator) getPodsInNamespace(pods []corev1.Pod, namespace string) []corev1.Pod {
	var namespacePods []corev1.Pod
	for _, pod := range pods {
		if pod.Namespace == namespace {
			namespacePods = append(namespacePods, pod)
		}
	}
	return namespacePods
}

func (v *NetworkingValidator) findPodsMatchingPolicy(policy networkingv1.NetworkPolicy, pods []corev1.Pod) []corev1.Pod {
	var matchingPods []corev1.Pod

	selector, err := metav1.LabelSelectorAsSelector(&policy.Spec.PodSelector)
	if err != nil {
		return matchingPods
	}

	for _, pod := range pods {
		if selector.Matches(labels.Set(pod.Labels)) {
			matchingPods = append(matchingPods, pod)
		}
	}

	return matchingPods
}

func (v *NetworkingValidator) validateIngressConnectivity(ctx context.Context) ([]ValidationError, error) {
	var errors []ValidationError

	// Get all Ingresses
	var ingresses networkingv1.IngressList
	if err := v.client.List(ctx, &ingresses); err != nil {
		return nil, fmt.Errorf("failed to list ingresses: %w", err)
	}

	// Get all Services
	var services corev1.ServiceList
	if err := v.client.List(ctx, &services); err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	// Get all Pods
	var pods corev1.PodList
	if err := v.client.List(ctx, &pods); err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	// Create service lookup map
	serviceMap := make(map[string]corev1.Service)
	for _, service := range services.Items {
		key := fmt.Sprintf("%s/%s", service.Namespace, service.Name)
		serviceMap[key] = service
	}

	// Validate each ingress
	for _, ingress := range ingresses.Items {
		ingressErrors := v.validateIngressBackends(ingress, serviceMap, pods.Items)
		errors = append(errors, ingressErrors...)
	}

	return errors, nil
}

func (v *NetworkingValidator) validateIngressBackends(ingress networkingv1.Ingress, serviceMap map[string]corev1.Service, pods []corev1.Pod) []ValidationError {
	var errors []ValidationError

	// Check default backend
	if ingress.Spec.DefaultBackend != nil && ingress.Spec.DefaultBackend.Service != nil {
		backendErrors := v.validateIngressServiceBackend(ingress, *ingress.Spec.DefaultBackend.Service, serviceMap, pods)
		errors = append(errors, backendErrors...)
	}

	// Check rule backends
	for _, rule := range ingress.Spec.Rules {
		if rule.HTTP != nil {
			for _, path := range rule.HTTP.Paths {
				if path.Backend.Service != nil {
					backendErrors := v.validateIngressServiceBackend(ingress, *path.Backend.Service, serviceMap, pods)
					errors = append(errors, backendErrors...)
				}
			}
		}
	}

	return errors
}

func (v *NetworkingValidator) validateIngressServiceBackend(ingress networkingv1.Ingress, backend networkingv1.IngressServiceBackend, serviceMap map[string]corev1.Service, pods []corev1.Pod) []ValidationError {
	var errors []ValidationError

	serviceKey := fmt.Sprintf("%s/%s", ingress.Namespace, backend.Name)
	service, exists := serviceMap[serviceKey]

	if !exists {
		// This should be caught by reference validator, but let's be thorough
		errorCode := v.getNetworkingErrorCode("ingress_service_missing")
		errors = append(errors, NewValidationErrorWithCode("Ingress", ingress.Name, ingress.Namespace, "ingress_service_missing", errorCode, fmt.Sprintf("Ingress references non-existent service '%s'", backend.Name)).
			WithSeverity(SeverityError).
			WithRemediationHint(fmt.Sprintf("Create service '%s' in namespace '%s' or update Ingress to reference an existing service", backend.Name, ingress.Namespace)).
			WithRelatedResources(fmt.Sprintf("Service/%s", backend.Name)).
			WithDetail("service_name", backend.Name).
			WithDetail("ingress_namespace", ingress.Namespace))
		return errors
	}

	// Check if service port matches
	if backend.Port != (networkingv1.ServiceBackendPort{}) {
		portExists := false
		for _, servicePort := range service.Spec.Ports {
			if backend.Port.Number != 0 && servicePort.Port == backend.Port.Number {
				portExists = true
				break
			}
			if backend.Port.Name != "" && servicePort.Name == backend.Port.Name {
				portExists = true
				break
			}
		}

		if !portExists {
			var portRef string
			if backend.Port.Number != 0 {
				portRef = fmt.Sprintf("%d", backend.Port.Number)
			} else {
				portRef = backend.Port.Name
			}
			errorCode := v.getNetworkingErrorCode("ingress_service_port_mismatch")
			errors = append(errors, NewValidationErrorWithCode("Ingress", ingress.Name, ingress.Namespace, "ingress_service_port_mismatch", errorCode, fmt.Sprintf("Ingress references service '%s' port '%s' that doesn't exist", backend.Name, portRef)).
				WithSeverity(SeverityError).
				WithRemediationHint(fmt.Sprintf("Add port '%s' to service '%s' or update Ingress to reference an existing port", portRef, backend.Name)).
				WithRelatedResources(fmt.Sprintf("Service/%s", backend.Name)).
				WithDetail("service_name", backend.Name).
				WithDetail("referenced_port", portRef).
				WithDetail("available_ports", fmt.Sprintf("%v", v.getServicePortNames(service))))
		}
	}

	// Check if service has ready backend pods
	namespacePods := v.getPodsInNamespace(pods, service.Namespace)
	matchingPods := v.findMatchingPods(service.Spec.Selector, namespacePods)
	readyPods := v.filterReadyPods(matchingPods)

	if len(readyPods) == 0 {
		errorCode := v.getNetworkingErrorCode("ingress_no_backend_pods")
		errors = append(errors, NewValidationErrorWithCode("Ingress", ingress.Name, ingress.Namespace, "ingress_no_backend_pods", errorCode, fmt.Sprintf("Ingress service '%s' has no ready backend pods", backend.Name)).
			WithSeverity(SeverityError).
			WithRemediationHint("Deploy pods with labels matching the service selector and ensure they pass readiness checks").
			WithRelatedResources(fmt.Sprintf("Service/%s", backend.Name)).
			WithDetail("service_name", backend.Name).
			WithDetail("matching_pods_count", fmt.Sprintf("%d", len(matchingPods))).
			WithDetail("ready_pods_count", "0"))
	}

	return errors
}

func (v *NetworkingValidator) filterReadyPods(pods []corev1.Pod) []corev1.Pod {
	var readyPods []corev1.Pod

	for _, pod := range pods {
		if v.isPodReady(pod) {
			readyPods = append(readyPods, pod)
		}
	}

	return readyPods
}

func (v *NetworkingValidator) isPodReady(pod corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func (v *NetworkingValidator) isSystemNamespace(namespace string) bool {
	return v.sharedConfig.IsNetworkingExcludedNamespace(namespace)
}

func (v *NetworkingValidator) getPoliciesInNamespace(policies []networkingv1.NetworkPolicy, namespace string) []networkingv1.NetworkPolicy {
	var namespacePolicies []networkingv1.NetworkPolicy
	for _, policy := range policies {
		if policy.Namespace == namespace {
			namespacePolicies = append(namespacePolicies, policy)
		}
	}
	return namespacePolicies
}

func (v *NetworkingValidator) getServicePortNames(service corev1.Service) []string {
	var portNames []string
	for _, port := range service.Spec.Ports {
		if port.Name != "" {
			portNames = append(portNames, fmt.Sprintf("%s:%d", port.Name, port.Port))
		} else {
			portNames = append(portNames, fmt.Sprintf("%d", port.Port))
		}
	}
	return portNames
}

// getNetworkingErrorCode returns the appropriate error code for networking validations
func (v *NetworkingValidator) getNetworkingErrorCode(validationType string) string {
	switch validationType {
	case "service_selector_mismatch":
		return "KOGARO-NET-001"
	case "service_no_endpoints":
		return "KOGARO-NET-002"
	case "service_port_mismatch":
		return "KOGARO-NET-003"
	case "pod_no_service":
		return "KOGARO-NET-004"
	case "network_policy_orphaned":
		return "KOGARO-NET-005"
	case "missing_network_policy_default_deny":
		return "KOGARO-NET-006"
	case "ingress_service_missing":
		return "KOGARO-NET-007"
	case "ingress_service_port_mismatch":
		return "KOGARO-NET-008"
	case "ingress_no_backend_pods":
		return "KOGARO-NET-009"
	}
	// For undocumented validations or unknown types
	return "KOGARO-NET-UNKNOWN"
}
