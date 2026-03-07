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
	"io"
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

// FlagConfig holds all CLI flag values
type FlagConfig struct {
	// Manager settings
	MetricsAddr          string
	EnableLeaderElection bool
	ProbeAddr            string
	ScanInterval         string

	// Reference validation flags
	EnableIngressValidation        bool
	EnableConfigMapValidation      bool
	EnableSecretValidation         bool
	EnablePVCValidation            bool
	EnableServiceAccountValidation bool

	// Resource limits validation flags
	EnableResourceLimitsValidation  bool
	EnableMissingRequestsValidation bool
	EnableMissingLimitsValidation   bool
	EnableQoSValidation             bool
	MinCPURequest                   string
	MinMemoryRequest                string

	// Security validation flags
	EnableSecurityValidation               bool
	EnableRootUserValidation               bool
	EnableSecurityContextValidation        bool
	EnableSecurityServiceAccountValidation bool
	EnableNetworkPolicyValidation          bool
	SecuritySensitiveNamespaces            string

	// Networking validation flags
	EnableNetworkingValidation         bool
	EnableNetworkingServiceValidation  bool
	EnableNetworkingIngressValidation  bool
	EnableNetworkingPolicyValidation   bool
	NetworkingPolicyRequiredNamespaces string
	WarnUnexposedPods                  bool

	// Image validation flags
	EnableImageValidation     bool
	AllowMissingImages        bool
	AllowArchitectureMismatch bool

	// Validate command flags
	ValidateMode     string
	ValidateConfig   string
	ValidateDuration string
	ValidateInterval string
	ValidateOutput   string
	ValidateScope    string
}

