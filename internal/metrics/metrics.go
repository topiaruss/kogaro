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
		[]string{"resource_type", "validation_type", "namespace"},
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

// RegisterMetrics registers all Kogaro metrics with the controller-runtime metrics registry.
// This function is safe to call multiple times due to sync.Once protection.
func RegisterMetrics() {
	once.Do(func() {
		metrics.Registry.MustRegister(ValidationErrors)
		metrics.Registry.MustRegister(ValidationRuns)
	})
}