package plugins

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/sofatutor/llm-proxy/internal/dispatcher"
)

func TestHeliconePayloadFromEvent_JSONAndNonJSON(t *testing.T) {
	now := time.Now()
	// JSON output
	evtJSON := dispatcher.EventPayload{
		RunID:     "run-json",
		Timestamp: now,
		Input:     json.RawMessage(`{"model":"gpt"}`),
		Output:    json.RawMessage(`{"reply":"ok"}`),
		Metadata:  map[string]any{"status": 201, "path": "/v1/chat"},
		Extra:     map[string]any{"team": "a"},
		UserProps: map[string]any{"plan": "pro"},
		Tags:      []string{"t1"},
	}
	payload, err := heliconePayloadFromEvent(evtJSON)
	if err != nil {
		t.Fatalf("heliconePayloadFromEvent error: %v", err)
	}
	pr, ok := payload["providerResponse"].(map[string]interface{})
	if !ok {
		t.Fatalf("providerResponse missing")
	}
	if pr["status"].(int) != 201 {
		t.Fatalf("status mismatch: %v", pr["status"])
	}
	if _, ok := pr["json"]; !ok {
		t.Fatalf("expected json in providerResponse for JSON output")
	}

	// Non-JSON output with base64
	evtBin := dispatcher.EventPayload{
		RunID:        "run-bin",
		Timestamp:    now,
		Input:        json.RawMessage(`{"q":"x"}`),
		Output:       []byte("not json"),
		OutputBase64: "bm90IGpzb24=",
		Metadata:     map[string]any{"status": 200},
	}
	payload2, err := heliconePayloadFromEvent(evtBin)
	if err != nil {
		t.Fatalf("heliconePayloadFromEvent error: %v", err)
	}
	pr2 := payload2["providerResponse"].(map[string]interface{})
	if _, hasJSON := pr2["json"]; hasJSON {
		t.Fatalf("did not expect json for non-JSON output")
	}
	if pr2["base64"].(string) != "bm90IGpzb24=" {
		t.Fatalf("expected base64 field present")
	}
}

func TestMustMarshalJSON_Error(t *testing.T) {
	// Channels cannot be marshaled by encoding/json → force error
	ch := make(chan int)
	s := mustMarshalJSON(ch)
	if s != "<marshal error>" {
		t.Fatalf("expected '<marshal error>', got %q", s)
	}
}
