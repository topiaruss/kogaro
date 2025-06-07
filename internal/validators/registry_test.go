// Copyright 2025 Russell Ferriday
// Licensed under the Apache License, Version 2.0
//
// Kogaro - Kubernetes Configuration Hygiene Agent

package validators

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"
)

// MockValidator is a test validator that implements the Validator interface
type MockValidator struct {
	validationType string
	shouldError    bool
	errorMessage   string
	callCount      int
	mu             sync.Mutex
}

func (m *MockValidator) ValidateCluster(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	
	if m.shouldError {
		return errors.New(m.errorMessage)
	}
	return nil
}

func (m *MockValidator) GetValidationType() string {
	return m.validationType
}

func (m *MockValidator) GetCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

func TestNewValidatorRegistry(t *testing.T) {
	registry := NewValidatorRegistry(logr.Discard())
	
	if registry == nil {
		t.Fatal("NewValidatorRegistry should return a non-nil registry")
	}
	
	// Test initial state
	validators := registry.GetValidators()
	if len(validators) != 0 {
		t.Errorf("New registry should have 0 validators, got %d", len(validators))
	}
}

func TestValidatorRegistry_Register(t *testing.T) {
	registry := NewValidatorRegistry(logr.Discard())
	
	// Test registering a single validator
	validator1 := &MockValidator{validationType: "test_validator_1"}
	registry.Register(validator1)
	
	validators := registry.GetValidators()
	if len(validators) != 1 {
		t.Errorf("Registry should have 1 validator after registration, got %d", len(validators))
	}
	
	// Test registering multiple validators
	validator2 := &MockValidator{validationType: "test_validator_2"}
	validator3 := &MockValidator{validationType: "test_validator_3"}
	
	registry.Register(validator2)
	registry.Register(validator3)
	
	validators = registry.GetValidators()
	if len(validators) != 3 {
		t.Errorf("Registry should have 3 validators after registration, got %d", len(validators))
	}
	
	// Verify all validators are present
	types := make(map[string]bool)
	for _, v := range validators {
		types[v.GetValidationType()] = true
	}
	
	expectedTypes := []string{"test_validator_1", "test_validator_2", "test_validator_3"}
	for _, expectedType := range expectedTypes {
		if !types[expectedType] {
			t.Errorf("Expected validator type '%s' not found in registry", expectedType)
		}
	}
}

func TestValidatorRegistry_ValidateCluster_Success(t *testing.T) {
	registry := NewValidatorRegistry(logr.Discard())
	
	// Register multiple successful validators
	validator1 := &MockValidator{validationType: "test_validator_1", shouldError: false}
	validator2 := &MockValidator{validationType: "test_validator_2", shouldError: false}
	validator3 := &MockValidator{validationType: "test_validator_3", shouldError: false}
	
	registry.Register(validator1)
	registry.Register(validator2)
	registry.Register(validator3)
	
	// Run validation
	err := registry.ValidateCluster(context.TODO())
	if err != nil {
		t.Errorf("ValidateCluster should succeed with all successful validators, got error: %v", err)
	}
	
	// Verify all validators were called
	if validator1.GetCallCount() != 1 {
		t.Errorf("Validator 1 should be called once, got %d calls", validator1.GetCallCount())
	}
	if validator2.GetCallCount() != 1 {
		t.Errorf("Validator 2 should be called once, got %d calls", validator2.GetCallCount())
	}
	if validator3.GetCallCount() != 1 {
		t.Errorf("Validator 3 should be called once, got %d calls", validator3.GetCallCount())
	}
}

func TestValidatorRegistry_ValidateCluster_WithErrors(t *testing.T) {
	registry := NewValidatorRegistry(logr.Discard())
	
	// Register validators with mixed success/failure
	validator1 := &MockValidator{validationType: "test_validator_1", shouldError: false}
	validator2 := &MockValidator{validationType: "test_validator_2", shouldError: true, errorMessage: "validation failed"}
	validator3 := &MockValidator{validationType: "test_validator_3", shouldError: false}
	
	registry.Register(validator1)
	registry.Register(validator2)
	registry.Register(validator3)
	
	// Run validation
	err := registry.ValidateCluster(context.TODO())
	if err == nil {
		t.Error("ValidateCluster should return error when a validator fails")
	}
	
	// Check that error contains information about the failed validator
	if err.Error() != "validator test_validator_2 failed: validation failed" {
		t.Errorf("Error message should contain validator type and error, got: %v", err)
	}
	
	// Verify validators were called until the first error (registry stops on first error)
	if validator1.GetCallCount() != 1 {
		t.Errorf("Validator 1 should be called once, got %d calls", validator1.GetCallCount())
	}
	if validator2.GetCallCount() != 1 {
		t.Errorf("Validator 2 should be called once, got %d calls", validator2.GetCallCount())
	}
	// Validator 3 should NOT be called since registry stops on first error
	if validator3.GetCallCount() != 0 {
		t.Errorf("Validator 3 should not be called after error, got %d calls", validator3.GetCallCount())
	}
}

