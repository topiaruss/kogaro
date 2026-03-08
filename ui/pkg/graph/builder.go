package graph

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/topiaruss/kogaro/internal/validators"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Builder constructs a FaultGraph from validation errors.
type Builder struct {
	client    client.Client
	maxDepth  int
	nodes     map[NodeID]*Node
	edges     []Edge
	edgeSet   map[string]bool
}

// NewBuilder creates a graph builder.
func NewBuilder(c client.Client, maxDepth int) *Builder {
	if maxDepth <= 0 {
		maxDepth = 2
	}
	return &Builder{
		client:   c,
		maxDepth: maxDepth,
		nodes:    make(map[NodeID]*Node),
		edgeSet:  make(map[string]bool),
	}
}

// Build constructs a FaultGraph from validation errors.
func (b *Builder) Build(ctx context.Context, errors []validators.ValidationError) (*FaultGraph, error) {
	// Phase 1: Seed from errors
	b.seedFromErrors(errors)

	// Phase 2: Expand neighbors
	if err := b.expandNeighbors(ctx); err != nil {
		return nil, fmt.Errorf("expanding neighbors: %w", err)
	}

	// Phase 3: Health propagation
	b.propagateHealth()

	// Phase 4: Collapse Pods into their owners
	b.collapseByOwner()

	// Build incidents
	incidents := BuildIncidents(errors, b.nodes)

	// Collect nodes
	nodes := make([]Node, 0, len(b.nodes))
	for _, n := range b.nodes {
		nodes = append(nodes, *n)
	}

	return &FaultGraph{
		Nodes:     nodes,
		Edges:     b.edges,
		Incidents: incidents,
		ScanTime:  time.Now(),
	}, nil
}

// seedFromErrors creates nodes and edges from validation errors (Phase 1).
func (b *Builder) seedFromErrors(errs []validators.ValidationError) {
	for _, ve := range errs {
		// Create source node (the resource that has the error)
		sourceID := MakeNodeID(ve.ResourceType, ve.Namespace, ve.ResourceName)
		if _, ok := b.nodes[sourceID]; !ok {
			b.nodes[sourceID] = &Node{
				ID:            sourceID,
				Kind:          ve.ResourceType,
				Name:          ve.ResourceName,
				Namespace:     ve.Namespace,
				Health:        HealthBroken,
				IsFaultOrigin: true,
				DistFromFault: 0,
				Details:       make(map[string]string),
			}
		}
		src := b.nodes[sourceID]
		src.ErrorCodes = appendUnique(src.ErrorCodes, ve.ErrorCode)
		src.IsFaultOrigin = true
		src.Health = HealthBroken

		// Parse target from error context
		target := b.parseTarget(ve)
		if target != nil {
			if _, ok := b.nodes[target.ID]; !ok {
				b.nodes[target.ID] = target
			}
			edgeType, edgeLabel := b.classifyEdge(ve)
			b.addEdge(sourceID, target.ID, edgeType, HealthBroken, edgeLabel, []string{ve.ErrorCode})
		}
	}
}

// parseTarget extracts the missing/broken target from a validation error.
func (b *Builder) parseTarget(ve validators.ValidationError) *Node {
	var kind, name, ns string
	ns = ve.Namespace

	switch {
	case strings.Contains(ve.ValidationType, "configmap"):
		kind = "ConfigMap"
		name = extractResourceName(ve.Message, "ConfigMap")
	case strings.Contains(ve.ValidationType, "secret") || strings.Contains(ve.ValidationType, "tls_secret"):
		kind = "Secret"
		name = extractResourceName(ve.Message, "Secret")
	case strings.Contains(ve.ValidationType, "pvc"):
		kind = "PersistentVolumeClaim"
		name = extractResourceName(ve.Message, "PVC")
	case strings.Contains(ve.ValidationType, "service_reference"):
		kind = "Service"
		name = extractResourceName(ve.Message, "Service")
	case strings.Contains(ve.ValidationType, "ingress_class"):
		kind = "IngressClass"
		name = extractResourceName(ve.Message, "IngressClass")
		ns = "" // cluster-scoped
	case strings.Contains(ve.ValidationType, "storage_class"):
		kind = "StorageClass"
		name = extractResourceName(ve.Message, "StorageClass")
		ns = "" // cluster-scoped
	case strings.Contains(ve.ValidationType, "service_account"):
		kind = "ServiceAccount"
		name = extractResourceName(ve.Message, "ServiceAccount")
	case strings.Contains(ve.ValidationType, "service_selector_mismatch"):
		return nil // no single target
	case strings.Contains(ve.ValidationType, "service_no_endpoints"):
		return nil
	default:
		// For errors without a clear target (security, resource limits), no target node
		return nil
	}

	if name == "" {
		return nil
	}

	id := MakeNodeID(kind, ns, name)
	return &Node{
		ID:            id,
		Kind:          kind,
		Name:          name,
		Namespace:     ns,
		Health:        HealthMissing,
		IsRootCause:   true,
		DistFromFault: 0,
		Details:       make(map[string]string),
	}
}

