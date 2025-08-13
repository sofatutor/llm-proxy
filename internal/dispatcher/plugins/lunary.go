package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/sofatutor/llm-proxy/internal/dispatcher"
)

// LunaryPlugin implements Lunary.ai backend integration
type LunaryPlugin struct {
	apiKey   string
	endpoint string
	client   *http.Client
}

// NewLunaryPlugin creates a new Lunary plugin
func NewLunaryPlugin() *LunaryPlugin {
	return &LunaryPlugin{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Init initializes the Lunary plugin with configuration
func (p *LunaryPlugin) Init(cfg map[string]string) error {
	apiKey, ok := cfg["api-key"]
	if !ok || apiKey == "" {
		return fmt.Errorf("lunary plugin requires 'api-key' configuration")
	}

	endpoint, ok := cfg["endpoint"]
	if !ok || endpoint == "" {
		endpoint = "https://api.lunary.ai/v1/runs/ingest"
	}

	p.apiKey = apiKey
	p.endpoint = endpoint

	return nil
}

// SendEvents sends events to Lunary.ai
func (p *LunaryPlugin) SendEvents(ctx context.Context, events []dispatcher.EventPayload) error {
	if len(events) == 0 {
		return nil
	}

	// Lunary expects an array of events
	data, err := json.Marshal(events)
	if err != nil {
		return fmt.Errorf("failed to marshal events: %w", err)
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
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("[lunary] failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("lunary API returned status %d", resp.StatusCode)
	}

	return nil
}

// Close cleans up the plugin resources
func (p *LunaryPlugin) Close() error {
	// Nothing to clean up for HTTP client
	return nil
}
