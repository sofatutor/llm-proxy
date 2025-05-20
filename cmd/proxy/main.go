package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/sofatutor/llm-proxy/internal/config"
	"github.com/sofatutor/llm-proxy/internal/server"
)

// Command line flags
var (
	envFile      string
	listenAddr   string
	databasePath string
	logLevel     string
)

// For testing
var (
	osExit           = os.Exit
	logFatalFunc     = log.Fatal
	signalNotifyFunc = signal.Notify
	flagParseFunc    = flag.Parse // Allow overriding flag parsing for tests
	osSetenvFunc     = os.Setenv  // Allow overriding os.Setenv for tests
)

func init() {
	// Define command line flags
	flag.StringVar(&envFile, "env", ".env", "Path to .env file")
	flag.StringVar(&listenAddr, "addr", "", "Address to listen on (overrides env var)")
	flag.StringVar(&databasePath, "db", "", "Path to SQLite database (overrides env var)")
	flag.StringVar(&logLevel, "log-level", "", "Log level: debug, info, warn, error (overrides env var)")
}

func main() {
	run()
}

// run is a variable to allow mocking in tests
var run = realRun

// realRun encapsulates the main logic for better testability
func realRun() {
	runWithHooks(nil, nil, false)
}

// runWithHooks allows injection of test hooks for the done channel, server, and skipping the testing.Testing() check.
// If doneCh is nil, a new channel is created. If srv is nil, a new server is created. If forceNoTest is false, the normal testing.Testing() check is used.
func runWithHooks(doneCh chan os.Signal, srv serverInterface, forceNoTest bool) {
	// Parse command line flags
	flagParseFunc()

	// Load .env file if it exists
	if _, err := os.Stat(envFile); err == nil {
		err := godotenv.Load(envFile)
		if err != nil {
			log.Printf("Warning: Error loading %s file: %v", envFile, err)
		} else {
			log.Printf("Loaded environment from %s", envFile)
		}
	}

	// Apply command line overrides to environment variables
	if listenAddr != "" {
		if err := osSetenvFunc("LISTEN_ADDR", listenAddr); err != nil {
			logFatalFunc("Failed to set LISTEN_ADDR environment variable: %v", err)
		}
	}
	if databasePath != "" {
		if err := osSetenvFunc("DATABASE_PATH", databasePath); err != nil {
			logFatalFunc("Failed to set DATABASE_PATH environment variable: %v", err)
		}
	}
	if logLevel != "" {
		if err := osSetenvFunc("LOG_LEVEL", logLevel); err != nil {
			logFatalFunc("Failed to set LOG_LEVEL environment variable: %v", err)
		}
	}

	// Load configuration
	cfg, err := config.New()
	if err != nil {
		logFatalFunc("Failed to load configuration: %v", err)
	}

	// For tests, return early to avoid starting the server,
	// but make sure we've gone through the signal setup first

	// Handle graceful shutdown
	var done chan os.Signal
	if doneCh != nil {
		done = doneCh
	} else {
		done = make(chan os.Signal, 1)
	}
	signalNotifyFunc(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	// The forceNoTest parameter allows tests to skip early exit
	if !forceNoTest {
		// Check if we're in a testing environment using the "GO_RUNNING_TESTS" environment variable
		// This is set in the test files
		if _, isTest := os.LookupEnv("GO_RUNNING_TESTS"); isTest {
			return
		}
	}

	// Create and start the server
	var s serverInterface
	if srv != nil {
		s = srv
	} else {
		s = server.New(cfg)
	}

	go func() {
		if err := s.Start(); err != nil && err != http.ErrServerClosed {
			logFatalFunc("Server error: %v", err)
		}
	}()

	log.Printf("LLM Proxy server started on %s", cfg.ListenAddr)
	log.Printf("Press Ctrl+C to stop")

	// Wait for interrupt signal
	<-done
	log.Println("Server shutting down...")

	// Create a deadline for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := s.Shutdown(ctx); err != nil {
		logFatalFunc("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited gracefully")
}

// serverInterface allows mocking the server in tests
type serverInterface interface {
	Start() error
	Shutdown(ctx context.Context) error
}
