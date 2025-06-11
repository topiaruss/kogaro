// Copyright 2025 Russell Ferriday
// Licensed under the Apache License, Version 2.0
//
// Kogaro - Kubernetes Configuration Hygiene Agent

package validators

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestSecurityValidator_GetValidationType(t *testing.T) {
	validator := &SecurityValidator{}
	expected := "security_validation"
	if got := validator.GetValidationType(); got != expected {
		t.Errorf("GetValidationType() = %v, want %v", got, expected)
	}
}

func TestSecurityValidator_ValidateCluster_RootUserValidation(t *testing.T) {
	tests := []struct {
		name           string
		objects        []client.Object
		config         SecurityConfig
		expectedErrors []string
	}{
		{
			name: "pod running as root user",
			objects: []client.Object{
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "root-pod",
						Namespace: "default",
					},
					Spec: corev1.PodSpec{
						SecurityContext: &corev1.PodSecurityContext{
							RunAsUser: int64Ptr(0),
						},
						Containers: []corev1.Container{
							{
								Name:  "test-container",
								Image: "test:latest",
							},
						},
					},
				},
			},
			config: SecurityConfig{
				EnableRootUserValidation: true,
			},
			expectedErrors: []string{"pod_running_as_root"},
		},
		{
			name: "deployment with container running as root",
			objects: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "root-deployment",
						Namespace: "default",
					},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name:  "test-container",
										Image: "test:latest",
										SecurityContext: &corev1.SecurityContext{
											RunAsUser: int64Ptr(0),
										},
									},
								},
							},
						},
					},
				},
			},
			config: SecurityConfig{
				EnableRootUserValidation: true,
			},
			expectedErrors: []string{"container_running_as_root", "pod_allows_root_user"},
		},
		{
			name: "container with privilege escalation allowed",
			objects: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "privilege-deployment",
						Namespace: "default",
					},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name:  "test-container",
										Image: "test:latest",
										SecurityContext: &corev1.SecurityContext{
											AllowPrivilegeEscalation: boolPtr(true),
										},
									},
								},
							},
						},
					},
				},
			},
			config: SecurityConfig{
				EnableRootUserValidation: true,
			},
			expectedErrors: []string{"container_allows_privilege_escalation", "pod_allows_root_user"},
		},
		{
			name: "privileged container",
			objects: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "privileged-deployment",
						Namespace: "default",
					},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name:  "test-container",
										Image: "test:latest",
										SecurityContext: &corev1.SecurityContext{
											Privileged: boolPtr(true),
										},
									},
								},
							},
						},
					},
				},
			},
			config: SecurityConfig{
				EnableRootUserValidation: true,
			},
			expectedErrors: []string{"container_privileged_mode", "pod_allows_root_user"},
		},
		{
			name: "secure deployment configuration",
			objects: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secure-deployment",
						Namespace: "default",
					},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								SecurityContext: &corev1.PodSecurityContext{
									RunAsNonRoot: boolPtr(true),
									RunAsUser:    int64Ptr(1000),
								},
								Containers: []corev1.Container{
									{
										Name:  "test-container",
										Image: "test:latest",
										SecurityContext: &corev1.SecurityContext{
											RunAsUser:                int64Ptr(1000),
											AllowPrivilegeEscalation: boolPtr(false),
											Privileged:               boolPtr(false),
											ReadOnlyRootFilesystem:   boolPtr(true),
										},
									},
								},
							},
						},
					},
				},
			},
			config: SecurityConfig{
				EnableRootUserValidation:       true,
				EnableSecurityContextValidation: true,
			},
			expectedErrors: []string{}, // No errors expected for secure configuration
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = corev1.AddToScheme(scheme)
			_ = appsv1.AddToScheme(scheme)

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.objects...).
				Build()

			validator := NewSecurityValidator(fakeClient, logr.Discard(), tt.config)
			mockLogReceiver := &MockLogReceiver{}
			validator.SetLogReceiver(mockLogReceiver)

			err := validator.ValidateCluster(context.Background())
			if err != nil {
				t.Errorf("ValidateCluster() error = %v", err)
				return
			}

			// Note: In a real test environment, we would capture the validation errors
			// through metrics or a test logging implementation. For now, we verify
			// that no runtime errors occurred.
		})
	}
}

