package diagnostics

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/topiaruss/kogaro/ui/pkg/graph"
)

// BuildFixPlan creates a dependency-sorted fix plan from an incident and its diagnostics.
func BuildFixPlan(fg *graph.FaultGraph, incident graph.Incident, diagnostics []DiagnosticResult) *FixPlan {
	plan := &FixPlan{
		IncidentID:  incident.ID,
		Namespace:   incident.Namespace,
		GeneratedAt: time.Now(),
	}

	// Build node lookup
	nodeMap := make(map[graph.NodeID]*graph.Node)
	for i := range fg.Nodes {
		nodeMap[fg.Nodes[i].ID] = &fg.Nodes[i]
	}

	// Build adjacency: which nodes depend on which (via edges)
	// dependsOn[A] = [B, C] means A depends on B and C (A's issues may resolve when B/C are fixed)
	dependsOn := make(map[graph.NodeID][]graph.NodeID)
	dependedBy := make(map[graph.NodeID][]graph.NodeID)
	for _, edge := range fg.Edges {
		// Source references/depends-on Target
		dependsOn[edge.Source] = append(dependsOn[edge.Source], edge.Target)
		dependedBy[edge.Target] = append(dependedBy[edge.Target], edge.Source)
	}

	// Collect all incident nodes
	incidentNodes := make(map[graph.NodeID]bool)
	for _, nid := range incident.AffectedNodes {
		incidentNodes[nid] = true
	}
	if incident.RootCauses != nil {
		for _, nid := range incident.RootCauses {
			incidentNodes[nid] = true
		}
	}

	// Build error lookup by node ID
	errorsByNode := make(map[graph.NodeID][]graph.ErrorDetail)
	for _, e := range incident.Errors {
		nid := graph.MakeNodeID(e.ResourceType, e.Namespace, e.ResourceName)
		errorsByNode[nid] = append(errorsByNode[nid], e)
	}

	// Build diagnostic lookup
	diagByKey := make(map[string][]DiagnosticResult)
	for _, d := range diagnostics {
		key := d.ErrorCode + "/" + d.Namespace + "/" + d.ResourceName
		diagByKey[key] = append(diagByKey[key], d)
	}

	// Topological sort: nodes with no broken dependencies come first (root causes)
	// Use Kahn's algorithm on the subgraph of incident nodes
	sorted := topoSort(incidentNodes, dependsOn, nodeMap)

	// Classify each node and build fix steps
	for i, nid := range sorted {
		node := nodeMap[nid]
		if node == nil {
			continue
		}

		errs := errorsByNode[nid]
		if len(errs) == 0 && !node.IsRootCause && node.Health != graph.HealthMissing {
			continue // skip nodes with no errors unless they're root causes
		}

		// Collect error codes
		codeSet := make(map[string]bool)
		var codes []string
		for _, e := range errs {
			if !codeSet[e.ErrorCode] {
				codeSet[e.ErrorCode] = true
				codes = append(codes, e.ErrorCode)
			}
		}

		// Determine if this will auto-resolve
		isRootCause := node.IsRootCause || node.Health == graph.HealthMissing || len(dependsOn[nid]) == 0
		willAutoResolve := !isRootCause && allDepsAreBroken(nid, dependsOn, nodeMap, incidentNodes) && allErrorsAreSymptoms(codes)

		// Collect dependencies that are in the incident
		var deps []string
		for _, dep := range dependsOn[nid] {
			if incidentNodes[dep] {
				deps = append(deps, string(dep))
			}
		}

		// Collect diagnostics for this node's errors
		var nodeDiags []DiagnosticResult
		for _, e := range errs {
			key := e.ErrorCode + "/" + e.Namespace + "/" + e.ResourceName
			nodeDiags = append(nodeDiags, diagByKey[key]...)
		}

		// Build remediation text
		remediation := buildRemediation(errs, node, willAutoResolve, deps)

		commands := generateCommands(node, errs, nodeDiags)

		step := FixStep{
			Order:           i + 1,
			NodeID:          string(nid),
			ResourceKind:    node.Kind,
			ResourceName:    node.Name,
			Namespace:       node.Namespace,
			ErrorCodes:      codes,
			IsRootCause:     isRootCause,
			WillAutoResolve: willAutoResolve,
			DependsOn:       deps,
			Remediation:     remediation,
			Commands:        commands,
			Diagnostics:     nodeDiags,
		}
		plan.Steps = append(plan.Steps, step)
	}

	// Re-sort: root causes first, then symptoms
	sort.SliceStable(plan.Steps, func(i, j int) bool {
		if plan.Steps[i].IsRootCause != plan.Steps[j].IsRootCause {
			return plan.Steps[i].IsRootCause
		}
		if plan.Steps[i].WillAutoResolve != plan.Steps[j].WillAutoResolve {
			return !plan.Steps[i].WillAutoResolve
		}
		return false
	})

	// Renumber after sort
	for i := range plan.Steps {
		plan.Steps[i].Order = i + 1
	}

	return plan
}

