package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/sofatutor/llm-proxy/internal/config"
	"github.com/sofatutor/llm-proxy/internal/database"
	"github.com/sofatutor/llm-proxy/internal/encryption"
	"github.com/sofatutor/llm-proxy/internal/eventtransformer"
	"github.com/sofatutor/llm-proxy/internal/logging"
	"github.com/sofatutor/llm-proxy/internal/proxy"
	"github.com/sofatutor/llm-proxy/internal/server"
	"github.com/sofatutor/llm-proxy/internal/token"
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

var newDatabaseFromConfig = database.NewFromConfig

func validateEncryptionKeyRequired() error {
	if os.Getenv("REQUIRE_ENCRYPTION_KEY") == "true" && os.Getenv("ENCRYPTION_KEY") == "" {
		return fmt.Errorf("ENCRYPTION_KEY is required but not set")
	}
	return nil
}

func buildDatabaseConfig(appConfig *config.Config) database.FullConfig {
	dbConfig := database.ConfigFromEnv()
	if dbConfig.Driver == database.DriverSQLite {
		// Preserve backward compatibility: if DATABASE_PATH is not set, use config.DatabasePath.
		if os.Getenv("DATABASE_PATH") == "" && appConfig != nil && appConfig.DatabasePath != "" {
			dbConfig.Path = appConfig.DatabasePath
		}
	}
	return dbConfig
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
	fileEventLogPath   string // Path to JSONL file for event logging
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
	serverCmd.Flags().BoolVarP(&daemonMode, "daemon", "d", config.EnvBoolOrDefault("DAEMON", false), "Run server in daemon mode (background)")
	serverCmd.Flags().StringVar(&serverEnvFile, "env", config.EnvOrDefault("ENV", ".env"), "Path to .env file")
	serverCmd.Flags().StringVar(&serverListenAddr, "addr", config.EnvOrDefault("LISTEN_ADDR", ""), "Address to listen on (overrides env var)")
	serverCmd.Flags().StringVar(&serverDatabasePath, "db", config.EnvOrDefault("DATABASE_PATH", ""), "Path to SQLite database (overrides env var)")
	serverCmd.Flags().StringVar(&serverLogLevel, "log-level", config.EnvOrDefault("LOG_LEVEL", ""), "Log level: debug, info, warn, error (overrides env var)")
	serverCmd.Flags().StringVar(&pidFile, "pid-file", config.EnvOrDefault("PID_FILE", "tmp/server.pid"), "PID file for daemon mode (relative to project root)")
	serverCmd.Flags().BoolVarP(&debugMode, "debug", "v", config.EnvBoolOrDefault("DEBUG", false), "Enable debug logging (overrides log-level)")
	serverCmd.Flags().StringVar(&serverLogFile, "log-file", config.EnvOrDefault("LOG_FILE", ""), "Path to log file (overrides env var, default: stdout)")
	serverCmd.Flags().StringVarP(&serverConfigPath, "config", "c", config.EnvOrDefault("API_CONFIG_PATH", ""), "Path to YAML config file for API providers (overrides API_CONFIG_PATH env var)")
	serverCmd.Flags().StringVar(&fileEventLogPath, "file-event-log", config.EnvOrDefault("FILE_EVENT_LOG", ""), "Path to JSONL file for event logging (single-process only)")
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

	// Load .env if present to resolve LISTEN_ADDR for preflight checks
	if _, err := os.Stat(serverEnvFile); err == nil {
		if err := godotenv.Load(serverEnvFile); err != nil {
			fmt.Printf("Warning: Error loading %s file: %v\n", serverEnvFile, err)
		}
	}
	// Fail fast if the configured address is already in use (best effort)
	listen := serverListenAddr
	if listen == "" {
		listen = os.Getenv("LISTEN_ADDR")
	}
	if listen == "" {
		listen = ":8080"
	}
	if ln, err := net.Listen("tcp", listen); err != nil {
		fmt.Fprintf(os.Stderr, "Listen address unavailable (already in use?): %s: %v\n", listen, err)
		osExit(1)
		return
	} else {
		_ = ln.Close()
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

	// Fail fast if the configured address is already in use
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":8080"
	}
	if ln, err := net.Listen("tcp", cfg.ListenAddr); err != nil {
		zapLogger.Fatal("Listen address unavailable (already in use?)", zap.String("addr", cfg.ListenAddr), zap.Error(err))
	} else {
		_ = ln.Close()
	}

	// Fail fast on missing encryption config before any heavyweight initialization.
	if err := validateEncryptionKeyRequired(); err != nil {
		zapLogger.Fatal(err.Error(),
			zap.String("hint", "Generate a valid key with: openssl rand -base64 32"))
	}

	// Database configuration
	dbConfig := buildDatabaseConfig(cfg)

	db, dbErr := newDatabaseFromConfig(dbConfig)
	if dbErr != nil {
		switch dbConfig.Driver {
		case database.DriverPostgres:
			zapLogger.Fatal("Failed to connect to PostgreSQL database", zap.Error(dbErr))
		case database.DriverMySQL:
			zapLogger.Fatal("Failed to connect to MySQL database", zap.Error(dbErr))
		default:
			zapLogger.Fatal("Failed to connect to SQLite database", zap.Error(dbErr))
		}
	}
	switch dbConfig.Driver {
	case database.DriverSQLite:
		if dbConfig.Path == ":memory:" {
			zapLogger.Info("Connected to in-memory SQLite database")
		} else {
			zapLogger.Info("Connected to SQLite database", zap.String("path", dbConfig.Path))
		}
	case database.DriverPostgres:
		zapLogger.Info("Connected to PostgreSQL database")
	case database.DriverMySQL:
		zapLogger.Info("Connected to MySQL database")
	}

	// Create base stores
	baseTokenStore := database.NewDBTokenStoreAdapter(db)
	baseProjectStore := proxy.ProjectStore(db)

	// Wrap stores with encryption/hashing if ENCRYPTION_KEY is set
	tokenStore := token.TokenStore(baseTokenStore)
	projectStore := baseProjectStore

	encryptionKey := os.Getenv("ENCRYPTION_KEY")
	if encryptionKey != "" {
		// Create encryptor for API keys
		encryptor, err := encryption.NewEncryptorFromBase64Key(encryptionKey)
		if err != nil {
			zapLogger.Fatal("Failed to initialize encryptor - check ENCRYPTION_KEY format",
				zap.Error(err),
				zap.String("hint", "Generate a valid key with: openssl rand -base64 32"))
		}

		// Create hasher for tokens
		hasher := encryption.NewTokenHasher()

		// Wrap stores with secure wrappers
		projectStore = encryption.NewSecureProjectStore(baseProjectStore, encryptor)
		tokenStore = encryption.NewSecureTokenStore(baseTokenStore, hasher)

		zapLogger.Info("Encryption enabled for sensitive data at rest",
			zap.Bool("api_keys_encrypted", true),
			zap.Bool("tokens_hashed", true))
	} else {
		zapLogger.Warn("ENCRYPTION_KEY not set - sensitive data will be stored in plaintext",
			zap.String("hint", "Set ENCRYPTION_KEY for production use: openssl rand -base64 32"))
	}

	// Create server with database support for audit logging
	s, err := server.NewWithDatabase(cfg, tokenStore, projectStore, db)
	if err != nil {
		zapLogger.Fatal("Failed to initialize server", zap.Error(err))
	}

	// File event log integration (single-process only)
	if fileEventLogPath != "" {
		f, err := os.OpenFile(fileEventLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			zapLogger.Fatal("Failed to open file event log", zap.Error(err))
		}
		bus := s.EventBus()
		if bus != nil {
			ch := bus.Subscribe()
			go func() {
				for evt := range ch {
					// Convert eventbus.Event to map[string]interface{} for transformer
					b, err := json.Marshal(evt)
					if err != nil {
						fmt.Fprintf(os.Stderr, "failed to marshal event: %v\n", err)
						continue
					}
					var m map[string]interface{}
					if err := json.Unmarshal(b, &m); err != nil {
						fmt.Fprintf(os.Stderr, "failed to unmarshal event: %v\n", err)
						continue
					}
					// TODO: Detect provider from path or config. For now, assume OpenAI for /v1/chat/completions
					provider := "openai"
					transformed, err := eventtransformer.DispatchTransformer(provider).TransformEvent(m)
					if err != nil {
						fmt.Fprintf(os.Stderr, "failed to transform event: %v\n", err)
						continue
					}
					if transformed == nil {
						continue // skip (e.g., OPTIONS)
					}
					line, err := json.Marshal(transformed)
					if err != nil {
						fmt.Fprintf(os.Stderr, "failed to marshal transformed event: %v\n", err)
						continue
					}
					if _, err := f.Write(append(line, '\n')); err != nil {
						fmt.Fprintf(os.Stderr, "failed to write event: %v\n", err)
					}
				}
				_ = f.Close()
			}()
			zapLogger.Info("File event log enabled", zap.String("path", fileEventLogPath))
		} else {
			zapLogger.Warn("File event log requested but event bus is not enabled (set observability in config)")
		}
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
