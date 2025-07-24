package validators

import (
	"context"
	"testing"

	"github.com/distribution/reference"
	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/topiaruss/kogaro/internal/metrics"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestImageValidator_ValidateCluster(t *testing.T) {
	// Register metrics for testing
	metrics.RegisterMetrics()

	tests := []struct {
		name           string
		objects        []client.Object
		nodes          []corev1.Node
		config         ImageValidatorConfig
		expectedErrors []string
	}{
		{
			name: "valid image reference",
			objects: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-deployment",
						Namespace: "test-namespace",
					},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name:  "test-container",
										Image: "nginx:latest",
									},
								},
							},
						},
					},
				},
			},
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-1",
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{
							Architecture: "amd64",
						},
					},
				},
			},
			config: ImageValidatorConfig{
				EnableImageValidation: true,
			},
			expectedErrors: []string{},
		},
		{
			name: "invalid image reference",
			objects: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-deployment",
						Namespace: "test-namespace",
					},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name:  "test-container",
										Image: "invalid:image:reference",
									},
								},
							},
						},
					},
				},
			},
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-1",
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{
							Architecture: "amd64",
						},
					},
				},
			},
			config: ImageValidatorConfig{
				EnableImageValidation: true,
			},
			expectedErrors: []string{"invalid_image_reference"},
		},
		{
			name: "missing image with warning",
			objects: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-deployment",
						Namespace: "test-namespace",
					},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name:  "test-container",
										Image: "nonexistent:latest",
									},
								},
							},
						},
					},
				},
			},
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-1",
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{
							Architecture: "amd64",
						},
					},
				},
			},
			config: ImageValidatorConfig{
				EnableImageValidation: true,
				AllowMissingImages:    true,
			},
			expectedErrors: []string{"missing_image_warning"},
		},
		{
			name: "architecture mismatch with warning",
			objects: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-deployment",
						Namespace: "test-namespace",
					},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name:  "test-container",
										Image: "arm64-image:latest",
									},
								},
							},
						},
					},
				},
			},
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-1",
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{
							Architecture: "amd64",
						},
					},
				},
			},
			config: ImageValidatorConfig{
				EnableImageValidation:     true,
				AllowArchitectureMismatch: true,
			},
			expectedErrors: []string{"architecture_mismatch_warning"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = corev1.AddToScheme(scheme)
			_ = appsv1.AddToScheme(scheme)

			fakeClient := crfake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.objects...).
				Build()

			fakeK8sClient := k8sfake.NewSimpleClientset()
			for _, node := range tt.nodes {
				_, err := fakeK8sClient.CoreV1().Nodes().Create(context.Background(), &node, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("failed to create test node: %v", err)
				}
			}

			validator := NewImageValidator(fakeClient, fakeK8sClient, logr.Discard(), tt.config)

			// Inject test-specific behavior for image existence and architecture
			validator.checkImageExistsFunc = func(ref reference.Reference) (bool, error) {
				img := ref.String()
				if img == "nonexistent:latest" {
					return false, nil
				}
				if img == "invalid:image:reference" {
					// This should not be called for invalid references, but just in case
					return false, nil
				}
				return true, nil
			}
			validator.getImageArchitectureFunc = func(ref reference.Reference) (string, error) {
				img := ref.String()
				if img == "arm64-image:latest" {
					return "arm64", nil
				}
				return "amd64", nil
			}

			mockLogReceiver := &MockLogReceiver{}
			validator.SetLogReceiver(mockLogReceiver)

			err := validator.ValidateCluster(context.Background())
			if err != nil {
				t.Errorf("ValidateCluster() error = %v", err)
				return
			}

			// Check for the correct metric label for each error type
			for _, expectedError := range tt.expectedErrors {
				// Determine the correct severity and expected pattern based on error type
				severity := "error"
				expectedPattern := "false"
				errorCode := "KOGARO-IMG-001" // Default error code

				// Warning errors have different severity and error codes
				switch expectedError {
				case "missing_image_warning":
					severity = "warning"
					errorCode = "KOGARO-IMG-003"
				case "architecture_mismatch_warning":
					severity = "warning"
					errorCode = "KOGARO-IMG-005"
				case "invalid_image_reference":
					errorCode = "KOGARO-IMG-001"
				}

				validationErrors, err := metrics.ValidationErrors.GetMetricWithLabelValues("Deployment", expectedError, "test-namespace", "test-deployment", severity, "application", expectedPattern, errorCode)
				if err != nil {
					t.Fatalf("failed to get validation errors metric for %s: %v", expectedError, err)
				}
				if validationErrors == nil {
					t.Fatalf("validation errors metric for %s is nil", expectedError)
				}
				errorCount := int(testutil.ToFloat64(validationErrors))
				if errorCount != 1 {
					t.Errorf("expected 1 validation error for %s, got %d", expectedError, errorCount)
				}
			}
		})
	}
}
