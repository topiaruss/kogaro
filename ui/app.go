package main

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/funcr"
	"github.com/topiaruss/kogaro/internal/validators"
	"github.com/topiaruss/kogaro/ui/pkg/datasource"
	"github.com/topiaruss/kogaro/ui/pkg/diagnostics"
	"github.com/topiaruss/kogaro/ui/pkg/graph"
	"github.com/topiaruss/kogaro/ui/pkg/history"
	"github.com/topiaruss/kogaro/ui/pkg/kubecontext"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ScanProgress is emitted as a Wails event during scanning.
type ScanProgress struct {
	Step       int    `json:"step"`
	TotalSteps int    `json:"totalSteps"`
	Validator  string `json:"validator"`
	Status     string `json:"status"` // "running", "done", "error"
	Errors     int    `json:"errors"`
	Elapsed    string `json:"elapsed"`
}

// App is the Wails application with bound methods.
type App struct {
	ctx        context.Context
	kubeMgr    *kubecontext.Manager
	k8sClient  client.Client
	dataSource datasource.DataSource
	lastGraph  *graph.FaultGraph
	scheme     *runtime.Scheme
	registry   *validators.ValidatorRegistry
	history    *history.Store
	diagRunner *diagnostics.Runner
}

// NewApp creates a new App instance.
func NewApp() *App {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	return &App{
		kubeMgr: kubecontext.NewManager(),
		scheme:  s,
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Open history database
	dbPath, err := history.DefaultPath()
	if err != nil {
		log.Printf("Warning: could not determine history path: %v", err)
	} else {
		store, err := history.Open(dbPath)
		if err != nil {
			log.Printf("Warning: could not open history database: %v", err)
		} else {
			a.history = store
		}
	}

	if err := a.initClient(""); err != nil {
		log.Printf("Warning: could not initialize Kubernetes client: %v", err)
	}
}

func (a *App) shutdown(_ context.Context) {
	if a.history != nil {
		a.history.Close()
	}
}

func (a *App) initClient(contextName string) error {
	restCfg, err := a.kubeMgr.GetRestConfig(contextName)
	if err != nil {
		return fmt.Errorf("getting rest config: %w", err)
	}

	c, err := client.New(restCfg, client.Options{Scheme: a.scheme})
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}
	a.k8sClient = c

	a.registry = a.setupRegistry(c)
	a.dataSource = datasource.NewKogaroDataSource(c, a.registry)
	a.diagRunner = diagnostics.NewRunner(c, a.history)
	return nil
}

func (a *App) setupRegistry(c client.Client) *validators.ValidatorRegistry {
	logger := discardLogger()
	registry := validators.NewValidatorRegistry(logger, c)

	registry.Register(validators.NewReferenceValidator(c, logger, validators.ValidationConfig{
		EnableIngressValidation:        true,
		EnableConfigMapValidation:      true,
		EnableSecretValidation:         true,
		EnablePVCValidation:            true,
		EnableServiceAccountValidation: false,
	}))

	registry.Register(validators.NewResourceLimitsValidator(c, logger, validators.ResourceLimitsConfig{
		EnableMissingRequestsValidation: true,
		EnableMissingLimitsValidation:   true,
		EnableQoSValidation:             true,
	}))

	registry.Register(validators.NewSecurityValidator(c, logger, validators.SecurityConfig{
		EnableRootUserValidation:        true,
		EnableSecurityContextValidation: true,
		EnableServiceAccountValidation:  true,
		EnableNetworkPolicyValidation:   true,
	}))

	registry.Register(validators.NewNetworkingValidator(c, logger, validators.NetworkingConfig{
		EnableServiceValidation:       true,
		EnableNetworkPolicyValidation: true,
		EnableIngressValidation:       true,
	}))

	return registry
}

