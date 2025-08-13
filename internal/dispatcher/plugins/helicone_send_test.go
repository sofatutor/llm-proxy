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
