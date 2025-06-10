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
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
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
	var (
		metricsAddr          string
		enableLeaderElection bool
		probeAddr            string
		scanInterval         string

		// Reference validation flags
		enableIngressValidation        bool
		enableConfigMapValidation      bool
		enableSecretValidation         bool
		enablePVCValidation            bool
		enableServiceAccountValidation bool

		// Resource limits validation flags
		enableResourceLimitsValidation  bool
		enableMissingRequestsValidation bool
		enableMissingLimitsValidation   bool
		enableQoSValidation             bool
		minCPURequest                   string
		minMemoryRequest                string

		// Security validation flags
		enableSecurityValidation               bool
		enableRootUserValidation               bool
		enableSecurityContextValidation        bool
		enableSecurityServiceAccountValidation bool
		enableNetworkPolicyValidation          bool
		securitySensitiveNamespaces            string

		// Networking validation flags
		enableNetworkingValidation         bool
		enableNetworkingServiceValidation  bool
		enableNetworkingIngressValidation  bool
		enableNetworkingPolicyValidation   bool
		networkingPolicyRequiredNamespaces string
		warnUnexposedPods                  bool

		// Image validation flags
		enableImageValidation        bool
		allowMissingImages           bool
		allowArchitectureMismatch    bool

		// New validate command flags
		validateMode     string
		validateConfig   string
		validateDuration string
		validateInterval string
		validateOutput   string
		validateScope    string
	)

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&scanInterval, "scan-interval", "5m", "Interval between cluster scans for reference validation")

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

	// Networking validation configuration flags
	flag.BoolVar(&enableNetworkingValidation, "enable-networking-validation", true, "Enable networking connectivity validation")
	flag.BoolVar(&enableNetworkingServiceValidation, "enable-networking-service-validation", true, "Enable validation for Service selector mismatches")
	flag.BoolVar(&enableNetworkingIngressValidation, "enable-networking-ingress-validation", true, "Enable validation for Ingress connectivity issues")
	flag.BoolVar(&enableNetworkingPolicyValidation, "enable-networking-policy-validation", true, "Enable validation for NetworkPolicy coverage")
	flag.StringVar(&networkingPolicyRequiredNamespaces, "networking-required-namespaces", "", "Comma-separated list of namespaces that require NetworkPolicies for networking validation")
	flag.BoolVar(&warnUnexposedPods, "warn-unexposed-pods", false, "Enable warnings for pods not exposed by any Service")

	// Image validation configuration flags
	flag.BoolVar(&enableImageValidation, "enable-image-validation", false, "Enable validation of container images (registry existence and architecture)")
	flag.BoolVar(&allowMissingImages, "allow-missing-images", false, "Allow deployment even if images are not found in registry")
	flag.BoolVar(&allowArchitectureMismatch, "allow-architecture-mismatch", false, "Allow deployment even if image architecture doesn't match nodes")

	// Add validate command flags
	flag.StringVar(&validateMode, "mode", "one-off", "Validation mode: one-off or monitor")
	flag.StringVar(&validateConfig, "config", "", "Path to configuration file to validate")
	flag.StringVar(&validateDuration, "duration", "", "Duration for monitor mode (e.g., 10m)")
	flag.StringVar(&validateInterval, "interval", "1m", "Interval between validations in monitor mode")
	flag.StringVar(&validateOutput, "output", "text", "Output format: text, json, or yaml")
	flag.StringVar(&validateScope, "scope", "all", "Validation scope: all (show all errors) or file-only (show only errors for config file resources)")

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
	registry := validators.NewValidatorRegistry(setupLog, mgr.GetClient())

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
			EnableRootUserValidation:        enableRootUserValidation,
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
			EnableServiceValidation:       enableNetworkingServiceValidation,
			EnableNetworkPolicyValidation: enableNetworkingPolicyValidation,
			EnableIngressValidation:       enableNetworkingIngressValidation,
			WarnUnexposedPods:             warnUnexposedPods,
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

	// Initialize and register the image validator if enabled
	if enableImageValidation {
		// Create Kubernetes clientset from the same config
		k8sClient, err := kubernetes.NewForConfig(mgr.GetConfig())
		if err != nil {
			setupLog.Error(err, "failed to get Kubernetes clientset for image validation")
			os.Exit(1)
		}

		imageConfig := validators.ImageValidatorConfig{
			EnableImageValidation:         enableImageValidation,
			AllowMissingImages:           allowMissingImages,
			AllowArchitectureMismatch:    allowArchitectureMismatch,
		}

		imageValidator := validators.NewImageValidator(mgr.GetClient(), k8sClient, setupLog, imageConfig)
		registry.Register(imageValidator)
	}

	// Handle validate command
	if validateMode != "" {
		// Early validation for one-off mode to catch Helm templates before Kubernetes connection
		if validateMode == "one-off" && validateConfig != "" {
			if err := validateConfigFile(validateConfig); err != nil {
				setupLog.Error(err, "validation failed")
				os.Exit(1)
			}
		}

		// Parse duration if provided
		var duration time.Duration
		if validateDuration != "" {
			var err error
			duration, err = time.ParseDuration(validateDuration)
			if err != nil {
				setupLog.Error(err, "invalid duration format")
				os.Exit(1)
			}
		}

		// Parse interval
		interval, err := time.ParseDuration(validateInterval)
		if err != nil {
			setupLog.Error(err, "invalid interval format")
			os.Exit(1)
		}

		// Start the manager cache briefly to allow cluster object retrieval
		setupLog.Info("starting manager cache for CLI validation")
		cacheCtx, cacheCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cacheCancel()
		
		go func() {
			if err := mgr.Start(cacheCtx); err != nil && err != context.Canceled {
				setupLog.Error(err, "failed to start manager for cache warmup")
			}
		}()
		
		// Wait for cache to sync
		if !mgr.GetCache().WaitForCacheSync(cacheCtx) {
			setupLog.Error(nil, "failed to sync cache")
			os.Exit(1)
		}
		setupLog.Info("cache synced successfully")

		// Create validation context
		ctx := context.Background()
		if duration > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, duration)
			defer cancel()
		}

		// Run validation based on mode
		switch validateMode {
		case "one-off":
			if validateConfig != "" {
				// Validate new configuration against cluster with scope filtering
				result, err := registry.ValidateNewConfigWithScope(ctx, validateConfig, validateScope)
				if err != nil {
					setupLog.Error(err, "validation failed")
					os.Exit(1)
				}

				// Format output based on mode
				if validateOutput == "ci" {
					output, err := registry.FormatCIOutput(*result)
					if err != nil {
						setupLog.Error(err, "failed to format CI output")
						os.Exit(1)
					}
					// Output to stderr for CI consumption
					fmt.Fprintf(os.Stderr, "%s\n", output)
					os.Exit(result.ExitCode)
				} else {
					// Regular output
					if result.ExitCode > 0 {
						setupLog.Error(nil, "validation failed",
							"total_errors", result.Summary.TotalErrors,
							"missing_refs", result.Summary.MissingRefs,
							"suggested_refs", result.Summary.SuggestedRefs)
						os.Exit(result.ExitCode)
					}
				}
			} else {
				// Validate existing cluster
				if err := registry.ValidateCluster(ctx); err != nil {
					setupLog.Error(err, "validation failed")
					os.Exit(1)
				}
			}
		case "monitor":
			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if err := registry.ValidateCluster(ctx); err != nil {
						setupLog.Error(err, "validation failed")
					}
				}
			}
		default:
			setupLog.Error(nil, "invalid validation mode", "mode", validateMode)
			os.Exit(1)
		}
		return
	}

	// Parse scan interval
	scanIntervalDuration, err := time.ParseDuration(scanInterval)
	if err != nil {
		setupLog.Error(err, "invalid scan interval format")
		os.Exit(1)
	}

	// Setup the validation controller
	validationController := &controllers.ValidationController{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		Log:          setupLog,
		Registry:     registry,
		ScanInterval: scanIntervalDuration,
	}

	if err = validationController.SetupWithManager(mgr); err != nil {
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

// validateConfigFile performs early validation of config file for Helm template detection
func validateConfigFile(configPath string) error {
	// Read the config file
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Check for Helm template syntax early
	configStr := string(configData)
	if strings.Contains(configStr, "{{") && strings.Contains(configStr, "}}") {
		return fmt.Errorf("file appears to contain Helm templates. Please render the template first using 'helm template' and validate the resulting YAML")
	}

	return nil
}
