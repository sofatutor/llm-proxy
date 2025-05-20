package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestInitFlags(t *testing.T) {
	// Since we can't test the init function directly in Go, we simply
	// verify that the flag variables are defined with correct defaults

	// This is mostly a placeholder test to improve coverage
	t.Run("FlagDefaults", func(t *testing.T) {
		// These assertions are indirect since we're not resetting the flags
		// The values might be affected by other tests

		// Simply verify that the package variables exist
		if envFile == "" {
			t.Error("envFile is not initialized")
		}
	})
}

// TestRunFunction tests the run function directly
func TestRunFunction(t *testing.T) {
	// Save original values
	originalArgs := os.Args
	originalExit := osExit
	originalLogFatal := logFatalFunc
	originalFlagParse := flagParseFunc
	originalOsSetenv := osSetenvFunc

	// Restore original values after test
	defer func() {
		os.Args = originalArgs
		osExit = originalExit
		logFatalFunc = originalLogFatal
		flagParseFunc = originalFlagParse
		osSetenvFunc = originalOsSetenv
	}()

	// Replace os.Exit with a mock version
	exitCode := 0
	osExit = func(code int) {
		exitCode = code
		panic("exit") // Simulate os.Exit by panicking
	}

	// Create a mock for log.Fatal
	var fatalMsg string
	logFatalFunc = func(v ...interface{}) {
		fatalMsg = fmt.Sprint(v...)
		osExit(1)
	}

	// Replace flag.Parse with a no-op function
	flagParseFunc = func() {} // Do nothing

	// No need to mock testing.Testing since tests automatically run in testing mode

	// Test with valid config
	t.Run("TestValidConfig", func(t *testing.T) {
		// Reset values for this test
		exitCode = 0
		fatalMsg = ""

		// Set environment for this test
		os.Clearenv()
		if err := osSetenvFunc("MANAGEMENT_TOKEN", "test-token"); err != nil {
			t.Fatalf("Failed to set test environment: %v", err)
		}

		// Call run directly
		run()

		// Verify no fatal errors occurred
		if exitCode != 0 {
			t.Errorf("Expected exitCode 0, got %d with message: %s", exitCode, fatalMsg)
		}
	})

	// Test env variable settings with errors
	t.Run("TestEnvVarSettingFailure", func(t *testing.T) {
		// Reset values for this test
		exitCode = 0
		fatalMsg = ""

		// Set environment for this test
		os.Clearenv()
		if err := osSetenvFunc("MANAGEMENT_TOKEN", "test-token"); err != nil {
			t.Fatalf("Failed to set test environment: %v", err)
		}

		// Override osSetenvFunc to simulate a failure
		origSetenv := osSetenvFunc
		osSetenvFunc = func(key, value string) error {
			return fmt.Errorf("simulated setenv failure")
		}
		defer func() { osSetenvFunc = origSetenv }()

		// Use defer/recover to catch the osExit panic
		recovered := false
		defer func() {
			if r := recover(); r != nil {
				if r == "exit" {
					recovered = true
				} else {
					t.Errorf("Unexpected panic: %v", r)
				}
			}

			// Verify we got a fatal error with the right message
			if !recovered {
				t.Error("osExit was not called as expected")
			}
			if exitCode != 1 {
				t.Errorf("Expected exitCode 1, got %d", exitCode)
			}
			if !strings.Contains(fatalMsg, "Failed to set LISTEN_ADDR") {
				t.Errorf("Expected error message about setting LISTEN_ADDR, got: %s", fatalMsg)
			}
		}()

		// Set the flag values to trigger the setenv calls
		listenAddr = ":9090"

		// Call run, which should hit our mocked Setenv and fail
		run()
	})

	// Test signal notification
	t.Run("TestSignalNotification", func(t *testing.T) {
		// Reset values for this test
		exitCode = 0
		fatalMsg = ""

		// Set environment for this test
		os.Clearenv()
		if err := osSetenvFunc("MANAGEMENT_TOKEN", "test-token"); err != nil {
			t.Fatalf("Failed to set test environment: %v", err)
		}

		// We can test our signal notification by overriding it directly
		var notifyCalled bool

		origSignalNotify := signalNotifyFunc
		signalNotifyFunc = func(c chan<- os.Signal, sig ...os.Signal) {
			notifyCalled = true
			// Don't actually send a signal
		}
		defer func() { signalNotifyFunc = origSignalNotify }()

		// Set an abort function for the test in case it hangs
		abortTimer := time.AfterFunc(100*time.Millisecond, func() {
			t.Log("Test timed out, aborting")
			osExit(0) // Force exit
		})
		defer abortTimer.Stop()

		// Call the run function, which should set up the signal handler
		// and then return since we're in testing mode
		run()

		// Verify the signal notification was configured
		if !notifyCalled {
			t.Error("Signal notification was not configured")
		}
	})

	// Test with invalid config (missing required token)
	t.Run("TestInvalidConfig", func(t *testing.T) {
		// Reset values for this test
		exitCode = 0
		fatalMsg = ""

		// Clear all environment variables
		os.Clearenv()

		// Use defer/recover to catch the osExit panic
		recovered := false
		defer func() {
			if r := recover(); r != nil {
				if r == "exit" {
					recovered = true
				} else {
					t.Errorf("Unexpected panic: %v", r)
				}
			}

			// Verify that a fatal error occurred
			if !recovered {
				t.Error("osExit was not called as expected")
			}
			if exitCode != 1 {
				t.Errorf("Expected exitCode 1 due to missing config, got %d", exitCode)
			}
			if !strings.Contains(fatalMsg, "MANAGEMENT_TOKEN") {
				t.Errorf("Expected error message about missing MANAGEMENT_TOKEN, got: %s", fatalMsg)
			}
		}()

		// Call run (should trigger log.Fatal due to missing token)
		run()
	})

	// Test loading non-existent env file
	t.Run("TestEnvFileLoading", func(t *testing.T) {
		// Reset values for this test
		exitCode = 0
		fatalMsg = ""

		// Prepare a test environment
		os.Clearenv()
		if err := osSetenvFunc("MANAGEMENT_TOKEN", "test-token"); err != nil {
			t.Fatalf("Failed to set test environment: %v", err)
		}

		// Override the envFile variable directly
		envFile = "nonexistent.env"

		// Call run - it should try to load the env file and fail, but continue
		run()

		// Verify we didn't have a fatal error
		if exitCode != 0 {
			t.Errorf("Unexpected fatal error: %s", fatalMsg)
		}
	})

	// Test command line flags
	t.Run("TestCommandLineOverrides", func(t *testing.T) {
		// Reset values for this test
		exitCode = 0
		fatalMsg = ""

		// Prepare a test environment
		os.Clearenv()
		if err := osSetenvFunc("MANAGEMENT_TOKEN", "test-token"); err != nil {
			t.Fatalf("Failed to set test environment: %v", err)
		}

		// Set the flag values directly
		listenAddr = ":9090"
		databasePath = "./test.db"
		logLevel = "debug"

		// Call run - it should set the environment variables from flags
		run()

		// Verify that the environment variables were set
		if addr := os.Getenv("LISTEN_ADDR"); addr != ":9090" {
			t.Errorf("Expected LISTEN_ADDR to be set to :9090, got %s", addr)
		}
		if db := os.Getenv("DATABASE_PATH"); db != "./test.db" {
			t.Errorf("Expected DATABASE_PATH to be set to ./test.db, got %s", db)
		}
		if level := os.Getenv("LOG_LEVEL"); level != "debug" {
			t.Errorf("Expected LOG_LEVEL to be set to debug, got %s", level)
		}

		// Verify we didn't have a fatal error
		if exitCode != 0 {
			t.Errorf("Unexpected fatal error: %s", fatalMsg)
		}
	})
}

