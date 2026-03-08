package graph

import (
	"fmt"
	"strings"

	"github.com/topiaruss/kogaro/internal/validators"
)

// unionFind implements a simple union-find (disjoint set) data structure.
type unionFind struct {
	parent map[int]int
	rank   map[int]int
}

func newUnionFind() *unionFind {
	return &unionFind{
		parent: make(map[int]int),
		rank:   make(map[int]int),
	}
}

func (uf *unionFind) find(x int) int {
	if _, ok := uf.parent[x]; !ok {
		uf.parent[x] = x
	}
	if uf.parent[x] != x {
		uf.parent[x] = uf.find(uf.parent[x])
	}
	return uf.parent[x]
}

func (uf *unionFind) union(x, y int) {
	rx, ry := uf.find(x), uf.find(y)
	if rx == ry {
		return
	}
	if uf.rank[rx] < uf.rank[ry] {
		rx, ry = ry, rx
	}
	uf.parent[ry] = rx
	if uf.rank[rx] == uf.rank[ry] {
		uf.rank[rx]++
	}
}

// BuildIncidents groups validation errors into incidents using union-find.
// Errors are only merged when they share resources within the same namespace.
func BuildIncidents(errs []validators.ValidationError, nodes map[NodeID]*Node) []Incident {
	if len(errs) == 0 {
		return nil
	}

	uf := newUnionFind()

	// Index errors by namespace-scoped source resource
	sourceIndex := make(map[string][]int) // "namespace/resourceKey" -> error indices

	for i, ve := range errs {
		// Scope source key by namespace to prevent cross-namespace merging
		sourceKey := ve.Namespace + "/" + ve.GetResourceKey()
		sourceIndex[sourceKey] = append(sourceIndex[sourceKey], i)
	}

	// Merge errors sharing the same source resource (within same namespace)
	for _, indices := range sourceIndex {
		for j := 1; j < len(indices); j++ {
			uf.union(indices[0], indices[j])
		}
	}

	// Merge errors sharing the same namespace-scoped target resource
	// Skip cluster-scoped resources (no namespace) to prevent mega-merges
	targetIndex := make(map[string][]int)
	for i, ve := range errs {
		for _, rel := range ve.RelatedResources {
			// Only merge on namespace-scoped targets
			// RelatedResources format varies; scope by error's namespace
			if ve.Namespace != "" {
				targetKey := ve.Namespace + "/" + rel
				targetIndex[targetKey] = append(targetIndex[targetKey], i)
			}
		}
	}
	for _, indices := range targetIndex {
		for j := 1; j < len(indices); j++ {
			uf.union(indices[0], indices[j])
		}
	}

	// Merge errors referencing the same missing resource (namespace-scoped)
	missingIndex := make(map[string][]int)
	for i, ve := range errs {
		targetName := extractResourceName(ve.Message, "")
		if targetName != "" && ve.Namespace != "" {
			key := ve.Namespace + "/" + targetName
			missingIndex[key] = append(missingIndex[key], i)
		}
	}
	for _, indices := range missingIndex {
		for j := 1; j < len(indices); j++ {
			uf.union(indices[0], indices[j])
		}
	}

	// Group errors by root
	groups := make(map[int][]int)
	for i := range errs {
		root := uf.find(i)
		groups[root] = append(groups[root], i)
	}

	// Build incidents
	incidents := make([]Incident, 0, len(groups))
	incidentNum := 0
	for _, indices := range groups {
		incidentNum++
		inc := buildIncident(incidentNum, errs, indices, nodes)
		incidents = append(incidents, inc)
	}

	return incidents
}

