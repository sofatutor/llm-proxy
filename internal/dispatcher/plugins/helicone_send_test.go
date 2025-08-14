package plugins

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sofatutor/llm-proxy/internal/dispatcher"
)

func TestHelicone_sendHeliconeEvent_StatusHandling(t *testing.T) {
	t.Parallel()

	// Success 200
	t.Run("status 200 ok", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") == "Bearer test-key" && r.Header.Get("Content-Type") == "application/json" {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`ok`))
				return
			}
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer srv.Close()

		p := NewHeliconePlugin()
		if err := p.Init(map[string]string{"api-key": "test-key", "endpoint": srv.URL}); err != nil {
			t.Fatalf("init: %v", err)
		}
		// Use server transport to avoid proxies
		p.client = srv.Client()

		err := p.sendHeliconeEvent(context.Background(), map[string]any{"a": 1})
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
	})

	// 500 -> PermanentBackendError
	t.Run("status 500 permanent error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`boom`))
		}))
		defer srv.Close()

		p := NewHeliconePlugin()
		if err := p.Init(map[string]string{"api-key": "test-key", "endpoint": srv.URL}); err != nil {
			t.Fatalf("init: %v", err)
		}
		p.client = srv.Client()

		err := p.sendHeliconeEvent(context.Background(), map[string]any{"a": 1})
		if err == nil {
			t.Fatalf("expected error")
		}
		if _, ok := err.(*dispatcher.PermanentBackendError); !ok {
			t.Fatalf("expected PermanentBackendError, got %T: %v", err, err)
		}
	})

	// Non-2xx (not 500) -> generic error
	t.Run("status 418 generic error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTeapot)
			_, _ = w.Write([]byte(`try again later`))
		}))
		defer srv.Close()

		p := NewHeliconePlugin()
		if err := p.Init(map[string]string{"api-key": "test-key", "endpoint": srv.URL}); err != nil {
			t.Fatalf("init: %v", err)
		}
		p.client = srv.Client()

		err := p.sendHeliconeEvent(context.Background(), map[string]any{"a": 1})
		if err == nil {
			t.Fatalf("expected generic error for non-2xx status")
		}
		if _, ok := err.(*dispatcher.PermanentBackendError); ok {
			t.Fatalf("did not expect PermanentBackendError for non-500: %v", err)
		}
	})

	// 400 -> PermanentBackendError with body logged
	t.Run("status 400 bad request permanent", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"bad payload"}`))
		}))
		defer srv.Close()

		p := NewHeliconePlugin()
		if err := p.Init(map[string]string{"api-key": "test-key", "endpoint": srv.URL}); err != nil {
			t.Fatalf("init: %v", err)
		}
		p.client = srv.Client()

		err := p.sendHeliconeEvent(context.Background(), map[string]any{"a": 1})
		if err == nil {
			t.Fatalf("expected PermanentBackendError for 400")
		}
		if _, ok := err.(*dispatcher.PermanentBackendError); !ok {
			t.Fatalf("expected PermanentBackendError, got %T: %v", err, err)
		}
	})
}

func TestHelicone_SendEvents_EdgeCases(t *testing.T) {
	t.Parallel()

	// Test empty events slice
	t.Run("empty events", func(t *testing.T) {
		p := NewHeliconePlugin()
		if err := p.Init(map[string]string{"api-key": "test-key", "endpoint": "http://test.example"}); err != nil {
			t.Fatalf("init: %v", err)
		}

		err := p.SendEvents(context.Background(), []dispatcher.EventPayload{})
		if err != nil {
			t.Fatalf("SendEvents with empty slice should not error, got %v", err)
		}
	})

	// Test events with empty output
	t.Run("events with empty output", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatalf("Should not receive any requests for events with empty output")
		}))
		defer srv.Close()

		p := NewHeliconePlugin()
		if err := p.Init(map[string]string{"api-key": "test-key", "endpoint": srv.URL}); err != nil {
			t.Fatalf("init: %v", err)
		}
		p.client = srv.Client()

		// Event with empty output and empty OutputBase64
		events := []dispatcher.EventPayload{
			{
				RunID:        "test-run-1",
				Output:       []byte{},
				OutputBase64: "",
				Metadata:     map[string]any{"path": "/v1/chat/completions"},
			},
		}

		err := p.SendEvents(context.Background(), events)
		if err != nil {
			t.Fatalf("SendEvents should succeed even when skipping events, got %v", err)
		}
	})

	// Test mixed events (some with output, some without)
	t.Run("mixed events", func(t *testing.T) {
		requestCount := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount++
			if r.Header.Get("Authorization") == "Bearer test-key" {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`ok`))
				return
			}
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer srv.Close()

		p := NewHeliconePlugin()
		if err := p.Init(map[string]string{"api-key": "test-key", "endpoint": srv.URL}); err != nil {
			t.Fatalf("init: %v", err)
		}
		p.client = srv.Client()

		events := []dispatcher.EventPayload{
			{
				RunID:        "test-run-1",
				Output:       []byte{}, // Empty output - should be skipped
				OutputBase64: "",
				Metadata:     map[string]any{"path": "/v1/chat/completions"},
			},
			{
				RunID:        "test-run-2",
				Output:       []byte(`{"choices":[{"message":{"content":"Hello"}}]}`),
				OutputBase64: "",
				Metadata:     map[string]any{"path": "/v1/chat/completions"},
			},
		}

		err := p.SendEvents(context.Background(), events)
		if err != nil {
			t.Fatalf("SendEvents should succeed, got %v", err)
		}

		// Should only receive 1 request (for the event with output)
		if requestCount != 1 {
			t.Fatalf("Expected 1 request, got %d", requestCount)
		}
	})

	// Test error in payload creation
	t.Run("payload creation error", func(t *testing.T) {
		p := NewHeliconePlugin()
		if err := p.Init(map[string]string{"api-key": "test-key", "endpoint": "http://test.example"}); err != nil {
			t.Fatalf("init: %v", err)
		}

		// Create an event that will cause payload creation to fail
		events := []dispatcher.EventPayload{
			{
				RunID:        "test-run-1",
				Output:       []byte(`invalid json`),
				OutputBase64: "",
				Metadata:     map[string]any{"path": "/v1/chat/completions"},
			},
		}

		err := p.SendEvents(context.Background(), events)
		if err == nil {
			t.Fatal("Expected error from payload creation")
		}
	})
}
