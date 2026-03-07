// Copyright 2025 Russell Ferriday
// Licensed under the Apache License, Version 2.0
//
// Kogaro - Kubernetes Configuration Hygiene Agent

package validators

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
)

func TestSharedConfig_IsSystemNamespace(t *testing.T) {
	config := DefaultSharedConfig()

	tests := []struct {
		name      string
		namespace string
		want      bool
	}{
		{"kube-system is system namespace", "kube-system", true},
		{"kube-public is system namespace", "kube-public", true},
		{"monitoring is system namespace", "monitoring", true},
		{"cert-manager is system namespace", "cert-manager", true},
		{"kogaro-system is system namespace", "kogaro-system", true},
		{"hcloud-csi is system namespace", "hcloud-csi", true},
		{"ingress-nginx is system namespace", "ingress-nginx", true},
		{"default is system namespace", "default", true},
		{"kube-node-lease is system namespace", "kube-node-lease", true},
		{"production is not system namespace", "production", false},
		{"my-app is not system namespace", "my-app", false},
		{"test is not system namespace", "test", false},
		{"empty string is not system namespace", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := config.IsSystemNamespace(tt.namespace)
			if got != tt.want {
				t.Errorf("IsSystemNamespace(%q) = %v, want %v", tt.namespace, got, tt.want)
			}
		})
	}
}

func TestSharedConfig_IsSecurityExcludedNamespace(t *testing.T) {
	config := DefaultSharedConfig()

	tests := []struct {
		name      string
		namespace string
		want      bool
	}{
		{"kube-system is excluded", "kube-system", true},
		{"monitoring is excluded", "monitoring", true},
		{"cert-manager is excluded", "cert-manager", true},
		{"hcloud-csi is excluded", "hcloud-csi", true},
		{"ingress-nginx is excluded", "ingress-nginx", true},
		{"production is not excluded", "production", false},
		{"my-app is not excluded", "my-app", false},
		{"default is not excluded (not in security exclusions)", "default", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := config.IsSecurityExcludedNamespace(tt.namespace)
			if got != tt.want {
				t.Errorf("IsSecurityExcludedNamespace(%q) = %v, want %v", tt.namespace, got, tt.want)
			}
		})
	}
}

func TestSharedConfig_IsNetworkingExcludedNamespace(t *testing.T) {
	config := DefaultSharedConfig()

	tests := []struct {
		name      string
		namespace string
		want      bool
	}{
		{"kube-system is excluded", "kube-system", true},
		{"monitoring is excluded", "monitoring", true},
		{"cert-manager is excluded", "cert-manager", true},
		{"hcloud-csi is excluded", "hcloud-csi", true},
		{"ingress-nginx is excluded", "ingress-nginx", true},
		{"production is not excluded", "production", false},
		{"my-app is not excluded", "my-app", false},
		{"default is not excluded (not in networking exclusions)", "default", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := config.IsNetworkingExcludedNamespace(tt.namespace)
			if got != tt.want {
				t.Errorf("IsNetworkingExcludedNamespace(%q) = %v, want %v", tt.namespace, got, tt.want)
			}
		})
	}
}

func TestSharedConfig_IsDangerousRole(t *testing.T) {
	config := DefaultSharedConfig()

	tests := []struct {
		name     string
		roleName string
		want     bool
	}{
		{"admin is dangerous", "admin", true},
		{"cluster-admin is dangerous", "cluster-admin", true},
		{"edit is dangerous", "edit", true},
		{"system:admin is dangerous", "system:admin", true},
		{"view is not dangerous", "view", false},
		{"developer is not dangerous", "developer", false},
		{"read-only is not dangerous", "read-only", false},
		{"empty string is not dangerous", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := config.IsDangerousRole(tt.roleName)
			if got != tt.want {
				t.Errorf("IsDangerousRole(%q) = %v, want %v", tt.roleName, got, tt.want)
			}
		})
	}
}

