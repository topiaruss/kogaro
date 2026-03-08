package diagnostics

import (
	"fmt"
	"strings"
)

// Suggestion is a next-step recommendation after running a command.
type Suggestion struct {
	Insight  string       `json:"insight"`  // what we learned from the output
	NextCmds []FixCommand `json:"nextCmds"` // suggested follow-up commands
}

// AnalyzeOutput looks at the output of a kubectl command and suggests next steps.
func AnalyzeOutput(command string, output string, success bool, errorCodes []string) *Suggestion {
	s := &Suggestion{}

	if !success {
		return analyzeError(command, output, s)
	}

	// Route based on command type
	switch {
	case strings.Contains(command, "describe "):
		return analyzeDescribe(command, output, errorCodes, s)
	case strings.Contains(command, "get pods"):
		return analyzeGetPods(command, output, s)
	case strings.Contains(command, "get endpointslices") || strings.Contains(command, "get endpoints"):
		return analyzeEndpoints(command, output, s)
	case strings.Contains(command, "get events"):
		return analyzeEvents(command, output, s)
	case strings.Contains(command, "-o jsonpath"):
		return analyzeJsonpath(command, output, s)
	default:
		s.Insight = "Command completed successfully"
		return s
	}
}

func analyzeError(command, output string, s *Suggestion) *Suggestion {
	ns := extractFlag(command, "-n")

	if strings.Contains(output, "NotFound") || strings.Contains(output, "not found") {
		resource := extractResourceFromCmd(command)
		s.Insight = fmt.Sprintf("Resource %s does not exist — this confirms it's missing from the cluster", resource)

		if ns != "" {
			s.NextCmds = append(s.NextCmds, FixCommand{
				Label:   "List similar resources in namespace",
				Command: fmt.Sprintf("kubectl get all -n %s", ns),
				Safe:    true,
			})
		}
		return s
	}

	if strings.Contains(output, "forbidden") || strings.Contains(output, "Forbidden") {
		s.Insight = "Permission denied — the current kubeconfig user lacks RBAC access to this resource"
		return s
	}

	if strings.Contains(output, "runAsNonRoot") && strings.Contains(output, "non-numeric user") {
		s.Insight = "runAsNonRoot failed: image uses a non-numeric USER. Add explicit runAsUser with the correct UID for this image"
		return s
	}

	if strings.Contains(output, "Read-only file system") || strings.Contains(output, "read-only file system") {
		s.Insight = "readOnlyRootFilesystem is blocking writes. This image needs a writable filesystem (e.g. nginx cache, postgres data). Remove readOnlyRootFilesystem or add emptyDir volume mounts for writable paths"
		return s
	}

	s.Insight = fmt.Sprintf("Command failed: %s", strings.TrimSpace(output))
	return s
}

