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

// LunaryPlugin sends events to Lunary observability platform.
type LunaryPlugin struct {
	apiKey   string
	endpoint string
	client   *http.Client
}

// NewLunaryPlugin creates a new Lunary dispatcher plugin.
func NewLunaryPlugin() *LunaryPlugin {
	return &LunaryPlugin{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Init initializes the Lunary plugin with configuration.
func (p *LunaryPlugin) Init(config map[string]string) error {
	apiKey, ok := config["api_key"]
	if !ok || apiKey == "" {
		return fmt.Errorf("api_key is required for Lunary plugin")
	}

	endpoint, ok := config["endpoint"]
	if !ok || endpoint == "" {
		endpoint = "https://api.lunary.ai/v1/runs/ingest" // Default Lunary endpoint
	}

	p.apiKey = apiKey
	p.endpoint = endpoint

	return nil
}

// LunaryRun represents the run format expected by Lunary.
type LunaryRun struct {
	Type      string                 `json:"type"`
	RunID     string                 `json:"runId"`
	Name      string                 `json:"name,omitempty"`
	Status    string                 `json:"status"`
	CreatedAt string                 `json:"createdAt"`
	EndedAt   string                 `json:"endedAt,omitempty"`
	Runtime   int64                  `json:"runtime,omitempty"`
	Input     interface{}            `json:"input,omitempty"`
	Output    interface{}            `json:"output,omitempty"`
	Tokens    *TokensUsage           `json:"tokens,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	UserProps map[string]interface{} `json:"userProps,omitempty"`
	Extra     map[string]interface{} `json:"extra,omitempty"`
	Tags      []string               `json:"tags,omitempty"`
}

// TokensUsage represents token usage information.
type TokensUsage struct {
	Completion int `json:"completion"`
	Prompt     int `json:"prompt"`
}

// Send converts the event to Lunary format and sends it.
func (p *LunaryPlugin) Send(ctx context.Context, event eventbus.Event) error {
	now := time.Now()
	createdAt := now.Add(-event.Duration).Format(time.RFC3339)
	endedAt := now.Format(time.RFC3339)

	// Determine status based on HTTP status code
	status := "success"
	if event.Status >= 400 {
		status = "error"
	}

	// Try to parse response body as JSON for structured output
	var output interface{}
	if len(event.ResponseBody) > 0 {
		var jsonOutput map[string]interface{}
		if json.Unmarshal(event.ResponseBody, &jsonOutput) == nil {
			output = jsonOutput
		} else {
			output = string(event.ResponseBody)
		}
	}

	// Convert event to Lunary format
	lunaryRun := LunaryRun{
		Type:      "llm",
		RunID:     event.RequestID,
		Name:      fmt.Sprintf("%s %s", event.Method, event.Path),
		Status:    status,
		CreatedAt: createdAt,
		EndedAt:   endedAt,
		Runtime:   event.Duration.Milliseconds(),
		Output:    output,
		Metadata: map[string]interface{}{
			"path":          event.Path,
			"method":        event.Method,
			"status_code":   event.Status,
			"response_time": event.Duration.Milliseconds(),
		},
		Extra: map[string]interface{}{
			"http_status": event.Status,
			"headers":     p.convertHeaders(event.ResponseHeaders),
		},
	}

	// Wrap in array as Lunary expects an array of runs
	payload := []LunaryRun{lunaryRun}

	// Marshal to JSON
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal Lunary request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", p.endpoint, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	// Send request
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to Lunary: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Lunary returned status %d", resp.StatusCode)
	}

	return nil
}

// convertHeaders converts http.Header to map[string]interface{} for JSON serialization.
func (p *LunaryPlugin) convertHeaders(headers http.Header) map[string]interface{} {
	result := make(map[string]interface{})
	for key, values := range headers {
		if len(values) == 1 {
			result[key] = values[0]
		} else {
			result[key] = values
		}
	}
	return result
}

// Name returns the plugin name.
func (p *LunaryPlugin) Name() string {
	return "lunary"
}