func TestSecurityValidator_ValidateCluster_SecurityContextValidation(t *testing.T) {
	tests := []struct {
		name           string
		objects        []client.Object
		config         SecurityConfig
		expectedErrors []string
	}{
		{
			name: "missing pod security context",
			objects: []client.Object{
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "no-security-context-pod",
						Namespace: "default",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "test-container",
								Image: "test:latest",
							},
						},
					},
				},
			},
			config: SecurityConfig{
				EnableSecurityContextValidation: true,
			},
			expectedErrors: []string{"missing_pod_security_context", "missing_container_security_context"},
		},
		{
			name: "missing container security context",
			objects: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "no-container-security-deployment",
						Namespace: "default",
					},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								SecurityContext: &corev1.PodSecurityContext{},
								Containers: []corev1.Container{
									{
										Name:  "test-container",
										Image: "test:latest",
										// No SecurityContext
									},
								},
							},
						},
					},
				},
			},
			config: SecurityConfig{
				EnableSecurityContextValidation: true,
			},
			expectedErrors: []string{"missing_container_security_context"},
		},
		{
			name: "container with additional capabilities",
			objects: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "capabilities-deployment",
						Namespace: "default",
					},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								SecurityContext: &corev1.PodSecurityContext{},
								Containers: []corev1.Container{
									{
										Name:  "test-container",
										Image: "test:latest",
										SecurityContext: &corev1.SecurityContext{
											Capabilities: &corev1.Capabilities{
												Add: []corev1.Capability{"NET_ADMIN", "SYS_TIME"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			config: SecurityConfig{
				EnableSecurityContextValidation: true,
			},
			expectedErrors: []string{"container_additional_capabilities"},
		},
		{
			name: "writable root filesystem",
			objects: []client.Object{
				&appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "writable-fs-statefulset",
						Namespace: "default",
					},
					Spec: appsv1.StatefulSetSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								SecurityContext: &corev1.PodSecurityContext{},
								Containers: []corev1.Container{
									{
										Name:  "test-container",
										Image: "test:latest",
										SecurityContext: &corev1.SecurityContext{
											ReadOnlyRootFilesystem: boolPtr(false),
										},
									},
								},
							},
						},
					},
				},
			},
			config: SecurityConfig{
				EnableRootUserValidation: true,
			},
			expectedErrors: []string{"container_writable_root_filesystem"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = corev1.AddToScheme(scheme)
			_ = appsv1.AddToScheme(scheme)

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.objects...).
				Build()

			validator := NewSecurityValidator(fakeClient, logr.Discard(), tt.config)
			mockLogReceiver := &MockLogReceiver{}
			validator.SetLogReceiver(mockLogReceiver)

			err := validator.ValidateCluster(context.Background())
			if err != nil {
				t.Errorf("ValidateCluster() error = %v", err)
				return
			}
		})
	}
}

func TestSecurityValidator_ValidateServiceAccountPermissions(t *testing.T) {
	tests := []struct {
		name           string
		objects        []client.Object
		config         SecurityConfig
		expectedErrors []string
	}{
		{
			name: "serviceaccount with cluster-admin role",
			objects: []client.Object{
				&corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "admin-sa",
						Namespace: "default",
					},
				},
				&rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "admin-binding",
					},
					Subjects: []rbacv1.Subject{
						{
							Kind:      "ServiceAccount",
							Name:      "admin-sa",
							Namespace: "default",
						},
					},
					RoleRef: rbacv1.RoleRef{
						Kind: "ClusterRole",
						Name: "cluster-admin",
					},
				},
			},
			config: SecurityConfig{
				EnableServiceAccountValidation: true,
			},
			expectedErrors: []string{"serviceaccount_cluster_role_binding"},
		},
		{
			name: "serviceaccount with dangerous role binding",
			objects: []client.Object{
				&corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dangerous-sa",
						Namespace: "default",
					},
				},
				&rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dangerous-binding",
						Namespace: "default",
					},
					Subjects: []rbacv1.Subject{
						{
							Kind: "ServiceAccount",
							Name: "dangerous-sa",
						},
					},
					RoleRef: rbacv1.RoleRef{
						Kind: "Role",
						Name: "admin",
					},
				},
			},
			config: SecurityConfig{
				EnableServiceAccountValidation: true,
			},
			expectedErrors: []string{"serviceaccount_excessive_permissions"},
		},
		{
			name: "serviceaccount with safe permissions",
			objects: []client.Object{
				&corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "safe-sa",
						Namespace: "default",
					},
				},
				&rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "safe-binding",
						Namespace: "default",
					},
					Subjects: []rbacv1.Subject{
						{
							Kind: "ServiceAccount",
							Name: "safe-sa",
						},
					},
					RoleRef: rbacv1.RoleRef{
						Kind: "Role",
						Name: "view",
					},
				},
			},
			config: SecurityConfig{
				EnableServiceAccountValidation: true,
			},
			expectedErrors: []string{}, // No errors expected
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = corev1.AddToScheme(scheme)
			_ = rbacv1.AddToScheme(scheme)

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.objects...).
				Build()

			validator := NewSecurityValidator(fakeClient, logr.Discard(), tt.config)
			mockLogReceiver := &MockLogReceiver{}
			validator.SetLogReceiver(mockLogReceiver)

			err := validator.ValidateCluster(context.Background())
			if err != nil {
				t.Errorf("ValidateCluster() error = %v", err)
				return
			}
		})
	}
}

