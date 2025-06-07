// Copyright 2025 Russell Ferriday
// Licensed under the Apache License, Version 2.0
//
// Kogaro - Kubernetes Configuration Hygiene Agent

package controllers

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/topiaruss/kogaro/internal/validators"
)

func TestValidationController_NeedLeaderElection(t *testing.T) {
	controller := &ValidationController{}
	
	// Should require leader election for cluster-wide validation
	if !controller.NeedLeaderElection() {
		t.Error("Expected NeedLeaderElection() to return true")
	}
}

func TestValidationController_Start(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = networkingv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	_ = storagev1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	config := validators.ValidationConfig{
		EnableIngressValidation: true,
	}
	validator := validators.NewReferenceValidator(fakeClient, logr.Discard(), config)
	
	registry := validators.NewValidatorRegistry(logr.Discard())
	registry.Register(validator)

	controller := &ValidationController{
		Client:       fakeClient,
		Scheme:       scheme,
		Log:          logr.Discard(),
		Registry:     registry,
		ScanInterval: 100 * time.Millisecond, // Short interval for test
	}

	// Test start with cancellation
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Start should run without error and respect context cancellation
	err := controller.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
}
