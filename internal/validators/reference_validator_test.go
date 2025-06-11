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
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestReferenceValidator_ValidateIngressReferences(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = networkingv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name           string
		objects        []client.Object
		expectedErrors int
		errorTypes     []string
	}{
		{
			name: "valid ingress with existing ingressclass and service",
			objects: []client.Object{
				&networkingv1.IngressClass{
					ObjectMeta: metav1.ObjectMeta{Name: "nginx"},
				},
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{Name: "test-service", Namespace: "default"},
				},
				&networkingv1.Ingress{
					ObjectMeta: metav1.ObjectMeta{Name: "test-ingress", Namespace: "default"},
					Spec: networkingv1.IngressSpec{
						IngressClassName: stringPtr("nginx"),
						Rules: []networkingv1.IngressRule{
							{
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "test-service",
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
			expectedErrors: 0,
			errorTypes:     []string{},
		},
		{
			name: "ingress with missing ingressclass",
			objects: []client.Object{
				&networkingv1.Ingress{
					ObjectMeta: metav1.ObjectMeta{Name: "test-ingress", Namespace: "default"},
					Spec: networkingv1.IngressSpec{
						IngressClassName: stringPtr("missing-class"),
					},
				},
			},
			expectedErrors: 1,
			errorTypes:     []string{"dangling_ingress_class"},
		},
		{
			name: "ingress with missing service",
			objects: []client.Object{
				&networkingv1.IngressClass{
					ObjectMeta: metav1.ObjectMeta{Name: "nginx"},
				},
				&networkingv1.Ingress{
					ObjectMeta: metav1.ObjectMeta{Name: "test-ingress", Namespace: "default"},
					Spec: networkingv1.IngressSpec{
						IngressClassName: stringPtr("nginx"),
						Rules: []networkingv1.IngressRule{
							{
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "missing-service",
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
			expectedErrors: 1,
			errorTypes:     []string{"dangling_service_reference"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.objects...).
				Build()

			config := ValidationConfig{EnableIngressValidation: true}
			validator := NewReferenceValidator(fakeClient, logr.Discard(), config)

			errors, err := validator.validateIngressReferences(context.TODO())
			if err != nil {
				t.Fatalf("validateIngressReferences() error = %v", err)
			}

			if len(errors) != tt.expectedErrors {
				t.Errorf("validateIngressReferences() got %d errors, want %d", len(errors), tt.expectedErrors)
			}

			for i, expectedType := range tt.errorTypes {
				if i >= len(errors) {
					t.Errorf("Expected error type %s but got fewer errors than expected", expectedType)
					continue
				}
				if errors[i].ValidationType != expectedType {
					t.Errorf("Expected error type %s, got %s", expectedType, errors[i].ValidationType)
				}
			}
		})
	}
}

func TestReferenceValidator_ValidateConfigMapReferences(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name           string
		objects        []client.Object
		expectedErrors int
		errorTypes     []string
	}{
		{
			name: "pod with existing configmap",
			objects: []client.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: "test-config", Namespace: "default"},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default"},
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{
							{
								Name: "config-volume",
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "test-config",
										},
									},
								},
							},
						},
						Containers: []corev1.Container{
							{
								Name:  "test-container",
								Image: "nginx",
								EnvFrom: []corev1.EnvFromSource{
									{
										ConfigMapRef: &corev1.ConfigMapEnvSource{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "test-config",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedErrors: 0,
			errorTypes:     []string{},
		},
		{
			name: "pod with missing configmap in volume",
			objects: []client.Object{
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default"},
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{
							{
								Name: "config-volume",
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "missing-config",
										},
									},
								},
							},
						},
						Containers: []corev1.Container{
							{
								Name:  "test-container",
								Image: "nginx",
							},
						},
					},
				},
			},
			expectedErrors: 1,
			errorTypes:     []string{"dangling_configmap_volume"},
		},
		{
			name: "pod with missing configmap in envfrom",
			objects: []client.Object{
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default"},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "test-container",
								Image: "nginx",
								EnvFrom: []corev1.EnvFromSource{
									{
										ConfigMapRef: &corev1.ConfigMapEnvSource{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "missing-config",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedErrors: 1,
			errorTypes:     []string{"dangling_configmap_envfrom"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.objects...).
				Build()

			config := ValidationConfig{EnableConfigMapValidation: true}
			validator := NewReferenceValidator(fakeClient, logr.Discard(), config)

			errors, err := validator.validateConfigMapReferences(context.TODO())
			if err != nil {
				t.Fatalf("validateConfigMapReferences() error = %v", err)
			}

			if len(errors) != tt.expectedErrors {
				t.Errorf("validateConfigMapReferences() got %d errors, want %d", len(errors), tt.expectedErrors)
			}

			for i, expectedType := range tt.errorTypes {
				if i >= len(errors) {
					t.Errorf("Expected error type %s but got fewer errors than expected", expectedType)
					continue
				}
				if errors[i].ValidationType != expectedType {
					t.Errorf("Expected error type %s, got %s", expectedType, errors[i].ValidationType)
				}
			}
		})
	}
}

func TestReferenceValidator_ValidateSecretReferences(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = networkingv1.AddToScheme(scheme)

	tests := []struct {
		name           string
		objects        []client.Object
		expectedErrors int
		errorTypes     []string
	}{
		{
			name: "pod with existing secret",
			objects: []client.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "test-secret", Namespace: "default"},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default"},
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{
							{
								Name: "secret-volume",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: "test-secret",
									},
								},
							},
						},
						Containers: []corev1.Container{
							{
								Name:  "test-container",
								Image: "nginx",
							},
						},
					},
				},
			},
			expectedErrors: 0,
			errorTypes:     []string{},
		},
		{
			name: "ingress with missing tls secret",
			objects: []client.Object{
				&networkingv1.Ingress{
					ObjectMeta: metav1.ObjectMeta{Name: "test-ingress", Namespace: "default"},
					Spec: networkingv1.IngressSpec{
						TLS: []networkingv1.IngressTLS{
							{
								SecretName: "missing-tls-secret",
							},
						},
					},
				},
			},
			expectedErrors: 1,
			errorTypes:     []string{"dangling_tls_secret"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.objects...).
				Build()

			config := ValidationConfig{EnableSecretValidation: true}
			validator := NewReferenceValidator(fakeClient, logr.Discard(), config)

			errors, err := validator.validateSecretReferences(context.TODO())
			if err != nil {
				t.Fatalf("validateSecretReferences() error = %v", err)
			}

			if len(errors) != tt.expectedErrors {
				t.Errorf("validateSecretReferences() got %d errors, want %d", len(errors), tt.expectedErrors)
			}

			for i, expectedType := range tt.errorTypes {
				if i >= len(errors) {
					t.Errorf("Expected error type %s but got fewer errors than expected", expectedType)
					continue
				}
				if errors[i].ValidationType != expectedType {
					t.Errorf("Expected error type %s, got %s", expectedType, errors[i].ValidationType)
				}
			}
		})
	}
}