func analyzeDescribe(command, output string, errorCodes []string, s *Suggestion) *Suggestion {
	ns := extractFlag(command, "-n")
	lines := strings.Split(output, "\n")
	var warnings []string
	var insights []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for missing/empty image
		if strings.Contains(trimmed, "Image:") && (strings.HasSuffix(trimmed, "Image:") || strings.Contains(trimmed, "<none>")) {
			insights = append(insights, "Container image is empty — pod cannot start. Restore the image with a patch")
		}

		// Check for Waiting pods — only flag if count > 0
		if strings.Contains(trimmed, "Pods Status:") {
			// Parse "X Running / Y Waiting" — flag only if waiting > 0 and running == 0
			if strings.Contains(trimmed, "0 Running") && !strings.Contains(trimmed, "0 Waiting") {
				insights = append(insights, "No pods are running — check pod events")
			}
		}

		// Check for common issues in describe output
		if strings.Contains(trimmed, "Warning") && strings.Contains(trimmed, "FailedScheduling") {
			insights = append(insights, "Pod cannot be scheduled — check node resources and taints")
		}
		if strings.Contains(trimmed, "CrashLoopBackOff") {
			insights = append(insights, "Container is crash-looping — check logs for the root cause")
			name := extractResourceFromCmd(command)
			if ns != "" {
				s.NextCmds = append(s.NextCmds, FixCommand{
					Label:   "View container logs",
					Command: fmt.Sprintf("kubectl logs %s -n %s --tail=50", name, ns),
					Safe:    true,
				})
				s.NextCmds = append(s.NextCmds, FixCommand{
					Label:   "View previous container logs",
					Command: fmt.Sprintf("kubectl logs %s -n %s --previous --tail=50", name, ns),
					Safe:    true,
				})
			}
		}
		if strings.Contains(trimmed, "ImagePullBackOff") || strings.Contains(trimmed, "ErrImagePull") {
			insights = append(insights, "Container image cannot be pulled — check image name and registry access")
		}
		if strings.Contains(trimmed, "Pending") && strings.Contains(trimmed, "Status:") {
			insights = append(insights, "Resource is in Pending state")
		}
		if strings.Contains(trimmed, "Selector:") && strings.Contains(output, "Endpoints") {
			// Service describe — extract selector and endpoints info
			if strings.Contains(output, "Endpoints:                    <none>") || strings.Contains(output, "Endpoints:         <none>") {
				insights = append(insights, "Service has no endpoints — selector doesn't match any ready pods")
				selector := ""
				for _, l := range lines {
					if strings.Contains(l, "Selector:") {
						parts := strings.SplitN(l, "Selector:", 2)
						if len(parts) == 2 {
							selector = strings.TrimSpace(parts[1])
						}
					}
				}
				if selector != "" && ns != "" {
					s.NextCmds = append(s.NextCmds, FixCommand{
						Label:   fmt.Sprintf("Find pods with selector: %s", selector),
						Command: fmt.Sprintf("kubectl get pods -n %s -l %s", ns, selector),
						Safe:    true,
					})
					s.NextCmds = append(s.NextCmds, FixCommand{
						Label:   "List all pods with labels",
						Command: fmt.Sprintf("kubectl get pods -n %s --show-labels", ns),
						Safe:    true,
					})
				}
			}
		}
		if strings.HasPrefix(trimmed, "Warning ") || strings.HasPrefix(trimmed, "  Warning ") {
			warnings = append(warnings, trimmed)
		}
	}

	if len(insights) > 0 {
		s.Insight = strings.Join(insights, ". ")
	} else if len(warnings) > 0 {
		s.Insight = fmt.Sprintf("Found %d warning(s): %s", len(warnings), warnings[0])
	} else {
		s.Insight = "Resource exists and appears configured. Expand the output below for details."
	}

	return s
}

