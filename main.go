package main

import (
	"flag"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/russ/kogaro/internal/controllers"
	"github.com/russ/kogaro/internal/validators"
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
	
	// Validation flags
	var enableIngressValidation bool
	var enableConfigMapValidation bool
	var enableSecretValidation bool
	var enablePVCValidation bool
	var enableServiceAccountValidation bool

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.DurationVar(&scanInterval, "scan-interval", 5*time.Minute, "Interval between cluster scans for reference validation")
	
	// Validation configuration flags
	flag.BoolVar(&enableIngressValidation, "enable-ingress-validation", true, "Enable validation of Ingress references (IngressClass, Services)")
	flag.BoolVar(&enableConfigMapValidation, "enable-configmap-validation", true, "Enable validation of ConfigMap references in Pods")
	flag.BoolVar(&enableSecretValidation, "enable-secret-validation", true, "Enable validation of Secret references (volumes, env, TLS)")
	flag.BoolVar(&enablePVCValidation, "enable-pvc-validation", true, "Enable validation of PVC and StorageClass references")
	flag.BoolVar(&enableServiceAccountValidation, "enable-serviceaccount-validation", false, "Enable validation of ServiceAccount references (may be noisy)")
	
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

	// Initialize the reference validator with configuration
	validationConfig := validators.ValidationConfig{
		EnableIngressValidation:        enableIngressValidation,
		EnableConfigMapValidation:      enableConfigMapValidation,
		EnableSecretValidation:         enableSecretValidation,
		EnablePVCValidation:           enablePVCValidation,
		EnableServiceAccountValidation: enableServiceAccountValidation,
	}
	validator := validators.NewReferenceValidator(mgr.GetClient(), setupLog, validationConfig)

	// Setup the reference validation controller
	if err = (&controllers.ValidationController{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		Log:          ctrl.Log.WithName("controllers").WithName("ValidationController"),
		Validator:    validator,
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