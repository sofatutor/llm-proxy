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
		endpoint = "https://api.worker.helicone.ai/custom/v1/log"
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

	for _, event := range events {
		// Skip events with empty output
		if len(event.Output) == 0 && event.OutputBase64 == "" {
			log.Printf("[helicone] Skipping event with empty output: RunID=%s, Path=%v", event.RunID, event.Metadata["path"])
			continue
		}
		payload, err := heliconePayloadFromEvent(event)
		if err != nil {
			return err
		}
		if err := p.sendHeliconeEvent(ctx, payload); err != nil {
			// Print payload for debugging on error
			log.Printf("[helicone] Error sending event. Payload: %s", mustMarshalJSON(payload))
			return err
		}
	}
	return nil
}

// heliconePayloadFromEvent maps EventPayload to Helicone manual logger format
func heliconePayloadFromEvent(event dispatcher.EventPayload) (map[string]interface{}, error) {
	// Extract request and response bodies
	var reqBody map[string]interface{}
	var respBody map[string]interface{}
	isJSON := false
	if len(event.Input) > 0 {
		_ = json.Unmarshal(event.Input, &reqBody)
	}
	if len(event.Output) > 0 {
		if err := json.Unmarshal(event.Output, &respBody); err == nil {
			isJSON = true
		}
	}

	// Timing (use event.Timestamp for both if no better info)
	timestamp := event.Timestamp
	sec := timestamp.Unix()
	ms := timestamp.Nanosecond() / 1e6
	timing := map[string]interface{}{
		"startTime": map[string]int64{"seconds": sec, "milliseconds": int64(ms)},
		"endTime":   map[string]int64{"seconds": sec, "milliseconds": int64(ms)},
	}

	// Meta
	meta := map[string]string{}
	if event.UserID != nil {
		meta["Helicone-User-Id"] = *event.UserID
	}
	if event.Extra != nil {
		for k, v := range event.Extra {
			if s, ok := v.(string); ok {
				meta[k] = s
			}
		}
	}
	if event.Metadata != nil {
		for k, v := range event.Metadata {
			if s, ok := v.(string); ok {
				meta[k] = s
			}
		}
	}

	// ProviderRequest
	providerRequest := map[string]interface{}{
		"url":  "custom-model-nopath",
		"json": reqBody,
		"meta": meta,
	}

	// ProviderResponse
	status := 200
	if s, ok := event.Metadata["status"].(int); ok {
		status = s
	}
	providerResponse := map[string]interface{}{
		"status":  status,
		"headers": map[string]string{}, // Optionally fill from event.Metadata
	}
	if isJSON {
		providerResponse["json"] = respBody
	} else {
		providerResponse["note"] = "response was not JSON; omitted"
	}
	// If OutputBase64 is set, include as base64
	if event.OutputBase64 != "" {
		providerResponse["base64"] = event.OutputBase64
	}

	return map[string]interface{}{
		"providerRequest":  providerRequest,
		"providerResponse": providerResponse,
		"timing":           timing,
	}, nil
}

// sendHeliconeEvent sends a single event to Helicone manual logger endpoint
func (p *HeliconePlugin) sendHeliconeEvent(ctx context.Context, payload map[string]interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", p.endpoint, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == 500 {
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(resp.Body)
		log.Printf("Helicone API error %d: %s", resp.StatusCode, buf.String())
		return &dispatcher.PermanentBackendError{Msg: fmt.Sprintf("helicone API returned status 500: %s", buf.String())}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(resp.Body)
		log.Printf("Helicone API error %d: %s", resp.StatusCode, buf.String())
		return fmt.Errorf("helicone API returned status %d", resp.StatusCode)
	}
	return nil
}

// Close cleans up the plugin resources
func (p *HeliconePlugin) Close() error {
	// Nothing to clean up for HTTP client
	return nil
}

// mustMarshalJSON marshals v to JSON or returns an error string
func mustMarshalJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "<marshal error>"
	}
	return string(b)
}