// Scan runs all validators one-by-one with progress events and returns a FaultGraph.
func (a *App) Scan() (*graph.FaultGraph, error) {
	if a.registry == nil || a.k8sClient == nil {
		return nil, fmt.Errorf("no data source available - check kubeconfig")
	}

	allValidators := a.registry.GetValidators()
	totalSteps := len(allValidators) + 1 // +1 for graph building
	var allErrors []validators.ValidationError
	scanStart := time.Now()

	validatorTimeout := 30 * time.Second

	logReceiver := &noopLogReceiver{}

	for i, v := range allValidators {
		step := i + 1
		name := v.GetValidationType()

		// Set log receiver (required before ValidateCluster)
		v.SetLogReceiver(logReceiver)

		// Emit "running" event
		wailsRuntime.EventsEmit(a.ctx, "scan:progress", ScanProgress{
			Step:       step,
			TotalSteps: totalSteps,
			Validator:  name,
			Status:     "running",
			Errors:     len(allErrors),
			Elapsed:    time.Since(scanStart).Round(time.Millisecond).String(),
		})

		// Run this validator with a timeout
		type result struct {
			err error
		}
		ch := make(chan result, 1)
		timeoutCtx, cancel := context.WithTimeout(a.ctx, validatorTimeout)

		go func() {
			ch <- result{err: v.ValidateCluster(timeoutCtx)}
		}()

		var validatorErr error
		select {
		case res := <-ch:
			validatorErr = res.err
		case <-timeoutCtx.Done():
			validatorErr = fmt.Errorf("timed out after %s", validatorTimeout)
		}
		cancel()

		errs := v.GetLastValidationErrors()
		allErrors = append(allErrors, errs...)

		status := "done"
		if validatorErr != nil {
			status = "error"
		}

		// Emit completion event for this validator
		wailsRuntime.EventsEmit(a.ctx, "scan:progress", ScanProgress{
			Step:       step,
			TotalSteps: totalSteps,
			Validator:  name,
			Status:     status,
			Errors:     len(allErrors),
			Elapsed:    time.Since(scanStart).Round(time.Millisecond).String(),
		})

		// For connection errors, abort early. For validator-specific errors, continue.
		if validatorErr != nil {
			msg := validatorErr.Error()
			if strings.Contains(msg, "connection refused") ||
				strings.Contains(msg, "no such host") ||
				strings.Contains(msg, "Unauthorized") {
				return nil, fmt.Errorf("cluster unreachable: %s", friendlyError(validatorErr))
			}
			// Non-fatal: log and continue with remaining validators
		}
	}

	// Graph building step
	wailsRuntime.EventsEmit(a.ctx, "scan:progress", ScanProgress{
		Step:       totalSteps,
		TotalSteps: totalSteps,
		Validator:  "graph_builder",
		Status:     "running",
		Errors:     len(allErrors),
		Elapsed:    time.Since(scanStart).Round(time.Millisecond).String(),
	})

	builder := graph.NewBuilder(a.k8sClient, 2)
	fg, err := builder.Build(a.ctx, allErrors)
	if err != nil {
		return nil, fmt.Errorf("building graph: %w", err)
	}

	wailsRuntime.EventsEmit(a.ctx, "scan:progress", ScanProgress{
		Step:       totalSteps,
		TotalSteps: totalSteps,
		Validator:  "graph_builder",
		Status:     "done",
		Errors:     len(allErrors),
		Elapsed:    time.Since(scanStart).Round(time.Millisecond).String(),
	})

	a.lastGraph = fg

	// Dump JSON for dev diagnostics
	if err := history.DumpJSON(fg); err != nil {
		log.Printf("Warning: could not dump scan JSON: %v", err)
	}

	// Save to history database
	if a.history != nil {
		ctxName, _ := a.kubeMgr.GetCurrentContext()
		if _, err := a.history.SaveScan(ctxName, fg); err != nil {
			log.Printf("Warning: could not save scan to history: %v", err)
		}
	}

	return fg, nil
}

// GetNodeDetail returns full evidence for a node.
func (a *App) GetNodeDetail(nodeID string) (*graph.NodeDetailResponse, error) {
	if a.lastGraph == nil {
		return nil, fmt.Errorf("no scan results available")
	}

	nid := graph.NodeID(nodeID)
	var node *graph.Node
	for i := range a.lastGraph.Nodes {
		if a.lastGraph.Nodes[i].ID == nid {
			node = &a.lastGraph.Nodes[i]
			break
		}
	}
	if node == nil {
		return nil, fmt.Errorf("node %s not found", nodeID)
	}

	resp := &graph.NodeDetailResponse{Node: *node}
	for _, e := range a.lastGraph.Edges {
		if e.Target == nid {
			resp.IncomingEdges = append(resp.IncomingEdges, e)
		}
		if e.Source == nid {
			resp.OutgoingEdges = append(resp.OutgoingEdges, e)
		}
	}
	for _, inc := range a.lastGraph.Incidents {
		for _, affected := range inc.AffectedNodes {
			if affected == nid {
				resp.Errors = append(resp.Errors, inc.Errors...)
			}
		}
	}

	return resp, nil
}

// ExpandNode adds one more hop around a node.
func (a *App) ExpandNode(nodeID string) (*graph.FaultGraph, error) {
	return a.Scan()
}

// GetKubeContexts returns available kubeconfig contexts.
func (a *App) GetKubeContexts() ([]string, error) {
	return a.kubeMgr.GetContexts()
}

// GetCurrentContext returns the active kubeconfig context.
func (a *App) GetCurrentContext() (string, error) {
	return a.kubeMgr.GetCurrentContext()
}

// SwitchContext changes the kubeconfig context and reinitializes.
func (a *App) SwitchContext(contextName string) error {
	if err := a.initClient(contextName); err != nil {
		return fmt.Errorf("switching to context %q: %w", contextName, err)
	}
	return nil
}

// GetScanHistory returns recent scans for the current context.
func (a *App) GetScanHistory(limit int) ([]history.ScanRecord, error) {
	if a.history == nil {
		return nil, fmt.Errorf("history database not available")
	}
	ctxName, _ := a.kubeMgr.GetCurrentContext()
	return a.history.GetScanHistory(ctxName, limit)
}

