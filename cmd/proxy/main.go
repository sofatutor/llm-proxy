package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/sofatutor/llm-proxy/internal/api"
	"github.com/sofatutor/llm-proxy/internal/token"
	"github.com/spf13/cobra"
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

// For test compatibility
var rootCmd *cobra.Command
var openaiCmd *cobra.Command
var benchmarkCmd *cobra.Command

func init() {
	// Initialize root command
	cobraRoot := &cobra.Command{Use: "llm-proxy"}

	// Register setup command flags
	setupCmd.Flags().StringVar(&configPath, "config", ".env", "Path to the configuration file")
	setupCmd.Flags().StringVar(&openAIAPIKey, "openai-key", "", "OpenAI API Key")
	setupCmd.Flags().StringVar(&managementToken, "management-token", "", "Management token for the proxy")
	setupCmd.Flags().StringVar(&databasePath, "db", "data/llm-proxy.db", "Path to SQLite database (default: data/llm-proxy.db, overridden by DATABASE_PATH env var or --db flag)")
	setupCmd.Flags().StringVar(&listenAddr, "addr", "localhost:8080", "Address to listen on")
	setupCmd.Flags().BoolVar(&interactiveSetup, "interactive", false, "Run interactive setup")
	setupCmd.Flags().StringVar(&projectName, "project", "DefaultProject", "Name of the project to create")
	setupCmd.Flags().IntVar(&tokenDuration, "duration", 24, "Duration of the token in hours")
	setupCmd.Flags().BoolVar(&skipProjectSetup, "skip-project", false, "Skip project and token setup")

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

			// Get duration (default 24)
			duration, _ := cmd.Flags().GetInt("duration")
			if duration <= 0 {
				duration = 24
			}

			// Prepare request
			body := map[string]interface{}{
				"project_id":     projectID,
				"duration_hours": duration,
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
	tokenGenerateCmd.Flags().Int("duration", 24, "Token duration in hours (default 24)")
	tokenGenerateCmd.Flags().Bool("json", false, "Output as JSON")

	// Register project subcommands
	projectCmd.AddCommand(projectListCmd, projectGetCmd, projectCreateCmd, projectUpdateCmd, projectDeleteCmd)
	// Register token subcommands
	tokenCmd.AddCommand(tokenGenerateCmd)
	// Register manage subcommands
	manageCmd.AddCommand(projectCmd)
	manageCmd.AddCommand(tokenCmd)
	// Register manage command with root
	cobraRoot.AddCommand(manageCmd)

	// In the same place where openaiCmd is assigned, assign benchmarkCmd as well
	benchmarkCmd = &cobra.Command{
		Use:   "benchmark",
		Short: "Run benchmarks against the LLM Proxy",
		Long:  `Benchmark performance metrics including latency, throughput, and errors.`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Benchmark command not yet implemented")
		},
	}
	cobraRoot.AddCommand(benchmarkCmd)

	// Add persistent flag for management API base URL
	cobraRoot.PersistentFlags().StringVar(&manageAPIBaseURL, "manage-api-base-url", "http://localhost:8080", "Base URL for management API (default: http://localhost:8080)")

	// For test compatibility, define rootCmd as alias
	rootCmd = cobraRoot
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}
