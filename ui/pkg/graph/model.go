package graph

import "time"

// NodeID uniquely identifies a Kubernetes resource as "Kind/Namespace/Name" or "Kind//Name" for cluster-scoped.
type NodeID string

// HealthState represents the health of a node or edge in the fault graph.
type HealthState string

const (
	HealthBroken  HealthState = "broken"
	HealthDegraded HealthState = "degraded"
	HealthHealthy HealthState = "healthy"
	HealthMissing HealthState = "missing"
	HealthUnknown HealthState = "unknown"
)

// Node represents a Kubernetes resource in the fault graph.
type Node struct {
	ID            NodeID            `json:"id"`
	Kind          string            `json:"kind"`
	Name          string            `json:"name"`
	Namespace     string            `json:"namespace"`
	Health        HealthState       `json:"health"`
	ErrorCodes    []string          `json:"errorCodes,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"`
	IsFaultOrigin bool              `json:"isFaultOrigin"`
	IsRootCause   bool              `json:"isRootCause"`
	DistFromFault int               `json:"distFromFault"`
	Details       map[string]string `json:"details,omitempty"`
	ResourceCount int               `json:"resourceCount,omitempty"` // >1 when collapsed (e.g. 10 Pods → 1 Deployment)
	CollapsedFrom []NodeID          `json:"collapsedFrom,omitempty"` // original node IDs that were collapsed
}

// EdgeType describes the relationship between two nodes.
type EdgeType string

const (
	EdgeReference     EdgeType = "reference"
	EdgeSelector      EdgeType = "selector"
	EdgeExposure      EdgeType = "exposure"
	EdgeOwnership     EdgeType = "ownership"
	EdgeNetworkPolicy EdgeType = "network_policy"
	EdgeTLS           EdgeType = "tls"
	EdgeStorageClass  EdgeType = "storage_class"
)

// Edge represents a relationship between two nodes.
type Edge struct {
	Source     NodeID      `json:"source"`
	Target     NodeID      `json:"target"`
	Type       EdgeType    `json:"type"`
	Health     HealthState `json:"health"`
	Label      string      `json:"label,omitempty"`
	ErrorCodes []string    `json:"errorCodes,omitempty"`
}

// ErrorDetail holds details about a single validation error for display.
type ErrorDetail struct {
	ErrorCode       string `json:"errorCode"`
	Message         string `json:"message"`
	Severity        string `json:"severity"`
	RemediationHint string `json:"remediationHint,omitempty"`
	ResourceType    string `json:"resourceType"`
	ResourceName    string `json:"resourceName"`
	Namespace       string `json:"namespace"`
}

// Incident groups related errors into a single actionable unit.
type Incident struct {
	ID            string        `json:"id"`
	Severity      string        `json:"severity"`
	Summary       string        `json:"summary"`
	ErrorCodes    []string      `json:"errorCodes"`
	AffectedNodes []NodeID      `json:"affectedNodes"`
	RootCauses    []NodeID      `json:"rootCauses"`
	Category      string        `json:"category"`
	Namespace     string        `json:"namespace"`
	Errors        []ErrorDetail `json:"errors"`
}

// FaultGraph is the top-level structure returned to the frontend.
type FaultGraph struct {
	Nodes     []Node     `json:"nodes"`
	Edges     []Edge     `json:"edges"`
	Incidents []Incident `json:"incidents"`
	ScanTime  time.Time  `json:"scanTime"`
}

// NodeDetailResponse provides full evidence for a single node.
type NodeDetailResponse struct {
	Node             Node          `json:"node"`
	Errors           []ErrorDetail `json:"errors"`
	IncomingEdges    []Edge        `json:"incomingEdges"`
	OutgoingEdges    []Edge        `json:"outgoingEdges"`
	OwnerChain       []NodeID      `json:"ownerChain,omitempty"`
	DependentCount   int           `json:"dependentCount"`
}

// MakeNodeID constructs a NodeID from kind, namespace, and name.
func MakeNodeID(kind, namespace, name string) NodeID {
	return NodeID(kind + "/" + namespace + "/" + name)
}
