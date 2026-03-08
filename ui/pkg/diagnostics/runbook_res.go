package diagnostics

import (
	"context"
	"fmt"

	"github.com/topiaruss/kogaro/ui/pkg/graph"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// diagnoseResourceLimits inspects the resource requests/limits on a workload.
func diagnoseResourceLimits(ctx context.Context, c client.Client, err graph.ErrorDetail) ([]DiagnosticFinding, error) {
	var findings []DiagnosticFinding

	podSpec, source, fetchErr := getPodSpec(ctx, c, err.Namespace, err.ResourceName, err.ResourceType)
	if fetchErr != nil {
		return nil, fetchErr
	}

	findings = append(findings, DiagnosticFinding{
		Category: "labels",
		Summary:  fmt.Sprintf("Inspecting resource constraints for %s", source),
	})

	for _, container := range podSpec.Containers {
		details := map[string]string{"container": container.Name}

		// Requests
		if container.Resources.Requests == nil {
			details["requests"] = "none"
			findings = append(findings, DiagnosticFinding{
				Category: "pod_status",
				Summary:  fmt.Sprintf("Container '%s' has no resource requests (CPU or memory)", container.Name),
				Details:  details,
			})
		} else {
			if cpu, ok := container.Resources.Requests["cpu"]; ok {
				details["cpu_request"] = cpu.String()
			} else {
				details["cpu_request"] = "not set"
			}
			if mem, ok := container.Resources.Requests["memory"]; ok {
				details["memory_request"] = mem.String()
			} else {
				details["memory_request"] = "not set"
			}
			findings = append(findings, DiagnosticFinding{
				Category: "pod_status",
				Summary:  fmt.Sprintf("Container '%s' requests: CPU=%s, Memory=%s", container.Name, details["cpu_request"], details["memory_request"]),
				Details:  details,
			})
		}

		// Limits
		limDetails := map[string]string{"container": container.Name}
		if container.Resources.Limits == nil {
			limDetails["limits"] = "none"
			findings = append(findings, DiagnosticFinding{
				Category: "pod_status",
				Summary:  fmt.Sprintf("Container '%s' has no resource limits (CPU or memory)", container.Name),
				Details:  limDetails,
			})
		} else {
			if cpu, ok := container.Resources.Limits["cpu"]; ok {
				limDetails["cpu_limit"] = cpu.String()
			} else {
				limDetails["cpu_limit"] = "not set"
			}
			if mem, ok := container.Resources.Limits["memory"]; ok {
				limDetails["memory_limit"] = mem.String()
			} else {
				limDetails["memory_limit"] = "not set"
			}
			findings = append(findings, DiagnosticFinding{
				Category: "pod_status",
				Summary:  fmt.Sprintf("Container '%s' limits: CPU=%s, Memory=%s", container.Name, limDetails["cpu_limit"], limDetails["memory_limit"]),
				Details:  limDetails,
			})
		}
	}

	return findings, nil
}
