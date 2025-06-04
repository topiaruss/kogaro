package validators

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	networkingv1 "k8s.io/api/networking/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	validationErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kogaro_validation_errors_total",
			Help: "Total number of validation errors found",
		},
		[]string{"resource_type", "validation_type", "namespace"},
	)
	
	validationRuns = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "kogaro_validation_runs_total",
			Help: "Total number of validation runs performed",
		},
	)
)

type ValidationError struct {
	ResourceType   string
	ResourceName   string
	Namespace      string
	ValidationType string
	Message        string
}

type ValidationConfig struct {
	EnableIngressValidation     bool
	EnableConfigMapValidation   bool
	EnableSecretValidation      bool
	EnablePVCValidation         bool
	EnableServiceAccountValidation bool
}

type ReferenceValidator struct {
	client client.Client
	log    logr.Logger
	config ValidationConfig
}

func NewReferenceValidator(client client.Client, log logr.Logger, config ValidationConfig) *ReferenceValidator {
	return &ReferenceValidator{
		client: client,
		log:    log.WithName("reference-validator"),
		config: config,
	}
}

func (v *ReferenceValidator) ValidateCluster(ctx context.Context) error {
	validationRuns.Inc()
	
	var allErrors []ValidationError

	// Validate Ingress references
	if v.config.EnableIngressValidation {
		ingressErrors, err := v.validateIngressReferences(ctx)
		if err != nil {
			return fmt.Errorf("failed to validate ingress references: %w", err)
		}
		allErrors = append(allErrors, ingressErrors...)
	}

	// Validate ConfigMap references
	if v.config.EnableConfigMapValidation {
		configMapErrors, err := v.validateConfigMapReferences(ctx)
		if err != nil {
			return fmt.Errorf("failed to validate configmap references: %w", err)
		}
		allErrors = append(allErrors, configMapErrors...)
	}

	// Validate Secret references
	if v.config.EnableSecretValidation {
		secretErrors, err := v.validateSecretReferences(ctx)
		if err != nil {
			return fmt.Errorf("failed to validate secret references: %w", err)
		}
		allErrors = append(allErrors, secretErrors...)
	}

	// Validate PVC references
	if v.config.EnablePVCValidation {
		pvcErrors, err := v.validatePVCReferences(ctx)
		if err != nil {
			return fmt.Errorf("failed to validate pvc references: %w", err)
		}
		allErrors = append(allErrors, pvcErrors...)
	}

	// Validate ServiceAccount references
	if v.config.EnableServiceAccountValidation {
		saErrors, err := v.validateServiceAccountReferences(ctx)
		if err != nil {
			return fmt.Errorf("failed to validate serviceaccount references: %w", err)
		}
		allErrors = append(allErrors, saErrors...)
	}

	// Log all validation errors
	for _, validationErr := range allErrors {
		v.log.Info("validation error found",
			"resource_type", validationErr.ResourceType,
			"resource_name", validationErr.ResourceName,
			"namespace", validationErr.Namespace,
			"validation_type", validationErr.ValidationType,
			"message", validationErr.Message,
		)
		
		validationErrors.WithLabelValues(
			validationErr.ResourceType,
			validationErr.ValidationType,
			validationErr.Namespace,
		).Inc()
	}

	v.log.Info("cluster validation completed", "total_errors", len(allErrors))
	return nil
}

func (v *ReferenceValidator) validateIngressReferences(ctx context.Context) ([]ValidationError, error) {
	var errors []ValidationError

	// Get all Ingresses
	var ingresses networkingv1.IngressList
	if err := v.client.List(ctx, &ingresses); err != nil {
		return nil, fmt.Errorf("failed to list ingresses: %w", err)
	}

	// Get all IngressClasses for validation
	var ingressClasses networkingv1.IngressClassList
	if err := v.client.List(ctx, &ingressClasses); err != nil {
		return nil, fmt.Errorf("failed to list ingress classes: %w", err)
	}

	// Build a map of existing IngressClass names
	existingClasses := make(map[string]bool)
	for _, ic := range ingressClasses.Items {
		existingClasses[ic.Name] = true
	}

	// Validate each Ingress
	for _, ingress := range ingresses.Items {
		if ingress.Spec.IngressClassName != nil {
			className := *ingress.Spec.IngressClassName
			if !existingClasses[className] {
				errors = append(errors, ValidationError{
					ResourceType:   "Ingress",
					ResourceName:   ingress.Name,
					Namespace:      ingress.Namespace,
					ValidationType: "dangling_ingress_class",
					Message:        fmt.Sprintf("IngressClass '%s' does not exist", className),
				})
			}
		}

		// Validate Service references in Ingress rules
		for _, rule := range ingress.Spec.Rules {
			if rule.HTTP != nil {
				for _, path := range rule.HTTP.Paths {
					serviceName := path.Backend.Service.Name
					
					// Check if the service exists
					var service corev1.Service
					err := v.client.Get(ctx, types.NamespacedName{
						Name:      serviceName,
						Namespace: ingress.Namespace,
					}, &service)
					
					if err != nil {
						errors = append(errors, ValidationError{
							ResourceType:   "Ingress",
							ResourceName:   ingress.Name,
							Namespace:      ingress.Namespace,
							ValidationType: "dangling_service_reference",
							Message:        fmt.Sprintf("Service '%s' referenced in Ingress does not exist", serviceName),
						})
					}
				}
			}
		}
	}

	return errors, nil
}

