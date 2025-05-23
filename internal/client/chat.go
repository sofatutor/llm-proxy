// Package client provides HTTP client functionality for communicating with the LLM Proxy API.
package client

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/chzyer/readline"
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

// ChatCompletionStreamResponse represents a chunked response in a stream
type ChatCompletionStreamResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int    `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role    string `json:"role,omitempty"`
			Content string `json:"content,omitempty"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// ChatClient handles communication with the LLM Proxy chat API
type ChatClient struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

// ChatOptions configures chat request parameters
type ChatOptions struct {
	Model        string
	Temperature  float64
	MaxTokens    int
	UseStreaming bool
	VerboseMode  bool
}

// NewChatClient creates a new chat client
func NewChatClient(baseURL, token string) *ChatClient {
	return &ChatClient{
		BaseURL: baseURL,
		Token:   token,
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// SendChatRequest sends a chat request and returns the response
func (c *ChatClient) SendChatRequest(messages []ChatMessage, options ChatOptions, readline *readline.Instance) (*ChatResponse, error) {
	if c.Token == "" {
		return nil, fmt.Errorf("token is required")
	}

	// Validate proxy URL
	if _, err := url.Parse(c.BaseURL); err != nil {
		return nil, fmt.Errorf("invalid proxy URL: %w", err)
	}

	request := ChatRequest{
		Model:       options.Model,
		Messages:    messages,
		Temperature: options.Temperature,
		MaxTokens:   options.MaxTokens,
		Stream:      options.UseStreaming,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	if options.VerboseMode {
		fmt.Printf("Request: %s\n", string(jsonData))
	}

	req, err := http.NewRequest("POST", c.BaseURL+"/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to close response body: %v\n", err)
		}
	}()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	if options.UseStreaming {
		return c.handleStreamingResponse(resp, readline, options.VerboseMode)
	}

	return c.handleNonStreamingResponse(resp, options.VerboseMode)
}

// handleStreamingResponse processes streaming chat responses
func (c *ChatClient) handleStreamingResponse(resp *http.Response, readline *readline.Instance, verbose bool) (*ChatResponse, error) {
	scanner := bufio.NewScanner(resp.Body)
	var finalResponse *ChatResponse

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var streamResp ChatCompletionStreamResponse
		if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
			if verbose {
				fmt.Printf("Failed to parse stream data: %v\n", err)
			}
			continue
		}

		if len(streamResp.Choices) > 0 {
			choice := streamResp.Choices[0]
			if choice.Delta.Content != "" {
				if readline != nil && readline.Config.Stdout != nil {
					if _, err := readline.Config.Stdout.Write([]byte(choice.Delta.Content)); err != nil {
						fmt.Fprintf(os.Stderr, "failed to write streaming content: %v\n", err)
					}
				} else {
					fmt.Print(choice.Delta.Content)
				}
			}

			// Convert to final response format
			if finalResponse == nil {
				finalResponse = &ChatResponse{
					ID:      streamResp.ID,
					Object:  streamResp.Object,
					Created: streamResp.Created,
					Model:   streamResp.Model,
					Choices: []struct {
						Index        int         `json:"index"`
						Message      ChatMessage `json:"message"`
						FinishReason string      `json:"finish_reason"`
					}{
						{
							Index: choice.Index,
							Message: ChatMessage{
								Role:    "assistant",
								Content: "",
							},
							FinishReason: choice.FinishReason,
						},
					},
					Usage: streamResp.Usage,
				}
			}

			// Accumulate content
			finalResponse.Choices[0].Message.Content += choice.Delta.Content
			if choice.FinishReason != "" {
				finalResponse.Choices[0].FinishReason = choice.FinishReason
			}
		}
	}

	if readline != nil && readline.Config.Stdout != nil {
		if _, err := readline.Config.Stdout.Write([]byte("\n")); err != nil {
			fmt.Fprintf(os.Stderr, "failed to write newline after streaming: %v\n", err)
		}
	} else {
		fmt.Println()
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("stream reading error: %w", err)
	}

	if finalResponse == nil {
		return nil, fmt.Errorf("no response received from stream")
	}

	return finalResponse, nil
}

// handleNonStreamingResponse processes non-streaming chat responses
func (c *ChatClient) handleNonStreamingResponse(resp *http.Response, verbose bool) (*ChatResponse, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if verbose {
		fmt.Printf("Response: %s\n", string(body))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &chatResp, nil
}
