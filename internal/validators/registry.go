// Copyright 2025 Russell Ferriday
// Licensed under the Apache License, Version 2.0
//
// Kogaro - Kubernetes Configuration Hygiene Agent

// Package validators provides a registry pattern for managing multiple validators.
package validators

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-logr/logr"
)

// ValidatorRegistry manages a collection of validators and coordinates their execution.
type ValidatorRegistry struct {
	validators []Validator
	log        logr.Logger
	mu         sync.RWMutex
}

// NewValidatorRegistry creates a new ValidatorRegistry with the given logger.
func NewValidatorRegistry(log logr.Logger) *ValidatorRegistry {
	return &ValidatorRegistry{
		validators: make([]Validator, 0),
		log:        log.WithName("validator-registry"),
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