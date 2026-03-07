// Copyright 2025 Russell Ferriday
// Licensed under the Apache License, Version 2.0
//
// Kogaro - Kubernetes Configuration Hygiene Agent

package validators

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Helper methods for NetworkingValidator

// podHasPort checks if a pod has a container with the specified port.
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

// hasNoReadyEndpointsInSlices checks if all EndpointSlices have no ready endpoints.
func (v *NetworkingValidator) hasNoReadyEndpointsInSlices(endpointSlices []discoveryv1.EndpointSlice) bool {
	for _, eps := range endpointSlices {
		for _, endpoint := range eps.Endpoints {
			if endpoint.Conditions.Ready != nil && *endpoint.Conditions.Ready {
				return false
			}
		}
	}
	return true
}

// isSpecialService checks if a service should be skipped from validation.
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

// isSystemPod checks if a pod is in a system namespace.
func (v *NetworkingValidator) isSystemPod(pod corev1.Pod) bool {
	return v.sharedConfig.IsNetworkingExcludedNamespace(pod.Namespace)
}

// isPodTypicallyUnexposed checks if a pod typically doesn't need a service.
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

// hasDefaultDenyPolicy checks if a namespace has a default deny NetworkPolicy.
func (v *NetworkingValidator) hasDefaultDenyPolicy(policies []networkingv1.NetworkPolicy, namespace string) bool {
	for _, policy := range policies {
		if policy.Namespace != namespace {
			continue
		}

		if v.isDefaultDenyPolicy(policy) {
			return true
		}
	}
	return false
}

// isDefaultDenyPolicy checks if a NetworkPolicy is a default deny policy.
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

// findPodsMatchingPolicy returns pods that match a NetworkPolicy's pod selector.
func (v *NetworkingValidator) findPodsMatchingPolicy(policy networkingv1.NetworkPolicy, pods []corev1.Pod) []corev1.Pod {
	var matchingPods []corev1.Pod

	selector, err := metav1.LabelSelectorAsSelector(&policy.Spec.PodSelector)
	if err != nil {
		v.log.Error(err, "failed to parse NetworkPolicy pod selector",
			"policy", policy.Name,
			"namespace", policy.Namespace)
		return matchingPods
	}

	for _, pod := range pods {
		if selector.Matches(labels.Set(pod.Labels)) {
			matchingPods = append(matchingPods, pod)
		}
	}

	return matchingPods
}

// filterReadyPods returns only pods that are in Ready condition.
func (v *NetworkingValidator) filterReadyPods(pods []corev1.Pod) []corev1.Pod {
	var readyPods []corev1.Pod

	for _, pod := range pods {
		if IsPodReady(pod) {
			readyPods = append(readyPods, pod)
		}
	}

	return readyPods
}

// isSystemNamespace checks if a namespace should be excluded from networking validation.
func (v *NetworkingValidator) isSystemNamespace(namespace string) bool {
	return v.sharedConfig.IsNetworkingExcludedNamespace(namespace)
}

// getPoliciesInNamespace returns all NetworkPolicies in the specified namespace.
func (v *NetworkingValidator) getPoliciesInNamespace(policies []networkingv1.NetworkPolicy, namespace string) []networkingv1.NetworkPolicy {
	var namespacePolicies []networkingv1.NetworkPolicy
	for _, policy := range policies {
		if policy.Namespace == namespace {
			namespacePolicies = append(namespacePolicies, policy)
		}
	}
	return namespacePolicies
}

// getServicePortNames returns a list of port names/numbers for a service.
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
