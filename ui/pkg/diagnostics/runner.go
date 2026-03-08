package diagnostics

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/topiaruss/kogaro/ui/pkg/graph"
	"github.com/topiaruss/kogaro/ui/pkg/history"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Runner executes diagnostic runbooks against the live cluster.
type Runner struct {
	client  client.Client
	history *history.Store
}

// NewRunner creates a diagnostic runner.
func NewRunner(c client.Client, h *history.Store) *Runner {
	return &Runner{client: c, history: h}
}

// RunForIncident runs diagnostics for all errors in an incident.
func (r *Runner) RunForIncident(ctx context.Context, incident graph.Incident) ([]DiagnosticResult, error) {
	var results []DiagnosticResult
	seen := make(map[string]bool) // deduplicate by code+resource

	for _, errDetail := range incident.Errors {
		dedupKey := fmt.Sprintf("%s/%s/%s", errDetail.ErrorCode, errDetail.Namespace, errDetail.ResourceName)
		if seen[dedupKey] {
			continue
		}
		seen[dedupKey] = true

		runbook := GetRunbook(errDetail.ErrorCode)
		if runbook == nil {
			continue
		}

		findings, err := runbook(ctx, r.client, errDetail)
		if err != nil {
			log.Printf("diagnostic runbook %s failed: %v", errDetail.ErrorCode, err)
			continue
		}

		results = append(results, DiagnosticResult{
			ErrorCode:    errDetail.ErrorCode,
			ResourceType: errDetail.ResourceType,
			ResourceName: errDetail.ResourceName,
			Namespace:    errDetail.Namespace,
			Findings:     findings,
			RanAt:        time.Now(),
		})
	}

	// Persist to SQLite
	if r.history != nil {
		r.persistDiagnostics(results)
	}

	return results, nil
}

func (r *Runner) persistDiagnostics(results []DiagnosticResult) {
	for _, res := range results {
		findingsJSON, _ := json.Marshal(res.Findings)
		record := history.DiagnosticRecord{
			ErrorCode:    res.ErrorCode,
			ResourceType: res.ResourceType,
			ResourceName: res.ResourceName,
			Namespace:    res.Namespace,
			FindingsJSON: string(findingsJSON),
			RanAt:        res.RanAt,
		}
		if err := r.history.SaveDiagnostic(&record); err != nil {
			log.Printf("failed to persist diagnostic: %v", err)
		}
	}
}

// Helper functions used by runbooks

// GetService fetches a Service by name and namespace.
func GetService(ctx context.Context, c client.Client, namespace, name string) (*corev1.Service, error) {
	svc := &corev1.Service{}
	err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, svc)
	return svc, err
}

// ListPodsByLabels lists pods matching a label selector in a namespace.
func ListPodsByLabels(ctx context.Context, c client.Client, namespace string, labels map[string]string) ([]corev1.Pod, error) {
	podList := &corev1.PodList{}
	matchLabels := client.MatchingLabels(labels)
	if err := c.List(ctx, podList, client.InNamespace(namespace), matchLabels); err != nil {
		return nil, err
	}
	return podList.Items, nil
}

// ListEndpointSlices lists EndpointSlices for a service.
func ListEndpointSlices(ctx context.Context, c client.Client, namespace, serviceName string) ([]discoveryv1.EndpointSlice, error) {
	epsList := &discoveryv1.EndpointSliceList{}
	err := c.List(ctx, epsList, client.InNamespace(namespace),
		client.MatchingLabels{"kubernetes.io/service-name": serviceName})
	if err != nil {
		return nil, err
	}
	return epsList.Items, nil
}

// ListEvents lists recent events for a resource.
func ListEvents(ctx context.Context, c client.Client, namespace, name, kind string) ([]corev1.Event, error) {
	eventList := &corev1.EventList{}
	if err := c.List(ctx, eventList, client.InNamespace(namespace)); err != nil {
		return nil, err
	}
	var relevant []corev1.Event
	for _, e := range eventList.Items {
		if e.InvolvedObject.Name == name && e.InvolvedObject.Kind == kind {
			relevant = append(relevant, e)
		}
	}
	return relevant, nil
}
