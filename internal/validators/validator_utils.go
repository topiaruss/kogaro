// Copyright 2025 Russell Ferriday
// Licensed under the Apache License, Version 2.0
//
// Kogaro - Kubernetes Configuration Hygiene Agent

// Copyright 2025 Russell Ferriday
// Licensed under the Apache License, Version 2.0
//
// Kogaro - Kubernetes Configuration Hygiene Agent

package validators

import (
	"github.com/topiaruss/kogaro/internal/metrics"
)

// LogAndRecordErrors logs and records metrics for all validation errors.
// This consolidates the common error handling pattern used across all validators.
func LogAndRecordErrors(logReceiver LogReceiver, validatorType string, errors []ValidationError) {
	for _, validationErr := range errors {
		// Log the error
		logReceiver.LogValidationError(validatorType, validationErr)

		// Record metrics with temporal awareness
		metrics.RecordValidationErrorWithState(
			validationErr.ResourceType,
			validationErr.ResourceName,
			validationErr.Namespace,
			validationErr.ValidationType,
			string(validationErr.Severity),
			validationErr.ErrorCode,
			false, // expectedPattern - false for actual errors
		)
	}
}
