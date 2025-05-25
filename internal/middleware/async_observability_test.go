package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sofatutor/llm-proxy/internal/eventbus"
	"github.com/stretchr/testify/require"
)

func TestObservabilityMiddleware_NonStreaming(t *testing.T) {
	bus := eventbus.NewInMemoryEventBus(10)
	mw := NewObservabilityMiddleware(ObservabilityConfig{Enabled: true, EventBus: bus}, nil)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	wrapped := mw.Middleware()(handler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", "req1")
	rr := httptest.NewRecorder()

	wrapped.ServeHTTP(rr, req)

	select {
	case evt := <-bus.Subscribe():
		require.Equal(t, "req1", evt.RequestID)
		require.Equal(t, http.StatusOK, evt.Status)
		require.Equal(t, "ok", string(evt.ResponseBody))
	case <-time.After(time.Second):
		t.Fatal("event not received")
	}
}

func TestObservabilityMiddleware_Streaming(t *testing.T) {
	bus := eventbus.NewInMemoryEventBus(10)
	mw := NewObservabilityMiddleware(ObservabilityConfig{Enabled: true, EventBus: bus}, nil)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		f, ok := w.(http.Flusher)
		require.True(t, ok)
		for i := 0; i < 3; i++ {
			_, _ = io.WriteString(w, "data: foo\n\n")
			f.Flush()
		}
	})

	wrapped := mw.Middleware()(handler)

	req := httptest.NewRequest(http.MethodGet, "/stream", nil)
	req.Header.Set("X-Request-ID", "req2")
	rr := httptest.NewRecorder()

	wrapped.ServeHTTP(rr, req)

	select {
	case evt := <-bus.Subscribe():
		require.Equal(t, 3*len("data: foo\n\n"), len(evt.ResponseBody))
		require.Equal(t, "req2", evt.RequestID)
	case <-time.After(time.Second):
		t.Fatal("event not received")
	}
}
