package adaptive

import (
	"testing"
)

func ptrBool(b bool) *bool    { return &b }
func ptrInt64(i int64) *int64 { return &i }

func TestDecideSEC002_KnownImage(t *testing.T) {
	profile := &WorkloadProfile{
		Kind: "Deployment", Name: "web", Namespace: "default",
		Containers: []ContainerProfile{{
			Name: "nginx", Image: "nginx:1.25", ImageBase: "nginx",
			IsKnownImage: true,
			KnownTraits:  &ImageTraits{DefaultUID: 101, DefaultGID: 101},
		}},
	}

	r := Decide("KOGARO-SEC-002", profile, "nginx")
	if r == nil {
		t.Fatal("expected decision result")
	}
	if r.TreePath != "SEC-002/known-image/uid-101" {
		t.Errorf("treePath = %q, want SEC-002/known-image/uid-101", r.TreePath)
	}
	if len(r.Options) != 2 {
		t.Fatalf("expected 2 options, got %d", len(r.Options))
	}
	if r.Options[0].Risk != "low" {
		t.Errorf("first option risk = %q, want low", r.Options[0].Risk)
	}
	if r.Options[1].Risk != "medium" {
		t.Errorf("second option risk = %q, want medium", r.Options[1].Risk)
	}
}

func TestDecideSEC002_KnownImage_UID65534(t *testing.T) {
	profile := &WorkloadProfile{
		Kind: "Deployment", Name: "app", Namespace: "default",
		Containers: []ContainerProfile{{
			Name: "alpine", Image: "alpine:3.18", ImageBase: "alpine",
			IsKnownImage: true,
			KnownTraits:  &ImageTraits{DefaultUID: 65534, DefaultGID: 65534},
		}},
	}

	r := Decide("KOGARO-SEC-002", profile, "alpine")
	if r == nil {
		t.Fatal("expected decision result")
	}
	// Should only have 1 option since known UID is already 65534
	if len(r.Options) != 1 {
		t.Fatalf("expected 1 option (UID already 65534), got %d", len(r.Options))
	}
}

func TestDecideSEC002_UnknownImage(t *testing.T) {
	profile := &WorkloadProfile{
		Kind: "Deployment", Name: "app", Namespace: "prod",
		Containers: []ContainerProfile{{
			Name: "myapp", Image: "myregistry.io/myapp:v2", ImageBase: "myregistry.io/myapp",
			IsKnownImage: false,
		}},
	}

	r := Decide("KOGARO-SEC-002", profile, "myapp")
	if r == nil {
		t.Fatal("expected decision result")
	}
	if r.TreePath != "SEC-002/unknown-image" {
		t.Errorf("treePath = %q, want SEC-002/unknown-image", r.TreePath)
	}
	if len(r.Warnings) == 0 {
		t.Error("expected warnings for unknown image")
	}
	if len(r.Options) != 1 {
		t.Fatalf("expected 1 option, got %d", len(r.Options))
	}
	if r.Options[0].Risk != "medium" {
		t.Errorf("option risk = %q, want medium", r.Options[0].Risk)
	}
}

func TestDecideSEC010_KnownImage(t *testing.T) {
	profile := &WorkloadProfile{
		Kind: "Deployment", Name: "cache", Namespace: "default",
		Containers: []ContainerProfile{{
			Name: "memcached", Image: "memcached:1.6", ImageBase: "memcached",
			IsKnownImage: true,
			KnownTraits:  &ImageTraits{DefaultUID: 11211, SafeForReadOnlyFS: true, WritablePaths: []string{"/tmp"}},
		}},
	}

	r := Decide("KOGARO-SEC-010", profile, "memcached")
	if r == nil {
		t.Fatal("expected decision result")
	}
	if !contains(r.TreePath, "SEC-010/known-image") {
		t.Errorf("treePath = %q, want prefix SEC-010/known-image", r.TreePath)
	}
	if len(r.Options) != 1 {
		t.Fatalf("expected 1 option, got %d", len(r.Options))
	}
	// Should include readOnlyRootFilesystem since SafeForReadOnlyFS=true
	if !containsStr(r.Options[0].Commands[0].Command, "readOnlyRootFilesystem") {
		t.Error("expected readOnlyRootFilesystem in command for safe image")
	}
}

