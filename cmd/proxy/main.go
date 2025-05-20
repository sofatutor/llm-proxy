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

func init() {
	// Define command line flags
	flag.StringVar(&envFile, "env", ".env", "Path to .env file")
	flag.StringVar(&listenAddr, "addr", "", "Address to listen on (overrides env var)")
	flag.StringVar(&databasePath, "db", "", "Path to SQLite database (overrides env var)")
	flag.StringVar(&logLevel, "log-level", "", "Log level: debug, info, warn, error (overrides env var)")
}

func main() {
	// Parse command line flags
	flag.Parse()

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
		os.Setenv("LISTEN_ADDR", listenAddr)
	}
	if databasePath != "" {
		os.Setenv("DATABASE_PATH", databasePath)
	}
	if logLevel != "" {
		os.Setenv("LOG_LEVEL", logLevel)
	}

	// Load configuration
	cfg, err := config.New()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create and start the server
	srv := server.New(cfg)

	// Handle graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.Start(); err != nil {
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
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited gracefully")
}