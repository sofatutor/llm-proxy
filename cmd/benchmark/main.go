// Package main is the entry point for the LLM Proxy CLI tool.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// Used to allow mocking in tests
	osExit = os.Exit
)

var rootCmd = &cobra.Command{
	Use:   "llm-proxy",
	Short: "LLM Proxy CLI tool",
	Long:  `CLI tool for working with the LLM Proxy, including setup and OpenAI chat functionality.`,
}

var openaiCmd = &cobra.Command{
	Use:   "openai",
	Short: "Commands for interacting with OpenAI",
	Long:  `Interact with OpenAI services via the LLM Proxy.`,
}

var benchmarkCmd = &cobra.Command{
	Use:   "benchmark",
	Short: "Run benchmarks against the LLM Proxy",
	Long:  `Benchmark performance metrics including latency, throughput, and errors.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Benchmark command not yet implemented")
	},
}

func init() {
	// Add subcommands to the root command
	rootCmd.AddCommand(openaiCmd)
	rootCmd.AddCommand(benchmarkCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		osExit(1)
	}
}
