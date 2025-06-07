// Copyright 2025 Russell Ferriday
// Licensed under the Apache License, Version 2.0
//
// Kogaro - Kubernetes Configuration Hygiene Agent

// Package utils provides common utilities for Kubernetes resource validation.
//
// This package contains shared utility functions used across all validators
// to reduce code duplication and provide consistent behavior for common
// validation tasks such as namespace classification, pod filtering, and
// resource existence checks.
package utils

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SystemNamespaces defines the list of Kubernetes system namespaces
var SystemNamespaces = []string{
	"kube-system",
	"kube-public",
	"kube-node-lease",
	"default",
}

// IsSystemNamespace checks if the given namespace is a Kubernetes system namespace
func IsSystemNamespace(namespace string) bool {
	for _, systemNS := range SystemNamespaces {
		if namespace == systemNS {
			return true
		}
	}
	return false
}

// IsSystemPod checks if a pod is running in a system namespace
func IsSystemPod(pod corev1.Pod) bool {
	systemNamespaces := []string{"kube-system", "kube-public", "kube-node-lease"}
	for _, ns := range systemNamespaces {
		if pod.Namespace == ns {
			return true
		}
	}
	return false
}

// HasOwnerReferences checks if a pod has any owner references (managed by controllers)
func HasOwnerReferences(pod corev1.Pod) bool {
	return len(pod.OwnerReferences) > 0
}

// IsOwnedByJobOrCronJob checks if a pod is owned by a Job or CronJob
func IsOwnedByJobOrCronJob(pod corev1.Pod) bool {
	for _, owner := range pod.OwnerReferences {
		if owner.Kind == "Job" || owner.Kind == "CronJob" {
			return true
		}
	}
	return false
}

// IsPodReady checks if a pod is in Ready condition
func IsPodReady(pod corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// FilterPodsByNamespace returns pods that belong to the specified namespace
func FilterPodsByNamespace(pods []corev1.Pod, namespace string) []corev1.Pod {
	var namespacePods []corev1.Pod
	for _, pod := range pods {
		if pod.Namespace == namespace {
			namespacePods = append(namespacePods, pod)
		}
	}
	return namespacePods
}

// FindPodsMatchingSelector returns pods that match the given label selector
func FindPodsMatchingSelector(selector map[string]string, pods []corev1.Pod) []corev1.Pod {
	var matchingPods []corev1.Pod
	
	selectorLabels := labels.Set(selector)
	
	for _, pod := range pods {
		if selectorLabels.AsSelector().Matches(labels.Set(pod.Labels)) {
			matchingPods = append(matchingPods, pod)
		}
	}
	
	return matchingPods
}

// FindPodsMatchingLabelSelector returns pods that match the given metav1.LabelSelector
func FindPodsMatchingLabelSelector(labelSelector *metav1.LabelSelector, pods []corev1.Pod) ([]corev1.Pod, error) {
	var matchingPods []corev1.Pod
	
	selector, err := metav1.LabelSelectorAsSelector(labelSelector)
	if err != nil {
		return matchingPods, err
	}
	
	for _, pod := range pods {
		if selector.Matches(labels.Set(pod.Labels)) {
			matchingPods = append(matchingPods, pod)
		}
	}
	
	return matchingPods, nil
}

// ResourceExistsChecker provides methods to check if various Kubernetes resources exist
type ResourceExistsChecker struct {
	client client.Client
}

// NewResourceExistsChecker creates a new ResourceExistsChecker
func NewResourceExistsChecker(client client.Client) *ResourceExistsChecker {
	return &ResourceExistsChecker{client: client}
}

// ConfigMapExists checks if a ConfigMap exists in the given namespace
func (r *ResourceExistsChecker) ConfigMapExists(ctx context.Context, name, namespace string) error {
	var configMap corev1.ConfigMap
	return r.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, &configMap)
}

// SecretExists checks if a Secret exists in the given namespace
func (r *ResourceExistsChecker) SecretExists(ctx context.Context, name, namespace string) error {
	var secret corev1.Secret
	return r.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, &secret)
}

// PVCExists checks if a PersistentVolumeClaim exists in the given namespace
func (r *ResourceExistsChecker) PVCExists(ctx context.Context, name, namespace string) error {
	var pvc corev1.PersistentVolumeClaim
	return r.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, &pvc)
}

// ServiceAccountExists checks if a ServiceAccount exists in the given namespace
func (r *ResourceExistsChecker) ServiceAccountExists(ctx context.Context, name, namespace string) error {
	var sa corev1.ServiceAccount
	return r.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, &sa)
}

// ServiceExists checks if a Service exists in the given namespace
func (r *ResourceExistsChecker) ServiceExists(ctx context.Context, name, namespace string) error {
	var service corev1.Service
	return r.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, &service)
}