func TestValidatorRegistry_ValidateCluster_EmptyRegistry(t *testing.T) {
	registry := NewValidatorRegistry(logr.Discard())
	
	// Run validation on empty registry
	err := registry.ValidateCluster(context.TODO())
	if err != nil {
		t.Errorf("ValidateCluster should succeed with empty registry, got error: %v", err)
	}
}

func TestValidatorRegistry_GetValidators(t *testing.T) {
	registry := NewValidatorRegistry(logr.Discard())
	
	// Test empty registry
	validators := registry.GetValidators()
	if len(validators) != 0 {
		t.Errorf("GetValidators should return empty slice for empty registry, got %d validators", len(validators))
	}
	
	// Add validators
	validator1 := &MockValidator{validationType: "test_validator_1"}
	validator2 := &MockValidator{validationType: "test_validator_2"}
	
	registry.Register(validator1)
	registry.Register(validator2)
	
	validators = registry.GetValidators()
	if len(validators) != 2 {
		t.Errorf("GetValidators should return 2 validators, got %d", len(validators))
	}
	
	// Verify returned slice is a copy (modifications shouldn't affect registry)
	originalLength := len(registry.GetValidators())
	returnedValidators := registry.GetValidators()
	_ = append(returnedValidators, &MockValidator{validationType: "external_validator"})
	
	newLength := len(registry.GetValidators())
	if originalLength != newLength {
		t.Errorf("Modifying returned slice should not affect registry. Original: %d, After modification: %d", originalLength, newLength)
	}
}

func TestValidatorRegistry_GetValidationType(t *testing.T) {
	registry := NewValidatorRegistry(logr.Discard())
	
	validationType := registry.GetValidationType()
	expectedType := "validator_registry"
	
	if validationType != expectedType {
		t.Errorf("GetValidationType should return '%s', got '%s'", expectedType, validationType)
	}
}

// TestValidatorRegistry_ConcurrentAccess tests thread safety
func TestValidatorRegistry_ConcurrentAccess(t *testing.T) {
	registry := NewValidatorRegistry(logr.Discard())
	
	// Number of concurrent operations
	numGoroutines := 10
	numValidators := 5
	
	var wg sync.WaitGroup
	
	// Concurrently register validators
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numValidators; j++ {
				validator := &MockValidator{
					validationType: fmt.Sprintf("validator_%d_%d", id, j),
				}
				registry.Register(validator)
			}
		}(i)
	}
	
	// Concurrently read validators
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numValidators; j++ {
				_ = registry.GetValidators()
				time.Sleep(time.Microsecond) // Small delay to increase chance of race conditions
			}
		}()
	}
	
	// Concurrently run validation
	for i := 0; i < numGoroutines/2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = registry.ValidateCluster(context.TODO())
		}()
	}
	
	wg.Wait()
	
	// Verify final state
	validators := registry.GetValidators()
	expectedCount := numGoroutines * numValidators
	if len(validators) != expectedCount {
		t.Errorf("Expected %d validators after concurrent operations, got %d", expectedCount, len(validators))
	}
}

// TestValidatorRegistry_ContextCancellation tests that context cancellation is respected
func TestValidatorRegistry_ContextCancellation(t *testing.T) {
	registry := NewValidatorRegistry(logr.Discard())
	
	// Create a validator that respects context cancellation
	validator := &ContextAwareValidator{shouldBlock: true}
	registry.Register(validator)
	
	// Create a context that will be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	
	// Start validation in a goroutine
	done := make(chan error, 1)
	go func() {
		done <- registry.ValidateCluster(ctx)
	}()
	
	// Cancel the context after a short delay
	time.Sleep(10 * time.Millisecond)
	cancel()
	
	// Wait for validation to complete
	select {
	case err := <-done:
		if err == nil {
			t.Error("ValidateCluster should return error when context is cancelled")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Expected context.Canceled error, got: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("ValidateCluster should respect context cancellation and return quickly")
	}
}

// ContextAwareValidator is a test validator that respects context cancellation
type ContextAwareValidator struct {
	shouldBlock bool
}

func (v *ContextAwareValidator) ValidateCluster(ctx context.Context) error {
	if v.shouldBlock {
		// Block until context is cancelled
		<-ctx.Done()
		return ctx.Err()
	}
	return nil
}

func (v *ContextAwareValidator) GetValidationType() string {
	return "context_aware_validator"
}