func TestDecideSEC010_UnknownImage(t *testing.T) {
	profile := &WorkloadProfile{
		Kind: "Deployment", Name: "app", Namespace: "default",
		Containers: []ContainerProfile{{
			Name: "app", Image: "custom:v1", ImageBase: "custom",
			IsKnownImage: false,
		}},
	}

	r := Decide("KOGARO-SEC-010", profile, "app")
	if r == nil {
		t.Fatal("expected decision result")
	}
	if r.TreePath != "SEC-010/unknown-image" {
		t.Errorf("treePath = %q, want SEC-010/unknown-image", r.TreePath)
	}
	// Should NOT include readOnlyRootFilesystem
	if containsStr(r.Options[0].Commands[0].Command, "readOnlyRootFilesystem") {
		t.Error("should not include readOnlyRootFilesystem for unknown image")
	}
}

func TestDecideSEC003(t *testing.T) {
	profile := &WorkloadProfile{
		Kind: "Deployment", Name: "web", Namespace: "default",
		Containers: []ContainerProfile{{
			Name: "web", Image: "nginx:1.25", ImageBase: "nginx",
		}},
	}

	r := Decide("KOGARO-SEC-003", profile, "web")
	if r == nil {
		t.Fatal("expected decision result")
	}
	if r.TreePath != "SEC-003/deny-escalation" {
		t.Errorf("treePath = %q, want SEC-003/deny-escalation", r.TreePath)
	}
	if r.Options[0].Risk != "low" {
		t.Errorf("risk = %q, want low", r.Options[0].Risk)
	}
}

func TestDecideSEC003_PrivPort(t *testing.T) {
	profile := &WorkloadProfile{
		Kind: "Deployment", Name: "web", Namespace: "default",
		Containers: []ContainerProfile{{
			Name: "web", Image: "nginx:1.25", ImageBase: "nginx",
			NeedsPrivPort: true,
		}},
	}

	r := Decide("KOGARO-SEC-003", profile, "web")
	if r == nil {
		t.Fatal("expected decision result")
	}
	if len(r.Warnings) == 0 {
		t.Error("expected warning about privileged port")
	}
}

func TestDecideSEC005(t *testing.T) {
	profile := &WorkloadProfile{
		Kind: "DaemonSet", Name: "agent", Namespace: "kube-system",
		Containers: []ContainerProfile{{
			Name: "agent", Image: "monitoring-agent:v1", ImageBase: "monitoring-agent",
		}},
	}

	r := Decide("KOGARO-SEC-005", profile, "agent")
	if r == nil {
		t.Fatal("expected decision result")
	}
	if r.Options[0].Risk != "medium" {
		t.Errorf("risk = %q, want medium", r.Options[0].Risk)
	}
	if len(r.Options[0].Rollback) == 0 {
		t.Error("expected rollback command")
	}
}

func TestDecideSEC006_SafeImage(t *testing.T) {
	profile := &WorkloadProfile{
		Kind: "Deployment", Name: "app", Namespace: "default",
		Containers: []ContainerProfile{{
			Name: "app", Image: "alpine:3.18", ImageBase: "alpine",
			IsKnownImage: true,
			KnownTraits:  &ImageTraits{SafeForReadOnlyFS: true, WritablePaths: []string{"/tmp"}},
		}},
	}

	r := Decide("KOGARO-SEC-006", profile, "app")
	if r == nil {
		t.Fatal("expected decision result")
	}
	if r.TreePath != "SEC-006/known-image/safe-readonly" {
		t.Errorf("treePath = %q", r.TreePath)
	}
	if r.Options[0].Risk != "low" {
		t.Errorf("risk = %q, want low", r.Options[0].Risk)
	}
}

