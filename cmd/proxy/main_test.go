package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/spf13/cobra"
)

// Variables that we'll override in tests
var (
	originalOsExit = osExit
)

func TestCommandHelp(t *testing.T) {
	// Save the original os.Exit function
	origExit := osExit
	
	// Create a mock exit function
	osExit = func(code int) {
		// Do nothing in tests
	}
	
	// Restore the original function after the test
	defer func() {
		osExit = origExit
	}()
	
	// Test each command's help
	commands := []*cobra.Command{rootCmd, setupCmd, openaiCmd, chatCmd, benchmarkCmd, serverCmd}
	
	for _, cmd := range commands {
		t.Run(cmd.Name(), func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w
			
			// Run the help command
			cmd.SetArgs([]string{"--help"})
			err := cmd.Execute()
			
			// Close the pipe and restore stdout
			w.Close()
			os.Stdout = oldStdout
			
			// Read the output
			var buf bytes.Buffer
			_, _ = buf.ReadFrom(r)
			output := buf.String()
			
			// Assert
			if err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
			
			if output == "" {
				t.Error("Expected help output, got empty string")
			}
		})
	}
}

func TestChatCommandArgs(t *testing.T) {
	// Test that the chat command correctly sets up flags
	if chatCmd.Flags().Lookup("token") == nil {
		t.Error("Expected 'token' flag to be defined for chat command")
	}
	
	if chatCmd.Flags().Lookup("model") == nil {
		t.Error("Expected 'model' flag to be defined for chat command")
	}
	
	if chatCmd.Flags().Lookup("temperature") == nil {
		t.Error("Expected 'temperature' flag to be defined for chat command")
	}
}

func TestSetupCommandArgs(t *testing.T) {
	// Test that the setup command correctly sets up flags
	if setupCmd.Flags().Lookup("config") == nil {
		t.Error("Expected 'config' flag to be defined for setup command")
	}
	
	if setupCmd.Flags().Lookup("openai-key") == nil {
		t.Error("Expected 'openai-key' flag to be defined for setup command")
	}
	
	if setupCmd.Flags().Lookup("interactive") == nil {
		t.Error("Expected 'interactive' flag to be defined for setup command")
	}
}

func TestServerCommandArgs(t *testing.T) {
	// Test that the server command correctly sets up flags
	if serverCmd.Flags().Lookup("daemon") == nil {
		t.Error("Expected 'daemon' flag to be defined for server command")
	}
	
	if serverCmd.Flags().Lookup("env") == nil {
		t.Error("Expected 'env' flag to be defined for server command")
	}
	
	if serverCmd.Flags().Lookup("pid-file") == nil {
		t.Error("Expected 'pid-file' flag to be defined for server command")
	}
}