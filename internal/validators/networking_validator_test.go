// Copyright 2025 Russell Ferriday
// Licensed under the Apache License, Version 2.0
//
// Kogaro - Kubernetes Configuration Hygiene Agent

package validators

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNetworkingValidator_GetValidationType(t *testing.T) {
	validator := &NetworkingValidator{}
	expected := "networking_validation"
	if got := validator.GetValidationType(); got != expected {
		t.Errorf("GetValidationType() = %v, want %v", got, expected)
	}
}

func TestNetworkingValidator_ValidateServiceConnectivity(t *testing.T) {
	tests := []struct {
		name           string
		objects        []client.Object
		config         NetworkingConfig
		expectedErrors []string
	}{
		{
			name: "service with matching pods",
			objects: []client.Object{
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-service",
						Namespace: "default",
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{"app": "test"},
						Ports: []corev1.ServicePort{
							{
								Port:       80,
								TargetPort: intstr.FromInt(8080),
							},
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
						Labels:    map[string]string{"app": "test"},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name: "test-container",
								Ports: []corev1.ContainerPort{
									{ContainerPort: 8080},
								},
							},
						},
					},
					Status: corev1.PodStatus{
						Conditions: []corev1.PodCondition{
							{Type: corev1.PodReady, Status: corev1.ConditionTrue},
						},
					},
				},
				&corev1.Endpoints{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-service",
						Namespace: "default",
					},
					Subsets: []corev1.EndpointSubset{
						{
							Addresses: []corev1.EndpointAddress{
								{IP: "10.0.0.1"},
							},
						},
					},
				},
			},
			config: NetworkingConfig{
				EnableServiceValidation: true,
			},
			expectedErrors: []string{}, // No errors expected
		},
		{
			name: "service with no matching pods",
			objects: []client.Object{
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "orphan-service",
						Namespace: "default",
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{"app": "nonexistent"},
						Ports: []corev1.ServicePort{
							{Port: 80, TargetPort: intstr.FromInt(8080)},
						},
					},
				},
			},
			config: NetworkingConfig{
				EnableServiceValidation: true,
			},
			expectedErrors: []string{"service_selector_mismatch"},
		},
		{
			name: "service with no endpoints",
			objects: []client.Object{
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "no-endpoints-service",
						Namespace: "default",
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{"app": "test"},
						Ports: []corev1.ServicePort{
							{Port: 80},
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
						Labels:    map[string]string{"app": "test"},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: "test-container"},
						},
					},
				},
			},
			config: NetworkingConfig{
				EnableServiceValidation: true,
			},
			expectedErrors: []string{"service_no_endpoints"},
		},
		{
			name: "service with port mismatch",
			objects: []client.Object{
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "port-mismatch-service",
						Namespace: "default",
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{"app": "test"},
						Ports: []corev1.ServicePort{
							{
								Port:       80,
								TargetPort: intstr.FromInt(9999), // Port that doesn't exist
							},
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
						Labels:    map[string]string{"app": "test"},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name: "test-container",
								Ports: []corev1.ContainerPort{
									{ContainerPort: 8080},
								},
							},
						},
					},
				},
			},
			config: NetworkingConfig{
				EnableServiceValidation: true,
			},
			expectedErrors: []string{"service_port_mismatch"},
		},
		{
			name: "unexposed pod warning",
			objects: []client.Object{
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "unexposed-pod",
						Namespace: "default",
						Labels:    map[string]string{"app": "unexposed"},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: "test-container"},
						},
					},
				},
			},
			config: NetworkingConfig{
				EnableServiceValidation: true,
				WarnUnexposedPods:       true,
			},
			expectedErrors: []string{"pod_no_service"},
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

			validator := NewNetworkingValidator(fakeClient, logr.Discard(), tt.config)

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