func TestDecideSEC006_NeedsWritable(t *testing.T) {
	profile := &WorkloadProfile{
		Kind: "Deployment", Name: "web", Namespace: "default",
		Containers: []ContainerProfile{{
			Name: "nginx", Image: "nginx:1.25", ImageBase: "nginx",
			IsKnownImage: true,
			KnownTraits:  &ImageTraits{SafeForReadOnlyFS: false, WritablePaths: []string{"/var/cache/nginx", "/var/run", "/tmp"}},
		}},
	}

	r := Decide("KOGARO-SEC-006", profile, "nginx")
	if r == nil {
		t.Fatal("expected decision result")
	}
	if r.TreePath != "SEC-006/known-image/needs-writable" {
		t.Errorf("treePath = %q", r.TreePath)
	}
	if len(r.Options) != 2 {
		t.Fatalf("expected 2 options, got %d", len(r.Options))
	}
	// First option should be medium risk (readOnlyFS + emptyDir)
	if r.Options[0].Risk != "medium" {
		t.Errorf("first option risk = %q, want medium", r.Options[0].Risk)
	}
	// Second should be skip (low risk)
	if r.Options[1].Risk != "low" {
		t.Errorf("second option risk = %q, want low", r.Options[1].Risk)
	}
}

func TestDecideRES(t *testing.T) {
	profile := &WorkloadProfile{
		Kind: "Deployment", Name: "app", Namespace: "default",
		Containers: []ContainerProfile{{
			Name: "app", Image: "myapp:v1", ImageBase: "myapp",
			IsKnownImage: false,
		}},
	}

	r := Decide("KOGARO-RES-002", profile, "app")
	if r == nil {
		t.Fatal("expected decision result")
	}
	if r.TreePath != "RES-002/resources" {
		t.Errorf("treePath = %q", r.TreePath)
	}
	if len(r.Options) < 1 {
		t.Fatal("expected at least 1 option")
	}
}

func TestDecideRES_DataIntensive(t *testing.T) {
	profile := &WorkloadProfile{
		Kind: "StatefulSet", Name: "db", Namespace: "default",
		Containers: []ContainerProfile{{
			Name: "postgres", Image: "postgres:16", ImageBase: "postgres",
			IsKnownImage: true,
			KnownTraits:  &ImageTraits{SafeForReadOnlyFS: false, WritablePaths: []string{"/var/lib/postgresql/data"}},
		}},
	}

	r := Decide("KOGARO-RES-002", profile, "postgres")
	if r == nil {
		t.Fatal("expected decision result")
	}
	// Should have 2 options: conservative + generous
	if len(r.Options) != 2 {
		t.Fatalf("expected 2 options (conservative + generous), got %d", len(r.Options))
	}
}

func TestDecideRESQoS(t *testing.T) {
	profile := &WorkloadProfile{
		Kind: "Deployment", Name: "web", Namespace: "default",
		Containers: []ContainerProfile{{
			Name: "web", Image: "nginx:1.25", ImageBase: "nginx",
			Resources: &ResourceSummary{
				CPULimit: "25m", MemoryLimit: "32Mi",
			},
		}},
	}

	r := Decide("KOGARO-RES-UNKNOWN", profile, "web")
	if r == nil {
		t.Fatal("expected decision result")
	}
	// Should use existing limits, not hardcoded defaults
	if !containsStr(r.Options[0].Commands[0].Command, "25m") {
		t.Error("expected command to use existing CPU limit 25m")
	}
	if !containsStr(r.Options[0].Commands[0].Command, "32Mi") {
		t.Error("expected command to use existing memory limit 32Mi")
	}
}

func TestDecideUnknownErrorCode(t *testing.T) {
	profile := &WorkloadProfile{
		Kind: "Deployment", Name: "app", Namespace: "default",
		Containers: []ContainerProfile{{Name: "app"}},
	}

	r := Decide("KOGARO-NET-001", profile, "app")
	if r != nil {
		t.Error("expected nil for unsupported error code")
	}
}

func TestDecideNilProfile(t *testing.T) {
	r := Decide("KOGARO-SEC-002", nil, "app")
	if r != nil {
		t.Error("expected nil for nil profile")
	}
}

func TestDecideFallbackContainer(t *testing.T) {
	profile := &WorkloadProfile{
		Kind: "Deployment", Name: "app", Namespace: "default",
		Containers: []ContainerProfile{{
			Name: "main", Image: "myapp:v1", ImageBase: "myapp",
		}},
	}

	// Request container name that doesn't exist — should fall back to first container
	r := Decide("KOGARO-SEC-003", profile, "nonexistent")
	if r == nil {
		t.Fatal("expected decision result with fallback container")
	}
	if !containsStr(r.Options[0].Commands[0].Command, "main") {
		t.Error("expected command to use fallback container name 'main'")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstr(s, substr))
}

func findSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
