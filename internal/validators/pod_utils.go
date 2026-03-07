// Copyright 2025 Russell Ferriday
// Licensed under the Apache License, Version 2.0
//
// Kogaro - Kubernetes Configuration Hygiene Agent

// Copyright 2025 Russell Ferriday
// Licensed under the Apache License, Version 2.0
//
// Kogaro - Kubernetes Configuration Hygiene Agent

package validators

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// GetPodsInNamespace returns all pods in the specified namespace from a given list of pods.
func GetPodsInNamespace(pods []corev1.Pod, namespace string) []corev1.Pod {
	var namespacePods []corev1.Pod
	for _, pod := range pods {
		if pod.Namespace == namespace {
			namespacePods = append(namespacePods, pod)
		}
	}
	return namespacePods
}

// FindMatchingPods returns pods that match the given label selector.
func FindMatchingPods(pods []corev1.Pod, selector map[string]string) []corev1.Pod {
	var matchingPods []corev1.Pod

	selectorLabels := labels.Set(selector)

	for _, pod := range pods {
		if selectorLabels.AsSelector().Matches(labels.Set(pod.Labels)) {
			matchingPods = append(matchingPods, pod)
		}
	}

	return matchingPods
}

// IsPodReady returns true if the pod is in Ready condition.
func IsPodReady(pod corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
