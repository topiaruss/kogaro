package diagnostics

import (
	"time"
)

// DiagnosticFinding is a single piece of evidence gathered from the cluster.
type DiagnosticFinding struct {
	Category string            `json:"category"` // "pod_status", "events", "logs", "endpoints", "labels", "ports"
	Summary  string            `json:"summary"`  // human-readable one-liner
	Details  map[string]string `json:"details"`  // structured k-v pairs
	Raw      string            `json:"raw,omitempty"`
}

// DiagnosticResult is the output of running diagnostics for one error.
type DiagnosticResult struct {
	ErrorCode    string              `json:"errorCode"`
	ResourceType string              `json:"resourceType"`
	ResourceName string              `json:"resourceName"`
	Namespace    string              `json:"namespace"`
	Findings     []DiagnosticFinding `json:"findings"`
	RanAt        time.Time           `json:"ranAt"`
}

// FixCommand is a kubectl command that can be copied or applied.
type FixCommand struct {
	Label       string `json:"label"`       // human-readable description
	Command     string `json:"command"`     // full kubectl command
	Safe        bool   `json:"safe"`        // true if this is a read-only/describe command
	Destructive bool   `json:"destructive"` // true if this modifies cluster state
}

// FixStep is one entry in the dependency-sorted fix plan.
type FixStep struct {
	Order           int                `json:"order"`
	NodeID          string             `json:"nodeId"`
	ResourceKind    string             `json:"resourceKind"`
	ResourceName    string             `json:"resourceName"`
	Namespace       string             `json:"namespace"`
	ErrorCodes      []string           `json:"errorCodes"`
	IsRootCause     bool               `json:"isRootCause"`
	WillAutoResolve bool               `json:"willAutoResolve"`
	DependsOn       []string           `json:"dependsOn"` // nodeIDs that must be fixed first
	Remediation     string             `json:"remediation"`
	Commands        []FixCommand       `json:"commands"`
	Diagnostics     []DiagnosticResult `json:"diagnostics"`
}

// FixPlan is the top-level response for the fix plan view.
type FixPlan struct {
	IncidentID  string    `json:"incidentId"`
	Namespace   string    `json:"namespace"`
	Steps       []FixStep `json:"steps"`
	GeneratedAt time.Time `json:"generatedAt"`
}