func TestSharedConfig_IsProductionLikeNamespace(t *testing.T) {
	config := DefaultSharedConfig()

	tests := []struct {
		name      string
		namespace string
		want      bool
	}{
		// Exact matches
		{"exact match: prod", "prod", true},
		{"exact match: production", "production", true},
		{"exact match: live", "live", true},
		{"exact match: api", "api", true},
		{"exact match: app", "app", true},
		{"exact match: web", "web", true},
		{"exact match: service", "service", true},

		// Prefix matches
		{"prefix: prod-app", "prod-app", true},
		{"prefix: production-api", "production-api", true},
		{"prefix: api-gateway", "api-gateway", true},
		{"prefix: web-frontend", "web-frontend", true},

		// Suffix matches
		{"suffix: my-prod", "my-prod", true},
		{"suffix: company-production", "company-production", true},
		{"suffix: backend-api", "backend-api", true},
		{"suffix: main-app", "main-app", true},

		// Non-production
		{"non-prod: dev", "dev", false},
		{"non-prod: development", "development", false},
		{"non-prod: staging", "staging", false},
		{"non-prod: test", "test", false},
		{"non-prod: kube-system", "kube-system", false},
		{"non-prod: empty", "", false},

		// Edge cases - contains but not prefix/suffix
		{"contains in middle: my-prod-test", "my-prod-test", false}, // "prod" is in middle, not prefix/suffix
		{"suffix match: xprod", "xprod", true},                      // "prod" is at suffix position
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := config.IsProductionLikeNamespace(tt.namespace)
			if got != tt.want {
				t.Errorf("IsProductionLikeNamespace(%q) = %v, want %v", tt.namespace, got, tt.want)
			}
		})
	}
}

func TestSharedConfig_IsUnexposedPodPattern(t *testing.T) {
	config := DefaultSharedConfig()

	tests := []struct {
		name    string
		podName string
		want    bool
	}{
		// Matching patterns (prefix only)
		{"migration prefix", "migration-job-12345", true},
		{"backup prefix", "backup-daily-xyz", true},
		{"setup prefix", "setup-database", true},
		{"init prefix", "init-container-abc", true},

		// Non-matching
		{"regular pod", "web-app-12345", false},
		{"api pod", "api-server-xyz", false},
		{"empty string", "", false},

		// Edge cases - too short to match
		{"too short: mig", "mig", false},
		{"too short: bak", "bak", false},

		// Edge cases - contains but not prefix
		{"contains migration", "db-migration-job", false},
		{"contains backup", "daily-backup", false},

		// Exact match (length must be greater than pattern)
		{"exact match migration (too short)", "migration", false},
		{"exact match backup (too short)", "backup", false},
		{"exact match with one char", "migrationx", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := config.IsUnexposedPodPattern(tt.podName)
			if got != tt.want {
				t.Errorf("IsUnexposedPodPattern(%q) = %v, want %v", tt.podName, got, tt.want)
			}
		})
	}
}

func TestSharedConfig_IsBatchOwnerKind(t *testing.T) {
	config := DefaultSharedConfig()

	tests := []struct {
		name      string
		ownerKind string
		want      bool
	}{
		{"Job is batch kind", "Job", true},
		{"CronJob is batch kind", "CronJob", true},
		{"Deployment is not batch kind", "Deployment", false},
		{"StatefulSet is not batch kind", "StatefulSet", false},
		{"DaemonSet is not batch kind", "DaemonSet", false},
		{"ReplicaSet is not batch kind", "ReplicaSet", false},
		{"empty string is not batch kind", "", false},
		{"lowercase job is not batch kind", "job", false},
		{"lowercase cronjob is not batch kind", "cronjob", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := config.IsBatchOwnerKind(tt.ownerKind)
			if got != tt.want {
				t.Errorf("IsBatchOwnerKind(%q) = %v, want %v", tt.ownerKind, got, tt.want)
			}
		})
	}
}

