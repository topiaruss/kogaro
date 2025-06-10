// Copyright 2025 Russell Ferriday
// Licensed under the Apache License, Version 2.0
//
// Kogaro - Kubernetes Configuration Hygiene Agent

// Package validators provides a registry pattern for managing multiple validators.
package validators

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"
)

// ValidatorRegistry manages a collection of validators and coordinates their execution.
type ValidatorRegistry struct {
	validators []Validator
	log        logr.Logger
	mu         sync.RWMutex
	client     client.Client
}

// NewValidatorRegistry creates a new ValidatorRegistry with the given logger.
func NewValidatorRegistry(log logr.Logger, client client.Client) *ValidatorRegistry {
	return &ValidatorRegistry{
		validators: make([]Validator, 0),
		log:        log.WithName("validator-registry"),
		client:     client,
	}
}

// Register adds a validator to the registry.
func (r *ValidatorRegistry) Register(validator Validator) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.validators = append(r.validators, validator)
	r.log.Info("validator registered", "type", validator.GetValidationType())
}

// ValidateCluster runs validation across all registered validators.
func (r *ValidatorRegistry) ValidateCluster(ctx context.Context) error {
	r.mu.RLock()
	validators := make([]Validator, len(r.validators))
	copy(validators, r.validators)
	r.mu.RUnlock()

	if len(validators) == 0 {
		r.log.Info("no validators registered, skipping validation")
		return nil
	}

	r.log.Info("starting cluster validation", "validator_count", len(validators))

	for _, validator := range validators {
		validatorType := validator.GetValidationType()
		r.log.V(1).Info("running validator", "type", validatorType)

		if err := validator.ValidateCluster(ctx); err != nil {
			return fmt.Errorf("validator %s failed: %w", validatorType, err)
		}

		r.log.V(1).Info("validator completed", "type", validatorType)
	}

	r.log.Info("cluster validation completed successfully", "validator_count", len(validators))
	return nil
}

// GetValidators returns a copy of all registered validators (for testing).
func (r *ValidatorRegistry) GetValidators() []Validator {
	r.mu.RLock()
	defer r.mu.RUnlock()

	validators := make([]Validator, len(r.validators))
	copy(validators, r.validators)
	return validators
}

// GetValidationType returns the validation type identifier for the registry.
func (r *ValidatorRegistry) GetValidationType() string {
	return "validator_registry"
}

// FormatCIOutput formats validation results for CI consumption
func (r *ValidatorRegistry) FormatCIOutput(result ValidationResult) (string, error) {
	// Create a buffer to build the output
	var output strings.Builder

	// Add summary header
	output.WriteString(fmt.Sprintf("Validation Summary:\n"))
	output.WriteString(fmt.Sprintf("Total Errors: %d\n", result.Summary.TotalErrors))
	output.WriteString(fmt.Sprintf("Missing References: %d\n", len(result.Summary.MissingRefs)))
	output.WriteString(fmt.Sprintf("Suggested References: %d\n", len(result.Summary.SuggestedRefs)))

	// Add detailed errors
	if len(result.Errors) > 0 {
		output.WriteString("\nDetailed Errors:\n")
		for _, err := range result.Errors {
			output.WriteString(fmt.Sprintf("- %s/%s: %s\n",
				err.ResourceType,
				err.ResourceName,
				err.Message))

			if err.RemediationHint != "" {
				output.WriteString(fmt.Sprintf("  Hint: %s\n", err.RemediationHint))
			}

			if len(err.RelatedResources) > 0 {
				output.WriteString(fmt.Sprintf("  Related Resources: %s\n",
					strings.Join(err.RelatedResources, ", ")))
			}
		}
	}

	// Add suggested references
	if len(result.SuggestedRefs) > 0 {
		output.WriteString("\nSuggested References:\n")
		for _, ref := range result.SuggestedRefs {
			output.WriteString(fmt.Sprintf("- %s/%s -> %s/%s (confidence: %.2f)\n",
				ref.SourceType,
				ref.SourceName,
				ref.TargetType,
				ref.TargetName,
				ref.Confidence))
			if ref.Reason != "" {
				output.WriteString(fmt.Sprintf("  Reason: %s\n", ref.Reason))
			}
		}
	}

	return output.String(), nil
}

