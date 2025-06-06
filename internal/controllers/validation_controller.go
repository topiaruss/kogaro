// Package controllers implements Kubernetes controllers for resource validation.
//
// This package provides the ValidationController which runs periodic scans
// of the Kubernetes cluster to validate resource references using the
// controller-runtime framework.
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

// ValidationController manages periodic validation of Kubernetes resource references
type ValidationController struct {
	Client       client.Client
	Scheme       *runtime.Scheme
	Log          logr.Logger
	Registry     *validators.ValidatorRegistry
	ScanInterval time.Duration
}

// SetupWithManager registers the ValidationController with the manager
func (r *ValidationController) SetupWithManager(mgr ctrl.Manager) error {
	// Start the periodic validation as a runnable
	return mgr.Add(r)
}

// Reconcile handles reconciliation requests (not used in this implementation)
func (r *ValidationController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("validation", req.NamespacedName)

	// Run the validation scan
	log.Info("starting cluster validation scan")

	if err := r.Registry.ValidateCluster(ctx); err != nil {
		log.Error(err, "failed to validate cluster")
		return ctrl.Result{RequeueAfter: r.ScanInterval}, err
	}

	log.Info("cluster validation scan completed successfully")

	// Requeue after the scan interval
	return ctrl.Result{RequeueAfter: r.ScanInterval}, nil
}

// Start begins the periodic validation process
func (r *ValidationController) Start(ctx context.Context) error {
	log := r.Log.WithName("periodic-validator")

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
			log.Info("stopping periodic validation")
			return nil
		case <-ticker.C:
			log.Info("running periodic cluster validation")
			if err := r.Registry.ValidateCluster(ctx); err != nil {
				log.Error(err, "periodic validation failed")
			}
		}
	}
}
