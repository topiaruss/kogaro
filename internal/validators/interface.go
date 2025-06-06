// Package validators defines the core interfaces and shared types for Kubernetes validation.
package validators

import (
	"context"
)

// ValidationError represents a validation failure found during cluster scanning
type ValidationError struct {
	ResourceType   string
	ResourceName   string
	Namespace      string
	ValidationType string
	Message        string
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