// Package main implements the Admin UI server for the LLM Proxy.
// This is a separate, optional component that provides a web interface
// for managing projects and tokens via the Management API.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sofatutor/llm-proxy/internal/admin"
	"github.com/sofatutor/llm-proxy/internal/config"
)

var (
	version = "0.1.0"
)

func main() {
	var (
		configPath      = flag.String("config", "", "Path to configuration file")
		listenAddr      = flag.String("listen", ":8081", "Admin UI server listen address")
		apiBaseURL      = flag.String("api-base-url", "http://localhost:8080", "Base URL for Management API")
		managementToken = flag.String("management-token", "", "Management token for API access")
		showVersion     = flag.Bool("version", false, "Show version and exit")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("llm-proxy admin-server v%s\n", version)
		return
	}

	// Load configuration
	cfg, err := loadConfig(*configPath, *listenAddr, *apiBaseURL, *managementToken)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create admin server
	server, err := admin.NewServer(cfg)
	if err != nil {
		log.Fatalf("Failed to create admin server: %v", err)
	}

	// Start server in goroutine
	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("Admin UI server starting on %s", cfg.AdminUI.ListenAddr)
		serverErrors <- server.Start()
	}()

	// Wait for interrupt signal
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		log.Fatalf("Server error: %v", err)
	case sig := <-shutdown:
		log.Printf("Received signal %v, starting graceful shutdown...", sig)

		// Create context with timeout for shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Shutdown server
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Error during shutdown: %v", err)
		}
		log.Println("Admin UI server stopped")
	}
}

// loadConfig loads configuration from file and command line flags
func loadConfig(configPath, listenAddr, apiBaseURL, managementToken string) (*config.Config, error) {
	var cfg *config.Config
	var err error

	if configPath != "" {
		cfg, err = config.LoadFromFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load config file: %w", err)
		}
	} else {
		cfg = config.DefaultConfig()
	}

	// Override with command line flags
	if listenAddr != ":8081" {
		cfg.AdminUI.ListenAddr = listenAddr
	}
	if apiBaseURL != "http://localhost:8080" {
		cfg.AdminUI.APIBaseURL = apiBaseURL
	}
	if managementToken != "" {
		cfg.AdminUI.ManagementToken = managementToken
	}

	// Validate required fields
	if cfg.AdminUI.ManagementToken == "" {
		if token := os.Getenv("MANAGEMENT_TOKEN"); token != "" {
			cfg.AdminUI.ManagementToken = token
		} else {
			return nil, fmt.Errorf("management token is required (use --management-token flag or MANAGEMENT_TOKEN env var)")
		}
	}

	return cfg, nil
}