// topoSort performs a topological sort of incident nodes.
// Nodes with no dependencies (root causes) come first.
func topoSort(nodes map[graph.NodeID]bool, dependsOn map[graph.NodeID][]graph.NodeID, nodeMap map[graph.NodeID]*graph.Node) []graph.NodeID {
	// Count in-degree (dependencies within the incident subgraph)
	inDegree := make(map[graph.NodeID]int)
	for nid := range nodes {
		inDegree[nid] = 0
	}
	for nid := range nodes {
		for _, dep := range dependsOn[nid] {
			if nodes[dep] {
				inDegree[nid]++
			}
		}
	}

	// Start with nodes that have no dependencies
	var queue []graph.NodeID
	for nid := range nodes {
		if inDegree[nid] == 0 {
			queue = append(queue, nid)
		}
	}

	var sorted []graph.NodeID
	for len(queue) > 0 {
		nid := queue[0]
		queue = queue[1:]
		sorted = append(sorted, nid)

		// Find nodes that depend on this one
		for other := range nodes {
			for _, dep := range dependsOn[other] {
				if dep == nid {
					inDegree[other]--
					if inDegree[other] == 0 {
						queue = append(queue, other)
					}
				}
			}
		}
	}

	// Add any remaining nodes (cycles) at the end
	for nid := range nodes {
		found := false
		for _, s := range sorted {
			if s == nid {
				found = true
				break
			}
		}
		if !found {
			sorted = append(sorted, nid)
		}
	}

	return sorted
}

// allDepsAreBroken checks if all dependencies of a node are broken/missing.
func allDepsAreBroken(nid graph.NodeID, dependsOn map[graph.NodeID][]graph.NodeID, nodeMap map[graph.NodeID]*graph.Node, incidentNodes map[graph.NodeID]bool) bool {
	deps := dependsOn[nid]
	if len(deps) == 0 {
		return false
	}
	for _, dep := range deps {
		if !incidentNodes[dep] {
			continue
		}
		node := nodeMap[dep]
		if node != nil && (node.Health == graph.HealthBroken || node.Health == graph.HealthMissing) {
			return true
		}
	}
	return false
}

// allErrorsAreSymptoms returns true if all error codes are typically symptoms of upstream issues.
func allErrorsAreSymptoms(codes []string) bool {
	for _, code := range codes {
		// NET-002 (no endpoints) and NET-009 (ingress no backend) are often symptoms
		if strings.HasPrefix(code, "KOGARO-NET-002") || strings.HasPrefix(code, "KOGARO-NET-009") {
			continue
		}
		// Everything else requires direct action
		return false
	}
	return true
}

// containerName extracts the first container name from diagnostic findings, or falls back to the resource name.
func containerName(node *graph.Node, diags []DiagnosticResult) string {
	for _, d := range diags {
		for _, f := range d.Findings {
			if name, ok := f.Details["container"]; ok && name != "" {
				return name
			}
		}
	}
	// For workloads, the container name often matches the resource name
	return node.Name
}

