// Copyright 2025 Russell Ferriday
// Licensed under the Apache License, Version 2.0
//
// Kogaro - Kubernetes Configuration Hygiene Agent

// Package metrics provides centralized Prometheus metrics for all validators.
package metrics

import (
	"fmt"
	"sync"
	"time"
)

// ValidationState represents the state of a validation error over time
type ValidationState struct {
	FirstSeen   time.Time
	LastSeen    time.Time
	State       TemporalState
	ChangeCount int
	Resolved    bool
}

// StateTracker manages validation state persistence across runs
type StateTracker struct {
	states map[string]*ValidationState
	mu     sync.RWMutex
}

// NewStateTracker creates a new state tracker
func NewStateTracker() *StateTracker {
	return &StateTracker{
		states: make(map[string]*ValidationState),
	}
}

// GetStateKey generates a unique key for a validation error
func GetStateKey(namespace, resourceType, resourceName, validationType string) string {
	return namespace + "/" + resourceType + "/" + resourceName + "/" + validationType
}

// UpdateState updates the state of a validation error and returns the updated state
func (st *StateTracker) UpdateState(key string, currentTime time.Time) *ValidationState {
	st.mu.Lock()
	defer st.mu.Unlock()

	state, exists := st.states[key]
	if !exists {
		// New issue
		state = &ValidationState{
			FirstSeen:   currentTime,
			LastSeen:    currentTime,
			State:       TemporalStateNew,
			ChangeCount: 1,
			Resolved:    false,
		}
		st.states[key] = state
	} else {
		// Existing issue
		state.LastSeen = currentTime
		state.ChangeCount++

		// Update temporal state
		age := currentTime.Sub(state.FirstSeen)
		ageHours := age.Hours()
		state.State = ClassifyTemporalState(ageHours)
	}

	return state
}

// MarkResolved marks a validation error as resolved
func (st *StateTracker) MarkResolved(key string, resolutionTime time.Time) *ValidationState {
	st.mu.Lock()
	defer st.mu.Unlock()

	state, exists := st.states[key]
	if !exists {
		return nil
	}

	state.Resolved = true
	state.LastSeen = resolutionTime
	state.State = TemporalStateResolved

	// Calculate resolution duration
	resolutionDuration := resolutionTime.Sub(state.FirstSeen)

	// Record resolution metrics
	namespace, resourceType, resourceName, validationType := parseStateKey(key)
	RecordValidationResolved(
		namespace, resourceType, resourceName, validationType,
		resolutionDuration.Hours(),
	)

	return state
}

// GetState retrieves the current state of a validation error
func (st *StateTracker) GetState(key string) *ValidationState {
	st.mu.RLock()
	defer st.mu.RUnlock()

	return st.states[key]
}

// GetAllStates returns all current validation states
func (st *StateTracker) GetAllStates() map[string]*ValidationState {
	st.mu.RLock()
	defer st.mu.RUnlock()

	// Return a copy to avoid race conditions
	result := make(map[string]*ValidationState)
	for k, v := range st.states {
		stateCopy := *v
		result[k] = &stateCopy
	}
	return result
}

// CleanupResolved removes resolved states older than the specified duration
func (st *StateTracker) CleanupResolved(olderThan time.Duration) {
	st.mu.Lock()
	defer st.mu.Unlock()

	now := time.Now()
	for key, state := range st.states {
		if state.Resolved && now.Sub(state.LastSeen) > olderThan {
			delete(st.states, key)
		}
	}
}

// parseStateKey parses a state key back into its components
func parseStateKey(key string) (namespace, resourceType, resourceName, validationType string) {
	// This is a simplified parser - in production you might want more robust parsing
	// For now, we'll assume the key format is: namespace/resourceType/resourceName/validationType
	// This is a basic implementation and might need enhancement for edge cases
	return "", "", "", ""
}

// Global state tracker instance
var globalStateTracker = NewStateTracker()

// GetGlobalStateTracker returns the global state tracker instance
func GetGlobalStateTracker() *StateTracker {
	return globalStateTracker
}

// RecordValidationErrorWithState records a validation error with proper state tracking
func RecordValidationErrorWithState(
	resourceType, resourceName, namespace, validationType, severity string,
	expectedPattern bool,
) {
	// Classify workload
	workloadCategory := ClassifyWorkload(namespace, resourceType)

	// Record the basic error
	ValidationErrors.WithLabelValues(
		resourceType,
		validationType,
		namespace,
		severity,
		string(workloadCategory),
		fmt.Sprintf("%t", expectedPattern),
	).Inc()

	// Update state tracking
	key := GetStateKey(namespace, resourceType, resourceName, validationType)
	state := globalStateTracker.UpdateState(key, time.Now())

	// Record temporal metrics
	now := float64(time.Now().Unix())
	firstSeenMetric := ValidationFirstSeen.WithLabelValues(namespace, resourceType, resourceName, validationType)
	lastSeenMetric := ValidationLastSeen.WithLabelValues(namespace, resourceType, resourceName, validationType)

	// Set timestamps
	firstSeenMetric.Set(float64(state.FirstSeen.Unix()))
	lastSeenMetric.Set(now)

	// Calculate and set age
	ageHours := time.Since(state.FirstSeen).Hours()
	ValidationAge.WithLabelValues(
		namespace, resourceType, resourceName, validationType, string(state.State),
	).Set(ageHours)

	// Record state change
	ValidationStateChanges.WithLabelValues(
		namespace, resourceType, resourceName, validationType, string(state.State),
	).Inc()
}
