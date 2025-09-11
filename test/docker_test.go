package test

import (
	"os/exec"
	"testing"
)

// TestDockerScanTarget verifies the docker-scan Makefile target exists and can be executed
func TestDockerScanTarget(t *testing.T) {
	// Test that make docker-scan target exists and has proper syntax
	cmd := exec.Command("make", "-n", "docker-scan")
	cmd.Dir = ".."
	
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to run 'make -n docker-scan': %v", err)
	}
	
	expectedCommands := []string{
		"echo \"Running Trivy security scan on llm-proxy:latest...\"",
		"trivy image --severity HIGH,CRITICAL llm-proxy:latest",
	}
	
	outputStr := string(output)
	for _, expected := range expectedCommands {
		if !contains(outputStr, expected) {
			t.Errorf("Expected command not found in make output: %s", expected)
		}
	}
}

// TestDockerfileEnhancements verifies the Dockerfile contains security enhancements
func TestDockerfileEnhancements(t *testing.T) {
	dockerfile := "../Dockerfile"
	
	// Read and check Dockerfile content
	cmd := exec.Command("grep", "-E", "(alpine:3\\.20|\\-s \\-extldflags)", dockerfile)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to verify Dockerfile enhancements: %v", err)
	}
	
	outputStr := string(output)
	if !contains(outputStr, "alpine:3.20") && !contains(outputStr, "-s -extldflags") {
		t.Error("Expected Dockerfile security enhancements not found")
	}
}

// contains checks if a string contains a substring
func contains(str, substr string) bool {
	return len(str) >= len(substr) && 
		   str[len(str)-len(substr):] == substr ||
		   str[:len(substr)] == substr ||
		   findInString(str, substr)
}

func findInString(str, substr string) bool {
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}