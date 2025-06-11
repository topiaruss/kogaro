// Copyright 2025 Russell Ferriday
// Licensed under the Apache License, Version 2.0
//
// Kogaro - Kubernetes Configuration Hygiene Agent

package validators

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	testResourceTypePod  = "Pod"
	testResourceNamePod  = "test-pod"
	testNamespaceDefault = "default"
)

// MockValidator is a test validator that implements the Validator interface
type MockValidator struct {
	validationType       string
	shouldError          bool
	errorMessage         string
	callCount            int
	mu                   sync.Mutex
	client               client.Client
	lastValidationErrors []ValidationError
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

func (m *MockValidator) SetClient(c client.Client) {
	m.client = c
}

func (m *MockValidator) SetLogReceiver(lr LogReceiver) {
	// Mock implementation - no-op for testing
}

func (m *MockValidator) GetLastValidationErrors() []ValidationError {
	return m.lastValidationErrors
}

func (m *MockValidator) GetCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

// mockValidator is a test implementation of the Validator interface
type mockValidator struct {
	validationType       string
	validateFunc         func(ctx context.Context) error
	client               client.Client
	lastValidationErrors []ValidationError
}

func (m *mockValidator) ValidateCluster(ctx context.Context) error {
	return m.validateFunc(ctx)
}

func (m *mockValidator) GetValidationType() string {
	return m.validationType
}

func (m *mockValidator) SetClient(c client.Client) {
	m.client = c
}

func (m *mockValidator) SetLogReceiver(lr LogReceiver) {
	// Mock implementation - no-op for testing
}

func (m *mockValidator) GetLastValidationErrors() []ValidationError {
	return m.lastValidationErrors
}

// ContextAwareValidator is a test validator that respects context cancellation
type ContextAwareValidator struct {
	shouldBlock          bool
	client               client.Client
	lastValidationErrors []ValidationError
}

func (v *ContextAwareValidator) ValidateCluster(ctx context.Context) error {
	if v.shouldBlock {
		<-ctx.Done()
		return ctx.Err()
	}
	return nil
}

func (v *ContextAwareValidator) GetValidationType() string {
	return "context_aware"
}

func (v *ContextAwareValidator) SetClient(c client.Client) {
	v.client = c
}

func (v *ContextAwareValidator) SetLogReceiver(lr LogReceiver) {
	// Mock implementation - no-op for testing
}

func (v *ContextAwareValidator) GetLastValidationErrors() []ValidationError {
	return v.lastValidationErrors
}

func TestNewValidatorRegistry(t *testing.T) {
	// Create a test logger
	logger := logr.Discard()

	// Create a fake client
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	// Create registry
	registry := NewValidatorRegistry(logger, fakeClient)

	// Verify registry was created
	if registry == nil {
		t.Error("Expected non-nil registry")
	}
}

func TestValidatorRegistry_Register(t *testing.T) {
	registry, _ := setupTestRegistry(t)

	// Test registering a single validator
	validator1 := &MockValidator{validationType: "test_validator_1"}
	registry.Register(validator1)

	validators := registry.GetValidators()
	if len(validators) != 2 { // 1 from setupTestRegistry + 1 from this test
		t.Errorf("Registry should have 2 validators after registration, got %d", len(validators))
	}

	// Test registering multiple validators
	validator2 := &MockValidator{validationType: "test_validator_2"}
	validator3 := &MockValidator{validationType: "test_validator_3"}

	registry.Register(validator2)
	registry.Register(validator3)

	validators = registry.GetValidators()
	if len(validators) != 4 { // 1 from setupTestRegistry + 3 from this test
		t.Errorf("Registry should have 4 validators after registration, got %d", len(validators))
	}

	// Verify all validators are present
	types := make(map[string]bool)
	for _, v := range validators {
		types[v.GetValidationType()] = true
	}

	expectedTypes := []string{"reference", "test_validator_1", "test_validator_2", "test_validator_3"}
	for _, expectedType := range expectedTypes {
		if !types[expectedType] {
			t.Errorf("Expected validator type '%s' not found in registry", expectedType)
		}
	}
}

func TestValidatorRegistry_ValidateCluster_Success(t *testing.T) {
	registry, _ := setupTestRegistry(t)

	// Clear existing validators
	registry.validators = make([]Validator, 0)

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
	registry, _ := setupTestRegistry(t)

	// Clear existing validators
	registry.validators = make([]Validator, 0)

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
		t.Errorf("Expected error message 'validator test_validator_2 failed: validation failed', got '%s'", err.Error())
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
	registry, _ := setupTestRegistry(t)

	// Clear existing validators
	registry.validators = make([]Validator, 0)

	// Run validation on empty registry
	err := registry.ValidateCluster(context.TODO())
	if err != nil {
		t.Errorf("ValidateCluster should succeed with empty registry, got error: %v", err)
	}
}

func TestValidatorRegistry_GetValidators(t *testing.T) {
	registry, _ := setupTestRegistry(t)

	// Test initial state (1 validator from setupTestRegistry)
	validators := registry.GetValidators()
	if len(validators) != 1 {
		t.Errorf("GetValidators should return 1 validator for initial state, got %d validators", len(validators))
	}

	// Clear existing validators
	registry.validators = make([]Validator, 0)

	// Test empty registry
	validators = registry.GetValidators()
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
	registry, _ := setupTestRegistry(t)

	validationType := registry.GetValidationType()
	expectedType := "validator_registry"

	if validationType != expectedType {
		t.Errorf("GetValidationType should return '%s', got '%s'", expectedType, validationType)
	}
}

// TestValidatorRegistry_ConcurrentAccess tests thread safety
func TestValidatorRegistry_ConcurrentAccess(t *testing.T) {
	registry, _ := setupTestRegistry(t)

	// Clear existing validators
	registry.validators = make([]Validator, 0)

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
	registry, _ := setupTestRegistry(t)

	// Clear existing validators
	registry.validators = make([]Validator, 0)

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
	case <-time.After(time.Second):
		t.Error("ValidateCluster did not respond to context cancellation")
	}
}

