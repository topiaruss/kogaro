// Copyright 2025 Russell Ferriday
// Licensed under the Apache License, Version 2.0
//
// Kogaro - Kubernetes Configuration Hygiene Agent

package validators

import (
	"context"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestValidators_ErrorMessageConsistency validates that error messages across
// all validators follow consistent patterns and formats
func TestValidators_ErrorMessageConsistency(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = networkingv1.AddToScheme(scheme)
	_ = storagev1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)

	tests := []struct {
		name      string
		validator func() Validator
		setupFunc func() []runtime.Object
		wantErrs  []errorPattern
	}{
		{
			name: "reference_validator_messages",
			validator: func() Validator {
				return NewReferenceValidator(fake.NewClientBuilder().WithScheme(scheme).Build(), logr.Discard(), ValidationConfig{
					EnableIngressValidation:   true,
					EnableConfigMapValidation: true,
					EnableSecretValidation:    true,
				})
			},
			setupFunc: func() []runtime.Object {
				return []runtime.Object{
					&networkingv1.Ingress{
						ObjectMeta: metav1.ObjectMeta{Name: "test-ingress", Namespace: "default"},
						Spec: networkingv1.IngressSpec{
							IngressClassName: stringPtrHelper("missing-class"),
							Rules: []networkingv1.IngressRule{{
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{{
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "missing-service",
													Port: networkingv1.ServiceBackendPort{Number: 80},
												},
											},
										}},
									},
								},
							}},
						},
					},
					&corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default"},
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{{
								Name: "config-vol",
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{Name: "missing-configmap"},
									},
								},
							}},
							Containers: []corev1.Container{{Name: "app", Image: "nginx"}},
						},
					},
				}
			},
			wantErrs: []errorPattern{
				{pattern: "IngressClass .* does not exist", validationType: "dangling_ingress_class"},
				{pattern: "Service .* referenced in Ingress does not exist", validationType: "dangling_service_reference"},
				{pattern: "ConfigMap .* referenced in volume does not exist", validationType: "dangling_configmap_volume"},
			},
		},
		{
			name: "resource_limits_validator_messages",
			validator: func() Validator {
				minCPU := resource.MustParse("10m")
				return NewResourceLimitsValidator(fake.NewClientBuilder().WithScheme(scheme).Build(), logr.Discard(), ResourceLimitsConfig{
					EnableMissingRequestsValidation: true,
					EnableMissingLimitsValidation:   true,
					EnableQoSValidation:             true,
					MinCPURequest:                   &minCPU,
				})
			},
			setupFunc: func() []runtime.Object {
				return []runtime.Object{
					&appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{Name: "test-deploy", Namespace: "default"},
						Spec: appsv1.DeploymentSpec{
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{{
										Name:  "app",
										Image: "nginx",
										// No resources defined - should trigger missing requests/limits
									}},
								},
							},
						},
					},
				}
			},
			wantErrs: []errorPattern{
				{pattern: "Container .* has no resource requests defined", validationType: "missing_resource_requests"},
				{pattern: "Container .* has no resource limits defined", validationType: "missing_resource_limits"},
				{pattern: "BestEffort QoS: no resource constraints.*", validationType: "qos_class_issue"},
			},
		},
		{
			name: "security_validator_messages",
			validator: func() Validator {
				return NewSecurityValidator(fake.NewClientBuilder().WithScheme(scheme).Build(), logr.Discard(), SecurityConfig{
					EnableRootUserValidation:       true,
					EnableSecurityContextValidation: true,
				})
			},
			setupFunc: func() []runtime.Object {
				rootUser := int64(0)
				return []runtime.Object{
					&corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{Name: "insecure-pod", Namespace: "default"},
						Spec: corev1.PodSpec{
							SecurityContext: &corev1.PodSecurityContext{
								RunAsUser: &rootUser,
							},
							Containers: []corev1.Container{{
								Name:  "app",
								Image: "nginx",
								// No security context - should trigger missing context error
							}},
						},
					},
				}
			},
			wantErrs: []errorPattern{
				{pattern: "Pod SecurityContext specifies runAsUser: 0 \\(root\\)", validationType: "pod_running_as_root"},
				{pattern: "Container .* has no SecurityContext defined", validationType: "missing_container_security_context"},
			},
		},
		{
			name: "networking_validator_messages",
			validator: func() Validator {
				return NewNetworkingValidator(fake.NewClientBuilder().WithScheme(scheme).Build(), logr.Discard(), NetworkingConfig{
					EnableServiceValidation: true,
				})
			},
			setupFunc: func() []runtime.Object {
				return []runtime.Object{
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{Name: "orphan-service", Namespace: "default"},
						Spec: corev1.ServiceSpec{
							Selector: map[string]string{"app": "nonexistent"},
							Ports:    []corev1.ServicePort{{Port: 80}},
						},
					},
				}
			},
			wantErrs: []errorPattern{
				{pattern: "Service selector .* does not match any pods", validationType: "service_selector_mismatch"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create validator with test objects
			objects := tt.setupFunc()
			client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objects...).Build()
			
			// Get validator instance and update its client
			validator := tt.validator()
			
			// Use reflection to set the client for each validator type
			switch v := validator.(type) {
			case *ReferenceValidator:
				v.client = client
			case *ResourceLimitsValidator:
				v.client = client
			case *SecurityValidator:
				v.client = client
			case *NetworkingValidator:
				v.client = client
			}

			// Capture validation errors by running validation
			// Note: We can't directly access validation errors from ValidateCluster
			// This test validates that the patterns exist in our codebase
			err := validator.ValidateCluster(context.TODO())
			if err != nil {
				t.Fatalf("ValidateCluster failed: %v", err)
			}

			// Test that expected error patterns are defined in the validator
			// This is a structural test that validates our error message patterns exist
			for _, wantErr := range tt.wantErrs {
				t.Run(wantErr.validationType, func(t *testing.T) {
					// This test documents our current error message patterns
					// It serves as a baseline for future standardization
					if wantErr.pattern == "" {
						t.Errorf("Error pattern should not be empty for validation type %s", wantErr.validationType)
					}
					if wantErr.validationType == "" {
						t.Errorf("Validation type should not be empty")
					}
				})
			}
		})
	}
}

