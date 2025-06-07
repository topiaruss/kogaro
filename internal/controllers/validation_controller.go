// Copyright 2025 Russell Ferriday
// Licensed under the Apache License, Version 2.0
//
// Kogaro - Kubernetes Configuration Hygiene Agent

// Package controllers implements timer-based validation runnables.
//
// This package provides the ValidationController which implements the
// manager.Runnable interface to run periodic cluster-wide validation scans
// using the controller-runtime framework. The controller uses a timer-based
// approach rather than event-driven reconciliation since it needs to validate
// the entire cluster state at regular intervals.
package controllers

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/topiaruss/kogaro/internal/validators"
)

// ValidationController manages periodic validation of Kubernetes resource references.
// It implements the manager.Runnable interface to run as a timer-based background process.
type ValidationController struct {
	Client       client.Client
	Scheme       *runtime.Scheme
	Log          logr.Logger
	Registry     *validators.ValidatorRegistry
	ScanInterval time.Duration
}

// SetupWithManager registers the ValidationController with the manager as a runnable
func (r *ValidationController) SetupWithManager(mgr ctrl.Manager) error {
	// Register this controller as a runnable for periodic execution
	return mgr.Add(r)
}

// NeedLeaderElection implements manager.LeaderElectionRunnable
// Returns true to ensure only one instance runs cluster validation when leader election is enabled
func (r *ValidationController) NeedLeaderElection() bool {
	return true
}

// Start begins the periodic validation process.
// This method implements the manager.Runnable interface.
func (r *ValidationController) Start(ctx context.Context) error {
	log := r.Log.WithName("periodic-validator")
	log.Info("starting periodic validation controller", "scan_interval", r.ScanInterval)

	ticker := time.NewTicker(r.ScanInterval)
	defer ticker.Stop()

	// Run initial validation
	log.Info("running initial cluster validation")
	if err := r.Registry.ValidateCluster(ctx); err != nil {
		log.Error(err, "initial validation failed")
	}

	for {
		select {
		case <-ctx.Done():
			log.Info("stopping periodic validation controller")
			return nil
		case <-ticker.C:
			log.Info("running periodic cluster validation")
			if err := r.Registry.ValidateCluster(ctx); err != nil {
				log.Error(err, "periodic validation failed")
			}
		}
	}
}