func setupTestRegistry(_ *testing.T, objects ...client.Object) (*ValidatorRegistry, client.Client) {
	// Create a test logger
	logger := logr.Discard()

	// Create a fake client with the provided objects
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()

	// Create and return the registry
	registry := NewValidatorRegistry(logger, fakeClient)

	// Add a mock validator that checks for missing references
	registry.Register(&mockValidator{
		validationType: "reference",
		validateFunc: func(_ context.Context) error {
			// Simulate finding a missing reference
			return NewValidationError(
				"Service",
				"test-service",
				"default",
				"reference",
				"Invalid selector",
			).WithRemediationHint("Update selector to match pod labels").
				WithRelatedResources("Pod/test-pod")
		},
	})

	return registry, fakeClient
}

func TestValidateNewConfig(t *testing.T) {
	registry := NewValidatorRegistry(logr.Discard(), nil)

	// Register a mock validator that returns a ValidationError for each test case
	registry.Register(&mockValidator{
		validationType: "test",
		validateFunc: func(_ context.Context) error {
			return NewValidationError(
				testResourceTypePod,
				testResourceNamePod,
				testNamespaceDefault,
				"test",
				"Test error",
			)
		},
	})

	// Run validation
	err := registry.ValidateCluster(context.Background())

	if err == nil {
		t.Fatal("Expected validation error, got nil")
	}

	// Unwrap the error to get the ValidationError
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("Expected ValidationError, got %T", err)
	}

	if !strings.Contains(validationErr.Message, "Test error") {
		t.Errorf("Expected error message to contain \"Test error\", got %q", validationErr.Message)
	}

	if validationErr.ResourceType != testResourceTypePod {
		t.Errorf("Expected ResourceType %q, got %q", testResourceTypePod, validationErr.ResourceType)
	}

	if validationErr.ResourceName != testResourceNamePod {
		t.Errorf("Expected ResourceName %q, got %q", testResourceNamePod, validationErr.ResourceName)
	}

	if validationErr.ValidationType != "test" {
		t.Errorf("Expected ValidationType 'test', got %q", validationErr.ValidationType)
	}

	if validationErr.Message != "Test error" {
		t.Errorf("Expected Message \"Test error\", got %q", validationErr.Message)
	}
}