// GetScanDiff compares two scans and returns new/fixed errors.
func (a *App) GetScanDiff(olderScanID, newerScanID uint) (*history.ScanDiff, error) {
	if a.history == nil {
		return nil, fmt.Errorf("history database not available")
	}
	return a.history.DiffScans(olderScanID, newerScanID)
}

// GetFixPlan runs diagnostics and generates a dependency-sorted fix plan.
func (a *App) GetFixPlan(incidentID string) (*diagnostics.FixPlan, error) {
	if a.lastGraph == nil {
		return nil, fmt.Errorf("no scan results available")
	}
	if a.diagRunner == nil {
		return nil, fmt.Errorf("diagnostic runner not available")
	}

	var incident *graph.Incident
	for i := range a.lastGraph.Incidents {
		if a.lastGraph.Incidents[i].ID == incidentID {
			incident = &a.lastGraph.Incidents[i]
			break
		}
	}
	if incident == nil {
		return nil, fmt.Errorf("incident %s not found", incidentID)
	}

	// Run diagnostics
	diagResults, err := a.diagRunner.RunForIncident(a.ctx, *incident)
	if err != nil {
		return nil, fmt.Errorf("running diagnostics: %w", err)
	}

	// Build fix plan
	plan := diagnostics.BuildFixPlan(a.lastGraph, *incident, diagResults)
	return plan, nil
}

// RunCommandResult is the output of running a kubectl command with analysis.
type RunCommandResult struct {
	Success    bool                    `json:"success"`
	Output     string                  `json:"output"`
	Error      string                  `json:"error,omitempty"`
	Suggestion *diagnostics.Suggestion `json:"suggestion,omitempty"`
}

// parseShellArgs splits a command string into arguments, respecting single and double quotes.
func parseShellArgs(s string) []string {
	var args []string
	var current strings.Builder
	inSingle := false
	inDouble := false

	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\'' && !inDouble:
			inSingle = !inSingle
		case c == '"' && !inSingle:
			inDouble = !inDouble
		case c == ' ' && !inSingle && !inDouble:
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(c)
		}
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args
}

// RunCommand executes a kubectl command, analyzes the output, and suggests next steps.
func (a *App) RunCommand(command string, errorCodes []string) (*RunCommandResult, error) {
	// Safety: only allow kubectl commands
	trimmed := strings.TrimSpace(command)
	if !strings.HasPrefix(trimmed, "kubectl ") {
		return &RunCommandResult{Success: false, Error: "only kubectl commands are allowed"}, nil
	}

	// Parse into args, respecting quotes (important for JSON patches)
	allArgs := parseShellArgs(trimmed)
	args := allArgs[1:] // skip "kubectl"

	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	output, err := cmd.CombinedOutput()
	success := err == nil
	outputStr := string(output)

	result := &RunCommandResult{
		Success: success,
		Output:  outputStr,
	}
	if err != nil {
		result.Error = err.Error()
	}

	// Analyze and suggest next steps
	result.Suggestion = diagnostics.AnalyzeOutput(command, outputStr, success, errorCodes)

	return result, nil
}

// ApplyFix is an alias for RunCommand for backwards compatibility.
func (a *App) ApplyFix(command string) (*RunCommandResult, error) {
	return a.RunCommand(command, nil)
}

// BuildInfo holds version/build metadata.
type BuildInfo struct {
	Commit    string `json:"commit"`
	BuildTime string `json:"buildTime"`
}

// GetBuildInfo returns the build commit and time.
func (a *App) GetBuildInfo() *BuildInfo {
	return &BuildInfo{
		Commit:    buildCommit,
		BuildTime: buildTime,
	}
}

func discardLogger() logr.Logger {
	return funcr.New(func(prefix, args string) {}, funcr.Options{})
}

// noopLogReceiver implements validators.LogReceiver as a no-op.
type noopLogReceiver struct{}

func (n *noopLogReceiver) LogValidationError(_ string, _ validators.ValidationError) {}

func friendlyError(err error) string {
	msg := err.Error()
	if strings.Contains(msg, "connection refused") {
		// Extract the host:port from the error
		if idx := strings.Index(msg, "dial tcp"); idx >= 0 {
			parts := strings.Fields(msg[idx:])
			addr := ""
			if len(parts) >= 3 {
				addr = strings.TrimSuffix(parts[2], ":")
			}
			if addr != "" {
				return fmt.Sprintf("cannot connect to cluster at %s - is it running?", addr)
			}
		}
		return "cannot connect to cluster - is it running?"
	}
	if strings.Contains(msg, "no such host") {
		return "cluster DNS cannot be resolved - check your kubeconfig"
	}
	if strings.Contains(msg, "certificate") {
		return "TLS certificate error - check cluster certificates"
	}
	if strings.Contains(msg, "Unauthorized") || strings.Contains(msg, "forbidden") {
		return "authentication failed - check cluster credentials"
	}
	return msg
}
