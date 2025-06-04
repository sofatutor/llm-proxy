package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/sofatutor/llm-proxy/internal/eventbus"
)

// HeliconePlugin sends events to Helicone observability platform.
type HeliconePlugin struct {
	apiKey   string
	endpoint string
	client   *http.Client
}

// NewHeliconePlugin creates a new Helicone dispatcher plugin.
func NewHeliconePlugin() *HeliconePlugin {
	return &HeliconePlugin{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Init initializes the Helicone plugin with configuration.
func (p *HeliconePlugin) Init(config map[string]string) error {
	apiKey, ok := config["api_key"]
	if !ok || apiKey == "" {
		return fmt.Errorf("api_key is required for Helicone plugin")
	}

	endpoint, ok := config["endpoint"]
	if !ok || endpoint == "" {
		endpoint = "https://api.hconeai.com/v1/request" // Default Helicone endpoint
	}

	p.apiKey = apiKey
	p.endpoint = endpoint

	return nil
}

// HeliconeRequest represents the request format expected by Helicone.
type HeliconeRequest struct {
	Model            string            `json:"model,omitempty"`
	Provider         string            `json:"provider,omitempty"`
	RequestID        string            `json:"request_id"`
	RequestCreatedAt string            `json:"request_created_at"`
	RequestPath      string            `json:"request_path"`
	RequestMethod    string            `json:"request_method"`
	ResponseStatus   int               `json:"response_status"`
	ResponseTime     int64             `json:"response_time_ms"`
	RequestHeaders   map[string]string `json:"request_headers,omitempty"`
	ResponseHeaders  map[string]string `json:"response_headers,omitempty"`
	RequestBody      string            `json:"request_body,omitempty"`
	ResponseBody     string            `json:"response_body,omitempty"`
	PromptTokens     int               `json:"prompt_tokens,omitempty"`
	CompletionTokens int               `json:"completion_tokens,omitempty"`
	TotalTokens      int               `json:"total_tokens,omitempty"`
	Cost             float64           `json:"cost,omitempty"`
	Properties       map[string]string `json:"properties,omitempty"`
}

// Send converts the event to Helicone format and sends it.
func (p *HeliconePlugin) Send(ctx context.Context, event eventbus.Event) error {
	// Convert event to Helicone format
	heliconeReq := HeliconeRequest{
		Provider:         "openai",
		RequestID:        event.RequestID,
		RequestCreatedAt: time.Now().Format(time.RFC3339),
		RequestPath:      event.Path,
		RequestMethod:    event.Method,
		ResponseStatus:   event.Status,
		ResponseTime:     event.Duration.Milliseconds(),
		ResponseBody:     string(event.ResponseBody),
		ResponseHeaders:  p.convertHeaders(event.ResponseHeaders),
	}

	// Marshal to JSON
	payload, err := json.Marshal(heliconeReq)
	if err != nil {
		return fmt.Errorf("failed to marshal Helicone request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", p.endpoint, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Helicone-Auth", "Bearer "+p.apiKey)

	// Send request
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to Helicone: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Helicone returned status %d", resp.StatusCode)
	}

	return nil
}

// convertHeaders converts http.Header to map[string]string for JSON serialization.
func (p *HeliconePlugin) convertHeaders(headers http.Header) map[string]string {
	result := make(map[string]string)
	for key, values := range headers {
		if len(values) > 0 {
			result[key] = values[0] // Take first value
		}
	}
	return result
}

// Name returns the plugin name.
func (p *HeliconePlugin) Name() string {
	return "helicone"
}