// classifyEdge determines edge type and label from a validation error.
func (b *Builder) classifyEdge(ve validators.ValidationError) (EdgeType, string) {
	switch {
	case strings.Contains(ve.ValidationType, "volume"):
		return EdgeReference, "volume"
	case strings.Contains(ve.ValidationType, "envfrom"):
		return EdgeReference, "envFrom"
	case strings.Contains(ve.ValidationType, "env"):
		return EdgeReference, "env"
	case strings.Contains(ve.ValidationType, "tls"):
		return EdgeTLS, "tls"
	case strings.Contains(ve.ValidationType, "service_reference"):
		return EdgeExposure, "backend"
	case strings.Contains(ve.ValidationType, "ingress_class"):
		return EdgeReference, "ingressClassName"
	case strings.Contains(ve.ValidationType, "storage_class"):
		return EdgeStorageClass, "storageClassName"
	case strings.Contains(ve.ValidationType, "service_account"):
		return EdgeReference, "serviceAccount"
	case strings.Contains(ve.ValidationType, "selector"):
		return EdgeSelector, "selector"
	default:
		return EdgeReference, ""
	}
}

// expandNeighbors does BFS expansion from seed nodes (Phase 2).
func (b *Builder) expandNeighbors(ctx context.Context) error {
	if b.client == nil {
		return nil
	}

	// Collect seed nodes
	frontier := make([]NodeID, 0)
	for id := range b.nodes {
		frontier = append(frontier, id)
	}

	for depth := 0; depth < b.maxDepth && len(frontier) > 0; depth++ {
		nextFrontier := make([]NodeID, 0)
		for _, nodeID := range frontier {
			node := b.nodes[nodeID]
			if node == nil {
				continue
			}
			newNodes, err := b.expandNode(ctx, node, depth+1)
			if err != nil {
				continue // best-effort expansion
			}
			nextFrontier = append(nextFrontier, newNodes...)
		}
		frontier = nextFrontier
	}

	return nil
}

// expandNode expands a single node's relationships.
func (b *Builder) expandNode(ctx context.Context, node *Node, dist int) ([]NodeID, error) {
	var newNodes []NodeID

	switch node.Kind {
	case "Pod":
		newNodes = append(newNodes, b.expandPodUp(ctx, node, dist)...)
	case "ReplicaSet":
		newNodes = append(newNodes, b.expandReplicaSetUp(ctx, node, dist)...)
	case "Service":
		newNodes = append(newNodes, b.expandServiceDown(ctx, node, dist)...)
	case "Ingress":
		newNodes = append(newNodes, b.expandIngressDown(ctx, node, dist)...)
	case "Deployment":
		newNodes = append(newNodes, b.expandDeploymentDown(ctx, node, dist)...)
	}

	return newNodes, nil
}

// expandPodUp finds the owner (ReplicaSet/Job) of a Pod.
func (b *Builder) expandPodUp(ctx context.Context, node *Node, dist int) []NodeID {
	var pod corev1.Pod
	if err := b.client.Get(ctx, client.ObjectKey{Name: node.Name, Namespace: node.Namespace}, &pod); err != nil {
		return nil
	}

	var newNodes []NodeID
	for _, ref := range pod.OwnerReferences {
		ownerID := MakeNodeID(ref.Kind, node.Namespace, ref.Name)
		if b.addNeighborNode(ownerID, ref.Kind, ref.Name, node.Namespace, dist) {
			newNodes = append(newNodes, ownerID)
		}
		b.addEdge(ownerID, node.ID, EdgeOwnership, HealthHealthy, "owner", nil)
	}
	return newNodes
}

// expandReplicaSetUp finds the Deployment owning a ReplicaSet.
func (b *Builder) expandReplicaSetUp(ctx context.Context, node *Node, dist int) []NodeID {
	var rs appsv1.ReplicaSet
	if err := b.client.Get(ctx, client.ObjectKey{Name: node.Name, Namespace: node.Namespace}, &rs); err != nil {
		return nil
	}

	var newNodes []NodeID
	for _, ref := range rs.OwnerReferences {
		ownerID := MakeNodeID(ref.Kind, node.Namespace, ref.Name)
		if b.addNeighborNode(ownerID, ref.Kind, ref.Name, node.Namespace, dist) {
			newNodes = append(newNodes, ownerID)
		}
		b.addEdge(ownerID, node.ID, EdgeOwnership, HealthHealthy, "owner", nil)
	}
	return newNodes
}