func TestGetMinResourceThresholds(t *testing.T) {
	tests := []struct {
		name      string
		minCPU    string
		minMemory string
		wantCPU   bool // whether CPU quantity should be non-nil
		wantMem   bool // whether Memory quantity should be non-nil
		wantErr   bool
	}{
		{
			name:      "valid CPU and memory",
			minCPU:    "100m",
			minMemory: "128Mi",
			wantCPU:   true,
			wantMem:   true,
			wantErr:   false,
		},
		{
			name:      "only CPU",
			minCPU:    "500m",
			minMemory: "",
			wantCPU:   true,
			wantMem:   false,
			wantErr:   false,
		},
		{
			name:      "only memory",
			minCPU:    "",
			minMemory: "256Mi",
			wantCPU:   false,
			wantMem:   true,
			wantErr:   false,
		},
		{
			name:      "both empty",
			minCPU:    "",
			minMemory: "",
			wantCPU:   false,
			wantMem:   false,
			wantErr:   false,
		},
		{
			name:      "invalid CPU format",
			minCPU:    "invalid",
			minMemory: "128Mi",
			wantCPU:   false,
			wantMem:   false,
			wantErr:   true,
		},
		{
			name:      "invalid memory format",
			minCPU:    "100m",
			minMemory: "invalid",
			wantCPU:   false,
			wantMem:   false,
			wantErr:   true,
		},
		{
			name:      "CPU with different units",
			minCPU:    "1",
			minMemory: "1Gi",
			wantCPU:   true,
			wantMem:   true,
			wantErr:   false,
		},
		{
			name:      "zero values",
			minCPU:    "0",
			minMemory: "0",
			wantCPU:   true,
			wantMem:   true,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cpuQty, memQty, err := GetMinResourceThresholds(tt.minCPU, tt.minMemory)

			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("GetMinResourceThresholds() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Check CPU quantity
			if (cpuQty != nil) != tt.wantCPU {
				t.Errorf("GetMinResourceThresholds() cpuQty = %v, want non-nil = %v", cpuQty, tt.wantCPU)
			}

			// Check Memory quantity
			if (memQty != nil) != tt.wantMem {
				t.Errorf("GetMinResourceThresholds() memQty = %v, want non-nil = %v", memQty, tt.wantMem)
			}

			// If not expecting error, validate the parsed values
			if !tt.wantErr {
				if tt.wantCPU && cpuQty != nil {
					expected := resource.MustParse(tt.minCPU)
					if cpuQty.Cmp(expected) != 0 {
						t.Errorf("GetMinResourceThresholds() cpuQty = %v, want %v", cpuQty, expected)
					}
				}

				if tt.wantMem && memQty != nil {
					expected := resource.MustParse(tt.minMemory)
					if memQty.Cmp(expected) != 0 {
						t.Errorf("GetMinResourceThresholds() memQty = %v, want %v", memQty, expected)
					}
				}
			}
		})
	}
}

func TestDefaultSharedConfig(t *testing.T) {
	config := DefaultSharedConfig()

	// Verify SystemNamespaces is populated
	if len(config.SystemNamespaces) == 0 {
		t.Error("DefaultSharedConfig() SystemNamespaces should not be empty")
	}

	// Verify SecurityExcludedNamespaces is populated
	if len(config.SecurityExcludedNamespaces) == 0 {
		t.Error("DefaultSharedConfig() SecurityExcludedNamespaces should not be empty")
	}

	// Verify NetworkingExcludedNamespaces is populated
	if len(config.NetworkingExcludedNamespaces) == 0 {
		t.Error("DefaultSharedConfig() NetworkingExcludedNamespaces should not be empty")
	}

	// Verify resource recommendations have values
	if config.DefaultResourceRecommendations.DefaultCPURequest == "" {
		t.Error("DefaultSharedConfig() DefaultCPURequest should not be empty")
	}
	if config.DefaultResourceRecommendations.DefaultMemoryRequest == "" {
		t.Error("DefaultSharedConfig() DefaultMemoryRequest should not be empty")
	}

	// Verify RBAC config has dangerous roles
	if len(config.RBACConfig.DangerousRoles) == 0 {
		t.Error("DefaultSharedConfig() DangerousRoles should not be empty")
	}

	// Verify namespace patterns
	if len(config.NamespacePatterns.ProductionIndicators) == 0 {
		t.Error("DefaultSharedConfig() ProductionIndicators should not be empty")
	}

	// Verify pod patterns
	if len(config.PodPatterns.UnexposedPodPatterns) == 0 {
		t.Error("DefaultSharedConfig() UnexposedPodPatterns should not be empty")
	}
	if len(config.PodPatterns.BatchOwnerKinds) == 0 {
		t.Error("DefaultSharedConfig() BatchOwnerKinds should not be empty")
	}

	// Verify specific expected values
	expectedSystemNS := []string{
		"kube-system", "kube-public", "kube-node-lease", "default",
		"monitoring", "cert-manager", "kogaro-system", "hcloud-csi", "ingress-nginx",
	}
	for _, ns := range expectedSystemNS {
		if !config.IsSystemNamespace(ns) {
			t.Errorf("DefaultSharedConfig() should include %q in SystemNamespaces", ns)
		}
	}

	// Verify dangerous roles
	expectedDangerousRoles := []string{"admin", "cluster-admin", "edit", "system:admin"}
	for _, role := range expectedDangerousRoles {
		if !config.IsDangerousRole(role) {
			t.Errorf("DefaultSharedConfig() should include %q in DangerousRoles", role)
		}
	}

	// Verify batch owner kinds
	expectedBatchKinds := []string{"Job", "CronJob"}
	for _, kind := range expectedBatchKinds {
		if !config.IsBatchOwnerKind(kind) {
			t.Errorf("DefaultSharedConfig() should include %q in BatchOwnerKinds", kind)
		}
	}
}
