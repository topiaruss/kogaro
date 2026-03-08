package adaptive

import (
	"strings"
	"testing"
)

func TestDecideForNode_NET001(t *testing.T) {
	nc := &NodeContext{
		Kind: "Service", Name: "web-svc", Namespace: "default",
		ErrorCode: "KOGARO-NET-001",
		Selector:  "app=web",
	}

	r := DecideForNode(nc)
	if r == nil {
		t.Fatal("expected result")
	}
	if r.TreePath != "NET-001/selector-mismatch" {
		t.Errorf("treePath = %q", r.TreePath)
	}
	if len(r.Options) < 2 {
		t.Errorf("expected at least 2 options, got %d", len(r.Options))
	}
}

func TestDecideForNode_NET001_NoSelector(t *testing.T) {
	nc := &NodeContext{
		Kind: "Service", Name: "web-svc", Namespace: "default",
		ErrorCode: "KOGARO-NET-001",
	}

	r := DecideForNode(nc)
	if r == nil {
		t.Fatal("expected result")
	}
	// Without selector, should have 1 option (investigate only)
	if len(r.Options) != 1 {
		t.Errorf("expected 1 option without selector, got %d", len(r.Options))
	}
}

func TestDecideForNode_NET002(t *testing.T) {
	nc := &NodeContext{
		Kind: "Service", Name: "api-svc", Namespace: "prod",
		ErrorCode: "KOGARO-NET-002",
		Selector:  "app=api,tier=backend",
	}

	r := DecideForNode(nc)
	if r == nil {
		t.Fatal("expected result")
	}
	if r.TreePath != "NET-002/no-endpoints" {
		t.Errorf("treePath = %q", r.TreePath)
	}
	if len(r.Options) < 2 {
		t.Errorf("expected at least 2 options with selector, got %d", len(r.Options))
	}
}

func TestDecideForNode_NET003(t *testing.T) {
	nc := &NodeContext{
		Kind: "Service", Name: "web-svc", Namespace: "default",
		ErrorCode: "KOGARO-NET-003",
		Selector:  "app=web",
	}

	r := DecideForNode(nc)
	if r == nil {
		t.Fatal("expected result")
	}
	if r.TreePath != "NET-003/port-mismatch" {
		t.Errorf("treePath = %q", r.TreePath)
	}
	if len(r.Warnings) == 0 {
		t.Error("expected warnings about port mismatch")
	}
}

func TestDecideForNode_NET005(t *testing.T) {
	nc := &NodeContext{
		Kind: "NetworkPolicy", Name: "deny-all", Namespace: "default",
		ErrorCode: "KOGARO-NET-005",
	}

	r := DecideForNode(nc)
	if r == nil {
		t.Fatal("expected result")
	}
	if len(r.Options) != 2 {
		t.Fatalf("expected 2 options (investigate + delete), got %d", len(r.Options))
	}
	if r.Options[1].Risk != "medium" {
		t.Errorf("delete option risk = %q, want medium", r.Options[1].Risk)
	}
}

func TestDecideForNode_NET007(t *testing.T) {
	nc := &NodeContext{
		Kind: "Ingress", Name: "web-ingress", Namespace: "default",
		ErrorCode:  "KOGARO-NET-007",
		TargetName: "missing-svc",
	}

	r := DecideForNode(nc)
	if r == nil {
		t.Fatal("expected result")
	}
	if !strings.Contains(r.KBInsights[0], "missing-svc") {
		t.Errorf("insight should mention target: %q", r.KBInsights[0])
	}
}

func TestDecideForNode_NET009(t *testing.T) {
	nc := &NodeContext{
		Kind: "Ingress", Name: "web-ingress", Namespace: "default",
		ErrorCode:  "KOGARO-NET-009",
		TargetName: "web-svc",
	}

	r := DecideForNode(nc)
	if r == nil {
		t.Fatal("expected result")
	}
	if r.TreePath != "NET-009/ingress-no-backends" {
		t.Errorf("treePath = %q", r.TreePath)
	}
}

func TestDecideForNode_REF001(t *testing.T) {
	nc := &NodeContext{
		Kind: "Ingress", Name: "web", Namespace: "default",
		ErrorCode:  "KOGARO-REF-001",
		TargetName: "nginx-internal",
	}

	r := DecideForNode(nc)
	if r == nil {
		t.Fatal("expected result")
	}
	if r.TreePath != "REF-001/missing-ingressclass" {
		t.Errorf("treePath = %q", r.TreePath)
	}
}

