package diagnostics

import (
	"context"
	"strings"

	"github.com/topiaruss/kogaro/ui/pkg/graph"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RunbookFunc is the signature for a diagnostic runbook.
type RunbookFunc func(ctx context.Context, c client.Client, errDetail graph.ErrorDetail) ([]DiagnosticFinding, error)

// runbook registry maps error codes to diagnostic functions.
var runbookRegistry = map[string]RunbookFunc{}

func init() {
	// NET family
	runbookRegistry["KOGARO-NET-001"] = diagnoseServiceSelectorMismatch
	runbookRegistry["KOGARO-NET-002"] = diagnoseServiceNoEndpoints
	runbookRegistry["KOGARO-NET-009"] = diagnoseIngressNoBackendPods

	// REF family — all share the same runbook
	for _, code := range []string{
		"KOGARO-REF-001", "KOGARO-REF-002", "KOGARO-REF-003",
		"KOGARO-REF-004", "KOGARO-REF-005", "KOGARO-REF-006",
		"KOGARO-REF-007", "KOGARO-REF-008", "KOGARO-REF-009",
		"KOGARO-REF-010", "KOGARO-REF-011",
	} {
		runbookRegistry[code] = diagnoseDanglingReference
	}

	// SEC family
	runbookRegistry["KOGARO-SEC-002"] = diagnoseSecurityContext
	runbookRegistry["KOGARO-SEC-010"] = diagnoseSecurityContext

	// RES family
	runbookRegistry["KOGARO-RES-002"] = diagnoseResourceLimits
	runbookRegistry["KOGARO-RES-005"] = diagnoseResourceLimits
	runbookRegistry["KOGARO-RES-UNKNOWN"] = diagnoseResourceLimits
}

// GetRunbook returns the runbook for an error code, or nil.
func GetRunbook(errorCode string) RunbookFunc {
	if fn, ok := runbookRegistry[errorCode]; ok {
		return fn
	}
	// Try prefix match for codes we haven't explicitly registered
	for prefix, fn := range runbookRegistry {
		if strings.HasPrefix(errorCode, prefix) {
			return fn
		}
	}
	return nil
}
