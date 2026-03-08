package diagnostics

import (
	"context"
	"fmt"
	"strings"

	"github.com/topiaruss/kogaro/ui/pkg/graph"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// diagnoseServiceSelectorMismatch checks if a service selector matches any pods.
func diagnoseServiceSelectorMismatch(ctx context.Context, c client.Client, err graph.ErrorDetail) ([]DiagnosticFinding, error) {
	var findings []DiagnosticFinding

	svc, svcErr := GetService(ctx, c, err.Namespace, err.ResourceName)
	if svcErr != nil {
		return nil, svcErr
	}

	selector := svc.Spec.Selector
	if len(selector) == 0 {
		findings = append(findings, DiagnosticFinding{
			Category: "labels",
			Summary:  "Service has no selector defined",
			Details:  map[string]string{"service": err.ResourceName},
		})
		return findings, nil
	}

	selectorParts := make([]string, 0, len(selector))
	for k, v := range selector {
		selectorParts = append(selectorParts, fmt.Sprintf("%s=%s", k, v))
	}

	findings = append(findings, DiagnosticFinding{
		Category: "labels",
		Summary:  fmt.Sprintf("Service selector: %s", strings.Join(selectorParts, ", ")),
		Details:  selector,
	})

	// Check for pods matching the selector
	pods, podErr := ListPodsByLabels(ctx, c, err.Namespace, selector)
	if podErr != nil {
		return findings, nil // return what we have
	}

	if len(pods) == 0 {
		findings = append(findings, DiagnosticFinding{
			Category: "pod_status",
			Summary:  fmt.Sprintf("No pods match selector {%s} in namespace %s", strings.Join(selectorParts, ", "), err.Namespace),
			Details:  map[string]string{"matching_pods": "0"},
		})
	} else {
		for _, pod := range pods {
			podLabels := make([]string, 0)
			for k, v := range pod.Labels {
				podLabels = append(podLabels, fmt.Sprintf("%s=%s", k, v))
			}
			findings = append(findings, DiagnosticFinding{
				Category: "pod_status",
				Summary:  fmt.Sprintf("Pod %s matches selector but labels are: %s", pod.Name, strings.Join(podLabels, ", ")),
				Details:  map[string]string{"pod": pod.Name, "phase": string(pod.Status.Phase)},
			})
		}
	}

	return findings, nil
}

// diagnoseServiceNoEndpoints checks why a service has no ready endpoints.
func diagnoseServiceNoEndpoints(ctx context.Context, c client.Client, err graph.ErrorDetail) ([]DiagnosticFinding, error) {
	var findings []DiagnosticFinding

	svc, svcErr := GetService(ctx, c, err.Namespace, err.ResourceName)
	if svcErr != nil {
		return nil, svcErr
	}

	selector := svc.Spec.Selector
	selectorParts := make([]string, 0, len(selector))
	for k, v := range selector {
		selectorParts = append(selectorParts, fmt.Sprintf("%s=%s", k, v))
	}

	// Check EndpointSlices
	epSlices, epsErr := ListEndpointSlices(ctx, c, err.Namespace, err.ResourceName)
	if epsErr == nil {
		readyCount := 0
		notReadyCount := 0
		for _, eps := range epSlices {
			for _, ep := range eps.Endpoints {
				if ep.Conditions.Ready != nil && *ep.Conditions.Ready {
					readyCount++
				} else {
					notReadyCount++
				}
			}
		}
		findings = append(findings, DiagnosticFinding{
			Category: "endpoints",
			Summary:  fmt.Sprintf("EndpointSlices: %d ready, %d not ready", readyCount, notReadyCount),
			Details: map[string]string{
				"ready":     fmt.Sprintf("%d", readyCount),
				"not_ready": fmt.Sprintf("%d", notReadyCount),
			},
		})
	}

	// Check pods matching selector
	if len(selector) > 0 {
		pods, podErr := ListPodsByLabels(ctx, c, err.Namespace, selector)
		if podErr == nil {
			if len(pods) == 0 {
				findings = append(findings, DiagnosticFinding{
					Category: "pod_status",
					Summary:  fmt.Sprintf("No pods match selector {%s}", strings.Join(selectorParts, ", ")),
					Details:  map[string]string{"matching_pods": "0"},
				})
			} else {
				for _, pod := range pods {
					phase := string(pod.Status.Phase)
					ready := "false"
					reason := ""
					for _, cond := range pod.Status.Conditions {
						if cond.Type == "Ready" {
							ready = string(cond.Status)
							if cond.Reason != "" {
								reason = cond.Reason
							}
						}
					}

					// Check container statuses for more detail
					for _, cs := range pod.Status.ContainerStatuses {
						if cs.State.Waiting != nil {
							reason = cs.State.Waiting.Reason
							if cs.State.Waiting.Message != "" {
								reason += ": " + cs.State.Waiting.Message
							}
						}
						if cs.State.Terminated != nil {
							reason = fmt.Sprintf("terminated: %s (exit %d)", cs.State.Terminated.Reason, cs.State.Terminated.ExitCode)
						}
					}

					summary := fmt.Sprintf("Pod %s: phase=%s, ready=%s", pod.Name, phase, ready)
					if reason != "" {
						summary += fmt.Sprintf(", reason=%s", reason)
					}

					findings = append(findings, DiagnosticFinding{
						Category: "pod_status",
						Summary:  summary,
						Details: map[string]string{
							"pod":    pod.Name,
							"phase":  phase,
							"ready":  ready,
							"reason": reason,
						},
					})
				}
			}
		}
	}

	// Get events for the service
	events, evErr := ListEvents(ctx, c, err.Namespace, err.ResourceName, "Service")
	if evErr == nil && len(events) > 0 {
		for _, ev := range events {
			findings = append(findings, DiagnosticFinding{
				Category: "events",
				Summary:  fmt.Sprintf("[%s] %s: %s", ev.Type, ev.Reason, ev.Message),
				Details: map[string]string{
					"type":   ev.Type,
					"reason": ev.Reason,
				},
			})
		}
	}

	return findings, nil
}

// diagnoseIngressNoBackendPods traces from Ingress → Service → Pods.
func diagnoseIngressNoBackendPods(ctx context.Context, c client.Client, err graph.ErrorDetail) ([]DiagnosticFinding, error) {
	var findings []DiagnosticFinding

	// Extract service name from error message (between single quotes)
	svcName := extractQuotedName(err.Message)
	if svcName == "" {
		svcName = err.ResourceName // fallback
	}

	findings = append(findings, DiagnosticFinding{
		Category: "endpoints",
		Summary:  fmt.Sprintf("Ingress %s routes to Service %s", err.ResourceName, svcName),
		Details:  map[string]string{"ingress": err.ResourceName, "service": svcName},
	})

	// Diagnose the backend service
	svcErr := graph.ErrorDetail{
		ErrorCode:    "KOGARO-NET-002",
		ResourceType: "Service",
		ResourceName: svcName,
		Namespace:    err.Namespace,
	}
	svcFindings, _ := diagnoseServiceNoEndpoints(ctx, c, svcErr)
	findings = append(findings, svcFindings...)

	return findings, nil
}

func extractQuotedName(msg string) string {
	start := strings.Index(msg, "'")
	if start < 0 {
		return ""
	}
	end := strings.Index(msg[start+1:], "'")
	if end < 0 {
		return ""
	}
	return msg[start+1 : start+1+end]
}
