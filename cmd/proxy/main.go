package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"github.com/sofatutor/llm-proxy/internal/api"
	"github.com/sofatutor/llm-proxy/internal/config"
	"github.com/sofatutor/llm-proxy/internal/dispatcher"
	"github.com/sofatutor/llm-proxy/internal/dispatcher/plugins"
	"github.com/sofatutor/llm-proxy/internal/eventbus"
	"github.com/sofatutor/llm-proxy/internal/logging"
	"github.com/sofatutor/llm-proxy/internal/setup"
	"github.com/sofatutor/llm-proxy/internal/token"
	"github.com/sofatutor/llm-proxy/internal/utils"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// Command line flags
var (
	listenAddr   string
	databasePath string
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
	osExit = os.Exit
)

// Global flag for management API base URL
var manageAPIBaseURL string

// Dispatcher command flags
var (
	dispatcherService  string
	dispatcherEndpoint string
	dispatcherAPIKey   string
	dispatcherBuffer   int
	dispatcherBatch    int
	dispatcherDetach   bool
)

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
		fmt.Printf("Token duration in minutes [%d]: ", tokenDuration)
		input, _ = reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input != "" {
			if duration, err := time.ParseDuration(input + "m"); err == nil {
				tokenDuration = int(duration.Minutes())
			} else if duration, err := time.ParseDuration(input); err == nil {
				tokenDuration = int(duration.Minutes())
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
	setupCfg := &setup.SetupConfig{
		ConfigPath:      configPath,
		OpenAIAPIKey:    openAIAPIKey,
		ManagementToken: managementToken,
		DatabasePath:    databasePath,
		ListenAddr:      listenAddr,
	}

	if err := setup.RunNonInteractiveSetup(setupCfg); err != nil {
		fmt.Printf("Error during setup: %v\n", err)
		osExit(1)
	}

	// Update global variables for compatibility
	managementToken = setupCfg.ManagementToken
	fmt.Printf("Generated Management Token: %s\n", token.ObfuscateToken(managementToken))
	fmt.Printf("Configuration written to %s\n", configPath)
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
	return utils.GenerateSecureTokenMustSucceed(length)
}

// For test compatibility
var rootCmd *cobra.Command
var openaiCmd *cobra.Command
var benchmarkCmd *cobra.Command

var dispatcherCmd = &cobra.Command{
	Use:   "dispatcher",
	Short: "Run the event dispatcher service",
	Long:  `Run the event dispatcher service with pluggable backends. Supports file, lunary, and helicone services.`,
	Run:   runDispatcher,
}

// runDispatcher is the main function for the dispatcher command
func runDispatcher(cmd *cobra.Command, args []string) {
	ctx := context.Background()

	// Initialize logger
	logger, err := logging.NewLogger("info", "text", "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		osExit(1)
	}
	defer func() {
		if err := logger.Sync(); err != nil {
			if !strings.Contains(err.Error(), "inappropriate ioctl for device") {
				log.Printf("Error syncing zap logger: %v", err)
			}
		}
	}()

	// Determine event bus backend
	busBackend := os.Getenv("LLM_PROXY_EVENT_BUS")
	if busBackend == "" {
		busBackend = "redis"
	}

	var eventBus eventbus.EventBus
	switch busBackend {
	case "redis":
		redisAddr := os.Getenv("REDIS_ADDR")
		if redisAddr == "" {
			redisAddr = "localhost:6379"
		}
		redisDB := 0
		if dbStr := os.Getenv("REDIS_DB"); dbStr != "" {
			if dbVal, err := strconv.Atoi(dbStr); err == nil {
				redisDB = dbVal
			}
		}
		client := redis.NewClient(&redis.Options{
			Addr: redisAddr,
			DB:   redisDB,
		})
		adapter := &eventbus.RedisGoClientAdapter{Client: client}
		eventBus = eventbus.NewRedisEventBusLog(adapter, "llm-proxy-events", 24*time.Hour, 100000)
		logger.Info("Using Redis event bus", zap.String("addr", redisAddr), zap.Int("db", redisDB))
	case "memory":
		if dispatcherService != "file" {
			logger.Fatal("In-memory event bus only works for single-process file logging. Use Redis for multi-process/event dispatching.")
		}
		eventBus = eventbus.NewInMemoryEventBus(dispatcherBuffer)
		logger.Warn("Using in-memory event bus (single-process only)")
	default:
		logger.Fatal("Unknown event bus backend: ", zap.String("backend", busBackend))
	}

	// Create plugin
	plugin, err := plugins.NewPlugin(dispatcherService)
	if err != nil {
		logger.Fatal("Failed to create plugin", zap.Error(err))
	}

	// Configure plugin
	config := make(map[string]string)
	if dispatcherEndpoint != "" {
		config["endpoint"] = dispatcherEndpoint
	}
	if dispatcherAPIKey != "" {
		config["api-key"] = dispatcherAPIKey
	}

	// Support environment variables for API key
	if dispatcherAPIKey == "" {
		if envKey := os.Getenv("LLM_PROXY_API_KEY"); envKey != "" {
			config["api-key"] = envKey
		}
	}

	if err := plugin.Init(config); err != nil {
		logger.Fatal("Failed to initialize plugin", zap.Error(err))
	}

	// Create dispatcher service
	dispatcherVerbose := false
	if v := os.Getenv("LLM_PROXY_DISPATCHER_VERBOSE"); v == "1" || strings.ToLower(v) == "true" || strings.ToLower(v) == "yes" {
		dispatcherVerbose = true
	}

	dispatcherConfig := dispatcher.Config{
		BufferSize:    dispatcherBuffer,
		BatchSize:     dispatcherBatch,
		FlushInterval: 5 * time.Second,
		RetryAttempts: 3,
		RetryBackoff:  time.Second,
		Plugin:        plugin,
		PluginName:    dispatcherService,
		Verbose:       dispatcherVerbose,
	}

	service, err := dispatcher.NewServiceWithBus(dispatcherConfig, logger, eventBus)
	if err != nil {
		logger.Fatal("Failed to create dispatcher service", zap.Error(err))
	}

	// Start the service
	logger.Info("Starting dispatcher service",
		zap.String("service", dispatcherService),
		zap.String("endpoint", dispatcherEndpoint),
		zap.Bool("detach", dispatcherDetach))

	if err := service.Run(ctx, dispatcherDetach); err != nil {
		logger.Fatal("Dispatcher service error", zap.Error(err))
	}
}

func init() {
	// Initialize root command
	cobraRoot := &cobra.Command{Use: "llm-proxy"}
	rootCmd = cobraRoot // Ensure rootCmd is initialized before any AddCommand calls

	// Register setup command flags
	setupCmd.Flags().StringVar(&configPath, "config", config.EnvOrDefault("CONFIG", ".env"), "Path to the configuration file")
	setupCmd.Flags().StringVar(&openAIAPIKey, "openai-key", config.EnvOrDefault("OPENAI_API_KEY", ""), "OpenAI API Key")
	setupCmd.Flags().StringVar(&managementToken, "management-token", config.EnvOrDefault("MANAGEMENT_TOKEN", ""), "Management token for the proxy")
	setupCmd.Flags().StringVar(&databasePath, "db", config.EnvOrDefault("DATABASE_PATH", "data/llm-proxy.db"), "Path to SQLite database (default: data/llm-proxy.db, overridden by DATABASE_PATH env var or --db flag)")
	setupCmd.Flags().StringVar(&listenAddr, "addr", config.EnvOrDefault("LISTEN_ADDR", "localhost:8080"), "Address to listen on")
	setupCmd.Flags().BoolVar(&interactiveSetup, "interactive", config.EnvBoolOrDefault("INTERACTIVE", false), "Run interactive setup")
	setupCmd.Flags().StringVar(&projectName, "project", config.EnvOrDefault("PROJECT", "DefaultProject"), "Name of the project to create")
	setupCmd.Flags().IntVar(&tokenDuration, "duration", config.EnvIntOrDefault("DURATION", 24), "Duration of the token in hours")
	setupCmd.Flags().BoolVar(&skipProjectSetup, "skip-project", config.EnvBoolOrDefault("SKIP_PROJECT", false), "Skip project and token setup")

	// Add openai parent command
	openaiCmd = &cobra.Command{
		Use:   "openai",
		Short: "Commands for interacting with OpenAI",
		Long:  `Interact with OpenAI services via the LLM Proxy.`,
	}
	openaiCmd.AddCommand(chatCmd)

	// Add all commands to root
	cobraRoot.AddCommand(setupCmd)
	cobraRoot.AddCommand(openaiCmd)
	cobraRoot.AddCommand(serverCmd)
	cobraRoot.AddCommand(adminCmd)

	// Manage command and subcommands
	var manageCmd = &cobra.Command{
		Use:   "manage",
		Short: "Manage projects and tokens",
		Long:  `Project and token management commands (CRUD, generation, validation).`,
	}

	// -- Project subcommands --
	var projectCmd = &cobra.Command{
		Use:   "project",
		Short: "Manage projects",
		Long:  `CRUD operations for projects.`,
	}

	var projectListCmd = &cobra.Command{
		Use:   "list",
		Short: "List all projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = godotenv.Load()
			mgmtToken, err := api.GetManagementToken(cmd)
			if err != nil {
				return err
			}
			url := manageAPIBaseURL + "/manage/projects"
			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				return err
			}
			req.Header.Set("Authorization", "Bearer "+mgmtToken)
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				return err
			}
			defer func() {
				if err := resp.Body.Close(); err != nil {
					log.Printf("Error closing response body: %v", err)
				}
			}()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("server error: %s", resp.Status)
			}
			var projects []api.ProjectCreateResponse
			if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
				return err
			}
			jsonOut, _ := cmd.Flags().GetBool("json")
			if jsonOut {
				out, _ := json.MarshalIndent(projects, "", "  ")
				fmt.Println(string(out))
			} else {
				fmt.Printf("%-36s  %-20s  %-32s  %-20s  %-20s\n", "ID", "Name", "OpenAI Key", "Created", "Updated")
				for _, p := range projects {
					fmt.Printf("%-36s  %-20s  %-32s  %-20s  %-20s\n", p.ID, p.Name, api.ObfuscateKey(p.OpenAIAPIKey), p.CreatedAt.Format("2006-01-02 15:04"), p.UpdatedAt.Format("2006-01-02 15:04"))
				}
			}
			return nil
		},
	}
	projectListCmd.Flags().Bool("json", false, "Output as JSON")
	projectListCmd.Flags().String("management-token", "", "Management token (overrides env)")

	var projectGetCmd = &cobra.Command{
		Use:   "get <project-id>",
		Short: "Get project details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = godotenv.Load()
			mgmtToken, err := api.GetManagementToken(cmd)
			if err != nil {
				return err
			}
			id := args[0]
			url := fmt.Sprintf("%s/manage/projects/%s", manageAPIBaseURL, id)
			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				return err
			}
			req.Header.Set("Authorization", "Bearer "+mgmtToken)
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				return err
			}
			defer func() {
				if err := resp.Body.Close(); err != nil {
					log.Printf("Error closing response body: %v", err)
				}
			}()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("server error: %s", resp.Status)
			}
			var p api.ProjectCreateResponse
			if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
				return err
			}
			jsonOut, _ := cmd.Flags().GetBool("json")
			if jsonOut {
				out, _ := json.MarshalIndent(p, "", "  ")
				fmt.Println(string(out))
			} else {
				fmt.Printf("ID: %s\nName: %s\nOpenAI Key: %s\nCreated: %s\nUpdated: %s\n", p.ID, p.Name, api.ObfuscateKey(p.OpenAIAPIKey), p.CreatedAt.Format("2006-01-02 15:04"), p.UpdatedAt.Format("2006-01-02 15:04"))
			}
			return nil
		},
	}
	projectGetCmd.Flags().Bool("json", false, "Output as JSON")
	projectGetCmd.Flags().String("management-token", "", "Management token (overrides env)")

	var projectCreateCmd = &cobra.Command{
		Use:   "create",
		Short: "Create a new project",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = godotenv.Load()
			mgmtToken, err := api.GetManagementToken(cmd)
			if err != nil {
				return err
			}
			name, _ := cmd.Flags().GetString("name")
			openaiKey, _ := cmd.Flags().GetString("openai-key")
			if name == "" || openaiKey == "" {
				return fmt.Errorf("--name and --openai-key are required")
			}
			body := api.ProjectCreateRequest{Name: name, OpenAIAPIKey: openaiKey}
			jsonBody, _ := json.Marshal(body)
			url := manageAPIBaseURL + "/manage/projects"
			req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
			if err != nil {
				return err
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+mgmtToken)
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				return err
			}
			defer func() {
				if err := resp.Body.Close(); err != nil {
					log.Printf("Error closing response body: %v", err)
				}
			}()
			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
				return fmt.Errorf("server error: %s", resp.Status)
			}
			var p api.ProjectCreateResponse
			if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
				return err
			}
			jsonOut, _ := cmd.Flags().GetBool("json")
			if jsonOut {
				out, _ := json.MarshalIndent(p, "", "  ")
				fmt.Println(string(out))
			} else {
				fmt.Printf("ID: %s\nName: %s\nOpenAI Key: %s\nCreated: %s\nUpdated: %s\n", p.ID, p.Name, api.ObfuscateKey(p.OpenAIAPIKey), p.CreatedAt.Format("2006-01-02 15:04"), p.UpdatedAt.Format("2006-01-02 15:04"))
			}
			return nil
		},
	}
	projectCreateCmd.Flags().String("name", "", "Project name (required)")
	projectCreateCmd.Flags().String("openai-key", "", "OpenAI API Key (required)")
	projectCreateCmd.Flags().Bool("json", false, "Output as JSON")
	projectCreateCmd.Flags().String("management-token", "", "Management token (overrides env)")

	var projectUpdateCmd = &cobra.Command{
		Use:   "update <project-id>",
		Short: "Update a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = godotenv.Load()
			mgmtToken, err := api.GetManagementToken(cmd)
			if err != nil {
				return err
			}
			id := args[0]
			name, _ := cmd.Flags().GetString("name")
			openaiKey, _ := cmd.Flags().GetString("openai-key")
			if name == "" && openaiKey == "" {
				return fmt.Errorf("at least one of --name or --openai-key must be set")
			}
			body := make(map[string]string)
			if name != "" {
				body["name"] = name
			}
			if openaiKey != "" {
				body["openai_api_key"] = openaiKey
			}
			jsonBody, _ := json.Marshal(body)
			url := fmt.Sprintf("%s/manage/projects/%s", manageAPIBaseURL, id)
			req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(jsonBody))
			if err != nil {
				return err
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+mgmtToken)
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				return err
			}
			defer func() {
				if err := resp.Body.Close(); err != nil {
					log.Printf("Error closing response body: %v", err)
				}
			}()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("server error: %s", resp.Status)
			}
			var p api.ProjectCreateResponse
			if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
				return err
			}
			jsonOut, _ := cmd.Flags().GetBool("json")
			if jsonOut {
				out, _ := json.MarshalIndent(p, "", "  ")
				fmt.Println(string(out))
			} else {
				fmt.Printf("ID: %s\nName: %s\nOpenAI Key: %s\nCreated: %s\nUpdated: %s\n", p.ID, p.Name, api.ObfuscateKey(p.OpenAIAPIKey), p.CreatedAt.Format("2006-01-02 15:04"), p.UpdatedAt.Format("2006-01-02 15:04"))
			}
			return nil
		},
	}
	projectUpdateCmd.Flags().String("name", "", "Project name")
	projectUpdateCmd.Flags().String("openai-key", "", "OpenAI API Key")
	projectUpdateCmd.Flags().Bool("json", false, "Output as JSON")
	projectUpdateCmd.Flags().String("management-token", "", "Management token (overrides env)")

	var projectDeleteCmd = &cobra.Command{
		Use:   "delete <project-id>",
		Short: "Delete a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = godotenv.Load()
			mgmtToken, err := api.GetManagementToken(cmd)
			if err != nil {
				return err
			}
			id := args[0]
			url := fmt.Sprintf("%s/manage/projects/%s", manageAPIBaseURL, id)
			req, err := http.NewRequest("DELETE", url, nil)
			if err != nil {
				return err
			}
			req.Header.Set("Authorization", "Bearer "+mgmtToken)
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				return err
			}
			defer func() {
				if err := resp.Body.Close(); err != nil {
					log.Printf("Error closing response body: %v", err)
				}
			}()
			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
				return fmt.Errorf("server error: %s", resp.Status)
			}
			fmt.Printf("Project %s deleted.\n", id)
			return nil
		},
	}
	projectDeleteCmd.Flags().Bool("json", false, "Output as JSON")
	projectDeleteCmd.Flags().String("management-token", "", "Management token (overrides env)")

	// -- Token subcommands --
	var tokenCmd = &cobra.Command{
		Use:   "token",
		Short: "Manage tokens",
		Long:  `Generate and validate tokens.`,
	}

	var tokenGenerateCmd = &cobra.Command{
		Use:   "generate",
		Short: "Generate a new token",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load .env if present
			_ = godotenv.Load()

			// Get management token: flag > env
			mgmtToken, _ := cmd.Flags().GetString("management-token")
			if mgmtToken == "" {
				mgmtToken = os.Getenv("MANAGEMENT_TOKEN")
			}
			if mgmtToken == "" {
				return fmt.Errorf("management token is required (set MANAGEMENT_TOKEN env or use --management-token)")
			}

			// Get project ID (required)
			projectID, _ := cmd.Flags().GetString("project-id")
			if projectID == "" {
				return fmt.Errorf("--project-id is required")
			}

			// Get duration (default 1440 = 24h)
			duration, _ := cmd.Flags().GetInt("duration")
			if duration <= 0 {
				duration = 1440
			}

			// Prepare request
			body := map[string]interface{}{
				"project_id":       projectID,
				"duration_minutes": duration,
			}
			jsonBody, _ := json.Marshal(body)
			url := manageAPIBaseURL + "/manage/tokens"
			req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
			if err != nil {
				return fmt.Errorf("failed to create request: %w", err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+mgmtToken)

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("request failed: %w", err)
			}
			defer func() {
				if err := resp.Body.Close(); err != nil {
					log.Printf("Error closing response body: %v", err)
				}
			}()

			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
				var errResp map[string]string
				if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
					return fmt.Errorf("failed to parse response: %w", err)
				}
				return fmt.Errorf("server error: %s", errResp["error"])
			}

			var result struct {
				Token     string `json:"token"`
				ExpiresAt string `json:"expires_at"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			jsonOut, _ := cmd.Flags().GetBool("json")
			if jsonOut {
				out, _ := json.MarshalIndent(result, "", "  ")
				fmt.Println(string(out))
			} else {
				fmt.Printf("Token: %s\n", result.Token)
				fmt.Printf("Obfuscated: %s\n", token.ObfuscateToken(result.Token))
				fmt.Printf("Expires at: %s\n", result.ExpiresAt)
			}
			return nil
		},
	}

	tokenGenerateCmd.Flags().String("management-token", "", "Management token (overrides env)")
	tokenGenerateCmd.Flags().String("project-id", "", "Project ID (required)")
	tokenGenerateCmd.Flags().Int("duration", 1440, "Token duration in minutes (default 1440 = 24h)")
	tokenGenerateCmd.Flags().Bool("json", false, "Output as JSON")

	// Cache command and subcommands
	var cacheCmd = &cobra.Command{
		Use:   "cache",
		Short: "Manage cache operations",
		Long:  `Cache management operations (purge).`,
	}

	var cachePurgeCmd = &cobra.Command{
		Use:   "purge",
		Short: "Purge cache entries",
		Long:  `Purge cache entries by exact key (method + URL) or by prefix.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = godotenv.Load()

			// Get flags
			apiBaseURL, _ := cmd.Flags().GetString("api-base-url")
			managementToken, _ := cmd.Flags().GetString("management-token")
			method, _ := cmd.Flags().GetString("method")
			url, _ := cmd.Flags().GetString("url")
			prefix, _ := cmd.Flags().GetString("prefix")
			jsonOutput, _ := cmd.Flags().GetBool("json")

			// Fallback to env vars if flags not provided
			if apiBaseURL == "" {
				apiBaseURL = os.Getenv("MANAGEMENT_API_BASE_URL")
				if apiBaseURL == "" {
					apiBaseURL = "http://localhost:8080"
				}
			}
			if managementToken == "" {
				managementToken = os.Getenv("MANAGEMENT_TOKEN")
			}

			if managementToken == "" {
				return fmt.Errorf("management token is required (use --management-token flag or MANAGEMENT_TOKEN env var)")
			}

			// Validate required fields
			if method == "" || url == "" {
				return fmt.Errorf("method and url are required")
			}

			// Prepare request body
			reqBody := map[string]interface{}{
				"method": method,
				"url":    url,
			}
			if prefix != "" {
				reqBody["prefix"] = prefix
			}

			jsonData, err := json.Marshal(reqBody)
			if err != nil {
				return fmt.Errorf("failed to marshal request: %w", err)
			}

			// Make HTTP request
			client := &http.Client{Timeout: 30 * time.Second}
			req, err := http.NewRequest("POST", strings.TrimRight(apiBaseURL, "/")+"/manage/cache/purge", bytes.NewBuffer(jsonData))
			if err != nil {
				return fmt.Errorf("failed to create request: %w", err)
			}

			req.Header.Set("Authorization", "Bearer "+managementToken)
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("failed to make request: %w", err)
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("failed to read response: %w", err)
			}

			if resp.StatusCode != http.StatusOK {
				if jsonOutput {
					fmt.Printf(`{"error": "HTTP %d: %s"}`+"\n", resp.StatusCode, string(body))
				} else {
					fmt.Printf("Error: HTTP %d: %s\n", resp.StatusCode, string(body))
				}
				return fmt.Errorf("cache purge failed")
			}

			if jsonOutput {
				fmt.Println(string(body))
			} else {
				var response map[string]interface{}
				if err := json.Unmarshal(body, &response); err != nil {
					fmt.Println("Cache purge completed")
				} else {
					if prefix != "" {
						fmt.Printf("Cache prefix purge completed: %v entries deleted\n", response["deleted"])
					} else {
						deleted := response["deleted"].(bool)
						if deleted {
							fmt.Println("Cache entry deleted successfully")
						} else {
							fmt.Println("Cache entry was not found")
						}
					}
				}
			}

			return nil
		},
	}

	cachePurgeCmd.Flags().String("api-base-url", "", "Management API base URL (overrides env)")
	cachePurgeCmd.Flags().String("management-token", "", "Management token (overrides env)")
	cachePurgeCmd.Flags().String("method", "", "HTTP method (required)")
	cachePurgeCmd.Flags().String("url", "", "URL path (required)")
	cachePurgeCmd.Flags().String("prefix", "", "Cache key prefix for bulk purge")
	cachePurgeCmd.Flags().Bool("json", false, "Output as JSON")

	// Register cache subcommands
	cacheCmd.AddCommand(cachePurgeCmd)

	// Register project subcommands
	projectCmd.AddCommand(projectListCmd, projectGetCmd, projectCreateCmd, projectUpdateCmd, projectDeleteCmd)
	// Register token subcommands
	tokenCmd.AddCommand(tokenGenerateCmd)
	// Register manage subcommands
	manageCmd.AddCommand(projectCmd)
	manageCmd.AddCommand(tokenCmd)
	manageCmd.AddCommand(cacheCmd)
	// Register manage command with root
	cobraRoot.AddCommand(manageCmd)

	// In the same place where openaiCmd is assigned, assign benchmarkCmd as well
	benchmarkCmd = &cobra.Command{
		Use:   "benchmark",
		Short: "Run benchmarks against the LLM Proxy",
		Long: `Benchmark latency, throughput, and error rates by sending concurrent POST requests.

Required flags:
  --base-url        Base URL of the target (e.g., http://localhost:8080 or https://api.openai.com/v1)
  --endpoint        API path to hit (e.g., /v1/chat/completions or /chat/completions for OpenAI)
  --token           Bearer token (proxy token or OpenAI API key)
  --requests, -r    Total number of requests to send
  --concurrency, -c Number of concurrent workers

Optional flags:
  --json            JSON request body for POST requests
  --debug           Print sample responses and headers by status code

Latency breakdown:
  Request includes X-REQUEST-START (ns). The proxy returns X-UPSTREAM-REQUEST-START and X-UPSTREAM-REQUEST-STOP (ns)
  so the tool can split upstream vs proxy latency precisely.
`,
		Example: `  # Proxy (local)
  llm-proxy benchmark \
    --base-url "http://localhost:8080" \
    --endpoint "/v1/chat/completions" \
    --token "$PROXY_TOKEN" \
    --requests 100 --concurrency 4 \
    --json '{"model":"gpt-4.1-nano","messages":[{"role":"user","content":"hi"}]}'

  # OpenAI (direct)
  llm-proxy benchmark \
    --base-url "https://api.openai.com/v1" \
    --endpoint "/chat/completions" \
    --token "$OPENAI_API_KEY" \
    --requests 100 --concurrency 4 \
    --json '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}'

  # Show sample responses and headers per status code
  llm-proxy benchmark \
    --base-url "http://localhost:8080" \
    --endpoint "/v1/chat/completions" \
    --token "$PROXY_TOKEN" \
    --requests 20 --concurrency 5 \
    --json '{"model":"gpt-4.1-nano","messages":[{"role":"user","content":"hi"}]}' \
    --debug`,
		Run: func(cmd *cobra.Command, args []string) {
			baseURL, _ := cmd.Flags().GetString("base-url")
			endpoint, _ := cmd.Flags().GetString("endpoint")
			token, _ := cmd.Flags().GetString("token")
			totalRequests, _ := cmd.Flags().GetInt("requests")
			concurrency, _ := cmd.Flags().GetInt("concurrency")
			jsonBody, _ := cmd.Flags().GetString("json")
			debug, _ := cmd.Flags().GetBool("debug")
			cacheMode, _ := cmd.Flags().GetBool("cache")
			cacheTTL, _ := cmd.Flags().GetInt("cache-ttl")
			method, _ := cmd.Flags().GetString("method")

			if baseURL == "" || endpoint == "" || token == "" || totalRequests <= 0 || concurrency <= 0 {
				fmt.Fprintln(os.Stderr, "Missing required flags or invalid values.")
				_ = cmd.Help()
				osExit(1)
			}

			type result struct {
				latency     time.Duration
				err         error
				errMsg      string
				response    string
				statusCode  int
				headers     http.Header
				reqHeaders  http.Header
				reqBody     string
				upstreamLat time.Duration
				proxyLat    time.Duration
			}
			results := make(chan result, totalRequests)
			start := time.Now()

			var wg sync.WaitGroup
			requestsPerWorker := totalRequests / concurrency
			extra := totalRequests % concurrency

			var (
				progressMu              sync.Mutex
				sent, completed, failed int
			)

			statusCounts := make(map[int]int)
			statusSamples := make(map[int]struct {
				response   string
				headers    http.Header
				errMsg     string
				reqHeaders http.Header
				reqBody    string
			})

			var latencies []time.Duration
			var upstreamLatencies []time.Duration
			var proxyLatencies []time.Duration

			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				workerID := i + 1
				count := requestsPerWorker
				if i < extra {
					count++
				}
				go func(id, n int) {
					defer wg.Done()
					for j := 0; j < n; j++ {
						progressMu.Lock()
						sent++
						fmt.Printf("\rRequests sent: %d, completed: %d, failed: %d", sent, completed, failed)
						progressMu.Unlock()
						requestStart := time.Now()
						requestStartNs := requestStart.UnixNano()
						verb := strings.ToUpper(strings.TrimSpace(method))
						if verb == "" {
							verb = "POST"
						}
						var bodyReader io.Reader
						var reqBodyStr string
						if verb == http.MethodPost || verb == http.MethodPut || verb == http.MethodPatch {
							if jsonBody != "" {
								reqBodyStr = jsonBody
								bodyReader = strings.NewReader(jsonBody)
							}
						}
						req, _ := http.NewRequest(verb, baseURL+endpoint, bodyReader)
						req.Header.Set("Authorization", "Bearer "+token)
						if jsonBody != "" && (verb == http.MethodPost || verb == http.MethodPut || verb == http.MethodPatch) {
							req.Header.Set("Content-Type", "application/json")
						}
						// If cache mode is enabled, ask intermediaries to treat the response as cacheable
						if cacheMode {
							if cacheTTL <= 0 {
								cacheTTL = 86400 // default to 1 day if not provided
							}
							req.Header.Set("Cache-Control", fmt.Sprintf("public, max-age=%d", cacheTTL))
						}
						// Add X-REQUEST-START header
						req.Header.Set("X-REQUEST-START", fmt.Sprintf("%d", requestStartNs))

						// Snapshot request headers for debug (with redacted Authorization)
						sentReqHeaders := req.Header.Clone()
						if v := sentReqHeaders.Get("Authorization"); v != "" {
							sentReqHeaders.Set("Authorization", "Bearer <redacted>")
						}

						client := &http.Client{Timeout: 10 * time.Second}
						resp, err := client.Do(req)
						responseEnd := time.Now()
						lat := responseEnd.Sub(requestStart)
						var respBody string
						statusCode := 0
						var upstreamLat, proxyLat time.Duration
						var headers http.Header
						var errMsg string
						if err == nil && resp != nil {
							statusCode = resp.StatusCode
							b, _ := io.ReadAll(resp.Body)
							respBody = string(b)
							_ = resp.Body.Close()
							headers = resp.Header.Clone()
							// Parse new latency headers (all as int64 nanoseconds)
							var reqStart, upStart, upStop int64
							if v := resp.Request.Header.Get("X-REQUEST-START"); v != "" {
								reqStart, _ = strconv.ParseInt(v, 10, 64)
							} else {
								reqStart = requestStartNs // fallback to local
							}
							if v := resp.Header.Get("X-UPSTREAM-REQUEST-START"); v != "" {
								if val, err := strconv.ParseInt(v, 10, 64); err == nil {
									upStart = val
								}
							}
							if v := resp.Header.Get("X-UPSTREAM-REQUEST-STOP"); v != "" {
								if val, err := strconv.ParseInt(v, 10, 64); err == nil {
									upStop = val
								}
							}
							if upStart > 0 && upStop > 0 && reqStart > 0 {
								upstreamLat = time.Duration(upStop - upStart)
								proxyLat = time.Duration((upStart - reqStart) + (responseEnd.UnixNano() - upStop))
							} else {
								upstreamLat = 0
								proxyLat = 0
							}
						} else {
							errMsg = err.Error()
						}
						results <- result{latency: lat, err: err, errMsg: errMsg, response: respBody, statusCode: statusCode, headers: headers, reqHeaders: sentReqHeaders, reqBody: reqBodyStr, upstreamLat: upstreamLat, proxyLat: proxyLat}
						progressMu.Lock()
						if err != nil || statusCode < 200 || statusCode >= 300 {
							failed++
						}
						completed++
						fmt.Printf("\rRequests sent: %d, completed: %d, failed: %d", sent, completed, failed)
						progressMu.Unlock()
						if err == nil && statusCode >= 200 && statusCode < 300 {
							latencies = append(latencies, lat)
							if upstreamLat > 0 {
								upstreamLatencies = append(upstreamLatencies, upstreamLat)
							}
							if proxyLat > 0 {
								proxyLatencies = append(proxyLatencies, proxyLat)
							}
						}
					}
				}(workerID, count)
			}
			wg.Wait()
			close(results)
			totalTime := time.Since(start)

			// Aggregate results
			var (
				success                                          int
				totalLatency                                     time.Duration
				minLatency, maxLatency                           time.Duration
				first                                            = true
				sampleResponse                                   string
				totalUpstreamLat, minUpstreamLat, maxUpstreamLat time.Duration
				totalProxyLat, minProxyLat, maxProxyLat          time.Duration
				upstreamLatCount, proxyLatCount                  int
				cacheHits                                        int
				sawCacheHeader                                   bool
			)
			for r := range results {
				isSuccess := r.err == nil && r.statusCode >= 200 && r.statusCode < 300
				statusCounts[r.statusCode]++
				if _, ok := statusSamples[r.statusCode]; !ok {
					statusSamples[r.statusCode] = struct {
						response   string
						headers    http.Header
						errMsg     string
						reqHeaders http.Header
						reqBody    string
					}{r.response, r.headers, r.errMsg, r.reqHeaders, r.reqBody}
				}
				if r.headers != nil {
					v := strings.ToLower(r.headers.Get("X-PROXY-CACHE"))
					if v != "" {
						sawCacheHeader = true
					}
					if v == "hit" || v == "conditional-hit" {
						cacheHits++
					}
				}
				if isSuccess {
					success++
					totalLatency += r.latency
					if first || r.latency < minLatency {
						minLatency = r.latency
					}
					if first || r.latency > maxLatency {
						maxLatency = r.latency
					}
					if r.upstreamLat > 0 {
						totalUpstreamLat += r.upstreamLat
						if upstreamLatCount == 0 || r.upstreamLat < minUpstreamLat {
							minUpstreamLat = r.upstreamLat
						}
						if upstreamLatCount == 0 || r.upstreamLat > maxUpstreamLat {
							maxUpstreamLat = r.upstreamLat
						}
						upstreamLatCount++
					}
					if r.proxyLat > 0 {
						totalProxyLat += r.proxyLat
						if proxyLatCount == 0 || r.proxyLat < minProxyLat {
							minProxyLat = r.proxyLat
						}
						if proxyLatCount == 0 || r.proxyLat > maxProxyLat {
							maxProxyLat = r.proxyLat
						}
						proxyLatCount++
					}
					if sampleResponse == "" && r.response != "" {
						sampleResponse = r.response
					}
					first = false
					if r.upstreamLat > 0 {
						upstreamLatencies = append(upstreamLatencies, r.upstreamLat)
					}
					if r.proxyLat > 0 {
						proxyLatencies = append(proxyLatencies, r.proxyLat)
					}
				}
			}
			avgLatency := time.Duration(0)
			if success > 0 {
				avgLatency = totalLatency / time.Duration(success)
			}
			requestsPerSec := float64(totalRequests) / totalTime.Seconds()
			avgUpstreamLat := time.Duration(0)
			if upstreamLatCount > 0 {
				avgUpstreamLat = totalUpstreamLat / time.Duration(upstreamLatCount)
			}
			avgProxyLat := time.Duration(0)
			if proxyLatCount > 0 {
				avgProxyLat = totalProxyLat / time.Duration(proxyLatCount)
			}

			// Calculate p90 latency
			p90Latency := time.Duration(0)
			if len(latencies) > 0 {
				sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
				idx := int(float64(len(latencies))*0.9) - 1
				if idx < 0 {
					idx = 0
				}
				p90Latency = latencies[idx]
			}
			upstreamP90 := time.Duration(0)
			if len(upstreamLatencies) > 0 {
				sort.Slice(upstreamLatencies, func(i, j int) bool { return upstreamLatencies[i] < upstreamLatencies[j] })
				idx := int(float64(len(upstreamLatencies))*0.9) - 1
				if idx < 0 {
					idx = 0
				}
				upstreamP90 = upstreamLatencies[idx]
			}
			proxyP90 := time.Duration(0)
			if len(proxyLatencies) > 0 {
				sort.Slice(proxyLatencies, func(i, j int) bool { return proxyLatencies[i] < proxyLatencies[j] })
				idx := int(float64(len(proxyLatencies))*0.9) - 1
				if idx < 0 {
					idx = 0
				}
				proxyP90 = proxyLatencies[idx]
			}

			// Calculate p90 mean (trimmed mean) for overall, upstream, and proxy latencies
			p90Mean := time.Duration(0)
			if len(latencies) > 0 {
				p90Idx := int(float64(len(latencies)) * 0.9)
				if p90Idx < 1 {
					p90Idx = 1
				}
				trimmed := latencies[:p90Idx]
				var sum time.Duration
				for _, v := range trimmed {
					sum += v
				}
				p90Mean = sum / time.Duration(len(trimmed))
			}
			upstreamP90Mean := time.Duration(0)
			if len(upstreamLatencies) > 0 {
				p90Idx := int(float64(len(upstreamLatencies)) * 0.9)
				if p90Idx < 1 {
					p90Idx = 1
				}
				trimmed := upstreamLatencies[:p90Idx]
				var sum time.Duration
				for _, v := range trimmed {
					sum += v
				}
				upstreamP90Mean = sum / time.Duration(len(trimmed))
			}
			proxyP90Mean := time.Duration(0)
			if len(proxyLatencies) > 0 {
				p90Idx := int(float64(len(proxyLatencies)) * 0.9)
				if p90Idx < 1 {
					p90Idx = 1
				}
				trimmed := proxyLatencies[:p90Idx]
				var sum time.Duration
				for _, v := range trimmed {
					sum += v
				}
				proxyP90Mean = sum / time.Duration(len(trimmed))
			}

			// Helper to format durations nicely
			formatDuration := func(d time.Duration) string {
				if d == 0 {
					return "0"
				}
				if d >= time.Second {
					return fmt.Sprintf("%.3fs", d.Seconds())
				} else if d >= time.Millisecond {
					return fmt.Sprintf("%.3fms", float64(d)/float64(time.Millisecond))
				} else if d >= time.Microsecond {
					return fmt.Sprintf("%.3fµs", float64(d)/float64(time.Microsecond))
				}
				return fmt.Sprintf("%dns", d.Nanoseconds())
			}

			// Table-like summary output
			tableWidth := 50
			border := "+" + strings.Repeat("-", tableWidth-2) + "+"
			fmt.Println("\n" + border)
			fmt.Printf("| %-21s | %-22d |\n", "Total requests", totalRequests)
			fmt.Printf("| %-21s | %-22d |\n", "Concurrency", concurrency)
			fmt.Printf("| %-21s | %-22.2f |\n", "Duration (s)", totalTime.Seconds())
			fmt.Printf("| %-21s | %-22d |\n", "Success", success)
			fmt.Printf("| %-21s | %-22d |\n", "Failed", failed)
			fmt.Printf("| %-21s | %-22.2f |\n", "Requests/sec", requestsPerSec)
			fmt.Printf("| %-21s | %-22s |\n", "Avg latency", formatDuration(avgLatency))
			fmt.Printf("| %-21s | %-22s |\n", "Min latency", formatDuration(minLatency))
			fmt.Printf("| %-21s | %-22s |\n", "Max latency", formatDuration(maxLatency))
			fmt.Printf("| %-21s | %-22s |\n", "p90 latency", formatDuration(p90Latency))
			fmt.Printf("| %-21s | %-22s |\n", "p90 mean latency", formatDuration(p90Mean))
			// Show cache hits if cache is in play
			if cacheHits > 0 || sawCacheHeader || cacheMode {
				fmt.Printf("| %-21s | %-22d |\n", "Cache hits", cacheHits)
			}
			if success > 0 {
				if upstreamLatCount > 0 {
					fmt.Printf("| %-21s | %-22s |\n", "Upstream latency avg", formatDuration(avgUpstreamLat))
					fmt.Printf("| %-21s | %-22s |\n", "Upstream latency min", formatDuration(minUpstreamLat))
					fmt.Printf("| %-21s | %-22s |\n", "Upstream latency max", formatDuration(maxUpstreamLat))
					fmt.Printf("| %-21s | %-22s |\n", "Upstream latency p90", formatDuration(upstreamP90))
					fmt.Printf("| %-21s | %-22s |\n", "Upstream latency p90 mean", formatDuration(upstreamP90Mean))
				} else {
					fmt.Printf("| %-21s | %-22s |\n", "Upstream latency", "N/A")
				}
				if proxyLatCount > 0 {
					fmt.Printf("| %-21s | %-22s |\n", "Proxy latency avg", formatDuration(avgProxyLat))
					fmt.Printf("| %-21s | %-22s |\n", "Proxy latency min", formatDuration(minProxyLat))
					fmt.Printf("| %-21s | %-22s |\n", "Proxy latency max", formatDuration(maxProxyLat))
					fmt.Printf("| %-21s | %-22s |\n", "Proxy latency p90", formatDuration(proxyP90))
					fmt.Printf("| %-21s | %-22s |\n", "Proxy latency p90 mean", formatDuration(proxyP90Mean))
				} else {
					fmt.Printf("| %-21s | %-22s |\n", "Proxy latency", "N/A")
				}
			}
			fmt.Println(border)
			// Add header for response code requests
			fmt.Printf("| %-21s%26s|\n", "Response code", "")
			// Print status codes as normal rows at the end of the main table
			for code, count := range statusCounts {
				if code == 0 {
					fmt.Printf("| %-21s | %-22d |\n", "Network error", count)
				} else {
					fmt.Printf("| %-21s | %-22d |\n", fmt.Sprintf("%d", code), count)
				}
			}
			fmt.Println(border)

			// Print sample responses in debug mode
			if debug {
				// Print a sample for each status code
				fmt.Println("\nSample responses by status code:")
				for code, sample := range statusSamples {
					if code == 0 {
						fmt.Println("\nNETWORK ERROR:")
						if sample.errMsg != "" {
							fmt.Printf("Error: %s\n", sample.errMsg)
						} else {
							fmt.Println("<no error message>")
						}
					} else {
						fmt.Printf("\nStatus %d:\n", code)
						if sample.response != "" {
							fmt.Printf("Response body: %s\n", sample.response)
						} else {
							fmt.Println("Response body: <empty body>")
						}
						if sample.headers != nil {
							fmt.Println("Response headers:")
							for k, v := range sample.headers {
								fmt.Printf("  %s: %s\n", k, strings.Join(v, ", "))
							}
						}
						if sample.reqHeaders != nil {
							fmt.Println("Request headers:")
							for k, v := range sample.reqHeaders {
								fmt.Printf("  %s: %s\n", k, strings.Join(v, ", "))
							}
						}
						if sample.reqBody != "" {
							fmt.Printf("Request body: %s\n", sample.reqBody)
						}
					}
				}
			}
		},
	}
	benchmarkCmd.Flags().String("base-url", "", "Base URL of the LLM Proxy (required)")
	benchmarkCmd.Flags().String("endpoint", "", "API endpoint to benchmark (required)")
	benchmarkCmd.Flags().String("token", "", "Token for authentication (required)")
	benchmarkCmd.Flags().IntP("requests", "r", 1, "Total number of requests to send (required)")
	benchmarkCmd.Flags().IntP("concurrency", "c", 1, "Number of concurrent workers (required)")
	benchmarkCmd.Flags().String("json", "", "Optional JSON string to use as the POST body for each request")
	benchmarkCmd.Flags().Bool("debug", false, "Show detailed sample response and headers")
	benchmarkCmd.Flags().Bool("cache", false, "Set Cache-Control: public with a high TTL for benchmarking cache behavior")
	benchmarkCmd.Flags().Int("cache-ttl", 0, "TTL seconds to use with --cache (default 86400)")
	benchmarkCmd.Flags().String("method", "POST", "HTTP method to use (GET, POST, PUT, PATCH)")
	rootCmd.AddCommand(benchmarkCmd)

	// Add persistent flag for management API base URL
	cobraRoot.PersistentFlags().StringVar(&manageAPIBaseURL, "manage-api-base-url", "http://localhost:8080", "Base URL for management API (default: http://localhost:8080)")

	// Add dispatcher command flags
	dispatcherCmd.Flags().StringVar(&dispatcherService, "service", config.EnvOrDefault("DISPATCHER_SERVICE", "file"), "Dispatcher service type (file, lunary, helicone)")
	dispatcherCmd.Flags().StringVar(&dispatcherEndpoint, "endpoint", config.EnvOrDefault("DISPATCHER_ENDPOINT", ""), "Dispatcher endpoint URL (file: path, lunary/helicone: API endpoint)")
	dispatcherCmd.Flags().StringVar(&dispatcherAPIKey, "api-key", config.EnvOrDefault("LLM_PROXY_API_KEY", ""), "API key for external services (lunary, helicone)")
	dispatcherCmd.Flags().IntVar(&dispatcherBuffer, "buffer", config.EnvIntOrDefault("DISPATCHER_BUFFER", 1000), "Event bus buffer size")
	dispatcherCmd.Flags().IntVar(&dispatcherBatch, "batch-size", config.EnvIntOrDefault("DISPATCHER_BATCH_SIZE", 100), "Batch size for sending events")
	dispatcherCmd.Flags().BoolVar(&dispatcherDetach, "detach", config.EnvBoolOrDefault("DISPATCHER_DETACH", false), "Run in background (daemon mode)")
	rootCmd.AddCommand(dispatcherCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}