// registerFlags defines and parses all CLI flags
func registerFlags() *FlagConfig {
	config := &FlagConfig{}

	flag.StringVar(&config.MetricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&config.ProbeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&config.EnableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&config.ScanInterval, "scan-interval", "5m", "Interval between cluster scans for reference validation")

	// Reference validation configuration flags
	flag.BoolVar(&config.EnableIngressValidation, "enable-ingress-validation", true, "Enable validation of Ingress references (IngressClass, Services)")
	flag.BoolVar(&config.EnableConfigMapValidation, "enable-configmap-validation", true, "Enable validation of ConfigMap references in Pods")
	flag.BoolVar(&config.EnableSecretValidation, "enable-secret-validation", true, "Enable validation of Secret references (volumes, env, TLS)")
	flag.BoolVar(&config.EnablePVCValidation, "enable-pvc-validation", true, "Enable validation of PVC and StorageClass references")
	flag.BoolVar(&config.EnableServiceAccountValidation, "enable-reference-serviceaccount-validation", false, "Enable validation of ServiceAccount references (may be noisy)")

	// Resource limits validation configuration flags
	flag.BoolVar(&config.EnableResourceLimitsValidation, "enable-resource-limits-validation", true, "Enable validation of resource requests and limits")
	flag.BoolVar(&config.EnableMissingRequestsValidation, "enable-missing-requests-validation", true, "Enable validation for missing resource requests")
	flag.BoolVar(&config.EnableMissingLimitsValidation, "enable-missing-limits-validation", true, "Enable validation for missing resource limits")
	flag.BoolVar(&config.EnableQoSValidation, "enable-qos-validation", true, "Enable QoS class analysis and validation")
	flag.StringVar(&config.MinCPURequest, "min-cpu-request", "", "Minimum CPU request threshold (e.g., '10m')")
	flag.StringVar(&config.MinMemoryRequest, "min-memory-request", "", "Minimum memory request threshold (e.g., '16Mi')")

	// Security validation configuration flags
	flag.BoolVar(&config.EnableSecurityValidation, "enable-security-validation", true, "Enable security configuration validation")
	flag.BoolVar(&config.EnableRootUserValidation, "enable-root-user-validation", true, "Enable validation for containers running as root")
	flag.BoolVar(&config.EnableSecurityContextValidation, "enable-security-context-validation", true, "Enable validation for missing SecurityContext configurations")
	flag.BoolVar(&config.EnableSecurityServiceAccountValidation, "enable-security-serviceaccount-validation", true, "Enable validation for ServiceAccount excessive permissions")
	flag.BoolVar(&config.EnableNetworkPolicyValidation, "enable-network-policy-validation", true, "Enable validation for missing NetworkPolicies in sensitive namespaces")
	flag.StringVar(&config.SecuritySensitiveNamespaces, "security-required-namespaces", "", "Comma-separated list of namespaces that require NetworkPolicies for security validation")

	// Networking validation configuration flags
	flag.BoolVar(&config.EnableNetworkingValidation, "enable-networking-validation", true, "Enable networking connectivity validation")
	flag.BoolVar(&config.EnableNetworkingServiceValidation, "enable-networking-service-validation", true, "Enable validation for Service selector mismatches")
	flag.BoolVar(&config.EnableNetworkingIngressValidation, "enable-networking-ingress-validation", true, "Enable validation for Ingress connectivity issues")
	flag.BoolVar(&config.EnableNetworkingPolicyValidation, "enable-networking-policy-validation", true, "Enable validation for NetworkPolicy coverage")
	flag.StringVar(&config.NetworkingPolicyRequiredNamespaces, "networking-required-namespaces", "", "Comma-separated list of namespaces that require NetworkPolicies for networking validation")
	flag.BoolVar(&config.WarnUnexposedPods, "warn-unexposed-pods", false, "Enable warnings for pods not exposed by any Service")

	// Image validation configuration flags
	flag.BoolVar(&config.EnableImageValidation, "enable-image-validation", false, "Enable validation of container images (registry existence and architecture)")
	flag.BoolVar(&config.AllowMissingImages, "allow-missing-images", false, "Allow deployment even if images are not found in registry")
	flag.BoolVar(&config.AllowArchitectureMismatch, "allow-architecture-mismatch", false, "Allow deployment even if image architecture doesn't match nodes")

	// Add validate command flags
	flag.StringVar(&config.ValidateMode, "mode", "", "Validation mode: one-off or monitor")
	flag.StringVar(&config.ValidateConfig, "config", "", "Path to configuration file to validate")
	flag.StringVar(&config.ValidateDuration, "duration", "", "Duration for monitor mode (e.g., 10m)")
	flag.StringVar(&config.ValidateInterval, "interval", "1m", "Interval between validations in monitor mode")
	flag.StringVar(&config.ValidateOutput, "output", "text", "Output format: text, json, or yaml")
	flag.StringVar(&config.ValidateScope, "scope", "all", "Validation scope: all (show all errors) or file-only (show only errors for config file resources)")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	return config
}