func analyzeGetPods(command, output string, s *Suggestion) *Suggestion {
	ns := extractFlag(command, "-n")
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) <= 1 {
		s.Insight = "No pods found matching the selector — this is likely the root cause"
		if ns != "" {
			s.NextCmds = append(s.NextCmds, FixCommand{
				Label:   "List all pods in namespace",
				Command: fmt.Sprintf("kubectl get pods -n %s --show-labels", ns),
				Safe:    true,
			})
			s.NextCmds = append(s.NextCmds, FixCommand{
				Label:   "List all deployments",
				Command: fmt.Sprintf("kubectl get deployments -n %s", ns),
				Safe:    true,
			})
		}
		return s
	}

	// Count pod statuses
	running, notReady, crashLoop, pending := 0, 0, 0, 0
	for _, line := range lines[1:] { // skip header
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		status := fields[2]
		switch status {
		case "Running":
			running++
			// Check if ready
			readyParts := strings.Split(fields[1], "/")
			if len(readyParts) == 2 && readyParts[0] != readyParts[1] {
				notReady++
			}
		case "CrashLoopBackOff", "Error":
			crashLoop++
		case "Pending":
			pending++
		}
	}

	total := len(lines) - 1
	if crashLoop > 0 {
		s.Insight = fmt.Sprintf("%d of %d pod(s) in CrashLoopBackOff — check container logs", crashLoop, total)
		// Suggest logs for the first crashing pod
		for _, line := range lines[1:] {
			fields := strings.Fields(line)
			if len(fields) >= 3 && (fields[2] == "CrashLoopBackOff" || fields[2] == "Error") {
				podName := fields[0]
				if ns != "" {
					s.NextCmds = append(s.NextCmds, FixCommand{
						Label:   fmt.Sprintf("View logs for %s", podName),
						Command: fmt.Sprintf("kubectl logs %s -n %s --tail=50", podName, ns),
						Safe:    true,
					})
					s.NextCmds = append(s.NextCmds, FixCommand{
						Label:   fmt.Sprintf("View previous logs for %s", podName),
						Command: fmt.Sprintf("kubectl logs %s -n %s --previous --tail=50", podName, ns),
						Safe:    true,
					})
				}
				break
			}
		}
	} else if pending > 0 {
		s.Insight = fmt.Sprintf("%d of %d pod(s) Pending — check scheduling constraints and node resources", pending, total)
	} else if notReady > 0 {
		s.Insight = fmt.Sprintf("%d of %d pod(s) not ready — readiness probes may be failing", notReady, total)
	} else if running > 0 {
		s.Insight = fmt.Sprintf("All %d pod(s) running and ready", running)
	} else {
		s.Insight = fmt.Sprintf("Found %d pod(s) — check their status above", total)
	}

	return s
}

func analyzeEndpoints(command, output string, s *Suggestion) *Suggestion {
	ns := extractFlag(command, "-n")
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) <= 1 || strings.Contains(output, "No resources found") {
		s.Insight = "No endpoint slices found — the service has no backing pods"
		if ns != "" {
			s.NextCmds = append(s.NextCmds, FixCommand{
				Label:   "List all pods in namespace",
				Command: fmt.Sprintf("kubectl get pods -n %s --show-labels", ns),
				Safe:    true,
			})
		}
	} else {
		s.Insight = "Endpoint slices exist — pods are registered with the service"
	}
	return s
}

func analyzeEvents(command, output string, s *Suggestion) *Suggestion {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) <= 1 || strings.Contains(output, "No resources found") {
		s.Insight = "No recent events — the issue may be stable/old or events have expired"
		return s
	}

	// Look for interesting event patterns
	var warnings []string
	for _, line := range lines {
		if strings.Contains(line, "Warning") {
			warnings = append(warnings, strings.TrimSpace(line))
		}
	}

	if len(warnings) > 0 {
		s.Insight = fmt.Sprintf("Found %d warning event(s) — review the output above for details", len(warnings))
	} else {
		s.Insight = fmt.Sprintf("Found %d event(s), no warnings detected", len(lines)-1)
	}
	return s
}

func analyzeJsonpath(command, output string, s *Suggestion) *Suggestion {
	trimmed := strings.TrimSpace(output)
	empty := trimmed == "" || trimmed == "{}" || trimmed == "null" || trimmed == "'{}'"

	if empty {
		// Context-sensitive insight based on what was queried
		switch {
		case strings.Contains(command, "securityContext"):
			s.Insight = "Not set at this level — may be inherited from pod-level or chart defaults"
		case strings.Contains(command, "resources"):
			s.Insight = "No resource requests or limits configured"
		default:
			s.Insight = "Value is empty or not set"
		}
	} else {
		s.Insight = fmt.Sprintf("Current value: %s", trimmed)
	}
	return s
}

// extractFlag extracts the value of a flag like -n from a command string.
func extractFlag(command, flag string) string {
	parts := strings.Fields(command)
	for i, p := range parts {
		if p == flag && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// extractResourceFromCmd extracts the resource identifier from a kubectl command.
func extractResourceFromCmd(command string) string {
	parts := strings.Fields(command)
	for i, p := range parts {
		if (p == "describe" || p == "get" || p == "logs") && i+2 < len(parts) {
			return parts[i+1] + "/" + parts[i+2]
		}
	}
	return ""
}