func buildIncident(num int, errs []validators.ValidationError, indices []int, nodes map[NodeID]*Node) Incident {
	inc := Incident{
		ID: fmt.Sprintf("INC-%03d", num),
	}

	affectedSet := make(map[NodeID]bool)
	rootCauseSet := make(map[NodeID]bool)
	codeSet := make(map[string]bool)
	seenErrors := make(map[string]bool)
	namespaceSet := make(map[string]bool)
	maxSeverity := validators.SeverityInfo

	for _, i := range indices {
		ve := errs[i]

		// Deduplicate errors by code + resource + namespace
		dedupKey := ve.ErrorCode + "/" + ve.Namespace + "/" + ve.ResourceName
		if seenErrors[dedupKey] {
			continue
		}
		seenErrors[dedupKey] = true

		// Error detail
		inc.Errors = append(inc.Errors, ErrorDetail{
			ErrorCode:       ve.ErrorCode,
			Message:         ve.Message,
			Severity:        string(ve.Severity),
			RemediationHint: ve.RemediationHint,
			ResourceType:    ve.ResourceType,
			ResourceName:    ve.ResourceName,
			Namespace:       ve.Namespace,
		})

		// Track codes
		if !codeSet[ve.ErrorCode] {
			codeSet[ve.ErrorCode] = true
			inc.ErrorCodes = append(inc.ErrorCodes, ve.ErrorCode)
		}

		// Track affected nodes
		sourceID := MakeNodeID(ve.ResourceType, ve.Namespace, ve.ResourceName)
		affectedSet[sourceID] = true

		// Track root causes: only nodes referenced by THIS error's related resources
		for _, rel := range ve.RelatedResources {
			// Try to find matching node by name
			for id, n := range nodes {
				if n.IsRootCause && strings.Contains(string(id), rel) {
					rootCauseSet[id] = true
				}
			}
		}

		// Category from error code
		if inc.Category == "" {
			inc.Category = categoryFromCode(ve.ErrorCode)
		}

		// Namespace
		if ve.Namespace != "" {
			namespaceSet[ve.Namespace] = true
		}

		// Severity
		if severityRank(ve.Severity) > severityRank(maxSeverity) {
			maxSeverity = ve.Severity
		}
	}

	inc.Severity = string(maxSeverity)

	for id := range affectedSet {
		inc.AffectedNodes = append(inc.AffectedNodes, id)
	}
	for id := range rootCauseSet {
		inc.RootCauses = append(inc.RootCauses, id)
	}

	// Pick namespace
	for ns := range namespaceSet {
		inc.Namespace = ns
		break
	}

	// Generate summary
	inc.Summary = generateSummary(errs, indices)

	return inc
}

func categoryFromCode(code string) string {
	switch {
	case strings.HasPrefix(code, "KOGARO-REF"):
		return "reference"
	case strings.HasPrefix(code, "KOGARO-NET"):
		return "networking"
	case strings.HasPrefix(code, "KOGARO-SEC"):
		return "security"
	case strings.HasPrefix(code, "KOGARO-RES"):
		return "resource_limits"
	case strings.HasPrefix(code, "KOGARO-IMG"):
		return "image"
	default:
		return "unknown"
	}
}

func severityRank(s validators.Severity) int {
	switch s {
	case validators.SeverityError:
		return 3
	case validators.SeverityWarning:
		return 2
	case validators.SeverityInfo:
		return 1
	default:
		return 0
	}
}

func generateSummary(errs []validators.ValidationError, indices []int) string {
	if len(indices) == 0 {
		return ""
	}

	// Use the first error's message as the primary summary
	ve := errs[indices[0]]

	// For single errors, use the message directly
	if len(indices) == 1 {
		return ve.Message
	}

	// For multiple errors on the same resource, lead with the message
	// Check if all errors are on the same resource
	sameResource := true
	for _, i := range indices[1:] {
		if errs[i].ResourceName != ve.ResourceName || errs[i].Namespace != ve.Namespace {
			sameResource = false
			break
		}
	}

	if sameResource {
		// Deduplicate
		seen := make(map[string]bool)
		unique := 0
		for _, i := range indices {
			key := errs[i].ErrorCode + "/" + errs[i].ResourceName
			if !seen[key] {
				seen[key] = true
				unique++
			}
		}
		if unique == 1 {
			return ve.Message
		}
		return fmt.Sprintf("%s — %d issues on %s", ve.Message, unique, ve.ResourceName)
	}

	// Multiple resources: count unique resources
	resources := make(map[string]bool)
	for _, i := range indices {
		resources[errs[i].ResourceName] = true
	}
	return fmt.Sprintf("%s — affects %d resources", ve.Message, len(resources))
}
