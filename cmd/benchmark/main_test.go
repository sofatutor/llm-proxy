package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestMain(t *testing.T) {
	// Call the main function
	// In the case of the benchmark command, it's just a placeholder that does nothing yet
	main()
}

func TestMain_Benchmark(t *testing.T) {
	// Save and restore os.Args
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	os.Args = []string{"llm-proxy-benchmark", "--help"}
	// main() will print help and exit
	main()
}

func TestMainWithError(t *testing.T) {
	// Save and restore os.Args and osExit
	origArgs := os.Args
	origExit := osExit
	defer func() { 
		os.Args = origArgs 
		osExit = origExit
	}()

	// Mock osExit to capture exit calls
	exitCalled := false
	exitCode := 0
	osExit = func(code int) {
		exitCalled = true
		exitCode = code
	}

	// Set invalid args to trigger error
	os.Args = []string{"llm-proxy-benchmark", "--invalid-flag"}
	
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	main()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !exitCalled {
		t.Error("Expected osExit to be called")
	}
	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(output, "unknown flag") {
		t.Errorf("Expected error message about unknown flag, got: %s", output)
	}
}

func TestBenchmarkCommand(t *testing.T) {
	// Save and restore os.Args
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	os.Args = []string{"llm-proxy-benchmark", "benchmark"}
	main()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	expected := "Benchmark command not yet implemented"
	if !strings.Contains(output, expected) {
		t.Errorf("Expected output to contain '%s', got: %s", expected, output)
	}
}

func TestRootCommandHelp(t *testing.T) {
	// Save and restore os.Args
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	os.Args = []string{"llm-proxy-benchmark"}
	main()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "CLI tool for working with the LLM Proxy") {
		t.Errorf("Expected help output, got: %s", output)
	}
}

func TestOpenAICommand(t *testing.T) {
	// Save and restore os.Args
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	os.Args = []string{"llm-proxy-benchmark", "openai", "--help"}
	main()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "Interact with OpenAI services") {
		t.Errorf("Expected OpenAI help output, got: %s", output)
	}
}
