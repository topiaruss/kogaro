// Copyright 2025 Russell Ferriday
// Licensed under the Apache License, Version 2.0
//
// Kogaro - Kubernetes Configuration Hygiene Agent

// Package validators provides Kubernetes resource reference validation functionality.
//
// This package implements comprehensive validation of resource references within
// a Kubernetes cluster, detecting dangling references that could cause silent
// failures in applications. It supports validation of Ingress, ConfigMap, Secret,
// PVC, and ServiceAccount references with configurable validation rules.
package validators

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/topiaruss/kogaro/internal/metrics"
)

// ValidationConfig defines which types of validation checks to perform
type ValidationConfig struct {
	EnableIngressValidation        bool
	EnableConfigMapValidation      bool
	EnableSecretValidation         bool
	EnablePVCValidation            bool
	EnableServiceAccountValidation bool
}

// ReferenceValidator validates Kubernetes resource references across the cluster
type ReferenceValidator struct {
	client               client.Client
	log                  logr.Logger
	config               ValidationConfig
	sharedConfig         SharedConfig
	lastValidationErrors []ValidationError
	logReceiver          LogReceiver
}

// GetValidationType returns the validation type identifier for reference validation
func (v *ReferenceValidator) GetValidationType() string {
	return "reference_validation"
}

// NewReferenceValidator creates a new ReferenceValidator with the given client, logger and config
func NewReferenceValidator(client client.Client, log logr.Logger, config ValidationConfig) *ReferenceValidator {
	return &ReferenceValidator{
		client:       client,
		log:          log.WithName("reference-validator"),
		config:       config,
		sharedConfig: DefaultSharedConfig(),
	}
}

// SetClient updates the client used by the validator
func (v *ReferenceValidator) SetClient(c client.Client) {
	v.client = c
}

// SetLogReceiver updates the log receiver used by the validator
func (v *ReferenceValidator) SetLogReceiver(lr LogReceiver) {
	v.logReceiver = lr
}

// GetLastValidationErrors returns the errors from the last validation run
func (v *ReferenceValidator) GetLastValidationErrors() []ValidationError {
	return v.lastValidationErrors
}

