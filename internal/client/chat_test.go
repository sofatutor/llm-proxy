package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chzyer/readline"
)

func TestNewChatClient(t *testing.T) {
	baseURL := "http://localhost:8080"
	token := "test-token"

	client := NewChatClient(baseURL, token)

	if client.BaseURL != baseURL {
		t.Errorf("BaseURL = %q, want %q", client.BaseURL, baseURL)
	}
	if client.Token != token {
		t.Errorf("Token = %q, want %q", client.Token, token)
	}
	if client.HTTPClient == nil {
		t.Error("HTTPClient should not be nil")
	}
	if client.HTTPClient.Timeout != 60*time.Second {
		t.Errorf("Timeout = %v, want %v", client.HTTPClient.Timeout, 60*time.Second)
	}
}

func TestChatClient_SendChatRequest_MissingToken(t *testing.T) {
	client := NewChatClient("http://localhost:8080", "")

	messages := []ChatMessage{{Role: "user", Content: "test"}}
	options := ChatOptions{Model: "gpt-4.1-mini"}

	_, err := client.SendChatRequest(messages, options, nil)
	if err == nil {
		t.Error("expected error for missing token, got nil")
	}
	if !strings.Contains(err.Error(), "token is required") {
		t.Errorf("error = %q, want to contain 'token is required'", err.Error())
	}
}

func TestChatClient_SendChatRequest_InvalidURL(t *testing.T) {
	client := NewChatClient(":invalid-url", "token")

	messages := []ChatMessage{{Role: "user", Content: "test"}}
	options := ChatOptions{Model: "gpt-4.1-mini"}

	_, err := client.SendChatRequest(messages, options, nil)
	if err == nil {
		t.Error("expected error for invalid URL, got nil")
	}
	if !strings.Contains(err.Error(), "invalid proxy URL") {
		t.Errorf("error = %q, want to contain 'invalid proxy URL'", err.Error())
	}
}

func TestChatClient_SendChatRequest_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := w.Write([]byte("Internal Server Error")); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	client := NewChatClient(server.URL, "token")
	messages := []ChatMessage{{Role: "user", Content: "test"}}
	options := ChatOptions{Model: "gpt-4.1-mini"}

	_, err := client.SendChatRequest(messages, options, nil)
	if err == nil {
		t.Error("expected error for API error, got nil")
	}
	if !strings.Contains(err.Error(), "API error 500") {
		t.Errorf("error = %q, want to contain 'API error 500'", err.Error())
	}
}