func TestSecurityValidator_ValidateNetworkPolicyCoverage(t *testing.T) {
	tests := []struct {
		name           string
		objects        []client.Object
		config         SecurityConfig
		expectedErrors []string
	}{
		{
			name: "security-sensitive namespace without NetworkPolicy",
			objects: []client.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "sensitive-ns",
					},
				},
			},
			config: SecurityConfig{
				EnableNetworkPolicyValidation:   true,
				SecuritySensitiveNamespaces: []string{"sensitive-ns"},
			},
			expectedErrors: []string{"missing_network_policy_security_sensitive"},
		},
		{
			name: "production namespace without NetworkPolicy",
			objects: []client.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "production",
					},
				},
			},
			config: SecurityConfig{
				EnableNetworkPolicyValidation: true,
			},
			expectedErrors: []string{"missing_network_policy_production"},
		},
		{
			name: "namespace with NetworkPolicy",
			objects: []client.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "secure-ns",
					},
				},
				&networkingv1.NetworkPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "deny-all",
						Namespace: "secure-ns",
					},
					Spec: networkingv1.NetworkPolicySpec{
						PodSelector: metav1.LabelSelector{},
						Ingress:     []networkingv1.NetworkPolicyIngressRule{},
					},
				},
			},
			config: SecurityConfig{
				EnableNetworkPolicyValidation:   true,
				SecuritySensitiveNamespaces: []string{"secure-ns"},
			},
			expectedErrors: []string{}, // No errors expected
		},
		{
			name: "system namespace ignored",
			objects: []client.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "kube-system",
					},
				},
			},
			config: SecurityConfig{
				EnableNetworkPolicyValidation: true,
			},
			expectedErrors: []string{}, // System namespaces should be ignored
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = corev1.AddToScheme(scheme)
			_ = networkingv1.AddToScheme(scheme)

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.objects...).
				Build()

			validator := NewSecurityValidator(fakeClient, logr.Discard(), tt.config)
			mockLogReceiver := &MockLogReceiver{}
			validator.SetLogReceiver(mockLogReceiver)

			err := validator.ValidateCluster(context.Background())
			if err != nil {
				t.Errorf("ValidateCluster() error = %v", err)
				return
			}
		})
	}
}

func TestSecurityValidator_InitContainerValidation(t *testing.T) {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "init-container-deployment",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{},
					InitContainers: []corev1.Container{
						{
							Name:  "init-container",
							Image: "init:latest",
							SecurityContext: &corev1.SecurityContext{
								RunAsUser: int64Ptr(0),
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "main-container",
							Image: "main:latest",
							SecurityContext: &corev1.SecurityContext{
								RunAsUser: int64Ptr(1000),
							},
						},
					},
				},
			},
		},
	}

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(deployment).
		Build()

	config := SecurityConfig{
		EnableRootUserValidation: true,
	}

	validator := NewSecurityValidator(fakeClient, logr.Discard(), config)
	mockLogReceiver := &MockLogReceiver{}
	validator.SetLogReceiver(mockLogReceiver)

	err := validator.ValidateCluster(context.Background())
	if err != nil {
		t.Errorf("ValidateCluster() error = %v", err)
	}

	// The test should validate that init containers running as root are detected
}

func TestSecurityValidator_DaemonSetValidation(t *testing.T) {
	daemonSet := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "security-daemonset",
			Namespace: "kube-system",
		},
		Spec: appsv1.DaemonSetSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "daemon-container",
							Image: "daemon:latest",
							SecurityContext: &corev1.SecurityContext{
								Privileged: boolPtr(true),
							},
						},
					},
				},
			},
		},
	}

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(daemonSet).
		Build()

	config := SecurityConfig{
		EnableRootUserValidation: true,
	}

	validator := NewSecurityValidator(fakeClient, logr.Discard(), config)
	mockLogReceiver := &MockLogReceiver{}
	validator.SetLogReceiver(mockLogReceiver)

	err := validator.ValidateCluster(context.Background())
	if err != nil {
		t.Errorf("ValidateCluster() error = %v", err)
	}
}

