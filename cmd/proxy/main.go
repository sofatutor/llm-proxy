package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/sofatutor/llm-proxy/internal/config"
	"github.com/sofatutor/llm-proxy/internal/database"
	"github.com/sofatutor/llm-proxy/internal/server"
	"github.com/sofatutor/llm-proxy/internal/token"
	"github.com/spf13/cobra"
)

// Command line flags
var (
	envFile      string
	listenAddr   string
	databasePath string
	logLevel     string
)

// Configuration options for setup
var (
	configPath       string
	openAIAPIKey     string
	managementToken  string
	interactiveSetup bool
	projectName      string
	tokenDuration    int
	skipProjectSetup bool
)

// For testing
var (
	osExit           = os.Exit
	logFatalFunc     = log.Fatalf
	signalNotifyFunc = signal.Notify
	flagParseFunc    = flag.Parse // Allow overriding flag parsing for tests
	osSetenvFunc     = os.Setenv  // Allow overriding os.Setenv for tests
)

// ProjectCreateRequest, ProjectCreateResponse, TokenCreateRequest, TokenCreateResponse types (copy from setup.go)
type ProjectCreateRequest struct {
	Name         string `json:"name"`
	OpenAIAPIKey string `json:"openai_api_key"`
}
type ProjectCreateResponse struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	OpenAIAPIKey string    `json:"openai_api_key"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
type TokenCreateRequest struct {
	ProjectID     string `json:"project_id"`
	DurationHours int    `json:"duration_hours"`
}
type TokenCreateResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Setup command definition
var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Setup the LLM Proxy configuration",
	Long:  `Configure the LLM Proxy with your API keys and settings.`,
	Run:   runSetup,
}

// runSetup runs the setup command
func runSetup(cmd *cobra.Command, args []string) {
	if interactiveSetup {
		runInteractiveSetup()
	} else {
		runNonInteractiveSetup()
	}
}

// runInteractiveSetup performs an interactive setup process
func runInteractiveSetup() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("LLM Proxy Interactive Setup")
	fmt.Println("===========================")

	// Load existing config if present
	existingConfig := make(map[string]string)
	if _, err := os.Stat(configPath); err == nil {
		content, err := os.ReadFile(configPath)
		if err == nil {
			lines := strings.Split(string(content), "\n")
			for _, line := range lines {
				if strings.Contains(line, "=") {
					parts := strings.SplitN(line, "=", 2)
					if len(parts) == 2 {
						key := strings.TrimSpace(parts[0])
						value := strings.TrimSpace(parts[1])
						existingConfig[key] = value
					}
				}
			}
		}
	}

	// Get configuration file path
	fmt.Printf("Configuration file path [%s]: ", configPath)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input != "" {
		configPath = input
	}

	// Get OpenAI API Key
	openAIHint := ""
	if v, ok := existingConfig["OPENAI_API_KEY"]; ok && v != "" {
		openAIHint = fmt.Sprintf(" [%s]", token.ObfuscateToken(v))
	}
	if openAIAPIKey == "" {
		fmt.Printf("OpenAI API Key%s: ", openAIHint)
		input, _ := reader.ReadString('\n')
		openAIAPIKey = strings.TrimSpace(input)
	}

	// Get management token
	mgmtHint := ""
	if v, ok := existingConfig["MANAGEMENT_TOKEN"]; ok && v != "" {
		mgmtHint = fmt.Sprintf(" [%s]", token.ObfuscateToken(v))
	}
	if managementToken == "" {
		fmt.Printf("Management Token (leave empty to generate)%s: ", mgmtHint)
		input, _ := reader.ReadString('\n')
		managementToken = strings.TrimSpace(input)
		if managementToken == "" {
			// Generate a secure random token
			managementToken = generateSecureToken(32)
			fmt.Printf("Generated Management Token: %s\n", token.ObfuscateToken(managementToken))
		}
	}

	// Get database path
	dbHint := ""
	if v, ok := existingConfig["DATABASE_PATH"]; ok && v != "" {
		dbHint = fmt.Sprintf(" [%s]", v)
	}
	fmt.Printf("Database path [%s]%s: ", databasePath, dbHint)
	input, _ = reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input != "" {
		databasePath = input
	}

	// Get listen address
	listenHint := ""
	if v, ok := existingConfig["LISTEN_ADDR"]; ok && v != "" {
		listenHint = fmt.Sprintf(" [%s]", v)
	}
	fmt.Printf("Listen address [%s]%s: ", listenAddr, listenHint)
	input, _ = reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input != "" {
		listenAddr = input
	}

	// Ask about project setup
	fmt.Print("Do you want to create a project and token? (Y/n): ")
	input, _ = reader.ReadString('\n')
	input = strings.TrimSpace(input)
	skipProjectSetup = (input == "n" || input == "N")

	if !skipProjectSetup {
		// Get project name
		fmt.Printf("Project name [%s]: ", projectName)
		input, _ = reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input != "" {
			projectName = input
		}

		// Get token duration
		fmt.Printf("Token duration in hours [%d]: ", tokenDuration)
		input, _ = reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input != "" {
			if duration, err := time.ParseDuration(input + "h"); err == nil {
				tokenDuration = int(duration.Hours())
			} else if duration, err := time.ParseDuration(input); err == nil {
				tokenDuration = int(duration.Hours())
			} else if val, err := fmt.Sscanf(input, "%d", &tokenDuration); err != nil || val != 1 {
				fmt.Println("Invalid duration format. Using default value.")
			}
		}
	}

	// Write configuration to file
	writeConfig()
}

// runNonInteractiveSetup performs a non-interactive setup
func runNonInteractiveSetup() {
	// Check if OpenAI API Key is provided
	if openAIAPIKey == "" {
		fmt.Println("Error: OpenAI API Key is required")
		fmt.Println("Use --openai-key flag or run with --interactive")
		osExit(1)
	}

	// Generate management token if not provided
	if managementToken == "" {
		// Generate a secure random token
		managementToken = generateSecureToken(32)
		fmt.Printf("Generated Management Token: %s\n", token.ObfuscateToken(managementToken))
	}

	// Write configuration to file
	writeConfig()
}

// writeConfig writes the configuration to a file
func writeConfig() {
	// Create the directory if it doesn't exist
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Printf("Error creating directory %s: %v\n", dir, err)
		osExit(1)
	}

	// Check if file exists and read existing config
	existingConfig := make(map[string]string)
	if _, err := os.Stat(configPath); err == nil {
		content, err := os.ReadFile(configPath)
		if err == nil {
			lines := strings.Split(string(content), "\n")
			for _, line := range lines {
				if strings.Contains(line, "=") {
					parts := strings.SplitN(line, "=", 2)
					if len(parts) == 2 {
						key := strings.TrimSpace(parts[0])
						value := strings.TrimSpace(parts[1])
						existingConfig[key] = value
					}
				}
			}
		}
	}

	// Write new config
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("OPENAI_API_KEY=%s\n", openAIAPIKey))
	buf.WriteString(fmt.Sprintf("MANAGEMENT_TOKEN=%s\n", managementToken))
	buf.WriteString(fmt.Sprintf("DATABASE_PATH=%s\n", databasePath))
	buf.WriteString(fmt.Sprintf("LISTEN_ADDR=%s\n", listenAddr))
	if err := os.WriteFile(configPath, buf.Bytes(), 0644); err != nil {
		fmt.Printf("Error writing config file: %v\n", err)
		osExit(1)
	}
	fmt.Printf("Configuration written to %s\n", configPath)
}

// generateSecureToken generates a secure random token of the given length
func generateSecureToken(length int) string {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		panic("failed to generate secure token")
	}
	return hex.EncodeToString(b)
}

// rootCmd, openaiCmd, chatCmd, benchmarkCmd are defined at the package level (imported from their respective files)
// Only one rootCmd declaration at the package level
// Remove any local cobraRoot variable and use rootCmd throughout

// Declare these as package-level variables so they are available for init and tests
// They are defined in their respective files (chat.go, benchmark.go)
var openaiCmd *cobra.Command    // defined in chat.go
var benchmarkCmd *cobra.Command // defined in benchmark.go

func init() {
	// Add subcommands to openaiCmd
	openaiCmd.AddCommand(chatCmd)
	// Add all commands to root
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(openaiCmd)
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(benchmarkCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		osExit(1)
	}
}

// For test compatibility
var rootCmd *cobra.Command

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
		ts := database.NewMockTokenStore()
		ps := database.NewMockProjectStore()
		tokenStoreAdapter := database.NewTokenStoreAdapter(ts)
		s, err = server.New(cfg, tokenStoreAdapter, ps)
		if err != nil {
			log.Fatalf("failed to initialize server: %v", err)
		}
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

func init() {
	// ... existing code ...
	// ... existing code ...
}

// Helper to get management token from flag or env
func getManagementToken(cmd *cobra.Command) (string, error) {
	mgmtToken, _ := cmd.Flags().GetString("management-token")
	if mgmtToken == "" {
		mgmtToken = os.Getenv("MANAGEMENT_TOKEN")
	}
	if mgmtToken == "" {
		return "", fmt.Errorf("management token is required (set MANAGEMENT_TOKEN env or use --management-token)")
	}
	return mgmtToken, nil
}

// Helper to obfuscate OpenAI key
func obfuscateKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
}