// TestMainFunction tests that main() calls run()
func TestMainFunction(t *testing.T) {
	// Save and restore the original run function
	origRun := run
	called := false
	run = func() {
		called = true
	}
	defer func() { run = origRun }()

	main()

	if !called {
		t.Error("main() did not call run()")
	}
}

// mockServer implements serverInterface for testing runWithHooks
type mockServer struct {
	mu             sync.Mutex
	startCalled    bool
	shutdownCalled bool
	startErr       error
	shutdownErr    error
}

func (m *mockServer) Start() error {
	m.mu.Lock()
	m.startCalled = true
	m.mu.Unlock()
	return m.startErr
}
func (m *mockServer) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	m.shutdownCalled = true
	m.mu.Unlock()
	return m.shutdownErr
}

// TestRunWithHooks covers the server startup and shutdown logic in runWithHooks
func TestRunWithHooks_ServerLifecycle(t *testing.T) {
	// Set up a mock server and a done channel
	ms := &mockServer{}
	done := make(chan os.Signal, 1)

	// Simulate sending a signal after a short delay
	go func() {
		time.Sleep(10 * time.Millisecond)
		done <- os.Interrupt
	}()

	// Set up required env for config.New()
	os.Clearenv()
	osSetenvFunc("MANAGEMENT_TOKEN", "test-token")

	// Call runWithHooks with forceNoTest=true to skip the testing.Testing() check
	runWithHooks(done, ms, true)

	ms.mu.Lock()
	if !ms.startCalled {
		t.Error("mockServer.Start was not called")
	}
	if !ms.shutdownCalled {
		t.Error("mockServer.Shutdown was not called")
	}
	ms.mu.Unlock()
}