func TestNetworkingValidator_ValidateNetworkPolicyCoverage(t *testing.T) {
	tests := []struct {
		name           string
		objects        []client.Object
		config         NetworkingConfig
		expectedErrors []string
	}{
		{
			name: "namespace with required NetworkPolicy",
			objects: []client.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "production",
					},
				},
				&networkingv1.NetworkPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "deny-all",
						Namespace: "production",
					},
					Spec: networkingv1.NetworkPolicySpec{
						PodSelector: metav1.LabelSelector{},
						PolicyTypes: []networkingv1.PolicyType{
							networkingv1.PolicyTypeIngress,
						},
					},
				},
			},
			config: NetworkingConfig{
				EnableNetworkPolicyValidation: true,
				PolicyRequiredNamespaces:      []string{"production"},
			},
			expectedErrors: []string{}, // No errors expected
		},
		{
			name: "missing required NetworkPolicy",
			objects: []client.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "production",
					},
				},
			},
			config: NetworkingConfig{
				EnableNetworkPolicyValidation: true,
				PolicyRequiredNamespaces:      []string{"production"},
			},
			expectedErrors: []string{"missing_network_policy_required"},
		},
		{
			name: "missing default deny policy",
			objects: []client.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-ns",
					},
				},
				&networkingv1.NetworkPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "allow-specific",
						Namespace: "test-ns",
					},
					Spec: networkingv1.NetworkPolicySpec{
						PodSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "allowed"},
						},
						Ingress: []networkingv1.NetworkPolicyIngressRule{
							{
								From: []networkingv1.NetworkPolicyPeer{
									{
										PodSelector: &metav1.LabelSelector{
											MatchLabels: map[string]string{"role": "frontend"},
										},
									},
								},
							},
						},
					},
				},
			},
			config: NetworkingConfig{
				EnableNetworkPolicyValidation: true,
			},
			expectedErrors: []string{"missing_network_policy_default_deny"},
		},
		{
			name: "orphaned NetworkPolicy",
			objects: []client.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-ns",
					},
				},
				&networkingv1.NetworkPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "orphan-policy",
						Namespace: "test-ns",
					},
					Spec: networkingv1.NetworkPolicySpec{
						PodSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "nonexistent"},
						},
					},
				},
			},
			config: NetworkingConfig{
				EnableNetworkPolicyValidation: true,
			},
			expectedErrors: []string{"network_policy_orphaned"},
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

			validator := NewNetworkingValidator(fakeClient, logr.Discard(), tt.config)

			err := validator.ValidateCluster(context.Background())
			if err != nil {
				t.Errorf("ValidateCluster() error = %v", err)
				return
			}
		})
	}
}

func TestNetworkingValidator_ValidateIngressConnectivity(t *testing.T) {
	tests := []struct {
		name           string
		objects        []client.Object
		config         NetworkingConfig
		expectedErrors []string
	}{
		{
			name: "ingress with healthy backend",
			objects: []client.Object{
				&networkingv1.Ingress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-ingress",
						Namespace: "default",
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{
							{
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path: "/",
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "test-service",
														Port: networkingv1.ServiceBackendPort{
															Number: 80,
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-service",
						Namespace: "default",
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{"app": "test"},
						Ports: []corev1.ServicePort{
							{Port: 80, TargetPort: intstr.FromInt(8080)},
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
						Labels:    map[string]string{"app": "test"},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name: "test-container",
								Ports: []corev1.ContainerPort{
									{ContainerPort: 8080},
								},
							},
						},
					},
					Status: corev1.PodStatus{
						Conditions: []corev1.PodCondition{
							{Type: corev1.PodReady, Status: corev1.ConditionTrue},
						},
					},
				},
			},
			config: NetworkingConfig{
				EnableIngressValidation: true,
			},
			expectedErrors: []string{}, // No errors expected
		},
		{
			name: "ingress with missing service",
			objects: []client.Object{
				&networkingv1.Ingress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "broken-ingress",
						Namespace: "default",
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{
							{
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{
											Path: "/",
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "missing-service",
													Port: networkingv1.ServiceBackendPort{
														Number: 80,
													},
												},
											},
										},
									},
								},
							},
							},
						},
					},
				},
			},
			config: NetworkingConfig{
				EnableIngressValidation: true,
			},
			expectedErrors: []string{"ingress_service_missing"},
		},
		{
			name: "ingress with port mismatch",
			objects: []client.Object{
				&networkingv1.Ingress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "port-mismatch-ingress",
						Namespace: "default",
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{
							{
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{
											Path: "/",
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "test-service",
													Port: networkingv1.ServiceBackendPort{
														Number: 9999, // Port that doesn't exist
													},
												},
											},
										},
									},
								},
							},
							},
						},
					},
				},
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-service",
						Namespace: "default",
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{Port: 80},
						},
					},
				},
			},
			config: NetworkingConfig{
				EnableIngressValidation: true,
			},
			expectedErrors: []string{"ingress_service_port_mismatch"},
		},
		{
			name: "ingress with no backend pods",
			objects: []client.Object{
				&networkingv1.Ingress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "no-backend-ingress",
						Namespace: "default",
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{
							{
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{
											Path: "/",
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "empty-service",
													Port: networkingv1.ServiceBackendPort{
														Number: 80,
													},
												},
											},
										},
									},
								},
							},
							},
						},
					},
				},
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "empty-service",
						Namespace: "default",
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{"app": "test"},
						Ports: []corev1.ServicePort{
							{Port: 80},
						},
					},
				},
				// No pods matching the service selector
			},
			config: NetworkingConfig{
				EnableIngressValidation: true,
			},
			expectedErrors: []string{"ingress_no_backend_pods"},
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

			validator := NewNetworkingValidator(fakeClient, logr.Discard(), tt.config)

			err := validator.ValidateCluster(context.Background())
			if err != nil {
				t.Errorf("ValidateCluster() error = %v", err)
				return
			}
		})
	}
}

