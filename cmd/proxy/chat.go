package main

import (
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/chzyer/readline"
	"github.com/sofatutor/llm-proxy/internal/client"
	"github.com/spf13/cobra"
)

// Chat command flags
var (
	proxyURL     string
	proxyToken   string
	model        string
	temperature  float64
	maxTokens    int
	systemPrompt string
	verboseMode  bool
	useStreaming bool
)

// Type aliases for backward compatibility
type ChatMessage = client.ChatMessage
type ChatResponse = client.ChatResponse

// Add this before init()
var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Interactive chat with OpenAI models",
	Long:  `Start an interactive chat session with OpenAI models via the LLM Proxy.`,
	Run:   runChat,
}

func init() {
	// Chat command flags
	chatCmd.Flags().StringVar(&proxyURL, "proxy", "http://localhost:8080", "LLM Proxy URL")
	chatCmd.Flags().StringVar(&proxyToken, "token", "", "LLM Proxy token")
	chatCmd.Flags().StringVar(&model, "model", "gpt-4.1-mini", "Model to use")
	chatCmd.Flags().Float64Var(&temperature, "temperature", 0.7, "Temperature for generation")
	chatCmd.Flags().IntVar(&maxTokens, "max-tokens", 0, "Maximum tokens to generate (0 = no limit)")
	chatCmd.Flags().StringVar(&systemPrompt, "system", "You are a helpful assistant.", "System prompt")
	chatCmd.Flags().BoolVarP(&verboseMode, "verbose", "v", false, "Show detailed timing information")
	chatCmd.Flags().BoolVar(&useStreaming, "stream", true, "Use streaming for responses")

	// Make token required
	if err := chatCmd.MarkFlagRequired("token"); err != nil {
		log.Printf("Warning: could not mark 'token' flag as required: %v", err)
	}
}

// runChat is the main function for the chat command
func runChat(cmd *cobra.Command, args []string) {
	// Print session information
	fmt.Println("Starting chat session with", model)
	if useStreaming {
		fmt.Println("Streaming mode enabled")
	}
	if verboseMode {
		fmt.Println("Verbose mode enabled")
	}
	fmt.Println("Type 'exit' or 'quit' to end the session")
	fmt.Println("System prompt:", systemPrompt)
	fmt.Println()

	// Initialize messages with system prompt
	messages := []ChatMessage{
		{Role: "system", Content: systemPrompt},
	}

	// Use readline for advanced input (arrow keys, history, etc)
	rl, err := readline.New("> ")
	if err != nil {
		fmt.Printf("Error initializing readline: %v\n", err)
		return
	}
	defer func() {
		if err := rl.Close(); err != nil {
			fmt.Printf("Error closing readline: %v\n", err)
		}
	}()

	for {
		// Read user input
		input, err := rl.Readline()
		if err == readline.ErrInterrupt {
			if len(input) == 0 {
				fmt.Println("Ending chat session")
				break
			} else {
				continue
			}
		} else if err == io.EOF {
			fmt.Println("Ending chat session")
			break
		} else if err != nil {
			fmt.Printf("Error reading input: %v\n", err)
			continue
		}

		// Trim input and check for exit commands
		input = strings.TrimSpace(input)
		if input == "exit" || input == "quit" {
			fmt.Println("Ending chat session")
			break
		}

		if input == "" {
			continue
		}

		// Add user message
		messages = append(messages, ChatMessage{Role: "user", Content: input})

		// Get response from API (with or without streaming)
		response, err := getChatResponse(messages, rl)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		// Extract assistant message
		if len(response.Choices) > 0 {
			assistantMessage := response.Choices[0].Message
			messages = append(messages, assistantMessage)

			// If streaming, the message has already been printed,
			// so we don't need to print it again
			if !useStreaming {
				fmt.Println(assistantMessage.Content)
			}

			// Print usage statistics if available
			if response.Usage.TotalTokens > 0 && verboseMode {
				grey := func(s string) string { return "\033[90m" + s + "\033[0m" }
				fmt.Printf("\n%s\n\n", grey(fmt.Sprintf("[Tokens: %d prompt, %d completion, %d total]",
					response.Usage.PromptTokens,
					response.Usage.CompletionTokens,
					response.Usage.TotalTokens)))
			} else if useStreaming && verboseMode {
				grey := func(s string) string { return "\033[90m" + s + "\033[0m" }
				fmt.Printf("\n%s\n\n", grey("[Token counts not available in streaming mode]"))
			}
		} else {
			fmt.Println("No response from model")
		}
	}
}

// getChatResponse sends a request to the chat API and returns the response
func getChatResponse(messages []ChatMessage, rl *readline.Instance) (*ChatResponse, error) {
	if proxyToken == "" {
		return nil, fmt.Errorf("token is required")
	}

	if proxyURL == "" {
		return nil, fmt.Errorf("proxy URL is required")
	}

	chatClient := client.NewChatClient(proxyURL, proxyToken)
	options := client.ChatOptions{
		Model:        model,
		Temperature:  temperature,
		MaxTokens:    maxTokens,
		UseStreaming: useStreaming,
		VerboseMode:  verboseMode,
	}

	return chatClient.SendChatRequest(messages, options, rl)
}
