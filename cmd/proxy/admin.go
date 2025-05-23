package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/sofatutor/llm-proxy/internal/admin"
	"github.com/sofatutor/llm-proxy/internal/config"
	"github.com/spf13/cobra"
)

// Admin command flags
var (
	adminListenAddr      string
	adminAPIBaseURL      string
	adminManagementToken string
	adminEnvFile         string
)

var adminCmd = &cobra.Command{
	Use:   "admin",
	Short: "Start the Admin UI server",
	Long:  `Start the optional Admin UI server for managing projects and tokens via web interface.`,
	Run:   runAdmin,
}

func init() {
	// Admin command flags
	adminCmd.Flags().StringVar(&adminListenAddr, "listen", ":8081", "Admin UI server listen address")
	adminCmd.Flags().StringVar(&adminAPIBaseURL, "api-base-url", "http://localhost:8080", "Base URL for Management API")
	adminCmd.Flags().StringVar(&adminManagementToken, "management-token", "", "Management token for API access")
	adminCmd.Flags().StringVar(&adminEnvFile, "env", ".env", "Path to .env file")
}

// runAdmin starts the Admin UI server
func runAdmin(cmd *cobra.Command, args []string) {
	// Load environment variables from file if specified
	if adminEnvFile != "" {
		if err := godotenv.Load(adminEnvFile); err != nil {
			log.Printf("Warning: Could not load env file %s: %v", adminEnvFile, err)
		}
	}

	// Create configuration from defaults
	cfg := config.DefaultConfig()

	// Override with command line flags if provided
	if adminListenAddr != ":8081" {
		cfg.AdminUI.ListenAddr = adminListenAddr
	}
	if adminAPIBaseURL != "http://localhost:8080" {
		cfg.AdminUI.APIBaseURL = adminAPIBaseURL
	}
	if adminManagementToken != "" {
		cfg.AdminUI.ManagementToken = adminManagementToken
	}

	// Check for management token in environment if not provided via flag
	if cfg.AdminUI.ManagementToken == "" {
		if token := os.Getenv("MANAGEMENT_TOKEN"); token != "" {
			cfg.AdminUI.ManagementToken = token
		} else {
			log.Fatal("Management token is required (use --management-token flag or MANAGEMENT_TOKEN env var)")
		}
	}

	// Create admin server
	adminServer, err := admin.NewServer(cfg)
	if err != nil {
		log.Fatalf("Failed to create admin server: %v", err)
	}

	// Handle graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	// Start server in goroutine
	go func() {
		log.Printf("Admin UI server starting on %s", cfg.AdminUI.ListenAddr)
		log.Printf("Management API: %s", cfg.AdminUI.APIBaseURL)
		log.Printf("Open your browser and go to http://localhost%s", cfg.AdminUI.ListenAddr)
		log.Printf("You'll be prompted to enter your management token for authentication")
		log.Printf("Press Ctrl+C to stop")

		if err := adminServer.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Admin server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	<-done
	log.Println("Admin UI server shutting down...")

	// Create a deadline for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := adminServer.Shutdown(ctx); err != nil {
		log.Fatalf("Admin server forced to shutdown: %v", err)
	}

	log.Println("Admin UI server exited gracefully")
}