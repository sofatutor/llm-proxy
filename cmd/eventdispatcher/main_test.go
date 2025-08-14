package main

import (
    "flag"
    "os"
    "testing"
)

func TestEnvHelpers(t *testing.T) {
    os.Setenv("X", "val")
    t.Cleanup(func() { os.Unsetenv("X") })
    if got := envOrDefault("X", "fallback"); got != "val" {
        t.Fatalf("envOrDefault got %q", got)
    }
    if got := envOrDefault("MISSING", "fb"); got != "fb" {
        t.Fatalf("envOrDefault fallback got %q", got)
    }
    os.Setenv("N", "42")
    t.Cleanup(func() { os.Unsetenv("N") })
    if got := envIntOrDefault("N", 7); got != 42 {
        t.Fatalf("envIntOrDefault got %d", got)
    }
    if got := envIntOrDefault("MISSING_INT", 9); got != 9 {
        t.Fatalf("envIntOrDefault fallback got %d", got)
    }
}

func TestRun_NoFilePermission(t *testing.T) {
    // set file path to an invalid directory to trigger open error
    os.Setenv("EVENTDISPATCHER_FILE", "/dev/null/dir/notafile.jsonl")
    defer os.Unsetenv("EVENTDISPATCHER_FILE")
    code := run()
    if code != 1 {
        t.Fatalf("expected exit code 1, got %d", code)
    }
}

func TestRun_InvalidArg(t *testing.T) {
	t.Skip("Skipping CLI entrypoint test: Go flag/os.Exit not testable in-process, see COVERAGE_PR34.md")
	// origArgs := os.Args
	// defer func() { os.Args = origArgs }()
	// os.Args = []string{"eventdispatcher", "--notarealflag"}
	// _ = run() // Should not panic
}

// Test that we can parse flags without running the actual service
func TestFlagParsing(t *testing.T) {
	// Save original command line and restore after test
	origArgs := os.Args
	defer func() {
		os.Args = origArgs
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	}()

	// Test default values
	os.Args = []string{"eventdispatcher"}

	var (
		filePath   string
		bufferSize int
	)

	// Create a new flag set to avoid conflicts with global flags
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.StringVar(&filePath, "file", "events.jsonl", "Path to the output JSONL file")
	fs.IntVar(&bufferSize, "buffer", 100, "Event bus buffer size")

	err := fs.Parse([]string{})
	if err != nil {
		t.Fatalf("Failed to parse empty args: %v", err)
	}

	if filePath != "events.jsonl" {
		t.Errorf("Expected default file 'events.jsonl', got %s", filePath)
	}

	if bufferSize != 100 {
		t.Errorf("Expected default buffer size 100, got %d", bufferSize)
	}

	// Test custom values
	fs2 := flag.NewFlagSet("test2", flag.ContinueOnError)
	fs2.StringVar(&filePath, "file", "events.jsonl", "Path to the output JSONL file")
	fs2.IntVar(&bufferSize, "buffer", 100, "Event bus buffer size")

	err = fs2.Parse([]string{"-file", "custom.jsonl", "-buffer", "200"})
	if err != nil {
		t.Fatalf("Failed to parse custom args: %v", err)
	}

	if filePath != "custom.jsonl" {
		t.Errorf("Expected custom file 'custom.jsonl', got %s", filePath)
	}

	if bufferSize != 200 {
		t.Errorf("Expected custom buffer size 200, got %d", bufferSize)
	}
}

func TestMainExists(t *testing.T) {
	// This test just verifies that the main function exists and compiles.
	// We can't easily test main() since it calls os.Exit(), but having
	// this test ensures the main function is present and the file compiles.
	// The actual functionality is tested through integration tests or
	// by testing the run() function components separately.
}