func TestChatClient_SendChatRequest_NonStreaming(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("Method = %s, want POST", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/v1/chat/completions") {
			t.Errorf("Path = %s, want to end with /v1/chat/completions", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer token" {
			t.Errorf("Authorization = %q, want 'Bearer token'", auth)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want 'application/json'", ct)
		}

		// Parse request body
		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}

		// Return valid response
		response := ChatResponse{
			ID:    "test-id",
			Model: "gpt-4.1-mini",
			Choices: []struct {
				Index        int         `json:"index"`
				Message      ChatMessage `json:"message"`
				FinishReason string      `json:"finish_reason"`
			}{
				{
					Index:        0,
					Message:      ChatMessage{Role: "assistant", Content: "Hello!"},
					FinishReason: "stop",
				},
			},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client := NewChatClient(server.URL, "token")
	messages := []ChatMessage{{Role: "user", Content: "Hello"}}
	options := ChatOptions{
		Model:        "gpt-4.1-mini",
		Temperature:  0.7,
		MaxTokens:    100,
		UseStreaming: false,
		VerboseMode:  false,
	}

	response, err := client.SendChatRequest(messages, options, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if response.ID != "test-id" {
		t.Errorf("ID = %q, want 'test-id'", response.ID)
	}
	if len(response.Choices) != 1 {
		t.Errorf("len(Choices) = %d, want 1", len(response.Choices))
	}
	if response.Choices[0].Message.Content != "Hello!" {
		t.Errorf("Content = %q, want 'Hello!'", response.Choices[0].Message.Content)
	}
}

func TestChatClient_SendChatRequest_Streaming(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		// Send streaming response
		chunks := []string{
			`data: {"id":"test-id","object":"chat.completion.chunk","created":123,"model":"gpt-4.1-mini","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"},"finish_reason":""}]}`,
			`data: {"id":"test-id","object":"chat.completion.chunk","created":123,"model":"gpt-4.1-mini","choices":[{"index":0,"delta":{"content":" there!"},"finish_reason":""}]}`,
			`data: {"id":"test-id","object":"chat.completion.chunk","created":123,"model":"gpt-4.1-mini","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
			`data: [DONE]`,
		}

		for _, chunk := range chunks {
			if _, err := w.Write([]byte(chunk + "\n")); err != nil {
				t.Errorf("failed to write chunk: %v", err)
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer server.Close()

	client := NewChatClient(server.URL, "token")
	messages := []ChatMessage{{Role: "user", Content: "Hello"}}
	options := ChatOptions{
		Model:        "gpt-4.1-mini",
		UseStreaming: true,
		VerboseMode:  false,
	}

	response, err := client.SendChatRequest(messages, options, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if response.ID != "test-id" {
		t.Errorf("ID = %q, want 'test-id'", response.ID)
	}
	if len(response.Choices) != 1 {
		t.Errorf("len(Choices) = %d, want 1", len(response.Choices))
	}
	if response.Choices[0].Message.Content != "Hello there!" {
		t.Errorf("Content = %q, want 'Hello there!'", response.Choices[0].Message.Content)
	}
}

func TestChatClient_SendChatRequest_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("invalid json")); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	client := NewChatClient(server.URL, "token")
	messages := []ChatMessage{{Role: "user", Content: "test"}}
	options := ChatOptions{Model: "gpt-4.1-mini", UseStreaming: false}

	_, err := client.SendChatRequest(messages, options, nil)
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
	if !strings.Contains(err.Error(), "failed to parse response") {
		t.Errorf("error = %q, want to contain 'failed to parse response'", err.Error())
	}
}

func TestChatClient_SendChatRequest_EmptyStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		// Empty stream
	}))
	defer server.Close()

	client := NewChatClient(server.URL, "token")
	messages := []ChatMessage{{Role: "user", Content: "test"}}
	options := ChatOptions{Model: "gpt-4.1-mini", UseStreaming: true}

	_, err := client.SendChatRequest(messages, options, nil)
	if err == nil {
		t.Error("expected error for empty stream, got nil")
	}
	if !strings.Contains(err.Error(), "no response received from stream") {
		t.Errorf("error = %q, want to contain 'no response received from stream'", err.Error())
	}
}

func TestChatClient_SendChatRequest_VerboseMode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return simple response
		response := ChatResponse{
			ID:    "test",
			Model: "gpt-4.1-mini",
			Choices: []struct {
				Index        int         `json:"index"`
				Message      ChatMessage `json:"message"`
				FinishReason string      `json:"finish_reason"`
			}{{Index: 0, Message: ChatMessage{Role: "assistant", Content: "Hi"}, FinishReason: "stop"}},
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client := NewChatClient(server.URL, "token")
	messages := []ChatMessage{{Role: "user", Content: "test"}}
	options := ChatOptions{
		Model:        "gpt-4.1-mini",
		VerboseMode:  true,
		UseStreaming: false,
	}

	// This should not error, though we can't easily test the verbose output
	_, err := client.SendChatRequest(messages, options, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleStreamingResponse_Errors(t *testing.T) {
	client := NewChatClient("http://localhost", "token")

	t.Run("malformed JSON in stream", func(t *testing.T) {
		resp := &http.Response{
			Body: io.NopCloser(bytes.NewBufferString("data: {not json}\ndata: [DONE]\n")),
		}
		_, err := client.handleStreamingResponse(resp, nil, false)
		if err == nil || !strings.Contains(err.Error(), "no response received from stream") {
			t.Errorf("expected error for no response, got %v", err)
		}
	})

	t.Run("scanner error", func(t *testing.T) {
		r := &errReader{err: errors.New("scanner fail")}
		resp := &http.Response{Body: io.NopCloser(r)}
		_, err := client.handleStreamingResponse(resp, nil, false)
		if err == nil || !strings.Contains(err.Error(), "stream reading error") {
			t.Errorf("expected scanner error, got %v", err)
		}
	})

	t.Run("write error to readline.Config.Stdout", func(t *testing.T) {
		var wrote bool
		fakeStdout := &errWriter{err: errors.New("write fail"), wrote: &wrote}
		fakeRL := &readline.Instance{Config: &readline.Config{Stdout: fakeStdout}}
		// valid streaming chunk
		chunk := `data: {"id":"id","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"content":"hi"},"finish_reason":""}]}` + "\ndata: [DONE]\n"
		resp := &http.Response{Body: io.NopCloser(bytes.NewBufferString(chunk))}
		_, err := client.handleStreamingResponse(resp, fakeRL, false)
		if err == nil || !strings.Contains(err.Error(), "failed to write streaming content") {
			t.Errorf("expected write error, got %v", err)
		}
		if !wrote {
			t.Error("expected write to be attempted")
		}
	})
}

type errReader struct{ err error }

func (e *errReader) Read(p []byte) (int, error) { return 0, e.err }
func (e *errReader) Close() error               { return nil }

// errWriter simulates a writer that always errors
// wrote is set to true if Write is called
type errWriter struct {
	err   error
	wrote *bool
}

func (e *errWriter) Write(p []byte) (int, error) {
	if e.wrote != nil {
		*e.wrote = true
	}
	return 0, e.err
}

func TestHandleNonStreamingResponse_Errors(t *testing.T) {
	client := NewChatClient("http://localhost", "token")

	t.Run("io.ReadAll error", func(t *testing.T) {
		resp := &http.Response{Body: &errReader{err: errors.New("read fail")}}
		_, err := client.handleNonStreamingResponse(resp, false)
		if err == nil || !strings.Contains(err.Error(), "failed to read response body") {
			t.Errorf("expected read error, got %v", err)
		}
	})

	t.Run("json.Unmarshal error", func(t *testing.T) {
		resp := &http.Response{Body: io.NopCloser(bytes.NewBufferString("not json"))}
		_, err := client.handleNonStreamingResponse(resp, false)
		if err == nil || !strings.Contains(err.Error(), "failed to parse response") {
			t.Errorf("expected parse error, got %v", err)
		}
	})
}
