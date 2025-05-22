package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/sofatutor/llm-proxy/internal/config"
	"github.com/sofatutor/llm-proxy/internal/database"
	"github.com/sofatutor/llm-proxy/internal/server"
	"github.com/spf13/cobra"
)

// For testing
type execCommand struct {
	*exec.Cmd
	path        string
	args        []string
	sysProcAttr *syscall.SysProcAttr
}

var osExec = func(name string, args ...string) *execCommand {
	cmd := &execCommand{
		Cmd:  exec.Command(name, args...),
		path: name,
		args: args,
	}
	return cmd
}

// Server command flags
var (
	daemonMode         bool
	serverEnvFile      string
	serverListenAddr   string
	serverDatabasePath string
	serverLogLevel     string
	pidFile            string
	debugMode          bool
)

// Add this before init()
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the LLM Proxy server",
	Long:  `Start the LLM Proxy server using the configuration from setup.`,
	Run:   runServer,
}

func init() {
	// Server command flags
	serverCmd.Flags().BoolVarP(&daemonMode, "daemon", "d", false, "Run server in daemon mode (background)")
	serverCmd.Flags().StringVar(&serverEnvFile, "env", ".env", "Path to .env file")
	serverCmd.Flags().StringVar(&serverListenAddr, "addr", "", "Address to listen on (overrides env var)")
	serverCmd.Flags().StringVar(&serverDatabasePath, "db", "", "Path to SQLite database (overrides env var)")
	serverCmd.Flags().StringVar(&serverLogLevel, "log-level", "", "Log level: debug, info, warn, error (overrides env var)")
	serverCmd.Flags().StringVar(&pidFile, "pid-file", "/tmp/llm-proxy.pid", "PID file for daemon mode")
	serverCmd.Flags().BoolVarP(&debugMode, "debug", "v", false, "Enable debug logging (overrides log-level)")
}

// runServer is the main function for the server command
func runServer(cmd *cobra.Command, args []string) {
	if daemonMode {
		runServerDaemon()
	} else {
		runServerForeground()
	}
}

// runServerDaemon starts the server in daemon mode
func runServerDaemon() {
	fmt.Println("Starting LLM Proxy server in daemon mode...")

	// Get the absolute path of the current executable
	execPath, err := os.Executable()
	if err != nil {
		fmt.Printf("Error getting executable path: %v\n", err)
		osExit(1)
	}

	// Create command to run the current executable with the server command, but without daemon flag
	serverArgs := []string{"server"}

	// Add all flags except daemon
	if serverEnvFile != ".env" {
		serverArgs = append(serverArgs, "--env", serverEnvFile)
	}
	if serverListenAddr != "" {
		serverArgs = append(serverArgs, "--addr", serverListenAddr)
	}
	if serverDatabasePath != "" {
		serverArgs = append(serverArgs, "--db", serverDatabasePath)
	}
	if serverLogLevel != "" {
		serverArgs = append(serverArgs, "--log-level", serverLogLevel)
	}

	// Set up the command
	cmd := osExec(execPath, serverArgs...)

	// Detach from the parent process
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.sysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // Set process group ID
	}
	cmd.Cmd.SysProcAttr = cmd.sysProcAttr

	// Start the process
	if err := cmd.Start(); err != nil {
		fmt.Printf("Error starting daemon: %v\n", err)
		osExit(1)
	}

	// Save the PID to file
	// In the real implementation, we would get the process PID
	// For now, we'll use a dummy value for testing
	pid := 12345
	if cmd.Cmd != nil && cmd.Cmd.Process != nil {
		pid = cmd.Cmd.Process.Pid
	}
	err = os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", pid)), 0644)
	if err != nil {
		fmt.Printf("Error writing PID file: %v\n", err)
		// Continue anyway
	}

	fmt.Printf("Server started in daemon mode with PID %d\n", pid)
	fmt.Printf("PID file: %s\n", pidFile)
	fmt.Println("Use 'kill $(cat " + pidFile + ")' to stop the server")
}

// runServerForeground starts the server in foreground mode
func runServerForeground() {
	// Load .env file if it exists
	if _, err := os.Stat(serverEnvFile); err == nil {
		err := godotenv.Load(serverEnvFile)
		if err != nil {
			log.Printf("Warning: Error loading %s file: %v", serverEnvFile, err)
		} else {
			log.Printf("Loaded environment from %s", serverEnvFile)
		}
	}

	// Apply command line overrides to environment variables
	if serverListenAddr != "" {
		if err := os.Setenv("LISTEN_ADDR", serverListenAddr); err != nil {
			log.Fatalf("Failed to set LISTEN_ADDR environment variable: %v", err)
		}
	}
	if serverDatabasePath != "" {
		if err := os.Setenv("DATABASE_PATH", serverDatabasePath); err != nil {
			log.Fatalf("Failed to set DATABASE_PATH environment variable: %v", err)
		}
	}
	if serverLogLevel != "" {
		if err := os.Setenv("LOG_LEVEL", serverLogLevel); err != nil {
			log.Fatalf("Failed to set LOG_LEVEL environment variable: %v", err)
		}
	}

	// Set debug log level if --debug/-v or DEBUG=1 or LOG_LEVEL=debug
	if debugMode || os.Getenv("DEBUG") == "1" || os.Getenv("LOG_LEVEL") == "debug" {
		_ = os.Setenv("LOG_LEVEL", "debug")
		fmt.Println("Debug logging enabled")
	}

	// Load configuration
	cfg, err := config.New()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Handle graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	// Make sure the database directory exists
	dbDir := filepath.Dir(cfg.DatabasePath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		log.Fatalf("Failed to create database directory: %v", err)
	}

	// Create and start the server with actual database stores (not mocks)
	dbConfig := database.DefaultConfig()
	dbConfig.Path = cfg.DatabasePath
	db, err := database.New(dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	if dbConfig.Path == ":memory:" {
		log.Printf("Connected to in-memory SQLite database")
	} else {
		log.Printf("Connected to SQLite database at %s", dbConfig.Path)
	}

	tokenStore := database.NewDBTokenStoreAdapter(db)
	projectStore := db

	// Create server
	s, err := server.New(cfg, tokenStore, projectStore)
	if err != nil {
		log.Fatalf("Failed to initialize server: %v", err)
	}

	go func() {
		if err := s.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
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
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited gracefully")
}
