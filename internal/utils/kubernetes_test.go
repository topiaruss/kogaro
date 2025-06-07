// Copyright 2025 Russell Ferriday
// Licensed under the Apache License, Version 2.0
//
// Kogaro - Kubernetes Configuration Hygiene Agent

package utils

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestIsSystemNamespace(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		want      bool
	}{
		{"kube-system", "kube-system", true},
		{"kube-public", "kube-public", true},
		{"kube-node-lease", "kube-node-lease", true},
		{"default", "default", true},
		{"user-namespace", "production", false},
		{"user-namespace-2", "app", false},
		{"empty", "", false},
		{"partial-match", "kube", false},
		{"case-sensitive", "KUBE-SYSTEM", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSystemNamespace(tt.namespace); got != tt.want {
				t.Errorf("IsSystemNamespace(%v) = %v, want %v", tt.namespace, got, tt.want)
			}
		})
	}
}

func TestIsSystemPod(t *testing.T) {
	tests := []struct {
		name string
		pod  corev1.Pod
		want bool
	}{
		{
			name: "kube-system pod",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "kube-system",
				},
			},
			want: true,
		},
		{
			name: "kube-public pod",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "kube-public",
				},
			},
			want: true,
		},
		{
			name: "kube-node-lease pod",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "kube-node-lease",
				},
			},
			want: true,
		},
		{
			name: "user namespace pod",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "production",
				},
			},
			want: false,
		},
		{
			name: "default namespace pod",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSystemPod(tt.pod); got != tt.want {
				t.Errorf("IsSystemPod() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasOwnerReferences(t *testing.T) {
	tests := []struct {
		name string
		pod  corev1.Pod
		want bool
	}{
		{
			name: "pod with owner reference",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "test-deployment",
						},
					},
				},
			},
			want: true,
		},
		{
			name: "pod with multiple owner references",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "apps/v1",
							Kind:       "ReplicaSet",
							Name:       "test-rs",
						},
						{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "test-deployment",
						},
					},
				},
			},
			want: true,
		},
		{
			name: "pod without owner references",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
			},
			want: false,
		},
		{
			name: "pod with empty owner references",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-pod",
					Namespace:       "default",
					OwnerReferences: []metav1.OwnerReference{},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasOwnerReferences(tt.pod); got != tt.want {
				t.Errorf("HasOwnerReferences() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsOwnedByJobOrCronJob(t *testing.T) {
	tests := []struct {
		name string
		pod  corev1.Pod
		want bool
	}{
		{
			name: "pod owned by Job",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "batch/v1",
							Kind:       "Job",
							Name:       "test-job",
						},
					},
				},
			},
			want: true,
		},
		{
			name: "pod owned by CronJob",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "batch/v1",
							Kind:       "CronJob",
							Name:       "test-cronjob",
						},
					},
				},
			},
			want: true,
		},
		{
			name: "pod owned by Deployment",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "test-deployment",
						},
					},
				},
			},
			want: false,
		},
		{
			name: "pod with no owner references",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "standalone-pod",
				},
			},
			want: false,
		},
		{
			name: "pod with mixed owner references including Job",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "apps/v1",
							Kind:       "ReplicaSet",
							Name:       "test-rs",
						},
						{
							APIVersion: "batch/v1",
							Kind:       "Job",
							Name:       "test-job",
						},
					},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsOwnedByJobOrCronJob(tt.pod); got != tt.want {
				t.Errorf("IsOwnedByJobOrCronJob() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsPodReady(t *testing.T) {
	tests := []struct {
		name string
		pod  corev1.Pod
		want bool
	}{
		{
			name: "ready pod",
			pod: corev1.Pod{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			want: true,
		},
		{
			name: "not ready pod",
			pod: corev1.Pod{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionFalse,
						},
					},
				},
			},
			want: false,
		},
		{
			name: "pod with no ready condition",
			pod: corev1.Pod{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodScheduled,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			want: false,
		},
		{
			name: "pod with mixed conditions including ready",
			pod: corev1.Pod{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodScheduled,
							Status: corev1.ConditionTrue,
						},
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						},
						{
							Type:   corev1.ContainersReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			want: true,
		},
		{
			name: "pod with no conditions",
			pod: corev1.Pod{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPodReady(tt.pod); got != tt.want {
				t.Errorf("IsPodReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterPodsByNamespace(t *testing.T) {
	pods := []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod1",
				Namespace: "default",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod2",
				Namespace: "kube-system",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod3",
				Namespace: "default",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod4",
				Namespace: "production",
			},
		},
	}

	tests := []struct {
		name      string
		namespace string
		want      int
		wantNames []string
	}{
		{
			name:      "default namespace",
			namespace: "default",
			want:      2,
			wantNames: []string{"pod1", "pod3"},
		},
		{
			name:      "kube-system namespace",
			namespace: "kube-system",
			want:      1,
			wantNames: []string{"pod2"},
		},
		{
			name:      "production namespace",
			namespace: "production",
			want:      1,
			wantNames: []string{"pod4"},
		},
		{
			name:      "nonexistent namespace",
			namespace: "nonexistent",
			want:      0,
			wantNames: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterPodsByNamespace(pods, tt.namespace)
			if len(got) != tt.want {
				t.Errorf("FilterPodsByNamespace() returned %d pods, want %d", len(got), tt.want)
			}

			gotNames := make([]string, len(got))
			for i, pod := range got {
				gotNames[i] = pod.Name
			}

			for _, wantName := range tt.wantNames {
				found := false
				for _, gotName := range gotNames {
					if gotName == wantName {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected pod '%s' not found in filtered results", wantName)
				}
			}
		})
	}
}

func TestFindPodsMatchingSelector(t *testing.T) {
	pods := []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pod1",
				Labels: map[string]string{
					"app":     "web",
					"version": "v1",
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pod2",
				Labels: map[string]string{
					"app":     "api",
					"version": "v1",
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pod3",
				Labels: map[string]string{
					"app":     "web",
					"version": "v2",
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "pod4",
				Labels: map[string]string{},
			},
		},
	}

	tests := []struct {
		name      string
		selector  map[string]string
		want      int
		wantNames []string
	}{
		{
			name:      "match app=web",
			selector:  map[string]string{"app": "web"},
			want:      2,
			wantNames: []string{"pod1", "pod3"},
		},
		{
			name:      "match app=api",
			selector:  map[string]string{"app": "api"},
			want:      1,
			wantNames: []string{"pod2"},
		},
		{
			name:      "match version=v1",
			selector:  map[string]string{"version": "v1"},
			want:      2,
			wantNames: []string{"pod1", "pod2"},
		},
		{
			name:      "match app=web and version=v1",
			selector:  map[string]string{"app": "web", "version": "v1"},
			want:      1,
			wantNames: []string{"pod1"},
		},
		{
			name:      "no matches",
			selector:  map[string]string{"app": "nonexistent"},
			want:      0,
			wantNames: []string{},
		},
		{
			name:      "empty selector matches all",
			selector:  map[string]string{},
			want:      4,
			wantNames: []string{"pod1", "pod2", "pod3", "pod4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindPodsMatchingSelector(tt.selector, pods)
			if len(got) != tt.want {
				t.Errorf("FindPodsMatchingSelector() returned %d pods, want %d", len(got), tt.want)
			}

			gotNames := make([]string, len(got))
			for i, pod := range got {
				gotNames[i] = pod.Name
			}

			for _, wantName := range tt.wantNames {
				found := false
				for _, gotName := range gotNames {
					if gotName == wantName {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected pod '%s' not found in matching results", wantName)
				}
			}
		})
	}
}

func TestFindPodsMatchingLabelSelector(t *testing.T) {
	pods := []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pod1",
				Labels: map[string]string{
					"app":     "web",
					"version": "v1",
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pod2",
				Labels: map[string]string{
					"app":     "api",
					"version": "v1",
				},
			},
		},
	}

	tests := []struct {
		name         string
		labelSelector *metav1.LabelSelector
		want         int
		wantNames    []string
		wantError    bool
	}{
		{
			name: "match labels selector",
			labelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "web"},
			},
			want:      1,
			wantNames: []string{"pod1"},
			wantError: false,
		},
		{
			name: "match expressions selector",
			labelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "app",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"web", "api"},
					},
				},
			},
			want:      2,
			wantNames: []string{"pod1", "pod2"},
			wantError: false,
		},
		{
			name:          "empty selector matches all",
			labelSelector: &metav1.LabelSelector{},
			want:          2,
			wantNames:     []string{"pod1", "pod2"},
			wantError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FindPodsMatchingLabelSelector(tt.labelSelector, pods)
			
			if tt.wantError && err == nil {
				t.Error("FindPodsMatchingLabelSelector() expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("FindPodsMatchingLabelSelector() unexpected error: %v", err)
			}
			
			if len(got) != tt.want {
				t.Errorf("FindPodsMatchingLabelSelector() returned %d pods, want %d", len(got), tt.want)
			}

			gotNames := make([]string, len(got))
			for i, pod := range got {
				gotNames[i] = pod.Name
			}

			for _, wantName := range tt.wantNames {
				found := false
				for _, gotName := range gotNames {
					if gotName == wantName {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected pod '%s' not found in matching results", wantName)
				}
			}
		})
	}
}

