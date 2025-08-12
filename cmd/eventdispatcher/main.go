package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/sofatutor/llm-proxy/internal/eventbus"
)

// Usage:
//   go run ./cmd/eventdispatcher/main.go --file events.jsonl --buffer 100
//
// This service subscribes to the in-memory event bus and writes each event as a JSONL entry to the specified file.

// envOrDefault returns the value of the environment variable if set, otherwise the fallback.
func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envIntOrDefault returns the int value of the environment variable if set and valid, otherwise the fallback.
func envIntOrDefault(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func main() {
	os.Exit(run())
}

func run() int {
	var (
		filePath   string
		bufferSize int
	)
	flag.StringVar(&filePath, "file", envOrDefault("EVENTDISPATCHER_FILE", "events.jsonl"), "Path to the output JSONL file")
	flag.IntVar(&bufferSize, "buffer", envIntOrDefault("EVENTDISPATCHER_BUFFER", 100), "Event bus buffer size")
	flag.Parse()

	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("failed to open file: %v", err)
		return 1
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Printf("failed to close file: %v", err)
		}
	}()

	bus := eventbus.NewInMemoryEventBus(bufferSize)
	defer bus.Stop()

	sub := bus.Subscribe()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle SIGINT/SIGTERM for graceful shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		fmt.Fprintln(os.Stderr, "\nShutting down event dispatcher...")
		cancel()
	}()

	log.Printf("File event dispatcher started. Writing events to %s", filePath)

	for {
		select {
		case evt, ok := <-sub:
			if !ok {
				return 0
			}
			line, err := json.Marshal(evt)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to marshal event: %v\n", err)
				continue
			}
			if _, err := f.Write(append(line, '\n')); err != nil {
				fmt.Fprintf(os.Stderr, "failed to write event: %v\n", err)
			}
		case <-ctx.Done():
			return 0
		}
	}
}