func TestFormatCIOutput(t *testing.T) {
	tests := []struct {
		name     string
		result   ValidationResult
		expected string
	}{
		{
			name: "no errors",
			result: ValidationResult{
				Summary: struct {
					TotalErrors   int      `json:"total_errors"`
					MissingRefs   []string `json:"missing_refs,omitempty"`
					SuggestedRefs []string `json:"suggested_refs,omitempty"`
				}{
					TotalErrors: 0,
				},
				ExitCode: 0,
			},
			expected: `Validation Summary:
Total Errors: 0
Missing References: 0
Suggested References: 0
`,
		},
		{
			name: "with_errors_and_refs",
			result: ValidationResult{
				Summary: struct {
					TotalErrors   int      `json:"total_errors"`
					MissingRefs   []string `json:"missing_refs,omitempty"`
					SuggestedRefs []string `json:"suggested_refs,omitempty"`
				}{
					TotalErrors: 2,
					MissingRefs: []string{"ConfigMap/test"},
				},
				Errors: []ValidationError{
					{
						ResourceType:     "Service",
						ResourceName:     "test-service",
						ValidationType:   "reference",
						Message:          "Invalid selector",
						RemediationHint:  "Update selector to match pod labels",
						RelatedResources: []string{"Pod/test-pod"},
					},
				},
				SuggestedRefs: []Reference{
					{
						SourceType: "ConfigMap",
						SourceName: "test-config",
						TargetType: "Secret",
						TargetName: "test-secret",
						Confidence: 0.85,
						Reason:     "Similar naming pattern",
					},
				},
				ExitCode: 1,
			},
			expected: `Validation Summary:
Total Errors: 2
Missing References: 1
Suggested References: 0

Detailed Errors:
- Service/test-service: Invalid selector
  Hint: Update selector to match pod labels
  Related Resources: Pod/test-pod

Suggested References:
- ConfigMap/test-config -> Secret/test-secret (confidence: 0.85)
  Reason: Similar naming pattern
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create registry with an empty fake client
			registry, _ := setupTestRegistry(t)

			output, err := registry.FormatCIOutput(tt.result)
			if err != nil {
				t.Fatalf("FormatCIOutput failed: %v", err)
			}

			if output != tt.expected {
				t.Errorf("Expected output:\n%s\n\nGot:\n%s", tt.expected, output)
			}
		})
	}
}

func TestRegister(t *testing.T) {
	registry, _ := setupTestRegistry(t)

	// Create a test validator
	validator := &mockValidator{
		validationType: "test",
		validateFunc: func(_ context.Context) error {
			return nil
		},
	}

	// Register the validator
	registry.Register(validator)

	// Verify the validator was registered
	validators := registry.GetValidators()
	if len(validators) != 2 { // 1 from setupTestRegistry + 1 from this test
		t.Errorf("Expected 2 validators, got %d", len(validators))
	}
}

func TestGetValidators(t *testing.T) {
	registry, _ := setupTestRegistry(t)

	// Get validators
	validators := registry.GetValidators()

	// Verify we got the expected number of validators
	if len(validators) != 1 { // 1 from setupTestRegistry
		t.Errorf("Expected 1 validator, got %d", len(validators))
	}
}

func TestValidateCluster(t *testing.T) {
	registry, _ := setupTestRegistry(t)

	// Run validation
	err := registry.ValidateCluster(context.Background())
	if err == nil {
		t.Fatal("Expected validation error, got nil")
	}

	// Check error type and message
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("Expected ValidationError, got %T", err)
	}

	// Check error message content
	expectedMsg := "Invalid selector"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error message to contain %q, got %q", expectedMsg, err.Error())
	}

	// Check error fields
	if validationErr.ResourceType != "Service" {
		t.Errorf("Expected ResourceType 'Service', got %q", validationErr.ResourceType)
	}
	if validationErr.ResourceName != "test-service" {
		t.Errorf("Expected ResourceName 'test-service', got %q", validationErr.ResourceName)
	}
	if validationErr.ValidationType != "reference" {
		t.Errorf("Expected ValidationType 'reference', got %q", validationErr.ValidationType)
	}
	if validationErr.Message != expectedMsg {
		t.Errorf("Expected Message %q, got %q", expectedMsg, validationErr.Message)
	}
	if len(validationErr.RelatedResources) == 0 || validationErr.RelatedResources[0] != "Pod/test-pod" {
		t.Errorf("Expected RelatedResources to contain 'Pod/test-pod', got %v", validationErr.RelatedResources)
	}
}

func TestValidateClusterWithNoErrors(t *testing.T) {
	// Create registry with an empty fake client
	registry, _ := setupTestRegistry(t)

	// Clear existing validators
	registry.validators = make([]Validator, 0)

	// Add a validator that returns no errors
	registry.Register(&mockValidator{
		validationType: "test",
		validateFunc: func(_ context.Context) error {
			return nil
		},
	})

	// Run validation
	err := registry.ValidateCluster(context.Background())
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