// ValidateCluster performs comprehensive validation of resource references across the entire cluster
func (v *ReferenceValidator) ValidateCluster(ctx context.Context) error {
	metrics.ValidationRuns.Inc()

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

	// Log all validation errors and update metrics
	for _, validationErr := range allErrors {
		// Always use LogReceiver for consistent dependency injection
		v.logReceiver.LogValidationError(
			"reference",
			validationErr.ResourceType,
			validationErr.ResourceName,
			validationErr.Namespace,
			validationErr.ValidationType,
			validationErr.Message,
		)

		// Use new temporal-aware metrics recording
		metrics.RecordValidationErrorWithState(
			validationErr.ResourceType,
			validationErr.ResourceName,
			validationErr.Namespace,
			validationErr.ValidationType,
			string(validationErr.Severity),
			false, // expectedPattern - false for actual errors
		)
	}

	v.log.Info("validation completed", "validator_type", "reference", "total_errors", len(allErrors))

	// Store errors for CLI reporting
	v.lastValidationErrors = allErrors
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
		// Skip system namespaces
		if v.sharedConfig.IsSystemNamespace(ingress.Namespace) {
			continue
		}

		if ingress.Spec.IngressClassName != nil {
			className := *ingress.Spec.IngressClassName
			if !existingClasses[className] {
				errors = append(errors, NewValidationErrorWithCode("Ingress", ingress.Name, ingress.Namespace, "dangling_ingress_class", "KOGARO-REF-001", fmt.Sprintf("IngressClass '%s' does not exist", className)).
					WithSeverity(SeverityError).
					WithRemediationHint(fmt.Sprintf("Create IngressClass '%s' or update Ingress to use an existing IngressClass", className)).
					WithRelatedResources(fmt.Sprintf("IngressClass/%s", className)).
					WithDetail("missing_class", className))
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
						errors = append(errors, NewValidationErrorWithCode("Ingress", ingress.Name, ingress.Namespace, "dangling_service_reference", "KOGARO-REF-002", fmt.Sprintf("Service '%s' referenced in Ingress does not exist", serviceName)).
							WithSeverity(SeverityError).
							WithRemediationHint(fmt.Sprintf("Create Service '%s' in namespace '%s' or update Ingress to reference an existing Service", serviceName, ingress.Namespace)).
							WithRelatedResources(fmt.Sprintf("Service/%s", serviceName)).
							WithDetail("missing_service", serviceName).
							WithDetail("ingress_path", rule.Host))
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
		// Skip system namespaces
		if v.sharedConfig.IsSystemNamespace(pod.Namespace) {
			continue
		}

		// Check ConfigMap references in volumes
		for _, volume := range pod.Spec.Volumes {
			if volume.ConfigMap != nil {
				configMapName := volume.ConfigMap.Name
				if err := v.validateConfigMapExists(ctx, configMapName, pod.Namespace); err != nil {
					errors = append(errors, NewValidationErrorWithCode("Pod", pod.Name, pod.Namespace, "dangling_configmap_volume", "KOGARO-REF-003", fmt.Sprintf("ConfigMap '%s' referenced in volume does not exist", configMapName)).
						WithSeverity(SeverityError).
						WithRemediationHint(fmt.Sprintf("Create ConfigMap '%s' in namespace '%s' or update the volume reference to use an existing ConfigMap", configMapName, pod.Namespace)).
						WithRelatedResources(fmt.Sprintf("ConfigMap/%s", configMapName)).
						WithDetail("missing_configmap", configMapName).
						WithDetail("volume_name", volume.Name))
				}
			}
		}

		// Check ConfigMap references in envFrom
		for _, container := range pod.Spec.Containers {
			for _, envFrom := range container.EnvFrom {
				if envFrom.ConfigMapRef != nil {
					configMapName := envFrom.ConfigMapRef.Name
					if err := v.validateConfigMapExists(ctx, configMapName, pod.Namespace); err != nil {
						errors = append(errors, NewValidationErrorWithCode("Pod", pod.Name, pod.Namespace, "dangling_configmap_envfrom", "KOGARO-REF-004", fmt.Sprintf("ConfigMap '%s' referenced in envFrom does not exist", configMapName)).
							WithSeverity(SeverityError).
							WithRemediationHint(fmt.Sprintf("Create ConfigMap '%s' in namespace '%s' or update the envFrom reference to use an existing ConfigMap", configMapName, pod.Namespace)).
							WithRelatedResources(fmt.Sprintf("ConfigMap/%s", configMapName)).
							WithDetail("missing_configmap", configMapName).
							WithDetail("container_name", container.Name))
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
		// Skip system namespaces
		if v.sharedConfig.IsSystemNamespace(pod.Namespace) {
			continue
		}

		// Check Secret references in volumes
		for _, volume := range pod.Spec.Volumes {
			if volume.Secret != nil {
				secretName := volume.Secret.SecretName
				if err := v.validateSecretExists(ctx, secretName, pod.Namespace); err != nil {
					errors = append(errors, NewValidationErrorWithCode("Pod", pod.Name, pod.Namespace, "dangling_secret_volume", "KOGARO-REF-005", fmt.Sprintf("Secret '%s' referenced in volume does not exist", secretName)).
						WithSeverity(SeverityError).
						WithRemediationHint(fmt.Sprintf("Create Secret '%s' in namespace '%s' or update the volume reference to use an existing Secret", secretName, pod.Namespace)).
						WithRelatedResources(fmt.Sprintf("Secret/%s", secretName)).
						WithDetail("missing_secret", secretName).
						WithDetail("volume_name", volume.Name))
				}
			}
		}

		// Check Secret references in envFrom and env
		for _, container := range pod.Spec.Containers {
			for _, envFrom := range container.EnvFrom {
				if envFrom.SecretRef != nil {
					secretName := envFrom.SecretRef.Name
					if err := v.validateSecretExists(ctx, secretName, pod.Namespace); err != nil {
						errors = append(errors, NewValidationErrorWithCode("Pod", pod.Name, pod.Namespace, "dangling_secret_envfrom", "KOGARO-REF-006", fmt.Sprintf("Secret '%s' referenced in envFrom does not exist", secretName)).
							WithSeverity(SeverityError).
							WithRemediationHint(fmt.Sprintf("Create Secret '%s' in namespace '%s' or update the envFrom reference to use an existing Secret", secretName, pod.Namespace)).
							WithRelatedResources(fmt.Sprintf("Secret/%s", secretName)).
							WithDetail("missing_secret", secretName).
							WithDetail("container_name", container.Name))
					}
				}
			}

			for _, env := range container.Env {
				if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
					secretName := env.ValueFrom.SecretKeyRef.Name
					if err := v.validateSecretExists(ctx, secretName, pod.Namespace); err != nil {
						errors = append(errors, NewValidationErrorWithCode("Pod", pod.Name, pod.Namespace, "dangling_secret_env", "KOGARO-REF-007", fmt.Sprintf("Secret '%s' referenced in env does not exist", secretName)).
							WithSeverity(SeverityError).
							WithRemediationHint(fmt.Sprintf("Create Secret '%s' in namespace '%s' or update the env reference to use an existing Secret", secretName, pod.Namespace)).
							WithRelatedResources(fmt.Sprintf("Secret/%s", secretName)).
							WithDetail("missing_secret", secretName).
							WithDetail("container_name", container.Name).
							WithDetail("env_var_name", env.Name))
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
		// Skip system namespaces
		if v.sharedConfig.IsSystemNamespace(ingress.Namespace) {
			continue
		}

		for _, tls := range ingress.Spec.TLS {
			if tls.SecretName != "" {
				if err := v.validateSecretExists(ctx, tls.SecretName, ingress.Namespace); err != nil {
					errors = append(errors, NewValidationErrorWithCode("Ingress", ingress.Name, ingress.Namespace, "dangling_tls_secret", "KOGARO-REF-008", fmt.Sprintf("TLS Secret '%s' referenced in Ingress does not exist", tls.SecretName)).
						WithSeverity(SeverityError).
						WithRemediationHint(fmt.Sprintf("Create TLS Secret '%s' in namespace '%s' or update the Ingress TLS configuration to use an existing Secret", tls.SecretName, ingress.Namespace)).
						WithRelatedResources(fmt.Sprintf("Secret/%s", tls.SecretName)).
						WithDetail("missing_tls_secret", tls.SecretName).
						WithDetail("tls_hosts", fmt.Sprintf("%v", tls.Hosts)))
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
		// Skip system namespaces
		if v.sharedConfig.IsSystemNamespace(pvc.Namespace) {
			continue
		}

		if pvc.Spec.StorageClassName != nil {
			className := *pvc.Spec.StorageClassName
			if !existingClasses[className] {
				errors = append(errors, NewValidationErrorWithCode("PersistentVolumeClaim", pvc.Name, pvc.Namespace, "dangling_storage_class", "KOGARO-REF-009", fmt.Sprintf("StorageClass '%s' does not exist", className)).
					WithSeverity(SeverityError).
					WithRemediationHint(fmt.Sprintf("Create StorageClass '%s' or update PVC to use an existing StorageClass", className)).
					WithRelatedResources(fmt.Sprintf("StorageClass/%s", className)).
					WithDetail("missing_storage_class", className))
			}
		}
	}

	// Check Pod volumes referencing PVCs
	var pods corev1.PodList
	if err := v.client.List(ctx, &pods); err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	for _, pod := range pods.Items {
		// Skip system namespaces
		if v.sharedConfig.IsSystemNamespace(pod.Namespace) {
			continue
		}

		for _, volume := range pod.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil {
				pvcName := volume.PersistentVolumeClaim.ClaimName
				if err := v.validatePVCExists(ctx, pvcName, pod.Namespace); err != nil {
					errors = append(errors, NewValidationErrorWithCode("Pod", pod.Name, pod.Namespace, "dangling_pvc_reference", "KOGARO-REF-010", fmt.Sprintf("PVC '%s' referenced in volume does not exist", pvcName)).
						WithSeverity(SeverityError).
						WithRemediationHint(fmt.Sprintf("Create PVC '%s' in namespace '%s' or update the volume reference to use an existing PVC", pvcName, pod.Namespace)).
						WithDetail("missing_pvc", pvcName).
						WithDetail("volume_name", volume.Name))
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
		// Skip system namespaces
		if v.sharedConfig.IsSystemNamespace(pod.Namespace) {
			continue
		}

		saName := pod.Spec.ServiceAccountName
		if saName == "" {
			saName = v.sharedConfig.DefaultSecurityContext.DefaultServiceAccountName
		}

		if err := v.validateServiceAccountExists(ctx, saName, pod.Namespace); err != nil {
			errors = append(errors, NewValidationErrorWithCode("Pod", pod.Name, pod.Namespace, "dangling_service_account", "KOGARO-REF-011", fmt.Sprintf("ServiceAccount '%s' does not exist", saName)).
				WithSeverity(SeverityError).
				WithRemediationHint(fmt.Sprintf("Create ServiceAccount '%s' in namespace '%s' or update Pod to use an existing ServiceAccount", saName, pod.Namespace)).
				WithRelatedResources(fmt.Sprintf("ServiceAccount/%s", saName)).
				WithDetail("missing_service_account", saName))
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
