package graph

import (
	"context"
	"testing"

	"github.com/topiaruss/kogaro/internal/validators"
)

func TestSeedFromErrors_SingleDanglingConfigMap(t *testing.T) {
	b := NewBuilder(nil, 0) // no client, no expansion

	errs := []validators.ValidationError{
		validators.NewValidationErrorWithCode(
			"Pod", "my-pod", "default",
			"dangling_configmap_volume", "KOGARO-REF-003",
			"ConfigMap 'app-config' referenced in volume does not exist",
		),
	}

	fg, err := b.Build(context.Background(), errs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(fg.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(fg.Nodes))
	}

	// Find source and target
	var source, target *Node
	for i := range fg.Nodes {
		if fg.Nodes[i].Kind == "Pod" {
			source = &fg.Nodes[i]
		}
		if fg.Nodes[i].Kind == "ConfigMap" {
			target = &fg.Nodes[i]
		}
	}

	if source == nil {
		t.Fatal("expected Pod source node")
	}
	if !source.IsFaultOrigin {
		t.Error("source should be fault origin")
	}
	if source.Health != HealthBroken {
		t.Errorf("source health should be broken, got %s", source.Health)
	}

	if target == nil {
		t.Fatal("expected ConfigMap target node")
	}
	if !target.IsRootCause {
		t.Error("target should be root cause")
	}
	if target.Health != HealthMissing {
		t.Errorf("target health should be missing, got %s", target.Health)
	}

	if len(fg.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(fg.Edges))
	}
	edge := fg.Edges[0]
	if edge.Health != HealthBroken {
		t.Errorf("edge health should be broken, got %s", edge.Health)
	}
	if edge.Type != EdgeReference {
		t.Errorf("edge type should be reference, got %s", edge.Type)
	}
}

func TestSeedFromErrors_MultipleDanglingRefs(t *testing.T) {
	b := NewBuilder(nil, 0)

	errs := []validators.ValidationError{
		validators.NewValidationErrorWithCode(
			"Pod", "my-pod", "default",
			"dangling_configmap_volume", "KOGARO-REF-003",
			"ConfigMap 'app-config' referenced in volume does not exist",
		),
		validators.NewValidationErrorWithCode(
			"Pod", "my-pod", "default",
			"dangling_secret_volume", "KOGARO-REF-005",
			"Secret 'db-credentials' referenced in volume does not exist",
		),
	}

	fg, err := b.Build(context.Background(), errs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Pod + ConfigMap + Secret = 3 nodes
	if len(fg.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(fg.Nodes))
	}

	if len(fg.Edges) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(fg.Edges))
	}
}

func TestSeedFromErrors_IngressWithMissingService(t *testing.T) {
	b := NewBuilder(nil, 0)

	errs := []validators.ValidationError{
		validators.NewValidationErrorWithCode(
			"Ingress", "my-ingress", "default",
			"dangling_service_reference", "KOGARO-REF-002",
			"Service 'backend-svc' referenced in Ingress does not exist",
		),
	}

	fg, err := b.Build(context.Background(), errs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(fg.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(fg.Nodes))
	}

	var svcNode *Node
	for i := range fg.Nodes {
		if fg.Nodes[i].Kind == "Service" {
			svcNode = &fg.Nodes[i]
		}
	}
	if svcNode == nil {
		t.Fatal("expected Service target node")
	}
	if svcNode.Name != "backend-svc" {
		t.Errorf("expected service name 'backend-svc', got %s", svcNode.Name)
	}
}

func TestSeedFromErrors_SecurityErrors_NoTarget(t *testing.T) {
	b := NewBuilder(nil, 0)

	errs := []validators.ValidationError{
		validators.NewValidationErrorWithCode(
			"Pod", "insecure-pod", "default",
			"pod_running_as_root", "KOGARO-SEC-001",
			"Pod is running as root user",
		),
	}

	fg, err := b.Build(context.Background(), errs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Security errors have no target - just the source node
	if len(fg.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(fg.Nodes))
	}
	if len(fg.Edges) != 0 {
		t.Fatalf("expected 0 edges, got %d", len(fg.Edges))
	}
}

func TestHealthPropagation(t *testing.T) {
	b := NewBuilder(nil, 0)

	// Manually add nodes and edges to test propagation
	b.nodes[MakeNodeID("Deployment", "default", "my-deploy")] = &Node{
		ID:        MakeNodeID("Deployment", "default", "my-deploy"),
		Kind:      "Deployment",
		Name:      "my-deploy",
		Namespace: "default",
		Health:    HealthHealthy,
	}
	b.nodes[MakeNodeID("Pod", "default", "my-pod")] = &Node{
		ID:            MakeNodeID("Pod", "default", "my-pod"),
		Kind:          "Pod",
		Name:          "my-pod",
		Namespace:     "default",
		Health:        HealthBroken,
		IsFaultOrigin: true,
	}
	b.addEdge(
		MakeNodeID("Deployment", "default", "my-deploy"),
		MakeNodeID("Pod", "default", "my-pod"),
		EdgeOwnership, HealthHealthy, "owner", nil,
	)

	b.propagateHealth()

	deploy := b.nodes[MakeNodeID("Deployment", "default", "my-deploy")]
	if deploy.Health != HealthDegraded {
		t.Errorf("deployment health should be degraded, got %s", deploy.Health)
	}
}

func TestIncidentGrouping_SharedSource(t *testing.T) {
	errs := []validators.ValidationError{
		validators.NewValidationErrorWithCode(
			"Pod", "my-pod", "default",
			"dangling_configmap_volume", "KOGARO-REF-003",
			"ConfigMap 'config-a' referenced in volume does not exist",
		),
		validators.NewValidationErrorWithCode(
			"Pod", "my-pod", "default",
			"dangling_secret_volume", "KOGARO-REF-005",
			"Secret 'secret-b' referenced in volume does not exist",
		),
	}

	incidents := BuildIncidents(errs, make(map[NodeID]*Node))

	if len(incidents) != 1 {
		t.Fatalf("expected 1 incident (same source), got %d", len(incidents))
	}

	if len(incidents[0].ErrorCodes) != 2 {
		t.Errorf("expected 2 error codes, got %d", len(incidents[0].ErrorCodes))
	}
}

func TestIncidentGrouping_DifferentSources(t *testing.T) {
	errs := []validators.ValidationError{
		validators.NewValidationErrorWithCode(
			"Pod", "pod-a", "ns-a",
			"dangling_configmap_volume", "KOGARO-REF-003",
			"ConfigMap 'unique-config-a' referenced in volume does not exist",
		),
		validators.NewValidationErrorWithCode(
			"Pod", "pod-b", "ns-b",
			"dangling_secret_volume", "KOGARO-REF-005",
			"Secret 'unique-secret-b' referenced in volume does not exist",
		),
	}

	incidents := BuildIncidents(errs, make(map[NodeID]*Node))

	if len(incidents) != 2 {
		t.Fatalf("expected 2 incidents (different sources), got %d", len(incidents))
	}
}

func TestMakeNodeID(t *testing.T) {
	id := MakeNodeID("Pod", "default", "my-pod")
	if id != "Pod/default/my-pod" {
		t.Errorf("expected Pod/default/my-pod, got %s", id)
	}

	clusterScoped := MakeNodeID("IngressClass", "", "nginx")
	if clusterScoped != "IngressClass//nginx" {
		t.Errorf("expected IngressClass//nginx, got %s", clusterScoped)
	}
}