func (v *ReferenceValidator) validateConfigMapReferences(ctx context.Context) ([]ValidationError, error) {
	var errors []ValidationError

	// Get all Pods to check ConfigMap references
	var pods corev1.PodList
	if err := v.client.List(ctx, &pods); err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	for _, pod := range pods.Items {
		// Check ConfigMap references in volumes
		for _, volume := range pod.Spec.Volumes {
			if volume.ConfigMap != nil {
				configMapName := volume.ConfigMap.Name
				if err := v.validateConfigMapExists(ctx, configMapName, pod.Namespace); err != nil {
					errors = append(errors, ValidationError{
						ResourceType:   "Pod",
						ResourceName:   pod.Name,
						Namespace:      pod.Namespace,
						ValidationType: "dangling_configmap_volume",
						Message:        fmt.Sprintf("ConfigMap '%s' referenced in volume does not exist", configMapName),
					})
				}
			}
		}

		// Check ConfigMap references in envFrom
		for _, container := range pod.Spec.Containers {
			for _, envFrom := range container.EnvFrom {
				if envFrom.ConfigMapRef != nil {
					configMapName := envFrom.ConfigMapRef.Name
					if err := v.validateConfigMapExists(ctx, configMapName, pod.Namespace); err != nil {
						errors = append(errors, ValidationError{
							ResourceType:   "Pod",
							ResourceName:   pod.Name,
							Namespace:      pod.Namespace,
							ValidationType: "dangling_configmap_envfrom",
							Message:        fmt.Sprintf("ConfigMap '%s' referenced in envFrom does not exist", configMapName),
						})
					}
				}
			}
		}
	}

	return errors, nil
}

func (v *ReferenceValidator) validateConfigMapExists(ctx context.Context, name, namespace string) error {
	var configMap corev1.ConfigMap
	return v.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, &configMap)
}

func (v *ReferenceValidator) validateSecretReferences(ctx context.Context) ([]ValidationError, error) {
	var errors []ValidationError

	// Get all Pods to check Secret references
	var pods corev1.PodList
	if err := v.client.List(ctx, &pods); err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	for _, pod := range pods.Items {
		// Check Secret references in volumes
		for _, volume := range pod.Spec.Volumes {
			if volume.Secret != nil {
				secretName := volume.Secret.SecretName
				if err := v.validateSecretExists(ctx, secretName, pod.Namespace); err != nil {
					errors = append(errors, ValidationError{
						ResourceType:   "Pod",
						ResourceName:   pod.Name,
						Namespace:      pod.Namespace,
						ValidationType: "dangling_secret_volume",
						Message:        fmt.Sprintf("Secret '%s' referenced in volume does not exist", secretName),
					})
				}
			}
		}

		// Check Secret references in envFrom and env
		for _, container := range pod.Spec.Containers {
			for _, envFrom := range container.EnvFrom {
				if envFrom.SecretRef != nil {
					secretName := envFrom.SecretRef.Name
					if err := v.validateSecretExists(ctx, secretName, pod.Namespace); err != nil {
						errors = append(errors, ValidationError{
							ResourceType:   "Pod",
							ResourceName:   pod.Name,
							Namespace:      pod.Namespace,
							ValidationType: "dangling_secret_envfrom",
							Message:        fmt.Sprintf("Secret '%s' referenced in envFrom does not exist", secretName),
						})
					}
				}
			}

			for _, env := range container.Env {
				if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
					secretName := env.ValueFrom.SecretKeyRef.Name
					if err := v.validateSecretExists(ctx, secretName, pod.Namespace); err != nil {
						errors = append(errors, ValidationError{
							ResourceType:   "Pod",
							ResourceName:   pod.Name,
							Namespace:      pod.Namespace,
							ValidationType: "dangling_secret_env",
							Message:        fmt.Sprintf("Secret '%s' referenced in env does not exist", secretName),
						})
					}
				}
			}
		}
	}

	// Check Ingress TLS secrets
	var ingresses networkingv1.IngressList
	if err := v.client.List(ctx, &ingresses); err != nil {
		return nil, fmt.Errorf("failed to list ingresses: %w", err)
	}

	for _, ingress := range ingresses.Items {
		for _, tls := range ingress.Spec.TLS {
			if tls.SecretName != "" {
				if err := v.validateSecretExists(ctx, tls.SecretName, ingress.Namespace); err != nil {
					errors = append(errors, ValidationError{
						ResourceType:   "Ingress",
						ResourceName:   ingress.Name,
						Namespace:      ingress.Namespace,
						ValidationType: "dangling_tls_secret",
						Message:        fmt.Sprintf("TLS Secret '%s' referenced in Ingress does not exist", tls.SecretName),
					})
				}
			}
		}
	}

	return errors, nil
}