// ValidateNewConfig validates a new configuration file against the existing cluster state.
// It performs all standard validations plus additional checks for potential matches
// when exact references don't exist.
func (r *ValidatorRegistry) ValidateNewConfig(ctx context.Context, configPath string) (*ValidationResult, error) {
	r.mu.RLock()
	validators := make([]Validator, len(r.validators))
	copy(validators, r.validators)
	r.mu.RUnlock()

	if len(validators) == 0 {
		r.log.Info("no validators registered, skipping validation")
		return &ValidationResult{ExitCode: 0}, nil
	}

	r.log.Info("starting new configuration validation", "config", configPath)

	// Read and parse the configuration file
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Create a temporary client that includes both cluster and new config resources
	client := r.createTemporaryClient(ctx, configData)
	if client == nil {
		return nil, fmt.Errorf("failed to create temporary client")
	}

	// Run all validators with the temporary client
	var allErrors []ValidationError
	var missingRefs []string
	var suggestedRefs []string

	for _, validator := range validators {
		validatorType := validator.GetValidationType()
		r.log.V(1).Info("running validator", "type", validatorType)

		// Update validator's client to use temporary client
		if err := r.updateValidatorClient(validator, client); err != nil {
			return nil, fmt.Errorf("failed to update validator client: %w", err)
		}

		// Run validation and collect errors
		if err := validator.ValidateCluster(ctx); err != nil {
			// Extract validation errors and suggestions
			if ve, ok := err.(ValidationError); ok {
				allErrors = append(allErrors, ve)
				if ve.ValidationType == "missing_reference" {
					missingRefs = append(missingRefs, ve.Message)
				}
				if ve.ValidationType == "suggested_reference" {
					suggestedRefs = append(suggestedRefs, ve.Message)
				}
			}
		}

		r.log.V(1).Info("validator completed", "type", validatorType)
	}

	// Prepare result
	result := &ValidationResult{
		Summary: struct {
			TotalErrors   int      `json:"total_errors"`
			MissingRefs   []string `json:"missing_refs,omitempty"`
			SuggestedRefs []string `json:"suggested_refs,omitempty"`
		}{
			TotalErrors:   len(allErrors),
			MissingRefs:   missingRefs,
			SuggestedRefs: suggestedRefs,
		},
		Errors:   allErrors,
		ExitCode: len(allErrors),
	}

	r.log.Info("new configuration validation completed",
		"total_errors", len(allErrors),
		"missing_refs", len(missingRefs),
		"suggested_refs", len(suggestedRefs))

	return result, nil
}

// createTemporaryClient creates a client that includes both cluster and new config resources
func (r *ValidatorRegistry) createTemporaryClient(ctx context.Context, configData []byte) client.Client {
	// Create a fake client builder
	builder := fake.NewClientBuilder()

	// Parse the config file into Kubernetes objects
	objects, err := parseConfigFile(configData)
	if err != nil {
		r.log.Error(err, "failed to parse config file")
		return nil
	}

	// Add new config objects to the fake client
	builder = builder.WithObjects(objects...)

	// Get existing cluster objects
	clusterObjects, err := r.getClusterObjects(ctx)
	if err != nil {
		r.log.Error(err, "failed to get cluster objects")
		return nil
	}

	// Add cluster objects to the fake client
	builder = builder.WithObjects(clusterObjects...)

	// Create the temporary client
	return builder.Build()
}

// parseConfigFile parses a Kubernetes config file into objects
func parseConfigFile(data []byte) ([]client.Object, error) {
	// Split the file into individual YAML documents
	docs := bytes.Split(data, []byte("---"))
	var objects []client.Object

	for _, doc := range docs {
		if len(bytes.TrimSpace(doc)) == 0 {
			continue
		}

		// Parse the YAML into an unstructured object
		obj := &unstructured.Unstructured{}
		if err := yaml.Unmarshal(doc, obj); err != nil {
			return nil, fmt.Errorf("failed to parse YAML: %w", err)
		}

		objects = append(objects, obj)
	}

	return objects, nil
}

// getClusterObjects retrieves all relevant objects from the cluster
func (r *ValidatorRegistry) getClusterObjects(ctx context.Context) ([]client.Object, error) {
	var objects []client.Object

	// Get all namespaces
	var namespaces corev1.NamespaceList
	if err := r.client.List(ctx, &namespaces); err != nil {
		return nil, fmt.Errorf("failed to list namespaces: %w", err)
	}

	// For each namespace, get all relevant resources
	for _, ns := range namespaces.Items {
		// Get ConfigMaps
		var configMaps corev1.ConfigMapList
		if err := r.client.List(ctx, &configMaps, client.InNamespace(ns.Name)); err != nil {
			return nil, fmt.Errorf("failed to list ConfigMaps: %w", err)
		}
		for i := range configMaps.Items {
			objects = append(objects, &configMaps.Items[i])
		}

		// Get Secrets
		var secrets corev1.SecretList
		if err := r.client.List(ctx, &secrets, client.InNamespace(ns.Name)); err != nil {
			return nil, fmt.Errorf("failed to list Secrets: %w", err)
		}
		for i := range secrets.Items {
			objects = append(objects, &secrets.Items[i])
		}

		// Get Services
		var services corev1.ServiceList
		if err := r.client.List(ctx, &services, client.InNamespace(ns.Name)); err != nil {
			return nil, fmt.Errorf("failed to list Services: %w", err)
		}
		for i := range services.Items {
			objects = append(objects, &services.Items[i])
		}

		// Get Ingresses
		var ingresses networkingv1.IngressList
		if err := r.client.List(ctx, &ingresses, client.InNamespace(ns.Name)); err != nil {
			return nil, fmt.Errorf("failed to list Ingresses: %w", err)
		}
		for i := range ingresses.Items {
			objects = append(objects, &ingresses.Items[i])
		}

		// Get PVCs
		var pvcs corev1.PersistentVolumeClaimList
		if err := r.client.List(ctx, &pvcs, client.InNamespace(ns.Name)); err != nil {
			return nil, fmt.Errorf("failed to list PVCs: %w", err)
		}
		for i := range pvcs.Items {
			objects = append(objects, &pvcs.Items[i])
		}
	}

	return objects, nil
}

// updateValidatorClient updates a validator's client to use the temporary client
func (r *ValidatorRegistry) updateValidatorClient(validator Validator, client client.Client) error {
	// Use the SetClient method on the Validator interface
	validator.SetClient(client)
	return nil
}