// expandServiceDown finds Pods matching a Service selector.
func (b *Builder) expandServiceDown(ctx context.Context, node *Node, dist int) []NodeID {
	var svc corev1.Service
	if err := b.client.Get(ctx, client.ObjectKey{Name: node.Name, Namespace: node.Namespace}, &svc); err != nil {
		return nil
	}
	if len(svc.Spec.Selector) == 0 {
		return nil
	}

	var podList corev1.PodList
	sel := labels.SelectorFromSet(svc.Spec.Selector)
	if err := b.client.List(ctx, &podList, client.InNamespace(node.Namespace), client.MatchingLabelsSelector{Selector: sel}); err != nil {
		return nil
	}

	var newNodes []NodeID
	for _, pod := range podList.Items {
		podID := MakeNodeID("Pod", pod.Namespace, pod.Name)
		if b.addNeighborNode(podID, "Pod", pod.Name, pod.Namespace, dist) {
			newNodes = append(newNodes, podID)
		}
		b.addEdge(node.ID, podID, EdgeSelector, HealthHealthy, "selector", nil)
	}
	return newNodes
}

// expandIngressDown finds Services referenced by an Ingress.
func (b *Builder) expandIngressDown(ctx context.Context, node *Node, dist int) []NodeID {
	var ing networkingv1.Ingress
	if err := b.client.Get(ctx, client.ObjectKey{Name: node.Name, Namespace: node.Namespace}, &ing); err != nil {
		return nil
	}

	var newNodes []NodeID
	for _, rule := range ing.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}
		for _, path := range rule.HTTP.Paths {
			svcName := path.Backend.Service.Name
			svcID := MakeNodeID("Service", node.Namespace, svcName)
			if b.addNeighborNode(svcID, "Service", svcName, node.Namespace, dist) {
				newNodes = append(newNodes, svcID)
			}
			b.addEdge(node.ID, svcID, EdgeExposure, HealthHealthy, fmt.Sprintf("backend: %s", svcName), nil)
		}
	}
	return newNodes
}

// expandDeploymentDown finds ReplicaSets owned by a Deployment.
func (b *Builder) expandDeploymentDown(ctx context.Context, node *Node, dist int) []NodeID {
	var rsList appsv1.ReplicaSetList
	if err := b.client.List(ctx, &rsList, client.InNamespace(node.Namespace)); err != nil {
		return nil
	}

	var newNodes []NodeID
	for _, rs := range rsList.Items {
		for _, ref := range rs.OwnerReferences {
			if ref.Kind == "Deployment" && ref.Name == node.Name {
				rsID := MakeNodeID("ReplicaSet", rs.Namespace, rs.Name)
				if b.addNeighborNode(rsID, "ReplicaSet", rs.Name, rs.Namespace, dist) {
					newNodes = append(newNodes, rsID)
				}
				b.addEdge(node.ID, rsID, EdgeOwnership, HealthHealthy, "owner", nil)
			}
		}
	}
	return newNodes
}

// addNeighborNode adds a node if it doesn't already exist. Returns true if new.
func (b *Builder) addNeighborNode(id NodeID, kind, name, namespace string, dist int) bool {
	if _, exists := b.nodes[id]; exists {
		return false
	}
	b.nodes[id] = &Node{
		ID:            id,
		Kind:          kind,
		Name:          name,
		Namespace:     namespace,
		Health:        HealthHealthy,
		DistFromFault: dist,
		Details:       make(map[string]string),
	}
	return true
}

// addEdge adds an edge if it doesn't already exist.
func (b *Builder) addEdge(source, target NodeID, edgeType EdgeType, health HealthState, label string, errorCodes []string) {
	key := fmt.Sprintf("%s->%s:%s", source, target, edgeType)
	if b.edgeSet[key] {
		return
	}
	b.edgeSet[key] = true
	b.edges = append(b.edges, Edge{
		Source:     source,
		Target:     target,
		Type:       edgeType,
		Health:     health,
		Label:      label,
		ErrorCodes: errorCodes,
	})
}

// propagateHealth does BFS from broken nodes to set degraded state (Phase 3).
func (b *Builder) propagateHealth() {
	// Collect broken nodes
	queue := make([]NodeID, 0)
	for id, n := range b.nodes {
		if n.Health == HealthBroken || n.Health == HealthMissing {
			queue = append(queue, id)
		}
	}

	// Build adjacency (undirected)
	adj := make(map[NodeID][]NodeID)
	for _, e := range b.edges {
		adj[e.Source] = append(adj[e.Source], e.Target)
		adj[e.Target] = append(adj[e.Target], e.Source)
	}

	visited := make(map[NodeID]bool)
	for _, id := range queue {
		visited[id] = true
	}

	// BFS
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		currentNode := b.nodes[current]
		if currentNode == nil {
			continue
		}

		for _, neighbor := range adj[current] {
			if visited[neighbor] {
				continue
			}
			visited[neighbor] = true

			n := b.nodes[neighbor]
			if n != nil && n.Health == HealthHealthy {
				n.Health = HealthDegraded
				n.DistFromFault = currentNode.DistFromFault + 1
				queue = append(queue, neighbor)
			}
		}
	}
}

