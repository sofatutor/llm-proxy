package dispatcher

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/sofatutor/llm-proxy/internal/eventbus"
)

// DefaultEventTransformer provides a basic transformation from eventbus.Event to EventPayload
type DefaultEventTransformer struct{}

// Transform converts an eventbus.Event to an EventPayload
func (t *DefaultEventTransformer) Transform(evt eventbus.Event) (*EventPayload, error) {
	// Skip non-POST requests (like OPTIONS, GET)
	if evt.Method != "POST" {
		return nil, nil
	}

	// Generate a unique run ID for this event
	runID := uuid.New().String()

	// Basic transformation
	payload := &EventPayload{
		Type:      "llm",
		Event:     "start", // For now, all events are considered "start" events
		RunID:     runID,
		Timestamp: time.Now(),
		Metadata: map[string]any{
			"method":      evt.Method,
			"path":        evt.Path,
			"status":      evt.Status,
			"duration_ms": evt.Duration.Milliseconds(),
			"request_id":  evt.RequestID,
		},
	}

	// Add request body as input if available
	if len(evt.RequestBody) > 0 {
		payload.Input = json.RawMessage(evt.RequestBody)
	}

	// Add response body as output if available
	if len(evt.ResponseBody) > 0 {
		payload.Output = json.RawMessage(evt.ResponseBody)
	}

	// Add response headers to metadata
	if evt.ResponseHeaders != nil {
		headers := make(map[string]any)
		for k, v := range evt.ResponseHeaders {
			if len(v) == 1 {
				headers[k] = v[0]
			} else {
				headers[k] = v
			}
		}
		payload.Metadata["response_headers"] = headers
	}

	return payload, nil
}
