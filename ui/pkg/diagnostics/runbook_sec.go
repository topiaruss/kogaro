package diagnostics

import (
	"context"
	"fmt"
	"strings"

	"github.com/topiaruss/kogaro/ui/pkg/graph"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// diagnoseSecurityContext inspects the security context of a pod/workload.
func diagnoseSecurityContext(ctx context.Context, c client.Client, err graph.ErrorDetail) ([]DiagnosticFinding, error) {
	var findings []DiagnosticFinding

	podSpec, source, fetchErr := getPodSpec(ctx, c, err.Namespace, err.ResourceName, err.ResourceType)
	if fetchErr != nil {
		return nil, fetchErr
	}

	findings = append(findings, DiagnosticFinding{
		Category: "labels",
		Summary:  fmt.Sprintf("Inspecting SecurityContext for %s", source),
	})

	// Pod-level security context
	if podSpec.SecurityContext == nil {
		findings = append(findings, DiagnosticFinding{
			Category: "pod_status",
			Summary:  "Pod has no SecurityContext defined",
			Details:  map[string]string{"level": "pod", "status": "missing"},
		})
	} else {
		sc := podSpec.SecurityContext
		details := map[string]string{"level": "pod"}
		if sc.RunAsNonRoot != nil {
			details["runAsNonRoot"] = fmt.Sprintf("%v", *sc.RunAsNonRoot)
		} else {
			details["runAsNonRoot"] = "not set"
		}
		if sc.RunAsUser != nil {
			details["runAsUser"] = fmt.Sprintf("%d", *sc.RunAsUser)
		}
		findings = append(findings, DiagnosticFinding{
			Category: "pod_status",
			Summary:  fmt.Sprintf("Pod SecurityContext: runAsNonRoot=%s", details["runAsNonRoot"]),
			Details:  details,
		})
	}

	// Container-level security contexts
	for _, container := range podSpec.Containers {
		if container.SecurityContext == nil {
			findings = append(findings, DiagnosticFinding{
				Category: "pod_status",
				Summary:  fmt.Sprintf("Container '%s' has no SecurityContext", container.Name),
				Details:  map[string]string{"container": container.Name, "level": "container", "status": "missing"},
			})
		} else {
			sc := container.SecurityContext
			parts := []string{}
			if sc.RunAsNonRoot != nil {
				parts = append(parts, fmt.Sprintf("runAsNonRoot=%v", *sc.RunAsNonRoot))
			}
			if sc.ReadOnlyRootFilesystem != nil {
				parts = append(parts, fmt.Sprintf("readOnlyRootFS=%v", *sc.ReadOnlyRootFilesystem))
			}
			if sc.Privileged != nil {
				parts = append(parts, fmt.Sprintf("privileged=%v", *sc.Privileged))
			}
			findings = append(findings, DiagnosticFinding{
				Category: "pod_status",
				Summary:  fmt.Sprintf("Container '%s': %s", container.Name, strings.Join(parts, ", ")),
				Details:  map[string]string{"container": container.Name, "level": "container"},
			})
		}
	}

	return findings, nil
}

func getPodSpec(ctx context.Context, c client.Client, namespace, name, kind string) (*corev1.PodSpec, string, error) {
	key := types.NamespacedName{Namespace: namespace, Name: name}
	switch kind {
	case "Deployment":
		dep := &appsv1.Deployment{}
		if err := c.Get(ctx, key, dep); err != nil {
			return nil, "", err
		}
		return &dep.Spec.Template.Spec, fmt.Sprintf("Deployment/%s", name), nil
	case "StatefulSet":
		ss := &appsv1.StatefulSet{}
		if err := c.Get(ctx, key, ss); err != nil {
			return nil, "", err
		}
		return &ss.Spec.Template.Spec, fmt.Sprintf("StatefulSet/%s", name), nil
	case "Pod":
		pod := &corev1.Pod{}
		if err := c.Get(ctx, key, pod); err != nil {
			return nil, "", err
		}
		return &pod.Spec, fmt.Sprintf("Pod/%s", name), nil
	default:
		return nil, "", fmt.Errorf("unsupported kind for security analysis: %s", kind)
	}
}