func (v *ReferenceValidator) validatePVCReferences(ctx context.Context) ([]ValidationError, error) {
	var errors []ValidationError

	// Get all PVCs to check StorageClass references
	var pvcs corev1.PersistentVolumeClaimList
	if err := v.client.List(ctx, &pvcs); err != nil {
		return nil, fmt.Errorf("failed to list pvcs: %w", err)
	}

	// Get all StorageClasses for validation
	var storageClasses storagev1.StorageClassList
	if err := v.client.List(ctx, &storageClasses); err != nil {
		return nil, fmt.Errorf("failed to list storage classes: %w", err)
	}

	// Build a map of existing StorageClass names
	existingClasses := make(map[string]bool)
	for _, sc := range storageClasses.Items {
		existingClasses[sc.Name] = true
	}

	for _, pvc := range pvcs.Items {
		if pvc.Spec.StorageClassName != nil {
			className := *pvc.Spec.StorageClassName
			if !existingClasses[className] {
				errors = append(errors, ValidationError{
					ResourceType:   "PersistentVolumeClaim",
					ResourceName:   pvc.Name,
					Namespace:      pvc.Namespace,
					ValidationType: "dangling_storage_class",
					Message:        fmt.Sprintf("StorageClass '%s' does not exist", className),
				})
			}
		}
	}

	// Check Pod volumes referencing PVCs
	var pods corev1.PodList
	if err := v.client.List(ctx, &pods); err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	for _, pod := range pods.Items {
		for _, volume := range pod.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil {
				pvcName := volume.PersistentVolumeClaim.ClaimName
				if err := v.validatePVCExists(ctx, pvcName, pod.Namespace); err != nil {
					errors = append(errors, ValidationError{
						ResourceType:   "Pod",
						ResourceName:   pod.Name,
						Namespace:      pod.Namespace,
						ValidationType: "dangling_pvc_reference",
						Message:        fmt.Sprintf("PVC '%s' referenced in volume does not exist", pvcName),
					})
				}
			}
		}
	}

	return errors, nil
}

func (v *ReferenceValidator) validateServiceAccountReferences(ctx context.Context) ([]ValidationError, error) {
	var errors []ValidationError

	// Get all Pods to check ServiceAccount references
	var pods corev1.PodList
	if err := v.client.List(ctx, &pods); err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	for _, pod := range pods.Items {
		saName := pod.Spec.ServiceAccountName
		if saName == "" {
			saName = "default"
		}
		
		if err := v.validateServiceAccountExists(ctx, saName, pod.Namespace); err != nil {
			errors = append(errors, ValidationError{
				ResourceType:   "Pod",
				ResourceName:   pod.Name,
				Namespace:      pod.Namespace,
				ValidationType: "dangling_service_account",
				Message:        fmt.Sprintf("ServiceAccount '%s' does not exist", saName),
			})
		}
	}

	return errors, nil
}

func (v *ReferenceValidator) validateSecretExists(ctx context.Context, name, namespace string) error {
	var secret corev1.Secret
	return v.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, &secret)
}

func (v *ReferenceValidator) validatePVCExists(ctx context.Context, name, namespace string) error {
	var pvc corev1.PersistentVolumeClaim
	return v.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, &pvc)
}

func (v *ReferenceValidator) validateServiceAccountExists(ctx context.Context, name, namespace string) error {
	var sa corev1.ServiceAccount
	return v.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, &sa)
}