package main

import (
	"os"
	"testing"
)

func TestMain(t *testing.T) {
	// Call the main function
	// In the case of the benchmark command, it's just a placeholder that does nothing yet
	main()
}

func TestMain_Benchmark(t *testing.T) {
	// Save and restore os.Args
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	os.Args = []string{"llm-proxy-benchmark", "--help"}
	// main() will print help and exit
	main()
}
