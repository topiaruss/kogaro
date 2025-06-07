// Copyright 2025 Russell Ferriday
// Licensed under the Apache License, Version 2.0
//
// Kogaro - Kubernetes Configuration Hygiene Agent

// Package validators defines the core interfaces and shared types for Kubernetes validation.
package validators

import (
	"context"
)

// Severity indicates the severity level of a validation error
type Severity string

const (
	// SeverityError indicates a critical issue that should be addressed immediately
	SeverityError Severity = "error"
	// SeverityWarning indicates an issue that should be reviewed but may not require immediate action
	SeverityWarning Severity = "warning"
	// SeverityInfo indicates informational findings that may be useful for optimization
	SeverityInfo Severity = "info"
)

// ValidationError represents a validation failure found during cluster scanning
type ValidationError struct {
	// Core identification fields
	ResourceType   string
	ResourceName   string
	Namespace      string
	ValidationType string
	Message        string
	
	// Enhanced context fields
	Severity         Severity
	RemediationHint  string
	RelatedResources []string
	
	// Additional metadata
	Details map[string]string
}

// NewValidationError creates a new ValidationError with the specified core fields
func NewValidationError(resourceType, resourceName, namespace, validationType, message string) ValidationError {
	return ValidationError{
		ResourceType:   resourceType,
		ResourceName:   resourceName,
		Namespace:      namespace,
		ValidationType: validationType,
		Message:        message,
		Severity:       SeverityError, // Default to error severity
		Details:        make(map[string]string),
	}
}

// WithSeverity sets the severity level and returns the ValidationError for method chaining
func (v ValidationError) WithSeverity(severity Severity) ValidationError {
	v.Severity = severity
	return v
}

// WithRemediationHint adds a remediation hint and returns the ValidationError for method chaining
func (v ValidationError) WithRemediationHint(hint string) ValidationError {
	v.RemediationHint = hint
	return v
}

// WithRelatedResources adds related resources and returns the ValidationError for method chaining
func (v ValidationError) WithRelatedResources(resources ...string) ValidationError {
	v.RelatedResources = append(v.RelatedResources, resources...)
	return v
}

// WithDetail adds a detail key-value pair and returns the ValidationError for method chaining
func (v ValidationError) WithDetail(key, value string) ValidationError {
	if v.Details == nil {
		v.Details = make(map[string]string)
	}
	v.Details[key] = value
	return v
}

// IsError returns true if the validation error has error severity
func (v ValidationError) IsError() bool {
	return v.Severity == SeverityError
}

// IsWarning returns true if the validation error has warning severity
func (v ValidationError) IsWarning() bool {
	return v.Severity == SeverityWarning
}

// IsInfo returns true if the validation error has info severity
func (v ValidationError) IsInfo() bool {
	return v.Severity == SeverityInfo
}

// GetResourceKey returns a unique key identifying the resource
func (v ValidationError) GetResourceKey() string {
	if v.Namespace != "" {
		return v.Namespace + "/" + v.ResourceName
	}
	return v.ResourceName
}

// Validator defines the interface that all validators must implement.
// This allows for a pluggable architecture where different types of
// validators can be easily added to the system.
type Validator interface {
	// ValidateCluster performs validation across the entire cluster
	// and reports any errors found via metrics and logging.
	ValidateCluster(ctx context.Context) error

	// GetValidationType returns a unique identifier for this validator type
	// used in metrics and logging.
	GetValidationType() string
}