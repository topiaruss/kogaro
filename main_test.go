package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIValidation(t *testing.T) {
	// Build the binary first
	buildCmd := exec.Command("go", "build", "-o", "kogaro-test", ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build kogaro binary: %v", err)
	}
	defer func() { _ = os.Remove("kogaro-test") }()

	t.Run("Plain YAML with ConfigMap reference issue", func(t *testing.T) {
		// Create plain YAML without Helm templates
		tmpDir := t.TempDir()
		configFile := filepath.Join(tmpDir, "deployment-missing-configmap.yaml")
		
		plainYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: kogaro-bad-configmap-envfrom
  labels:
    app: kogaro-bad-configmap
spec:
  replicas: 1
  selector:
    matchLabels:
      app: kogaro-bad-configmap
  template:
    metadata:
      labels:
        app: kogaro-bad-configmap
    spec:
      containers:
      - name: bad-container
        image: busybox:1.35
        command: ['sleep', '3600']
        envFrom:
        - configMapRef:
            name: nonexistent-config
`

		if err := os.WriteFile(configFile, []byte(plainYAML), 0644); err != nil {
			t.Fatalf("Failed to create test YAML file: %v", err)
		}

		// Run CLI validation
		cmd := exec.Command("./kogaro-test", "--mode=one-off", "--config="+configFile)
		output, _ := cmd.CombinedOutput()
		
		// Should parse successfully (no YAML parsing errors)
		outputStr := string(output)
		if strings.Contains(outputStr, "failed to parse YAML") {
			t.Errorf("Plain YAML should parse successfully, got:\n%s", outputStr)
		}
		
		t.Logf("Validation output:\n%s", outputStr)
	})

	t.Run("Helm template should fail with helpful error", func(t *testing.T) {
		// Test with actual Helm template file
		helmFile := "sample/kogaro-testbed/templates/deployment-missing-configmap.yaml"
		if _, err := os.Stat(helmFile); os.IsNotExist(err) {
			t.Skipf("Helm template file not found: %s", helmFile)
		}

		cmd := exec.Command("./kogaro-test", "--mode=one-off", "--config="+helmFile)
		output, err := cmd.CombinedOutput()
		
		// Should fail with parsing error
		exitCode := 0
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		}
		
		if exitCode == 0 {
			t.Error("Helm template file should fail validation")
		}
		
		outputStr := string(output)
		if !strings.Contains(outputStr, "Helm templates") {
			t.Errorf("Expected Helm template error message, got:\n%s", outputStr)
		}
		
		t.Logf("Helm template error output:\n%s", outputStr)
	})
}