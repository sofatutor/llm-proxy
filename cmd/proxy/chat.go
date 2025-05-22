package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/chzyer/readline"
	"github.com/sofatutor/llm-proxy/internal/api"
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

// ChatMessage represents a message in the chat
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest represents a request to the chat API
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Stream      bool          `json:"stream"`
}

// ChatCompletionStreamResponse represents a chunked response in a stream
type ChatCompletionStreamResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int    `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int    `json:"index"`
		Delta        Delta  `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

// Delta represents a piece of a message
type Delta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

// ChatResponse represents a response from the chat API
type ChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int    `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int         `json:"index"`
		Message      ChatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

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
	chatCmd.Flags().StringVar(&model, "model", "gpt-3.5-turbo", "Model to use")
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
	// Record start time for verbose mode
	startTime := time.Now()

	// Construct request
	request := ChatRequest{
		Model:       model,
		Messages:    messages,
		Temperature: temperature,
		MaxTokens:   maxTokens,
		Stream:      useStreaming,
	}

	// Convert request to JSON
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %v", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", proxyURL+"/v1/chat/completions", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+proxyToken)
	if useStreaming {
		req.Header.Set("Accept", "text/event-stream")
	}

	// Always make a real HTTP request to the proxy, regardless of streaming mode
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Error closing response body: %v", err)
		}
	}()

	// Check for error status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(body))
	}

	// If verbose mode, print detailed timing headers for both streaming and non-streaming
	if verboseMode {
		grey := func(s string) string { return "\033[90m" + s + "\033[0m" }
		onset := startTime
		receivedAt := api.ParseTimeHeader(resp.Header.Get("X-Proxy-Received-At"))
		sentAt := api.ParseTimeHeader(resp.Header.Get("X-Proxy-Sent-Backend-At"))
		firstRespAt := api.ParseTimeHeader(resp.Header.Get("X-Proxy-First-Response-At"))
		finalRespAt := api.ParseTimeHeader(resp.Header.Get("X-Proxy-Final-Response-At"))

		fmt.Printf("\n%s\n", grey(fmt.Sprintf("[Verbose] Response status: %s", resp.Status)))
		if !receivedAt.IsZero() {
			fmt.Printf("%s\n", grey(fmt.Sprintf("[Verbose] Proxy received: %s", receivedAt.Format(time.RFC3339Nano))))
		}
		if !sentAt.IsZero() {
			fmt.Printf("%s\n", grey(fmt.Sprintf("[Verbose] Proxy sent to backend: %s", sentAt.Format(time.RFC3339Nano))))
		}
		if !firstRespAt.IsZero() {
			fmt.Printf("%s\n", grey(fmt.Sprintf("[Verbose] First response from backend: %s", firstRespAt.Format(time.RFC3339Nano))))
		}
		if !finalRespAt.IsZero() {
			fmt.Printf("%s\n", grey(fmt.Sprintf("[Verbose] Final response from backend: %s", finalRespAt.Format(time.RFC3339Nano))))
		}
		if !onset.IsZero() && !receivedAt.IsZero() {
			fmt.Printf("%s\n", grey(fmt.Sprintf("[Verbose] Client → Proxy: %s", receivedAt.Sub(onset))))
		}
		if !receivedAt.IsZero() && !sentAt.IsZero() {
			fmt.Printf("%s\n", grey(fmt.Sprintf("[Verbose] Proxy overhead (pre-backend): %s", sentAt.Sub(receivedAt))))
		}
		if !sentAt.IsZero() && !firstRespAt.IsZero() {
			fmt.Printf("%s\n", grey(fmt.Sprintf("[Verbose] Backend latency (first byte): %s", firstRespAt.Sub(sentAt))))
		}
		if !firstRespAt.IsZero() && !finalRespAt.IsZero() {
			fmt.Printf("%s\n", grey(fmt.Sprintf("[Verbose] Streaming duration: %s", finalRespAt.Sub(firstRespAt))))
		}
		if !onset.IsZero() && !finalRespAt.IsZero() {
			fmt.Printf("%s\n", grey(fmt.Sprintf("[Verbose] Total end-to-end: %s", finalRespAt.Sub(onset))))
		}
	}

	// Handle streaming response
	if useStreaming {
		var fullContent strings.Builder
		model := ""
		scanner := bufio.NewScanner(resp.Body)

		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			// Handle the SSE data
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}

			// Parse the JSON in the SSE data
			var streamResp ChatCompletionStreamResponse
			if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
				if _, err := fmt.Fprintf(rl.Stdout(), "Error parsing SSE data: %v\n", err); err != nil {
					log.Printf("Error writing SSE parse error: %v", err)
				}
				continue
			}

			// Extract content if available
			if len(streamResp.Choices) > 0 {
				delta := streamResp.Choices[0].Delta
				if delta.Content != "" {
					fullContent.WriteString(delta.Content)
					if _, err := rl.Stdout().Write([]byte(delta.Content)); err != nil {
						log.Printf("Error writing streamed content: %v", err)
					}
				}

				// Save model info
				if model == "" && streamResp.Model != "" {
					model = streamResp.Model
				}

				// Check for finish reason (no action needed)
			}
		}

		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("error reading stream: %v", err)
		}

		// After streaming, print a newline and refresh the prompt
		if _, err := rl.Stdout().Write([]byte("\n")); err != nil {
			log.Printf("Error writing newline after stream: %v", err)
		}
		rl.Refresh()

		// Create a synthetic response from the streamed content
		syntheticResponse := &ChatResponse{
			ID:      "stream-response",
			Object:  "chat.completion",
			Created: int(time.Now().Unix()),
			Model:   model,
			Choices: []struct {
				Index        int         `json:"index"`
				Message      ChatMessage `json:"message"`
				FinishReason string      `json:"finish_reason"`
			}{
				{
					Index: 0,
					Message: ChatMessage{
						Role:    "assistant",
						Content: fullContent.String(),
					},
					FinishReason: "stop",
				},
			},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{
				// We don't have exact token counts from streaming
				PromptTokens:     0,
				CompletionTokens: 0,
				TotalTokens:      0,
			},
		}

		// After streaming loop (for streaming responses), print verbose timing block using proxy timing headers
		// For non-streaming, print after reading the response
		if verboseMode {
			grey := func(s string) string { return "\033[90m" + s + "\033[0m" }
			if _, err := fmt.Fprintf(rl.Stdout(), "\n%s\n", grey(fmt.Sprintf("[Verbose] Total request duration: %s", time.Since(startTime)))); err != nil {
				log.Printf("Error writing verbose timing: %v", err)
			}
			remoteDuration := resp.Header.Get("X-LLM-Proxy-Remote-Duration-Ms")
			if remoteDuration != "" {
				if _, err := fmt.Fprintf(rl.Stdout(), "%s\n", grey(fmt.Sprintf("[Verbose] Proxy reported remote duration: %s ms", remoteDuration))); err != nil {
					log.Printf("Error writing remote duration: %v", err)
				}
				if callDuration, err := time.ParseDuration(remoteDuration + "ms"); err == nil {
					localOverhead := time.Since(startTime) - callDuration
					if _, err := fmt.Fprintf(rl.Stdout(), "%s\n", grey(fmt.Sprintf("[Verbose] Proxy overhead: %s", localOverhead))); err != nil {
						log.Printf("Error writing proxy overhead: %v", err)
					}
				}
			}
		}

		return syntheticResponse, nil
	} else {
		// Handle non-streaming response
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("error reading response: %v", err)
		}

		// Parse response
		var response ChatResponse
		if err := json.Unmarshal(body, &response); err != nil {
			return nil, fmt.Errorf("error parsing response: %v", err)
		}

		// If verbose mode, show timing information
		if verboseMode {
			grey := func(s string) string { return "\033[90m" + s + "\033[0m" }
			duration := time.Since(startTime)
			fmt.Printf("\n%s\n", grey(fmt.Sprintf("[Verbose] Total request duration: %s", duration)))
			remoteDuration := resp.Header.Get("X-LLM-Proxy-Remote-Duration-Ms")
			if remoteDuration != "" {
				fmt.Printf("%s\n", grey(fmt.Sprintf("[Verbose] Proxy reported remote duration: %s ms", remoteDuration)))
				if callDuration, err := time.ParseDuration(remoteDuration + "ms"); err == nil {
					localOverhead := duration - callDuration
					fmt.Printf("%s\n", grey(fmt.Sprintf("[Verbose] Proxy overhead: %s", localOverhead)))
				}
			}
		}

		return &response, nil
	}
}