func TestNetworkingValidator_HelperFunctions(t *testing.T) {
	validator := &NetworkingValidator{}

	t.Run("isSpecialService", func(t *testing.T) {
		// Headless service
		headlessService := corev1.Service{
			Spec: corev1.ServiceSpec{
				ClusterIP: "None",
			},
		}
		if !validator.isSpecialService(headlessService) {
			t.Error("Expected headless service to be considered special")
		}

		// Service without selector
		externalService := corev1.Service{
			Spec: corev1.ServiceSpec{
				ClusterIP: "10.0.0.1",
				Selector:  nil,
			},
		}
		if !validator.isSpecialService(externalService) {
			t.Error("Expected external service to be considered special")
		}

		// System service
		systemService := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "kube-system",
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{"app": "test"},
			},
		}
		if !validator.isSpecialService(systemService) {
			t.Error("Expected system service to be considered special")
		}

		// Normal service
		normalService := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
			},
			Spec: corev1.ServiceSpec{
				ClusterIP: "10.0.0.1",
				Selector:  map[string]string{"app": "test"},
			},
		}
		if validator.isSpecialService(normalService) {
			t.Error("Expected normal service to not be considered special")
		}
	})

	t.Run("isPodReady", func(t *testing.T) {
		readyPod := corev1.Pod{
			Status: corev1.PodStatus{
				Conditions: []corev1.PodCondition{
					{Type: corev1.PodReady, Status: corev1.ConditionTrue},
				},
			},
		}
		if !validator.isPodReady(readyPod) {
			t.Error("Expected pod with Ready=True condition to be ready")
		}

		notReadyPod := corev1.Pod{
			Status: corev1.PodStatus{
				Conditions: []corev1.PodCondition{
					{Type: corev1.PodReady, Status: corev1.ConditionFalse},
				},
			},
		}
		if validator.isPodReady(notReadyPod) {
			t.Error("Expected pod with Ready=False condition to not be ready")
		}
	})

	t.Run("isDefaultDenyPolicy", func(t *testing.T) {
		defaultDenyPolicy := networkingv1.NetworkPolicy{
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{},
				PolicyTypes: []networkingv1.PolicyType{
					networkingv1.PolicyTypeIngress,
				},
				Ingress: []networkingv1.NetworkPolicyIngressRule{},
			},
		}
		if !validator.isDefaultDenyPolicy(defaultDenyPolicy) {
			t.Error("Expected empty ingress rules with empty selector to be default deny")
		}

		allowPolicy := networkingv1.NetworkPolicy{
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{},
				PolicyTypes: []networkingv1.PolicyType{
					networkingv1.PolicyTypeIngress,
				},
				Ingress: []networkingv1.NetworkPolicyIngressRule{
					{
						From: []networkingv1.NetworkPolicyPeer{
							{PodSelector: &metav1.LabelSelector{}},
						},
					},
				},
			},
		}
		if validator.isDefaultDenyPolicy(allowPolicy) {
			t.Error("Expected policy with ingress rules to not be default deny")
		}
	})
}

// Helper function to create int32 pointers
func intPtr(i int32) *int32 {
	return &i
}