// collapseByOwner merges Pod nodes into their owner (Deployment/ReplicaSet/StatefulSet/Job).
// The owner inherits all error codes and becomes the fault origin. The Pod is removed.
func (b *Builder) collapseByOwner() {
	// Find ownership edges: owner -> pod
	ownerOf := make(map[NodeID]NodeID)   // pod -> owner
	childrenOf := make(map[NodeID][]NodeID) // owner -> pods
	for _, e := range b.edges {
		if e.Type != EdgeOwnership {
			continue
		}
		child := b.nodes[e.Target]
		if child == nil || child.Kind != "Pod" {
			continue
		}
		owner := b.nodes[e.Source]
		if owner == nil {
			continue
		}
		ownerOf[e.Target] = e.Source
		childrenOf[e.Source] = append(childrenOf[e.Source], e.Target)
	}

	// For each owner with Pod children, merge Pod state into owner
	for ownerID, podIDs := range childrenOf {
		owner := b.nodes[ownerID]
		if owner == nil {
			continue
		}

		for _, podID := range podIDs {
			pod := b.nodes[podID]
			if pod == nil {
				continue
			}

			// Merge error codes
			for _, code := range pod.ErrorCodes {
				owner.ErrorCodes = appendUnique(owner.ErrorCodes, code)
			}

			// Promote health
			if pod.IsFaultOrigin {
				owner.IsFaultOrigin = true
			}
			if pod.Health == HealthBroken && owner.Health != HealthBroken {
				owner.Health = HealthBroken
			}
			if pod.DistFromFault < owner.DistFromFault {
				owner.DistFromFault = pod.DistFromFault
			}

			owner.CollapsedFrom = append(owner.CollapsedFrom, podID)

			// Retarget edges that pointed to/from the pod to point to the owner
			b.retargetEdges(podID, ownerID)

			// Remove the pod node
			delete(b.nodes, podID)
		}

		owner.ResourceCount = len(podIDs)
	}

	// Also collapse ReplicaSets into their Deployment owner
	rsOwnerOf := make(map[NodeID]NodeID)
	rsChildrenOf := make(map[NodeID][]NodeID)
	for _, e := range b.edges {
		if e.Type != EdgeOwnership {
			continue
		}
		child := b.nodes[e.Target]
		if child == nil || child.Kind != "ReplicaSet" {
			continue
		}
		owner := b.nodes[e.Source]
		if owner == nil || owner.Kind != "Deployment" {
			continue
		}
		rsOwnerOf[e.Target] = e.Source
		rsChildrenOf[e.Source] = append(rsChildrenOf[e.Source], e.Target)
	}

	for ownerID, rsIDs := range rsChildrenOf {
		owner := b.nodes[ownerID]
		if owner == nil {
			continue
		}
		for _, rsID := range rsIDs {
			rs := b.nodes[rsID]
			if rs == nil {
				continue
			}
			for _, code := range rs.ErrorCodes {
				owner.ErrorCodes = appendUnique(owner.ErrorCodes, code)
			}
			if rs.IsFaultOrigin {
				owner.IsFaultOrigin = true
			}
			if rs.Health == HealthBroken && owner.Health != HealthBroken {
				owner.Health = HealthBroken
			}
			owner.CollapsedFrom = append(owner.CollapsedFrom, rsID)
			if rs.ResourceCount > 0 && owner.ResourceCount == 0 {
				owner.ResourceCount = rs.ResourceCount
			}
			b.retargetEdges(rsID, ownerID)
			delete(b.nodes, rsID)
		}
	}

	// Remove orphaned ownership edges and self-loops
	filtered := b.edges[:0]
	for _, e := range b.edges {
		if e.Source == e.Target {
			continue
		}
		if _, ok := b.nodes[e.Source]; !ok {
			continue
		}
		if _, ok := b.nodes[e.Target]; !ok {
			continue
		}
		filtered = append(filtered, e)
	}
	b.edges = filtered
}

// retargetEdges changes all edges from/to oldID to point to newID instead.
func (b *Builder) retargetEdges(oldID, newID NodeID) {
	for i := range b.edges {
		if b.edges[i].Source == oldID {
			b.edges[i].Source = newID
		}
		if b.edges[i].Target == oldID {
			b.edges[i].Target = newID
		}
	}
}

// extractResourceName extracts a quoted resource name from an error message.
func extractResourceName(msg, hint string) string {
	// Find text between single quotes
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

func appendUnique(slice []string, val string) []string {
	for _, v := range slice {
		if v == val {
			return slice
		}
	}
	return append(slice, val)
}
