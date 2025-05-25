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
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/sofatutor/llm-proxy/internal/config"
	"github.com/sofatutor/llm-proxy/internal/database"
	"github.com/sofatutor/llm-proxy/internal/logging"
	"github.com/sofatutor/llm-proxy/internal/server"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/term"
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
	serverLogFile      string
	serverConfigPath   string
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
	serverCmd.Flags().StringVar(&pidFile, "pid-file", "tmp/server.pid", "PID file for daemon mode (relative to project root)")
	serverCmd.Flags().BoolVarP(&debugMode, "debug", "v", false, "Enable debug logging (overrides log-level)")
	serverCmd.Flags().StringVar(&serverLogFile, "log-file", "", "Path to log file (overrides env var, default: stdout)")
	serverCmd.Flags().StringVarP(&serverConfigPath, "config", "c", "", "Path to YAML config file for API providers (overrides API_CONFIG_PATH env var)")
}

// runServer is the main function for the server command
func runServer(cmd *cobra.Command, args []string) {
	if daemonMode {
		if serverLogFile == "" {
			fmt.Fprintln(os.Stderr, "Error: --log-file must be specified when running in daemon mode (-d)")
			osExit(1)
		}
		runServerDaemon()
	} else {
		runServerForeground()
	}
}

// runServerDaemon starts the server in daemon mode
func runServerDaemon() {
	fmt.Println("Starting LLM Proxy server in daemon mode...")

	// Print the effective log level for daemon mode
	if serverLogLevel != "" {
		fmt.Printf("[daemon] Log level: %s\n", serverLogLevel)
	} else if debugMode {
		fmt.Println("[daemon] Log level: debug (via -v/--debug)")
	} else {
		fmt.Println("[daemon] Log level: info (default)")
	}

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
	if serverLogFile != "" {
		serverArgs = append(serverArgs, "--log-file", serverLogFile)
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

	// Start the process
	if err := cmd.Start(); err != nil {
		fmt.Printf("Error starting daemon: %v\n", err)
		osExit(1)
	}

	// Save the actual process PID to the specified pidFile (default: tmp/server.pid)
	if cmd.Process != nil {
		pid := cmd.Process.Pid
		if err := os.MkdirAll("tmp", 0755); err != nil {
			fmt.Printf("Error creating tmp directory: %v\n", err)
		}
		if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", pid)), 0644); err != nil {
			fmt.Printf("Error writing PID file: %v\n", err)
		} else {
			fmt.Printf("Server started in daemon mode with PID %d\n", pid)
			fmt.Printf("PID file: %s\n", pidFile)
			fmt.Println("Use 'kill $(cat " + pidFile + ")' to stop the server")
		}
	}
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
	if serverLogFile != "" {
		if err := os.Setenv("LOG_FILE", serverLogFile); err != nil {
			log.Fatalf("Failed to set LOG_FILE environment variable: %v", err)
		}
	}
	if serverConfigPath != "" {
		if err := os.Setenv("API_CONFIG_PATH", serverConfigPath); err != nil {
			log.Fatalf("Failed to set API_CONFIG_PATH environment variable: %v", err)
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
	fmt.Printf("Log level: %s\n", cfg.LogLevel)

	// Initialize zap logger
	zapLogger, err := logging.NewLogger(cfg.LogLevel, cfg.LogFormat, cfg.LogFile)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer func() {
		if err := zapLogger.Sync(); err != nil {
			if !strings.Contains(err.Error(), "inappropriate ioctl for device") {
				log.Printf("Error syncing zap logger: %v", err)
			}
		}
	}()

	// Handle graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	// Make sure the database directory exists
	dbDir := filepath.Dir(cfg.DatabasePath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		zapLogger.Fatal("Failed to create database directory", zap.Error(err))
	}

	// Create and start the server with actual database stores (not mocks)
	dbConfig := database.DefaultConfig()
	dbConfig.Path = cfg.DatabasePath
	db, err := database.New(dbConfig)
	if err != nil {
		zapLogger.Fatal("Failed to connect to database", zap.Error(err))
	}
	if dbConfig.Path == ":memory:" {
		zapLogger.Info("Connected to in-memory SQLite database")
	} else {
		zapLogger.Info("Connected to SQLite database", zap.String("path", dbConfig.Path))
	}

	tokenStore := database.NewDBTokenStoreAdapter(db)
	projectStore := db

	// Create server
	s, err := server.New(cfg, tokenStore, projectStore)
	if err != nil {
		zapLogger.Fatal("Failed to initialize server", zap.Error(err))
	}

	// Log server starting before launching goroutine
	zapLogger.Info("Server starting", zap.String("addr", cfg.ListenAddr))

	go func() {
		if err := s.Start(); err != nil && err != http.ErrServerClosed {
			zapLogger.Fatal("Server error", zap.Error(err))
		}
	}()

	// Log server started after goroutine is launched
	zapLogger.Info("Server started", zap.String("addr", cfg.ListenAddr))

	// Print interactive controls message last
	if term.IsTerminal(int(os.Stdout.Fd())) {
		fmt.Println("Press Ctrl+C to stop")
	}

	// Wait for interrupt signal
	<-done
	zapLogger.Info("Server shutting down...")

	// Create a deadline for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := s.Shutdown(ctx); err != nil {
		zapLogger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	zapLogger.Info("Server exited gracefully")
}