func TestResourceExistsChecker(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	// Create test objects
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-configmap",
			Namespace: "default",
		},
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(configMap, secret).
		Build()

	checker := NewResourceExistsChecker(client)

	tests := []struct {
		name      string
		checkFunc func() error
		wantError bool
	}{
		{
			name: "existing configmap",
			checkFunc: func() error {
				return checker.ConfigMapExists(context.TODO(), "test-configmap", "default")
			},
			wantError: false,
		},
		{
			name: "missing configmap",
			checkFunc: func() error {
				return checker.ConfigMapExists(context.TODO(), "missing-configmap", "default")
			},
			wantError: true,
		},
		{
			name: "existing secret",
			checkFunc: func() error {
				return checker.SecretExists(context.TODO(), "test-secret", "default")
			},
			wantError: false,
		},
		{
			name: "missing secret",
			checkFunc: func() error {
				return checker.SecretExists(context.TODO(), "missing-secret", "default")
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.checkFunc()
			if tt.wantError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestNewResourceExistsChecker(t *testing.T) {
	scheme := runtime.NewScheme()
	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	
	checker := NewResourceExistsChecker(client)
	if checker == nil {
		t.Error("NewResourceExistsChecker should return non-nil checker")
	}
	
	if checker.client != client {
		t.Error("ResourceExistsChecker should store the provided client")
	}
}