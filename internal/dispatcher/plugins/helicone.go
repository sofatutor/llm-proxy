package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/sofatutor/llm-proxy/internal/dispatcher"
)

// HeliconePlugin implements Helicone backend integration
type HeliconePlugin struct {
	apiKey   string
	endpoint string
	client   *http.Client
}

// NewHeliconePlugin creates a new Helicone plugin
func NewHeliconePlugin() *HeliconePlugin {
	return &HeliconePlugin{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Init initializes the Helicone plugin with configuration
func (p *HeliconePlugin) Init(cfg map[string]string) error {
	apiKey, ok := cfg["api-key"]
	if !ok || apiKey == "" {
		return fmt.Errorf("helicone plugin requires 'api-key' configuration")
	}

	endpoint, ok := cfg["endpoint"]
	if !ok || endpoint == "" {
		endpoint = "https://api.hconeai.com/v1/request"
	}

	p.apiKey = apiKey
	p.endpoint = endpoint

	return nil
}

// SendEvents sends events to Helicone
func (p *HeliconePlugin) SendEvents(ctx context.Context, events []dispatcher.EventPayload) error {
	if len(events) == 0 {
		return nil
	}

	// Helicone expects individual requests, so we send them one by one
	for _, event := range events {
		if err := p.sendSingleEvent(ctx, event); err != nil {
			return fmt.Errorf("failed to send event %s: %w", event.RunID, err)
		}
	}

	return nil
}

// sendSingleEvent sends a single event to Helicone
func (p *HeliconePlugin) sendSingleEvent(ctx context.Context, event dispatcher.EventPayload) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("helicone API returned status %d", resp.StatusCode)
	}

	return nil
}

// Close cleans up the plugin resources
func (p *HeliconePlugin) Close() error {
	// Nothing to clean up for HTTP client
	return nil
}
