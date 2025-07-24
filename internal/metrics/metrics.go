// Copyright 2025 Russell Ferriday
// Licensed under the Apache License, Version 2.0
//
// Kogaro - Kubernetes Configuration Hygiene Agent

// Package metrics provides centralized Prometheus metrics for all validators.
//
// This package defines and registers all Prometheus metrics used by Kogaro
// validators to prevent registration collisions and provide a consistent
// metrics interface across all validation types.
package metrics

import (
	"fmt"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// ValidationErrors tracks the total number of validation errors found
	ValidationErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kogaro_validation_errors_total",
			Help: "Total number of validation errors found",
		},
		[]string{"resource_type", "validation_type", "namespace", "resource_name", "severity", "workload_category", "expected_pattern", "error_code"},
	)

	// ValidationFirstSeen tracks when validation errors were first detected
	ValidationFirstSeen = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kogaro_validation_first_seen_timestamp",
			Help: "Timestamp when validation error was first detected",
		},
		[]string{"namespace", "resource_type", "resource_name", "validation_type", "severity", "workload_category", "expected_pattern", "error_code"},
	)

	// ValidationLastSeen tracks when validation errors were last seen
	ValidationLastSeen = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kogaro_validation_last_seen_timestamp",
			Help: "Timestamp when validation error was last detected",
		},
		[]string{"namespace", "resource_type", "resource_name", "validation_type"},
	)

	// ValidationAge tracks the age of validation errors in hours
	ValidationAge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kogaro_validation_age_hours",
			Help: "Age of validation error in hours",
		},
		[]string{"namespace", "resource_type", "resource_name", "validation_type", "temporal_state"},
	)

	// ValidationStateChanges tracks the number of validation state changes
	ValidationStateChanges = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kogaro_validation_state_changes_total",
			Help: "Number of validation state changes",
		},
		[]string{"namespace", "resource_type", "resource_name", "validation_type", "change_type"},
	)

	// ValidationResolved tracks the number of validation errors resolved
	ValidationResolved = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kogaro_validation_resolved_total",
			Help: "Number of validation errors resolved",
		},
		[]string{"namespace", "resource_type", "resource_name", "validation_type", "resolution_duration_hours"},
	)

	// ValidationRuns tracks the total number of validation runs performed
	ValidationRuns = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "kogaro_validation_runs_total",
			Help: "Total number of validation runs performed",
		},
	)

	once sync.Once
)

// TemporalState represents the temporal classification of a validation error
type TemporalState string

const (
	// TemporalStateNew indicates a validation error that was first seen recently (< 1 hour)
	TemporalStateNew TemporalState = "new"
	// TemporalStateRecent indicates a validation error that has been seen for 1-24 hours
	TemporalStateRecent TemporalState = "recent"
	// TemporalStateStable indicates a validation error that has been seen for > 24 hours
	TemporalStateStable TemporalState = "stable"
	// TemporalStateResolved indicates a validation error that was previously seen but is now resolved
	TemporalStateResolved TemporalState = "resolved"
)

// WorkloadCategory represents the classification of the workload
type WorkloadCategory string

const (
	// WorkloadCategoryInfrastructure indicates infrastructure components (system namespaces, etc.)
	WorkloadCategoryInfrastructure WorkloadCategory = "infrastructure"
	// WorkloadCategoryApplication indicates application workloads
	WorkloadCategoryApplication WorkloadCategory = "application"
)

// RegisterMetrics registers all Kogaro metrics with the controller-runtime metrics registry.
// This function is safe to call multiple times due to sync.Once protection.
func RegisterMetrics() {
	once.Do(func() {
		metrics.Registry.MustRegister(ValidationErrors)
		metrics.Registry.MustRegister(ValidationFirstSeen)
		metrics.Registry.MustRegister(ValidationLastSeen)
		metrics.Registry.MustRegister(ValidationAge)
		metrics.Registry.MustRegister(ValidationStateChanges)
		metrics.Registry.MustRegister(ValidationResolved)
		metrics.Registry.MustRegister(ValidationRuns)
	})
}

// ClassifyWorkload determines the workload category based on namespace and resource type
func ClassifyWorkload(namespace, resourceType string) WorkloadCategory {
	// System namespaces are always infrastructure
	systemNamespaces := []string{
		"kube-system", "kube-public", "cert-manager", "monitoring",
		"ingress-nginx", "istio-system", "linkerd", "calico-system",
		"prometheus", "grafana", "alertmanager", "kogaro",
	}

	for _, ns := range systemNamespaces {
		if namespace == ns {
			return WorkloadCategoryInfrastructure
		}
	}

	// Additional infrastructure indicators
	if strings.Contains(namespace, "system") ||
		strings.Contains(namespace, "monitoring") ||
		strings.Contains(namespace, "logging") ||
		strings.Contains(namespace, "security") {
		return WorkloadCategoryInfrastructure
	}

	// Default to application
	return WorkloadCategoryApplication
}

// ClassifyTemporalState determines the temporal state based on age in hours
func ClassifyTemporalState(ageHours float64) TemporalState {
	switch {
	case ageHours < 1:
		return TemporalStateNew
	case ageHours < 24:
		return TemporalStateRecent
	default:
		return TemporalStateStable
	}
}

// RecordValidationError records a validation error with temporal awareness
// This is a legacy function that uses the new state-aware recording
func RecordValidationError(
	resourceType, resourceName, namespace, validationType, severity string,
	expectedPattern bool,
) {
	RecordValidationErrorWithState(resourceType, resourceName, namespace, validationType, severity, "", expectedPattern)
}

// RecordValidationResolved records when a validation error is resolved
func RecordValidationResolved(
	namespace, resourceType, resourceName, validationType, severity, errorCode string,
	resolutionDurationHours float64,
) {
	// Record resolution
	ValidationResolved.WithLabelValues(
		namespace, resourceType, resourceName, validationType,
		fmt.Sprintf("%.1f", resolutionDurationHours),
	).Inc()

	// Record state change
	ValidationStateChanges.WithLabelValues(
		namespace, resourceType, resourceName, validationType, "resolved",
	).Inc()

	// Remove temporal metrics for resolved errors
	// Note: We need to provide all required labels, but we'll use empty strings for missing ones
	ValidationFirstSeen.WithLabelValues(namespace, resourceType, resourceName, validationType, severity, "", "", errorCode).Set(0)
	ValidationLastSeen.WithLabelValues(namespace, resourceType, resourceName, validationType).Set(0)
	ValidationAge.WithLabelValues(namespace, resourceType, resourceName, validationType, "resolved").Set(0)
}
