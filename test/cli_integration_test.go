package test

import (
	"os/exec"
	"strings"
	"testing"
)

func TestCLI_Help(t *testing.T) {
	cmd := exec.Command("go", "run", "../cmd/proxy", "--help")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("CLI --help failed: %v\nOutput: %s", err, string(output))
	}
	if !strings.Contains(string(output), "Usage:") {
		t.Errorf("Expected usage info in help output, got: %s", string(output))
	}
}

func TestCLI_Setup_NonInteractive(t *testing.T) {
	outputFile := "test.env"
	cmd := exec.Command("go", "run", "../cmd/proxy", "setup", "--openai-key", "test-key", "--config", outputFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("CLI setup failed: %v\nOutput: %s", err, string(output))
	}
	if !strings.Contains(string(output), "Configuration written") {
		t.Errorf("Expected 'Configuration written' in output, got: %s", string(output))
	}
	if _, err := exec.Command("test", "-f", outputFile).Output(); err != nil {
		t.Errorf("Expected %s to be created, but it was not", outputFile)
	}
	_ = exec.Command("rm", "-f", outputFile).Run()
}
