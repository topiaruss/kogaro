// Copyright 2025 Russell Ferriday
// Licensed under the Apache License, Version 2.0
//
// Kogaro - Kubernetes Configuration Hygiene Agent

package metrics

import (
	"testing"
	"time"
)

func TestClassifyWorkload(t *testing.T) {
	tests := []struct {
		name         string
		namespace    string
		resourceType string
		expected     WorkloadCategory
	}{
		{
			name:         "kube-system is infrastructure",
			namespace:    "kube-system",
			resourceType: "Pod",
			expected:     WorkloadCategoryInfrastructure,
		},
		{
			name:         "monitoring namespace is infrastructure",
			namespace:    "monitoring",
			resourceType: "Service",
			expected:     WorkloadCategoryInfrastructure,
		},
		{
			name:         "default namespace is application",
			namespace:    "default",
			resourceType: "Deployment",
			expected:     WorkloadCategoryApplication,
		},
		{
			name:         "production namespace is application",
			namespace:    "production",
			resourceType: "Service",
			expected:     WorkloadCategoryApplication,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyWorkload(tt.namespace, tt.resourceType)
			if result != tt.expected {
				t.Errorf("ClassifyWorkload(%s, %s) = %v, want %v", tt.namespace, tt.resourceType, result, tt.expected)
			}
		})
	}
}

func TestClassifyTemporalState(t *testing.T) {
	tests := []struct {
		name     string
		ageHours float64
		expected TemporalState
	}{
		{
			name:     "new error (< 1 hour)",
			ageHours: 0.5,
			expected: TemporalStateNew,
		},
		{
			name:     "recent error (1-24 hours)",
			ageHours: 12.0,
			expected: TemporalStateRecent,
		},
		{
			name:     "stable error (> 24 hours)",
			ageHours: 48.0,
			expected: TemporalStateStable,
		},
		{
			name:     "exactly 1 hour is recent",
			ageHours: 1.0,
			expected: TemporalStateRecent,
		},
		{
			name:     "exactly 24 hours is stable",
			ageHours: 24.0,
			expected: TemporalStateStable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyTemporalState(tt.ageHours)
			if result != tt.expected {
				t.Errorf("ClassifyTemporalState(%f) = %v, want %v", tt.ageHours, result, tt.expected)
			}
		})
	}
}

func TestStateTracker(t *testing.T) {
	tracker := NewStateTracker()

	// Test new state creation
	key := "default/Pod/test-pod/missing_resource_requests"
	state := tracker.UpdateState(key, time.Now())

	if state == nil {
		t.Fatal("Expected state to be created")
	}

	if state.State != TemporalStateNew {
		t.Errorf("Expected state to be NEW, got %v", state.State)
	}

	if state.ChangeCount != 1 {
		t.Errorf("Expected change count to be 1, got %d", state.ChangeCount)
	}

	// Test state update
	time.Sleep(100 * time.Millisecond) // Small delay to ensure different timestamp
	updatedState := tracker.UpdateState(key, time.Now())

	if updatedState.ChangeCount != 2 {
		t.Errorf("Expected change count to be 2, got %d", updatedState.ChangeCount)
	}

	// Test state retrieval
	retrievedState := tracker.GetState(key)
	if retrievedState == nil {
		t.Fatal("Expected to retrieve state")
	}

	if retrievedState.ChangeCount != 2 {
		t.Errorf("Expected retrieved state change count to be 2, got %d", retrievedState.ChangeCount)
	}
}

func TestGetStateKey(t *testing.T) {
	key := GetStateKey("default", "Pod", "test-pod", "missing_resource_requests")
	expected := "default/Pod/test-pod/missing_resource_requests"

	if key != expected {
		t.Errorf("GetStateKey() = %s, want %s", key, expected)
	}
}

func TestRecordValidationErrorWithState(t *testing.T) {
	// This test verifies that the function doesn't panic
	// In a real test environment, you'd want to verify the metrics were actually recorded

	// Reset the global state tracker for clean test
	globalStateTracker = NewStateTracker()

	// Record a validation error
	RecordValidationErrorWithState(
		"Pod",
		"test-pod",
		"default",
		"missing_resource_requests",
		"error",
		false,
	)

	// Verify state was created
	key := GetStateKey("default", "Pod", "test-pod", "missing_resource_requests")
	state := globalStateTracker.GetState(key)

	if state == nil {
		t.Fatal("Expected state to be created")
	}

	if state.State != TemporalStateNew {
		t.Errorf("Expected state to be NEW, got %v", state.State)
	}
}
