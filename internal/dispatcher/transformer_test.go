package dispatcher

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/sofatutor/llm-proxy/internal/eventbus"
)

func TestDefaultEventTransformer_Transform(t *testing.T) {
	transformer := &DefaultEventTransformer{}

	tests := []struct {
		name     string
		event    eventbus.Event
		wantNil  bool
		wantType string
	}{
		{
			name: "POST request should be transformed",
			event: eventbus.Event{
				RequestID:    "test-123",
				Method:       "POST",
				Path:         "/v1/chat/completions",
				Status:       200,
				Duration:     100 * time.Millisecond,
				RequestBody:  []byte(`{"model":"gpt-4","messages":[]}`),
				ResponseBody: []byte(`{"choices":[]}`),
				ResponseHeaders: http.Header{
					"Content-Type": []string{"application/json"},
				},
			},
			wantNil:  false,
			wantType: "llm",
		},
		{
			name: "GET request should be filtered out",
			event: eventbus.Event{
				RequestID: "test-456",
				Method:    "GET",
				Path:      "/health",
				Status:    200,
			},
			wantNil: true,
		},
		{
			name: "OPTIONS request should be filtered out",
			event: eventbus.Event{
				RequestID: "test-789",
				Method:    "OPTIONS",
				Path:      "/v1/chat/completions",
				Status:    204,
			},
			wantNil: true,
		},
		{
			name: "binary body should use base64",
			event: eventbus.Event{
				RequestID:    "test-bin",
				Method:       "POST",
				Path:         "/v1/chat/completions",
				Status:       200,
				Duration:     100 * time.Millisecond,
				RequestBody:  []byte{0x1f, 0x8b, 0x08, 0x00}, // not valid JSON
				ResponseBody: []byte{0x1f, 0x8b, 0x08, 0x00},
			},
			wantNil:  false,
			wantType: "llm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := transformer.Transform(tt.event)
			if err != nil {
				t.Fatalf("Transform failed: %v", err)
			}

			if tt.wantNil {
				if result != nil {
					t.Errorf("Expected nil result for %s request", tt.event.Method)
				}
				return
			}

			if result == nil {
				t.Fatal("Expected non-nil result")
			}

			if result.Type != tt.wantType {
				t.Errorf("Expected Type %s, got %s", tt.wantType, result.Type)
			}

			if result.RunID == "" {
				t.Error("Expected non-empty RunID")
			}

			if result.Event != "start" {
				t.Errorf("Expected Event 'start', got %s", result.Event)
			}

			// Check metadata
			if result.Metadata == nil {
				t.Fatal("Expected non-nil Metadata")
			}

			if result.Metadata["method"] != tt.event.Method {
				t.Errorf("Expected method %s in metadata, got %s",
					tt.event.Method, result.Metadata["method"])
			}

			if result.Metadata["request_id"] != tt.event.RequestID {
				t.Errorf("Expected request_id %s in metadata, got %s",
					tt.event.RequestID, result.Metadata["request_id"])
			}

			// Check input/output
			if len(tt.event.RequestBody) > 0 {
				if json.Valid(tt.event.RequestBody) {
					if result.Input == nil {
						t.Error("Expected Input to be set for valid JSON RequestBody")
					}
					if result.InputBase64 != "" {
						t.Error("Expected InputBase64 to be empty for valid JSON RequestBody")
					}
				} else {
					if result.Input == nil {
						t.Error("Expected Input to be set for non-JSON (binary) RequestBody: should be a placeholder string")
					}
					if result.InputBase64 != "" {
						t.Error("Expected InputBase64 to be empty for non-JSON (binary) RequestBody: now always a placeholder string")
					}
				}
			}

			if len(tt.event.ResponseBody) > 0 {
				if json.Valid(tt.event.ResponseBody) {
					if result.Output == nil {
						t.Error("Expected Output to be set for valid JSON ResponseBody")
					}
					if result.OutputBase64 != "" {
						t.Error("Expected OutputBase64 to be empty for valid JSON ResponseBody")
					}
				} else {
					if result.Output == nil {
						t.Error("Expected Output to be set for non-JSON (binary) ResponseBody: should be a placeholder string")
					}
					if result.OutputBase64 != "" {
						t.Error("Expected OutputBase64 to be empty for non-JSON (binary) ResponseBody: now always a placeholder string")
					}
				}
			}
		})
	}
}
