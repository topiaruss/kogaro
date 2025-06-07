// Copyright 2025 Russell Ferriday
// Licensed under the Apache License, Version 2.0
//
// Kogaro - Kubernetes Configuration Hygiene Agent

// Package main provides the Kogaro Kubernetes configuration hygiene validation agent.
//
// Kogaro is a Kubernetes controller that continuously monitors cluster resources
// to detect and report configuration hygiene issues such as dangling references
// to non-existent ConfigMaps, Secrets, PVCs, and other resources.
package main

import (
	"flag"
	"os"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/topiaruss/kogaro/internal/controllers"
	"github.com/topiaruss/kogaro/internal/metrics"
	"github.com/topiaruss/kogaro/internal/validators"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var scanInterval time.Duration

	// Reference validation flags
	var enableIngressValidation bool
	var enableConfigMapValidation bool
	var enableSecretValidation bool
	var enablePVCValidation bool
	var enableServiceAccountValidation bool

	// Resource limits validation flags
	var enableResourceLimitsValidation bool
	var enableMissingRequestsValidation bool
	var enableMissingLimitsValidation bool
	var enableQoSValidation bool
	var minCPURequest string
	var minMemoryRequest string

	// Security validation flags
	var enableSecurityValidation bool
	var enableRootUserValidation bool
	var enableSecurityContextValidation bool
	var enableSecurityServiceAccountValidation bool
	var enableNetworkPolicyValidation bool
	var securitySensitiveNamespaces string

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.DurationVar(&scanInterval, "scan-interval", 5*time.Minute, "Interval between cluster scans for reference validation")

	// Reference validation configuration flags
	flag.BoolVar(&enableIngressValidation, "enable-ingress-validation", true, "Enable validation of Ingress references (IngressClass, Services)")
	flag.BoolVar(&enableConfigMapValidation, "enable-configmap-validation", true, "Enable validation of ConfigMap references in Pods")
	flag.BoolVar(&enableSecretValidation, "enable-secret-validation", true, "Enable validation of Secret references (volumes, env, TLS)")
	flag.BoolVar(&enablePVCValidation, "enable-pvc-validation", true, "Enable validation of PVC and StorageClass references")
	flag.BoolVar(&enableServiceAccountValidation, "enable-reference-serviceaccount-validation", false, "Enable validation of ServiceAccount references (may be noisy)")

	// Resource limits validation configuration flags
	flag.BoolVar(&enableResourceLimitsValidation, "enable-resource-limits-validation", true, "Enable validation of resource requests and limits")
	flag.BoolVar(&enableMissingRequestsValidation, "enable-missing-requests-validation", true, "Enable validation for missing resource requests")
	flag.BoolVar(&enableMissingLimitsValidation, "enable-missing-limits-validation", true, "Enable validation for missing resource limits")
	flag.BoolVar(&enableQoSValidation, "enable-qos-validation", true, "Enable QoS class analysis and validation")
	flag.StringVar(&minCPURequest, "min-cpu-request", "", "Minimum CPU request threshold (e.g., '10m')")
	flag.StringVar(&minMemoryRequest, "min-memory-request", "", "Minimum memory request threshold (e.g., '16Mi')")

	// Security validation configuration flags
	flag.BoolVar(&enableSecurityValidation, "enable-security-validation", true, "Enable security configuration validation")
	flag.BoolVar(&enableRootUserValidation, "enable-root-user-validation", true, "Enable validation for containers running as root")
	flag.BoolVar(&enableSecurityContextValidation, "enable-security-context-validation", true, "Enable validation for missing SecurityContext configurations")
	flag.BoolVar(&enableSecurityServiceAccountValidation, "enable-security-serviceaccount-validation", true, "Enable validation for ServiceAccount excessive permissions")
	flag.BoolVar(&enableNetworkPolicyValidation, "enable-network-policy-validation", true, "Enable validation for missing NetworkPolicies in sensitive namespaces")
	flag.StringVar(&securitySensitiveNamespaces, "security-required-namespaces", "", "Comma-separated list of namespaces that require NetworkPolicies for security validation")

	// Networking validation flags
	var enableNetworkingValidation bool
	var enableNetworkingServiceValidation bool
	var enableNetworkingIngressValidation bool
	var enableNetworkingPolicyValidation bool
	var networkingPolicyRequiredNamespaces string
	var warnUnexposedPods bool

	// Networking validation configuration flags
	flag.BoolVar(&enableNetworkingValidation, "enable-networking-validation", true, "Enable networking connectivity validation")
	flag.BoolVar(&enableNetworkingServiceValidation, "enable-networking-service-validation", true, "Enable validation for Service selector mismatches")
	flag.BoolVar(&enableNetworkingIngressValidation, "enable-networking-ingress-validation", true, "Enable validation for Ingress connectivity issues")
	flag.BoolVar(&enableNetworkingPolicyValidation, "enable-networking-policy-validation", true, "Enable validation for NetworkPolicy coverage")
	flag.StringVar(&networkingPolicyRequiredNamespaces, "networking-required-namespaces", "", "Comma-separated list of namespaces that require NetworkPolicies for networking validation")
	flag.BoolVar(&warnUnexposedPods, "warn-unexposed-pods", false, "Enable warnings for pods not exposed by any Service")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: server.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "kogaro.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Register metrics
	metrics.RegisterMetrics()

	// Initialize the validator registry
	registry := validators.NewValidatorRegistry(setupLog)

	// Initialize the reference validator with configuration
	validationConfig := validators.ValidationConfig{
		EnableIngressValidation:        enableIngressValidation,
		EnableConfigMapValidation:      enableConfigMapValidation,
		EnableSecretValidation:         enableSecretValidation,
		EnablePVCValidation:            enablePVCValidation,
		EnableServiceAccountValidation: enableServiceAccountValidation,
	}
	referenceValidator := validators.NewReferenceValidator(mgr.GetClient(), setupLog, validationConfig)
	
	// Register the reference validator
	registry.Register(referenceValidator)

	// Initialize and register the resource limits validator if enabled
	if enableResourceLimitsValidation {
		resourceLimitsConfig := validators.ResourceLimitsConfig{
			EnableMissingRequestsValidation: enableMissingRequestsValidation,
			EnableMissingLimitsValidation:   enableMissingLimitsValidation,
			EnableQoSValidation:             enableQoSValidation,
		}

		// Parse minimum resource thresholds if provided
		if minCPURequest != "" {
			if cpuQuantity, err := resource.ParseQuantity(minCPURequest); err != nil {
				setupLog.Info("invalid min-cpu-request value, using default", "invalid_value", minCPURequest, "error", err, "default", "10m")
				defaultCPU := resource.MustParse("10m")
				resourceLimitsConfig.MinCPURequest = &defaultCPU
			} else {
				resourceLimitsConfig.MinCPURequest = &cpuQuantity
			}
		}

		if minMemoryRequest != "" {
			if memoryQuantity, err := resource.ParseQuantity(minMemoryRequest); err != nil {
				setupLog.Info("invalid min-memory-request value, using default", "invalid_value", minMemoryRequest, "error", err, "default", "64Mi")
				defaultMemory := resource.MustParse("64Mi")
				resourceLimitsConfig.MinMemoryRequest = &defaultMemory
			} else {
				resourceLimitsConfig.MinMemoryRequest = &memoryQuantity
			}
		}

		resourceLimitsValidator := validators.NewResourceLimitsValidator(mgr.GetClient(), setupLog, resourceLimitsConfig)
		registry.Register(resourceLimitsValidator)
	}

	// Initialize and register the security validator if enabled
	if enableSecurityValidation {
		securityConfig := validators.SecurityConfig{
			EnableRootUserValidation:       enableRootUserValidation,
			EnableSecurityContextValidation: enableSecurityContextValidation,
			EnableServiceAccountValidation:  enableSecurityServiceAccountValidation,
			EnableNetworkPolicyValidation:   enableNetworkPolicyValidation,
		}

		// Parse security-sensitive namespaces if provided
		if securitySensitiveNamespaces != "" {
			namespaces := strings.Split(securitySensitiveNamespaces, ",")
			for i, ns := range namespaces {
				namespaces[i] = strings.TrimSpace(ns)
			}
			securityConfig.SecuritySensitiveNamespaces = namespaces
		}

		securityValidator := validators.NewSecurityValidator(mgr.GetClient(), setupLog, securityConfig)
		registry.Register(securityValidator)
	}

	// Initialize and register the networking validator if enabled
	if enableNetworkingValidation {
		networkingConfig := validators.NetworkingConfig{
			EnableServiceValidation:      enableNetworkingServiceValidation,
			EnableNetworkPolicyValidation: enableNetworkingPolicyValidation,
			EnableIngressValidation:      enableNetworkingIngressValidation,
			WarnUnexposedPods:           warnUnexposedPods,
		}

		// Parse networking policy required namespaces if provided
		if networkingPolicyRequiredNamespaces != "" {
			namespaces := strings.Split(networkingPolicyRequiredNamespaces, ",")
			for i, ns := range namespaces {
				namespaces[i] = strings.TrimSpace(ns)
			}
			networkingConfig.PolicyRequiredNamespaces = namespaces
		}

		networkingValidator := validators.NewNetworkingValidator(mgr.GetClient(), setupLog, networkingConfig)
		registry.Register(networkingValidator)
	}

	// Setup the validation controller
	if err = (&controllers.ValidationController{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		Log:          ctrl.Log.WithName("controllers").WithName("ValidationController"),
		Registry:     registry,
		ScanInterval: scanInterval,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ValidationController")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
