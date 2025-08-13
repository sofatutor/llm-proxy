package plugins

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sofatutor/llm-proxy/internal/dispatcher"
)

func TestLunary_SendEvents_StatusHandling(t *testing.T) {
	plugin := NewLunaryPlugin()

	// Success 200
	tsOK := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer test-key" && r.Header.Get("Content-Type") == "application/json" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer tsOK.Close()

	cfg := map[string]string{"api-key": "test-key", "endpoint": tsOK.URL}
	if err := plugin.Init(cfg); err != nil {
		t.Fatalf("init err: %v", err)
	}
	defer func() { _ = plugin.Close() }()

	events := []dispatcher.EventPayload{{RunID: "r1", Metadata: map[string]any{"status": 200}}}
	if err := plugin.SendEvents(context.Background(), events); err != nil {
		t.Fatalf("send ok err: %v", err)
	}

	// Non-2xx
	tsBad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad"))
	}))
	defer tsBad.Close()
	_ = plugin.Init(map[string]string{"api-key": "test-key", "endpoint": tsBad.URL})
	if err := plugin.SendEvents(context.Background(), events); err == nil {
		t.Fatalf("expected error for non-2xx status")
	}
}

func TestLunary_Init_MissingAPIKey(t *testing.T) {
	plugin := NewLunaryPlugin()
	if err := plugin.Init(map[string]string{}); err == nil {
		t.Fatalf("expected error for missing api-key")
	}
}

func TestMustMarshalJSON_ReturnsString(t *testing.T) {
	obj := map[string]string{"a": "b"}
	got := mustMarshalJSON(obj)
	var m map[string]string
	if err := json.Unmarshal([]byte(got), &m); err != nil || m["a"] != "b" {
		t.Fatalf("mustMarshalJSON roundtrip failed: %v, %v", err, m)
	}
}