// TestRunWithHooks_ServerStartError covers the error path when server.Start returns a non-ErrServerClosed error
func TestRunWithHooks_ServerStartError(t *testing.T) {
	ms := &mockServer{startErr: fmt.Errorf("boom")}
	done := make(chan os.Signal, 1)

	// Simulate sending a signal after a short delay
	go func() {
		time.Sleep(10 * time.Millisecond)
		done <- os.Interrupt
	}()

	os.Clearenv()
	osSetenvFunc("MANAGEMENT_TOKEN", "test-token")

	logFatalCalled := make(chan bool, 1)
	origLogFatal := logFatalFunc
	logFatalFunc = func(v ...interface{}) {
		logFatalCalled <- true
	}
	defer func() { logFatalFunc = origLogFatal }()

	runWithHooks(done, ms, true)

	select {
	case <-logFatalCalled:
		// Success: logFatalFunc was called in the goroutine
	case <-time.After(100 * time.Millisecond):
		t.Error("logFatalFunc was not called for server.Start error (timeout)")
	}
}

// TestRunWithHooks_ServerShutdownError covers the error path when server.Shutdown returns an error
func TestRunWithHooks_ServerShutdownError(t *testing.T) {
	ms := &mockServer{shutdownErr: fmt.Errorf("shutdown fail")}
	done := make(chan os.Signal, 1)

	go func() {
		time.Sleep(10 * time.Millisecond)
		done <- os.Interrupt
	}()

	os.Clearenv()
	osSetenvFunc("MANAGEMENT_TOKEN", "test-token")

	logFatalCalled := make(chan bool, 1)
	origLogFatal := logFatalFunc
	logFatalFunc = func(v ...interface{}) {
		logFatalCalled <- true
	}
	defer func() { logFatalFunc = origLogFatal }()

	runWithHooks(done, ms, true)

	select {
	case <-logFatalCalled:
		// Success: logFatalFunc was called for server.Shutdown error
	case <-time.After(100 * time.Millisecond):
		t.Error("logFatalFunc was not called for server.Shutdown error (timeout)")
	}
}