// setupValidators initializes and registers all validators based on configuration
func setupValidators(mgr ctrl.Manager, config *FlagConfig) *validators.ValidatorRegistry {
	registry := validators.NewValidatorRegistry(setupLog, mgr.GetClient())

	// Initialize the reference validator with configuration
	validationConfig := validators.ValidationConfig{
		EnableIngressValidation:        config.EnableIngressValidation,
		EnableConfigMapValidation:      config.EnableConfigMapValidation,
		EnableSecretValidation:         config.EnableSecretValidation,
		EnablePVCValidation:            config.EnablePVCValidation,
		EnableServiceAccountValidation: config.EnableServiceAccountValidation,
	}
	referenceValidator := validators.NewReferenceValidator(mgr.GetClient(), setupLog, validationConfig)
	registry.Register(referenceValidator)

	// Initialize and register the resource limits validator if enabled
	if config.EnableResourceLimitsValidation {
		resourceLimitsConfig := validators.ResourceLimitsConfig{
			EnableMissingRequestsValidation: config.EnableMissingRequestsValidation,
			EnableMissingLimitsValidation:   config.EnableMissingLimitsValidation,
			EnableQoSValidation:             config.EnableQoSValidation,
		}

		// Parse minimum resource thresholds if provided
		if config.MinCPURequest != "" {
			if cpuQuantity, err := resource.ParseQuantity(config.MinCPURequest); err != nil {
				setupLog.Info("invalid min-cpu-request value, using default", "invalid_value", config.MinCPURequest, "error", err, "default", "10m")
				defaultCPU := resource.MustParse("10m")
				resourceLimitsConfig.MinCPURequest = &defaultCPU
			} else {
				resourceLimitsConfig.MinCPURequest = &cpuQuantity
			}
		}

		if config.MinMemoryRequest != "" {
			if memoryQuantity, err := resource.ParseQuantity(config.MinMemoryRequest); err != nil {
				setupLog.Info("invalid min-memory-request value, using default", "invalid_value", config.MinMemoryRequest, "error", err, "default", "64Mi")
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
	if config.EnableSecurityValidation {
		securityConfig := validators.SecurityConfig{
			EnableRootUserValidation:        config.EnableRootUserValidation,
			EnableSecurityContextValidation: config.EnableSecurityContextValidation,
			EnableServiceAccountValidation:  config.EnableSecurityServiceAccountValidation,
			EnableNetworkPolicyValidation:   config.EnableNetworkPolicyValidation,
		}

		// Parse security-sensitive namespaces if provided
		if config.SecuritySensitiveNamespaces != "" {
			namespaces := strings.Split(config.SecuritySensitiveNamespaces, ",")
			for i, ns := range namespaces {
				namespaces[i] = strings.TrimSpace(ns)
			}
			securityConfig.SecuritySensitiveNamespaces = namespaces
		}

		securityValidator := validators.NewSecurityValidator(mgr.GetClient(), setupLog, securityConfig)
		registry.Register(securityValidator)
	}

	// Initialize and register the networking validator if enabled
	if config.EnableNetworkingValidation {
		networkingConfig := validators.NetworkingConfig{
			EnableServiceValidation:       config.EnableNetworkingServiceValidation,
			EnableNetworkPolicyValidation: config.EnableNetworkingPolicyValidation,
			EnableIngressValidation:       config.EnableNetworkingIngressValidation,
			WarnUnexposedPods:             config.WarnUnexposedPods,
		}

		// Parse networking policy required namespaces if provided
		if config.NetworkingPolicyRequiredNamespaces != "" {
			namespaces := strings.Split(config.NetworkingPolicyRequiredNamespaces, ",")
			for i, ns := range namespaces {
				namespaces[i] = strings.TrimSpace(ns)
			}
			networkingConfig.PolicyRequiredNamespaces = namespaces
		}

		networkingValidator := validators.NewNetworkingValidator(mgr.GetClient(), setupLog, networkingConfig)
		registry.Register(networkingValidator)
	}

	// Initialize and register the image validator if enabled
	if config.EnableImageValidation {
		// Create Kubernetes clientset from the same config
		k8sClient, err := kubernetes.NewForConfig(mgr.GetConfig())
		if err != nil {
			setupLog.Error(err, "failed to get Kubernetes clientset for image validation")
			os.Exit(1)
		}

		imageConfig := validators.ImageValidatorConfig{
			EnableImageValidation:     config.EnableImageValidation,
			AllowMissingImages:        config.AllowMissingImages,
			AllowArchitectureMismatch: config.AllowArchitectureMismatch,
		}

		imageValidator := validators.NewImageValidator(mgr.GetClient(), k8sClient, setupLog, imageConfig)
		registry.Register(imageValidator)
	}

	return registry
}

// runValidationMode handles one-off and monitor validation modes
func runValidationMode(mgr ctrl.Manager, registry *validators.ValidatorRegistry, config *FlagConfig, configData []byte) {
	// Parse duration if provided
	var duration time.Duration
	if config.ValidateDuration != "" {
		var err error
		duration, err = time.ParseDuration(config.ValidateDuration)
		if err != nil {
			setupLog.Error(err, "invalid duration format")
			os.Exit(1)
		}
	}

	// Parse interval
	interval, err := time.ParseDuration(config.ValidateInterval)
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
	switch config.ValidateMode {
	case "one-off":
		if config.ValidateConfig != "" {
			// Validate new configuration against cluster with scope filtering
			var result *validators.ValidationResult
			var err error
			if configData != nil {
				// Use pre-read data for stdin
				result, err = registry.ValidateNewConfigWithScopeAndData(ctx, config.ValidateConfig, config.ValidateScope, configData)
			} else {
				result, err = registry.ValidateNewConfigWithScope(ctx, config.ValidateConfig, config.ValidateScope)
			}
			if err != nil {
				setupLog.Error(err, "validation failed")
				os.Exit(1)
			}

			// Format output based on mode
			if config.ValidateOutput == "ci" {
				output, err := registry.FormatCIOutput(*result)
				if err != nil {
					setupLog.Error(err, "failed to format CI output")
					os.Exit(1)
				}
				// Output to stderr for CI consumption
				fmt.Fprintf(os.Stderr, "%s\n", output)
				os.Exit(result.ExitCode)
			}
			// Regular output
			if result.ExitCode > 0 {
				setupLog.Error(nil, "validation failed",
					"total_errors", result.Summary.TotalErrors,
					"missing_refs", result.Summary.MissingRefs,
					"suggested_refs", result.Summary.SuggestedRefs)
				os.Exit(result.ExitCode)
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
		setupLog.Error(nil, "invalid validation mode", "mode", config.ValidateMode)
		os.Exit(1)
	}
}

// setupController configures and registers the validation controller with health checks
func setupController(mgr ctrl.Manager, registry *validators.ValidatorRegistry, scanInterval string) error {
	// Parse scan interval
	scanIntervalDuration, err := time.ParseDuration(scanInterval)
	if err != nil {
		return fmt.Errorf("invalid scan interval format: %w", err)
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
		return fmt.Errorf("unable to create controller: %w", err)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up health check: %w", err)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up ready check: %w", err)
	}

	return nil
}

func main() {
	config := registerFlags()

	// Handle one-off validation mode - read config once if using stdin
	var configData []byte
	if config.ValidateMode == "one-off" && config.ValidateConfig != "" {
		var err error
		if config.ValidateConfig == "-" {
			// Read stdin once and store for both syntax and full validation
			configData, err = io.ReadAll(os.Stdin)
			if err != nil {
				setupLog.Error(err, "failed to read from stdin")
				os.Exit(1)
			}
		}

		if err := validateConfigFileSyntax(config.ValidateConfig, configData); err != nil {
			setupLog.Error(err, "validation failed")
			os.Exit(1)
		}
		setupLog.Info("config file syntax validation passed")
		// Continue to cluster validation - don't return here
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: server.Options{
			BindAddress: config.MetricsAddr,
		},
		HealthProbeBindAddress: config.ProbeAddr,
		LeaderElection:         config.EnableLeaderElection,
		LeaderElectionID:       "kogaro.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Register metrics
	metrics.RegisterMetrics()

	// Initialize validators
	registry := setupValidators(mgr, config)

	// Handle validate command
	if config.ValidateMode != "" {
		runValidationMode(mgr, registry, config, configData)
		return
	}

	// Setup the controller
	if err := setupController(mgr, registry, config.ScanInterval); err != nil {
		setupLog.Error(err, "failed to setup controller")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

// validateConfigFileSyntax performs early validation with optional pre-read data
func validateConfigFileSyntax(configPath string, preReadData []byte) error {
	var configData []byte
	var err error

	if preReadData != nil {
		// Use pre-read data (from stdin)
		configData = preReadData
	} else if configPath == "-" {
		configData, err = io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}
	} else {
		// Read the config file
		configData, err = os.ReadFile(configPath) // nolint:gosec // Config file path is user-provided
		if err != nil {
			return fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Check for Helm template syntax early
	configStr := string(configData)
	if strings.Contains(configStr, "{{") && strings.Contains(configStr, "}}") {
		return fmt.Errorf("file appears to contain Helm templates. Please render the template first using 'helm template' and validate the resulting YAML")
	}

	return nil
}
