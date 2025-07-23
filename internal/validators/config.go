// Copyright 2025 Russell Ferriday
// Licensed under the Apache License, Version 2.0
//
// Kogaro - Kubernetes Configuration Hygiene Agent

// Package validators provides shared configuration for all validators.
package validators

import (
	"k8s.io/apimachinery/pkg/api/resource"
)

// SharedConfig contains common configuration values used across all validators
// to eliminate hardcoded values and make the system more configurable.
type SharedConfig struct {
	// System namespaces to exclude from various validations
	SystemNamespaces []string

	// Context-specific namespace exclusion sets
	SecurityExcludedNamespaces   []string
	NetworkingExcludedNamespaces []string

	// Default resource recommendations
	DefaultResourceRecommendations ResourceRecommendations

	// Default security context values
	DefaultSecurityContext SecurityContextDefaults

	// RBAC configuration
	RBACConfig RBACConfiguration

	// Namespace classification patterns
	NamespacePatterns NamespaceClassification

	// Pod classification patterns
	PodPatterns PodClassification
}

// ResourceRecommendations contains default resource values for recommendations
type ResourceRecommendations struct {
	// Default CPU request recommendation
	DefaultCPURequest string
	// Default memory request recommendation
	DefaultMemoryRequest string
	// Default CPU limit recommendation
	DefaultCPULimit string
	// Default memory limit recommendation
	DefaultMemoryLimit string
}

// SecurityContextDefaults contains default security context values
type SecurityContextDefaults struct {
	// Recommended non-root user ID
	RecommendedUserID int64
	// Recommended group ID
	RecommendedGroupID int64
	// Recommended fs group ID
	RecommendedFSGroup int64
	// Default service account name
	DefaultServiceAccountName string
}

// RBACConfiguration contains RBAC-related configuration
type RBACConfiguration struct {
	// List of role names considered dangerous/excessive
	DangerousRoles []string
}

// NamespaceClassification contains patterns for classifying namespaces
type NamespaceClassification struct {
	// Patterns that indicate production-like namespaces
	ProductionIndicators []string
}

// PodClassification contains patterns for classifying pods
type PodClassification struct {
	// Pod name patterns for pods that typically don't need services
	UnexposedPodPatterns []string
	// Owner reference kinds for pods that don't need services
	BatchOwnerKinds []string
}

// DefaultSharedConfig returns the default shared configuration values
func DefaultSharedConfig() SharedConfig {
	return SharedConfig{
		SystemNamespaces: []string{
			"kube-system",
			"kube-public",
			"kube-node-lease",
			"default",
			"monitoring",
		},
		SecurityExcludedNamespaces: []string{
			"kube-system",
			"kube-public",
			"kube-node-lease",
			"monitoring",
		},
		NetworkingExcludedNamespaces: []string{
			"kube-system",
			"kube-public",
			"kube-node-lease",
			"monitoring",
		},
		DefaultResourceRecommendations: ResourceRecommendations{
			DefaultCPURequest:    "100m",
			DefaultMemoryRequest: "128Mi",
			DefaultCPULimit:      "500m",
			DefaultMemoryLimit:   "256Mi",
		},
		DefaultSecurityContext: SecurityContextDefaults{
			RecommendedUserID:         1000,
			RecommendedGroupID:        3000,
			RecommendedFSGroup:        2000,
			DefaultServiceAccountName: "default",
		},
		RBACConfig: RBACConfiguration{
			DangerousRoles: []string{
				"admin",
				"cluster-admin",
				"edit",
				"system:admin",
			},
		},
		NamespacePatterns: NamespaceClassification{
			ProductionIndicators: []string{
				"prod",
				"production",
				"live",
				"api",
				"app",
				"web",
				"service",
			},
		},
		PodPatterns: PodClassification{
			UnexposedPodPatterns: []string{
				"migration",
				"backup",
				"setup",
				"init",
			},
			BatchOwnerKinds: []string{
				"Job",
				"CronJob",
			},
		},
	}
}

// IsSystemNamespace checks if a namespace is considered a system namespace
func (c *SharedConfig) IsSystemNamespace(namespace string) bool {
	for _, systemNS := range c.SystemNamespaces {
		if namespace == systemNS {
			return true
		}
	}
	return false
}

// IsSecurityExcludedNamespace checks if a namespace should be excluded from security validation
func (c *SharedConfig) IsSecurityExcludedNamespace(namespace string) bool {
	for _, excludedNS := range c.SecurityExcludedNamespaces {
		if namespace == excludedNS {
			return true
		}
	}
	return false
}

// IsNetworkingExcludedNamespace checks if a namespace should be excluded from networking validation
func (c *SharedConfig) IsNetworkingExcludedNamespace(namespace string) bool {
	for _, excludedNS := range c.NetworkingExcludedNamespaces {
		if namespace == excludedNS {
			return true
		}
	}
	return false
}

// IsDangerousRole checks if a role name is considered dangerous/excessive
func (c *SharedConfig) IsDangerousRole(roleName string) bool {
	for _, dangerous := range c.RBACConfig.DangerousRoles {
		if roleName == dangerous {
			return true
		}
	}
	return false
}

// IsProductionLikeNamespace checks if a namespace appears to be production-like
func (c *SharedConfig) IsProductionLikeNamespace(namespace string) bool {
	for _, indicator := range c.NamespacePatterns.ProductionIndicators {
		if namespace == indicator ||
			(len(namespace) > len(indicator) &&
				(namespace[:len(indicator)] == indicator ||
					namespace[len(namespace)-len(indicator):] == indicator)) {
			return true
		}
	}
	return false
}

// IsUnexposedPodPattern checks if a pod name matches patterns for pods that don't need services
func (c *SharedConfig) IsUnexposedPodPattern(podName string) bool {
	for _, pattern := range c.PodPatterns.UnexposedPodPatterns {
		if len(podName) > len(pattern) && podName[:len(pattern)] == pattern {
			return true
		}
	}
	return false
}

// IsBatchOwnerKind checks if an owner reference kind indicates a batch/temporary workload
func (c *SharedConfig) IsBatchOwnerKind(ownerKind string) bool {
	for _, batchKind := range c.PodPatterns.BatchOwnerKinds {
		if ownerKind == batchKind {
			return true
		}
	}
	return false
}

// GetMinResourceThresholds returns parsed minimum resource thresholds if configured
func GetMinResourceThresholds(minCPU, minMemory string) (*resource.Quantity, *resource.Quantity, error) {
	var minCPUQuantity, minMemoryQuantity *resource.Quantity

	if minCPU != "" {
		cpuQuantity, err := resource.ParseQuantity(minCPU)
		if err != nil {
			return nil, nil, err
		}
		minCPUQuantity = &cpuQuantity
	}

	if minMemory != "" {
		memoryQuantity, err := resource.ParseQuantity(minMemory)
		if err != nil {
			return nil, nil, err
		}
		minMemoryQuantity = &memoryQuantity
	}

	return minCPUQuantity, minMemoryQuantity, nil
}