func TestReferenceValidator_ValidatePVCReferences(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = storagev1.AddToScheme(scheme)

	tests := []struct {
		name           string
		objects        []client.Object
		expectedErrors int
		errorTypes     []string
	}{
		{
			name: "pvc with existing storageclass",
			objects: []client.Object{
				&storagev1.StorageClass{
					ObjectMeta: metav1.ObjectMeta{Name: "fast-ssd"},
				},
				&corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pvc", Namespace: "default"},
					Spec: corev1.PersistentVolumeClaimSpec{
						StorageClassName: stringPtr("fast-ssd"),
					},
				},
			},
			expectedErrors: 0,
			errorTypes:     []string{},
		},
		{
			name: "pvc with missing storageclass",
			objects: []client.Object{
				&corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pvc", Namespace: "default"},
					Spec: corev1.PersistentVolumeClaimSpec{
						StorageClassName: stringPtr("missing-class"),
					},
				},
			},
			expectedErrors: 1,
			errorTypes:     []string{"dangling_storage_class"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.objects...).
				Build()

			config := ValidationConfig{EnablePVCValidation: true}
			validator := NewReferenceValidator(fakeClient, logr.Discard(), config)

			errors, err := validator.validatePVCReferences(context.TODO())
			if err != nil {
				t.Fatalf("validatePVCReferences() error = %v", err)
			}

			if len(errors) != tt.expectedErrors {
				t.Errorf("validatePVCReferences() got %d errors, want %d", len(errors), tt.expectedErrors)
			}

			for i, expectedType := range tt.errorTypes {
				if i >= len(errors) {
					t.Errorf("Expected error type %s but got fewer errors than expected", expectedType)
					continue
				}
				if errors[i].ValidationType != expectedType {
					t.Errorf("Expected error type %s, got %s", expectedType, errors[i].ValidationType)
				}
			}
		})
	}
}

func TestReferenceValidator_ValidateCluster(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = networkingv1.AddToScheme(scheme)
	_ = storagev1.AddToScheme(scheme)

	// Test with all validations enabled
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects().
		Build()

	config := ValidationConfig{
		EnableIngressValidation:        true,
		EnableConfigMapValidation:      true,
		EnableSecretValidation:         true,
		EnablePVCValidation:            true,
		EnableServiceAccountValidation: true,
	}

	validator := NewReferenceValidator(fakeClient, zap.New(), config)
	mockLogReceiver := &MockLogReceiver{}
	validator.SetLogReceiver(mockLogReceiver)

	err := validator.ValidateCluster(context.TODO())
	if err != nil {
		t.Fatalf("ValidateCluster() error = %v", err)
	}

	// Test with all validations disabled
	config = ValidationConfig{}
	validator = NewReferenceValidator(fakeClient, zap.New(), config)

	err = validator.ValidateCluster(context.TODO())
	if err != nil {
		t.Fatalf("ValidateCluster() with disabled validations error = %v", err)
	}
}

