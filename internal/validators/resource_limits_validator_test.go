package validators

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestResourceLimitsValidator_ValidateDeploymentResources(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name           string
		deployments    []appsv1.Deployment
		config         ResourceLimitsConfig
		expectedErrors int
		errorTypes     []string
	}{
		{
			name: "deployment with missing requests",
			deployments: []appsv1.Deployment{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-deployment",
						Namespace: "default",
					},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name:  "test-container",
										Image: "test:latest",
										// No resources defined
									},
								},
							},
						},
					},
				},
			},
			config: ResourceLimitsConfig{
				EnableMissingRequestsValidation: true,
				EnableMissingLimitsValidation:   false,
				EnableQoSValidation:             false,
			},
			expectedErrors: 1,
			errorTypes:     []string{"missing_resource_requests"},
		},
		{
			name: "deployment with missing limits",
			deployments: []appsv1.Deployment{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-deployment",
						Namespace: "default",
					},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name:  "test-container",
										Image: "test:latest",
										Resources: corev1.ResourceRequirements{
											Requests: corev1.ResourceList{
												corev1.ResourceCPU:    resource.MustParse("100m"),
												corev1.ResourceMemory: resource.MustParse("128Mi"),
											},
											// No limits defined
										},
									},
								},
							},
						},
					},
				},
			},
			config: ResourceLimitsConfig{
				EnableMissingRequestsValidation: false,
				EnableMissingLimitsValidation:   true,
				EnableQoSValidation:             false,
			},
			expectedErrors: 1,
			errorTypes:     []string{"missing_resource_limits"},
		},
		{
			name: "deployment with proper resources",
			deployments: []appsv1.Deployment{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-deployment",
						Namespace: "default",
					},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name:  "test-container",
										Image: "test:latest",
										Resources: corev1.ResourceRequirements{
											Requests: corev1.ResourceList{
												corev1.ResourceCPU:    resource.MustParse("100m"),
												corev1.ResourceMemory: resource.MustParse("128Mi"),
											},
											Limits: corev1.ResourceList{
												corev1.ResourceCPU:    resource.MustParse("200m"),
												corev1.ResourceMemory: resource.MustParse("256Mi"),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			config: ResourceLimitsConfig{
				EnableMissingRequestsValidation: true,
				EnableMissingLimitsValidation:   true,
				EnableQoSValidation:             false,
			},
			expectedErrors: 0,
			errorTypes:     []string{},
		},
		{
			name: "deployment with insufficient CPU request",
			deployments: []appsv1.Deployment{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-deployment",
						Namespace: "default",
					},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name:  "test-container",
										Image: "test:latest",
										Resources: corev1.ResourceRequirements{
											Requests: corev1.ResourceList{
												corev1.ResourceCPU:    resource.MustParse("5m"), // Below minimum
												corev1.ResourceMemory: resource.MustParse("128Mi"),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			config: ResourceLimitsConfig{
				EnableMissingRequestsValidation: true,
				MinCPURequest:                   func() *resource.Quantity { q := resource.MustParse("10m"); return &q }(),
			},
			expectedErrors: 1,
			errorTypes:     []string{"insufficient_cpu_request"},
		},
		{
			name: "deployment with QoS analysis",
			deployments: []appsv1.Deployment{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-deployment",
						Namespace: "default",
					},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name:  "test-container",
										Image: "test:latest",
										Resources: corev1.ResourceRequirements{
											Requests: corev1.ResourceList{
												corev1.ResourceCPU:    resource.MustParse("100m"),
												corev1.ResourceMemory: resource.MustParse("128Mi"),
											},
											Limits: corev1.ResourceList{
												corev1.ResourceCPU:    resource.MustParse("200m"), // Different from requests
												corev1.ResourceMemory: resource.MustParse("256Mi"),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			config: ResourceLimitsConfig{
				EnableMissingRequestsValidation: false,
				EnableMissingLimitsValidation:   false,
				EnableQoSValidation:             true,
			},
			expectedErrors: 1,
			errorTypes:     []string{"qos_class_issue"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(convertDeploymentsToObjects(tt.deployments)...).
				Build()

			validator := NewResourceLimitsValidator(fakeClient, logr.Discard(), tt.config)

			errors, err := validator.validateDeploymentResources(context.TODO())
			if err != nil {
				t.Fatalf("validateDeploymentResources() error = %v", err)
			}

			if len(errors) != tt.expectedErrors {
				t.Errorf("validateDeploymentResources() got %d errors, want %d", len(errors), tt.expectedErrors)
				for _, e := range errors {
					t.Logf("Error: %s - %s", e.ValidationType, e.Message)
				}
			}

			// Verify error types
			errorTypeMap := make(map[string]bool)
			for _, err := range errors {
				errorTypeMap[err.ValidationType] = true
			}

			for _, expectedType := range tt.errorTypes {
				if !errorTypeMap[expectedType] {
					t.Errorf("Expected error type %s not found", expectedType)
				}
			}
		})
	}
}

func TestResourceLimitsValidator_ValidateCluster(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	deployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test-container",
							Image: "test:latest",
							// No resources defined
						},
					},
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(&deployment).
		Build()

	config := ResourceLimitsConfig{
		EnableMissingRequestsValidation: true,
		EnableMissingLimitsValidation:   true,
		EnableQoSValidation:             true,
	}

	validator := NewResourceLimitsValidator(fakeClient, logr.Discard(), config)

	err := validator.ValidateCluster(context.TODO())
	if err != nil {
		t.Fatalf("ValidateCluster() error = %v", err)
	}
}

func TestResourceLimitsValidator_GetValidationType(t *testing.T) {
	validator := &ResourceLimitsValidator{}
	expected := "resource_limits_validation"
	if got := validator.GetValidationType(); got != expected {
		t.Errorf("GetValidationType() = %v, want %v", got, expected)
	}
}

// Helper function to convert deployments to client.Object slice
func convertDeploymentsToObjects(deployments []appsv1.Deployment) []client.Object {
	objects := make([]client.Object, len(deployments))
	for i, deployment := range deployments {
		d := deployment // Create a copy to avoid pointer issues
		objects[i] = &d
	}
	return objects
}