// generateCommands creates kubectl commands for investigating and fixing a step.
func generateCommands(node *graph.Node, errs []graph.ErrorDetail, diags []DiagnosticResult) []FixCommand {
	var cmds []FixCommand
	kind := strings.ToLower(node.Kind)
	ns := node.Namespace
	cName := containerName(node, diags)

	// Always add describe as first command
	if ns != "" {
		cmds = append(cmds, FixCommand{
			Label:   fmt.Sprintf("Describe %s", node.Kind),
			Command: fmt.Sprintf("kubectl describe %s %s -n %s", kind, node.Name, ns),
			Safe:    true,
		})
	} else {
		cmds = append(cmds, FixCommand{
			Label:   fmt.Sprintf("Describe %s", node.Kind),
			Command: fmt.Sprintf("kubectl describe %s %s", kind, node.Name),
			Safe:    true,
		})
	}

	// Missing resources: suggest get to confirm, then a stub create
	if node.Health == graph.HealthMissing {
		if ns != "" {
			cmds = append(cmds, FixCommand{
				Label:   fmt.Sprintf("Check if %s exists", node.Kind),
				Command: fmt.Sprintf("kubectl get %s %s -n %s", kind, node.Name, ns),
				Safe:    true,
			})
		}
		// Don't generate a create command — we can't know the spec
		return cmds
	}

	// Error-code-specific commands
	codeSet := make(map[string]bool)
	for _, e := range errs {
		codeSet[e.ErrorCode] = true
	}

	// Service selector issues
	if codeSet["KOGARO-NET-001"] || codeSet["KOGARO-NET-002"] {
		// Get the selector from diagnostics
		selector := ""
		for _, d := range diags {
			for _, f := range d.Findings {
				if s, ok := f.Details["selector"]; ok && s != "" {
					selector = s
				}
			}
		}
		if selector != "" {
			cmds = append(cmds, FixCommand{
				Label:   "Find pods matching selector",
				Command: fmt.Sprintf("kubectl get pods -n %s -l %s", ns, selector),
				Safe:    true,
			})
			cmds = append(cmds, FixCommand{
				Label:   "Show all pod labels",
				Command: fmt.Sprintf("kubectl get pods -n %s --show-labels", ns),
				Safe:    true,
			})
		}
		cmds = append(cmds, FixCommand{
			Label:   "Check endpoints",
			Command: fmt.Sprintf("kubectl get endpointslices -n %s -l kubernetes.io/service-name=%s", ns, node.Name),
			Safe:    true,
		})
	}

	// Ingress issues
	if codeSet["KOGARO-NET-009"] {
		cmds = append(cmds, FixCommand{
			Label:   "Show ingress details",
			Command: fmt.Sprintf("kubectl get ingress %s -n %s -o yaml", node.Name, ns),
			Safe:    true,
		})
	}

	// Security issues — show current state and suggest patches
	if codeSet["KOGARO-SEC-002"] || codeSet["KOGARO-SEC-010"] {
		cmds = append(cmds, FixCommand{
			Label:   "Show pod-level security context",
			Command: fmt.Sprintf("kubectl get %s %s -n %s -o jsonpath='{.spec.template.spec.securityContext}'", kind, node.Name, ns),
			Safe:    true,
		})
		cmds = append(cmds, FixCommand{
			Label:   "Show container-level security context",
			Command: fmt.Sprintf("kubectl get %s %s -n %s -o jsonpath='{.spec.template.spec.containers[0].securityContext}'", kind, node.Name, ns),
			Safe:    true,
		})
		if kind == "deployment" || kind == "statefulset" {
			if codeSet["KOGARO-SEC-002"] {
				// Pod allows root — add runAsNonRoot
				cmds = append(cmds, FixCommand{
					Label:       "Set runAsNonRoot: true (prevents root)",
					Command:     fmt.Sprintf(`kubectl patch %s %s -n %s --type=strategic -p '{"spec":{"template":{"spec":{"securityContext":{"runAsNonRoot":true}}}}}'`, kind, node.Name, ns),
					Destructive: true,
				})
			}
			if codeSet["KOGARO-SEC-010"] {
				// Missing container security context — add restricted defaults
				cmds = append(cmds, FixCommand{
					Label:       fmt.Sprintf("Add container security context for %s (restricted defaults)", cName),
					Command:     fmt.Sprintf(`kubectl patch %s %s -n %s --type=strategic -p '{"spec":{"template":{"spec":{"containers":[{"name":"%s","securityContext":{"allowPrivilegeEscalation":false,"readOnlyRootFilesystem":true,"capabilities":{"drop":["ALL"]}}}]}}}}'`, kind, node.Name, ns, cName),
					Destructive: true,
				})
			}
		}
	}

	// RES-UNKNOWN — QoS optimization
	if codeSet["KOGARO-RES-UNKNOWN"] {
		cmds = append(cmds, FixCommand{
			Label:   "Show current resource requests/limits",
			Command: fmt.Sprintf("kubectl get %s %s -n %s -o jsonpath='{.spec.template.spec.containers[*].resources}'", kind, node.Name, ns),
			Safe:    true,
		})
		if kind == "deployment" || kind == "statefulset" {
			cmds = append(cmds, FixCommand{
				Label:       fmt.Sprintf("Set Guaranteed QoS for %s (requests = limits)", cName),
				Command:     fmt.Sprintf(`kubectl patch %s %s -n %s --type=strategic -p '{"spec":{"template":{"spec":{"containers":[{"name":"%s","resources":{"requests":{"memory":"256Mi","cpu":"200m"},"limits":{"memory":"256Mi","cpu":"200m"}}}]}}}}'`, kind, node.Name, ns, cName),
				Destructive: true,
			})
		}
	}

	// Resource limits — show current and suggest patch
	if codeSet["KOGARO-RES-002"] || codeSet["KOGARO-RES-005"] {
		cmds = append(cmds, FixCommand{
			Label:   "Show current resource requests/limits",
			Command: fmt.Sprintf("kubectl get %s %s -n %s -o jsonpath='{.spec.template.spec.containers[*].resources}'", kind, node.Name, ns),
			Safe:    true,
		})
		// Suggest a patch for common defaults — use strategic merge with container name
		if kind == "deployment" || kind == "statefulset" {
			cmds = append(cmds, FixCommand{
				Label:       fmt.Sprintf("Set default resource requests for %s (128Mi/100m)", cName),
				Command:     fmt.Sprintf(`kubectl patch %s %s -n %s --type=strategic -p '{"spec":{"template":{"spec":{"containers":[{"name":"%s","resources":{"requests":{"memory":"128Mi","cpu":"100m"},"limits":{"memory":"256Mi","cpu":"200m"}}}]}}}}'`, kind, node.Name, ns, cName),
				Destructive: true,
			})
		}
	}

	// Events — always useful
	if ns != "" {
		cmds = append(cmds, FixCommand{
			Label:   "Recent events",
			Command: fmt.Sprintf("kubectl get events -n %s --sort-by=.lastTimestamp --field-selector involvedObject.name=%s", ns, node.Name),
			Safe:    true,
		})
	}

	return cmds
}

func buildRemediation(errs []graph.ErrorDetail, node *graph.Node, willAutoResolve bool, deps []string) string {
	if willAutoResolve && len(deps) > 0 {
		return "Will likely resolve automatically when upstream dependencies are fixed"
	}

	if node.Health == graph.HealthMissing {
		return "Create the missing resource"
	}

	// Collect unique remediation hints
	seen := make(map[string]bool)
	var hints []string
	for _, e := range errs {
		if e.RemediationHint != "" && !seen[e.RemediationHint] {
			seen[e.RemediationHint] = true
			hints = append(hints, e.RemediationHint)
		}
	}

	if len(hints) == 1 {
		return hints[0]
	}
	if len(hints) > 1 {
		return strings.Join(hints, ". ")
	}

	return "Run the commands below to investigate and apply fixes"
}
