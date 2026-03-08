package history

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/topiaruss/kogaro/ui/pkg/graph"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSaveScanAndHistory(t *testing.T) {
	s := testStore(t)

	fg := &graph.FaultGraph{
		ScanTime: time.Now(),
		Nodes:    []graph.Node{{ID: "Pod/default/foo", Kind: "Pod", Name: "foo", Namespace: "default"}},
		Edges:    []graph.Edge{},
		Incidents: []graph.Incident{
			{
				ID:       "INC-001",
				Severity: "error",
				Category: "reference",
				Errors: []graph.ErrorDetail{
					{ErrorCode: "KOGARO-REF-004", ResourceType: "Pod", ResourceName: "foo", Namespace: "default", Message: "missing configmap", Severity: "error"},
				},
			},
		},
	}

	scanID, err := s.SaveScan("docker-desktop", fg)
	if err != nil {
		t.Fatal(err)
	}
	if scanID != 1 {
		t.Fatalf("expected scan ID 1, got %d", scanID)
	}

	history, err := s.GetScanHistory("docker-desktop", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 scan, got %d", len(history))
	}
	if history[0].NodeCount != 1 {
		t.Errorf("expected 1 node, got %d", history[0].NodeCount)
	}
	if history[0].ErrorCount != 1 {
		t.Errorf("expected 1 error, got %d", history[0].ErrorCount)
	}
}

func TestGetScanErrors(t *testing.T) {
	s := testStore(t)

	fg := &graph.FaultGraph{
		ScanTime: time.Now(),
		Nodes:    []graph.Node{{ID: "Pod/default/foo"}},
		Incidents: []graph.Incident{{
			ID: "INC-001",
			Errors: []graph.ErrorDetail{
				{ErrorCode: "REF-004", ResourceType: "Pod", ResourceName: "foo", Namespace: "default", Message: "missing cm", Severity: "error"},
				{ErrorCode: "REF-006", ResourceType: "Pod", ResourceName: "foo", Namespace: "default", Message: "missing secret", Severity: "error"},
			},
		}},
	}
	scanID, _ := s.SaveScan("ctx", fg)

	errors, err := s.GetScanErrors(scanID)
	if err != nil {
		t.Fatal(err)
	}
	if len(errors) != 2 {
		t.Fatalf("expected 2 errors, got %d", len(errors))
	}
}

func TestDiffScans(t *testing.T) {
	s := testStore(t)

	fg1 := &graph.FaultGraph{
		ScanTime: time.Now().Add(-time.Hour),
		Nodes:    []graph.Node{{ID: "Pod/default/foo"}},
		Incidents: []graph.Incident{{
			ID: "INC-001",
			Errors: []graph.ErrorDetail{
				{ErrorCode: "REF-004", ResourceType: "Pod", ResourceName: "foo", Namespace: "default", Message: "missing cm", Severity: "error"},
			},
		}},
	}
	id1, _ := s.SaveScan("ctx", fg1)

	fg2 := &graph.FaultGraph{
		ScanTime: time.Now(),
		Nodes:    []graph.Node{{ID: "Pod/default/bar"}},
		Incidents: []graph.Incident{{
			ID: "INC-002",
			Errors: []graph.ErrorDetail{
				{ErrorCode: "REF-006", ResourceType: "Pod", ResourceName: "bar", Namespace: "default", Message: "missing secret", Severity: "error"},
			},
		}},
	}
	id2, _ := s.SaveScan("ctx", fg2)

	diff, err := s.DiffScans(id1, id2)
	if err != nil {
		t.Fatal(err)
	}
	if len(diff.NewErrors) != 1 {
		t.Errorf("expected 1 new error, got %d", len(diff.NewErrors))
	}
	if len(diff.FixedErrors) != 1 {
		t.Errorf("expected 1 fixed error, got %d", len(diff.FixedErrors))
	}
	if diff.Unchanged != 0 {
		t.Errorf("expected 0 unchanged, got %d", diff.Unchanged)
	}
}

func TestDumpJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	fg := &graph.FaultGraph{
		ScanTime: time.Now(),
		Nodes:    []graph.Node{{ID: "Pod/default/test", Kind: "Pod"}},
	}

	if err := DumpJSON(fg); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".kogaro", "last-scan.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty JSON")
	}
}

func TestHistoryFiltersByContext(t *testing.T) {
	s := testStore(t)

	fg := &graph.FaultGraph{ScanTime: time.Now(), Nodes: []graph.Node{{ID: "Pod/default/x"}}}
	s.SaveScan("ctx-a", fg)
	s.SaveScan("ctx-b", fg)
	s.SaveScan("ctx-a", fg)

	histA, _ := s.GetScanHistory("ctx-a", 10)
	histB, _ := s.GetScanHistory("ctx-b", 10)

	if len(histA) != 2 {
		t.Errorf("expected 2 scans for ctx-a, got %d", len(histA))
	}
	if len(histB) != 1 {
		t.Errorf("expected 1 scan for ctx-b, got %d", len(histB))
	}
}