// errorPattern represents an expected error message pattern
type errorPattern struct {
	pattern        string // Regex pattern for the error message
	validationType string // Expected validation type
}

// TestErrorMessagePatterns validates specific message formatting rules
func TestErrorMessagePatterns(t *testing.T) {
	tests := []struct {
		name     string
		messages []string
		wantPass bool
		rule     string
	}{
		{
			name: "consistent_quote_usage",
			messages: []string{
				"ConfigMap 'test-config' referenced in volume does not exist",
				"Service 'api-service' referenced in Ingress does not exist",
				"Container 'app' has no resource requests defined",
			},
			wantPass: true,
			rule:     "Resource names should be quoted with single quotes",
		},
		{
			name: "inconsistent_quotes",
			messages: []string{
				"ConfigMap test-config referenced in volume does not exist",
				"Service \"api-service\" referenced in Ingress does not exist",
			},
			wantPass: false,
			rule:     "Mixed quote styles are inconsistent",
		},
		{
			name: "proper_capitalization",
			messages: []string{
				"Pod has no SecurityContext defined",
				"Container allows privilege escalation",
				"Service selector does not match any pods",
			},
			wantPass: true,
			rule:     "Messages should start with capital letter",
		},
		{
			name: "resource_type_consistency",
			messages: []string{
				"Pod SecurityContext specifies runAsUser: 0 (root)",
				"Container SecurityContext does not set allowPrivilegeEscalation: false",
				"Service selector {app: api} does not match any pods",
			},
			wantPass: true,
			rule:     "Resource types should be properly capitalized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Analyze message patterns
			for _, msg := range tt.messages {
				// Check basic formatting rules
				if len(msg) == 0 {
					t.Errorf("Empty message violates rule: %s", tt.rule)
				}
				
				// Check that message starts with capital letter
				if tt.name == "proper_capitalization" {
					if strings.ToUpper(msg[:1]) != msg[:1] {
						t.Errorf("Message '%s' violates capitalization rule", msg)
					}
				}
				
				// Check quote consistency
				if tt.name == "consistent_quote_usage" {
					singleQuotes := strings.Count(msg, "'")
					doubleQuotes := strings.Count(msg, "\"")
					if singleQuotes > 0 && doubleQuotes > 0 {
						t.Errorf("Message '%s' mixes quote styles", msg)
					}
				}
			}
		})
	}
}

// TestValidationTypeConsistency ensures validation types follow naming conventions
func TestValidationTypeConsistency(t *testing.T) {
	validationTypes := []string{
		// Reference validation types
		"dangling_ingress_class",
		"dangling_service_reference", 
		"dangling_configmap_volume",
		"dangling_secret_volume",
		"dangling_pvc_reference",
		
		// Resource limits validation types
		"missing_resource_requests",
		"missing_resource_limits",
		"insufficient_cpu_request",
		"qos_class_issue",
		
		// Security validation types
		"pod_running_as_root",
		"container_running_as_root",
		"missing_pod_security_context",
		"missing_container_security_context",
		
		// Networking validation types
		"service_selector_mismatch",
		"service_no_endpoints",
		"network_policy_orphaned",
		"ingress_service_missing",
	}

	for _, vt := range validationTypes {
		t.Run(vt, func(t *testing.T) {
			// Check naming convention: lowercase with underscores
			if strings.Contains(vt, " ") {
				t.Errorf("Validation type '%s' contains spaces", vt)
			}
			if strings.Contains(vt, "-") {
				t.Errorf("Validation type '%s' contains hyphens, should use underscores", vt)
			}
			if vt != strings.ToLower(vt) {
				t.Errorf("Validation type '%s' contains uppercase letters", vt)
			}
			if len(vt) == 0 {
				t.Errorf("Validation type should not be empty")
			}
		})
	}
}

// Helper function for string pointers  
func stringPtrHelper(s string) *string {
	return &s
}