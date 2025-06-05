package dispatcher

import (
	"context"
	"encoding/json"
	"time"

	"github.com/sofatutor/llm-proxy/internal/eventbus"
)

// EventPayload represents the extended event format for external services
type EventPayload struct {
	Type        string          `json:"type"`
	Event       string          `json:"event"`
	RunID       string          `json:"runId"`
	ParentRunID *string         `json:"parentRunId,omitempty"`
	Name        *string         `json:"name,omitempty"`
	Timestamp   time.Time       `json:"timestamp"`
	Input       json.RawMessage `json:"input,omitempty"`
	Output      json.RawMessage `json:"output,omitempty"`

	// Additional fields for enhanced observability
	UserID      *string            `json:"userId,omitempty"`
	TokensUsage *TokensUsage       `json:"tokensUsage,omitempty"`
	UserProps   map[string]any     `json:"userProps,omitempty"`
	Extra       map[string]any     `json:"extra,omitempty"`
	Metadata    map[string]any     `json:"metadata,omitempty"`
	Tags        []string           `json:"tags,omitempty"`
}

// TokensUsage represents token usage statistics
type TokensUsage struct {
	Completion int `json:"completion"`
	Prompt     int `json:"prompt"`
}

// BackendPlugin defines the interface for dispatcher backends
type BackendPlugin interface {
	// Init initializes the plugin with configuration
	Init(cfg map[string]string) error
	
	// SendEvents sends a batch of events to the backend
	SendEvents(ctx context.Context, events []EventPayload) error
	
	// Close cleans up any resources used by the plugin
	Close() error
}

// EventTransformer transforms eventbus.Event to EventPayload
type EventTransformer interface {
	Transform(evt eventbus.Event) (*EventPayload, error)
}