func TestDecideForNode_REF004_ConfigMapVolume(t *testing.T) {
	nc := &NodeContext{
		Kind: "Deployment", Name: "app", Namespace: "default",
		ErrorCode:  "KOGARO-REF-004",
		TargetName: "app-config",
		OwnerKind:  "Deployment",
		OwnerName:  "app",
	}

	r := DecideForNode(nc)
	if r == nil {
		t.Fatal("expected result")
	}
	if !strings.Contains(r.TreePath, "missing-configmap") {
		t.Errorf("treePath = %q", r.TreePath)
	}
	// Should have create + make optional options
	if len(r.Options) < 2 {
		t.Errorf("expected at least 2 options (create + optional), got %d", len(r.Options))
	}
}

func TestDecideForNode_REF003_TLSSecret(t *testing.T) {
	nc := &NodeContext{
		Kind: "Ingress", Name: "web", Namespace: "default",
		ErrorCode:  "KOGARO-REF-003",
		TargetName: "web-tls",
	}

	r := DecideForNode(nc)
	if r == nil {
		t.Fatal("expected result")
	}
	if !strings.Contains(r.TreePath, "missing-secret/tls") {
		t.Errorf("treePath = %q", r.TreePath)
	}
	// Should mention cert-manager
	hasCertMgr := false
	for _, opt := range r.Options {
		if strings.Contains(opt.Label, "cert-manager") {
			hasCertMgr = true
		}
	}
	if !hasCertMgr {
		t.Error("expected cert-manager option for TLS secret")
	}
}

func TestDecideForNode_REF006_SecretVolume(t *testing.T) {
	nc := &NodeContext{
		Kind: "Deployment", Name: "app", Namespace: "default",
		ErrorCode:  "KOGARO-REF-006",
		TargetName: "db-creds",
	}

	r := DecideForNode(nc)
	if r == nil {
		t.Fatal("expected result")
	}
	if !strings.Contains(r.TreePath, "missing-secret/volume") {
		t.Errorf("treePath = %q", r.TreePath)
	}
}

func TestDecideForNode_REF009_StorageClass(t *testing.T) {
	nc := &NodeContext{
		Kind: "PersistentVolumeClaim", Name: "data-pvc", Namespace: "default",
		ErrorCode:  "KOGARO-REF-009",
		TargetName: "fast-ssd",
	}

	r := DecideForNode(nc)
	if r == nil {
		t.Fatal("expected result")
	}
	if r.TreePath != "REF-009/missing-storageclass" {
		t.Errorf("treePath = %q", r.TreePath)
	}
	// Should suggest listing StorageClasses
	found := false
	for _, opt := range r.Options {
		for _, cmd := range opt.Commands {
			if strings.Contains(cmd.Command, "storageclasses") {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected command to list StorageClasses")
	}
}

func TestDecideForNode_REF011_ServiceAccount(t *testing.T) {
	nc := &NodeContext{
		Kind: "Deployment", Name: "app", Namespace: "default",
		ErrorCode:  "KOGARO-REF-011",
		TargetName: "app-sa",
		OwnerKind:  "Deployment",
		OwnerName:  "app",
	}

	r := DecideForNode(nc)
	if r == nil {
		t.Fatal("expected result")
	}
	if r.TreePath != "REF-011/missing-serviceaccount" {
		t.Errorf("treePath = %q", r.TreePath)
	}
	if len(r.Options) != 2 {
		t.Fatalf("expected 2 options (create + use default), got %d", len(r.Options))
	}
	if r.Options[1].Risk != "medium" {
		t.Errorf("use-default option risk = %q, want medium", r.Options[1].Risk)
	}
}

func TestDecideForNode_Nil(t *testing.T) {
	r := DecideForNode(nil)
	if r != nil {
		t.Error("expected nil for nil context")
	}
}

func TestDecideForNode_UnknownCode(t *testing.T) {
	nc := &NodeContext{
		Kind: "Pod", Name: "test", Namespace: "default",
		ErrorCode: "KOGARO-IMG-001",
	}

	r := DecideForNode(nc)
	if r != nil {
		t.Error("expected nil for unsupported code")
	}
}
