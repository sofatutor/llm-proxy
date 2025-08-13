package middleware

import (
	"bufio"
	"errors"
	"io"
	"net"
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

	// Subscribe before invoking handler to avoid racing with async publish
	ch := bus.Subscribe()
	wrapped.ServeHTTP(rr, req)

	select {
	case evt := <-ch:
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

func TestObservabilityMiddleware_Disabled(t *testing.T) {
	bus := eventbus.NewInMemoryEventBus(10)
	mw := NewObservabilityMiddleware(ObservabilityConfig{Enabled: false, EventBus: bus}, nil)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	wrapped := mw.Middleware()(handler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	wrapped.ServeHTTP(rr, req)

	select {
	case <-bus.Subscribe():
		t.Fatal("event should not be emitted when disabled")
	case <-time.After(100 * time.Millisecond):
		// pass
	}
}

func TestObservabilityMiddleware_NilEventBus(t *testing.T) {
	mw := NewObservabilityMiddleware(ObservabilityConfig{Enabled: true, EventBus: nil}, nil)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	wrapped := mw.Middleware()(handler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	wrapped.ServeHTTP(rr, req)
	// Should not panic or emit events
}

// mockResponseWriter for error simulation

type mockResponseWriter struct {
	http.ResponseWriter
	writeErr error
	wrote    []byte
}

func (m *mockResponseWriter) Write(b []byte) (int, error) {
	m.wrote = append(m.wrote, b...)
	if m.writeErr != nil {
		return 0, m.writeErr
	}
	return len(b), nil
}

func (m *mockResponseWriter) Header() http.Header {
	return http.Header{}
}

func (m *mockResponseWriter) WriteHeader(statusCode int) {}

func TestCaptureResponseWriter_Write_Error(t *testing.T) {
	mrw := &mockResponseWriter{writeErr: io.ErrClosedPipe}
	crw := &captureResponseWriter{ResponseWriter: mrw}
	n, err := crw.Write([]byte("fail"))
	if err != io.ErrClosedPipe {
		t.Errorf("expected error from underlying Write, got %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 bytes written, got %d", n)
	}
	if crw.body.String() != "fail" {
		t.Errorf("expected body to be captured even on error")
	}
}

// --- Helpers for interface delegation tests ---
var flushedFlag *bool

type testFlusher struct{ http.ResponseWriter }

func (f *testFlusher) Flush() {
	if flushedFlag != nil {
		*flushedFlag = true
	}
}

type hijacker struct {
	http.ResponseWriter
	called *bool
}

func (h *hijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h.called != nil {
		*h.called = true
	}
	return nil, nil, nil
}

type pusher struct {
	http.ResponseWriter
	called *bool
}

func (p *pusher) Push(target string, opts *http.PushOptions) error {
	if p.called != nil {
		*p.called = true
	}
	return nil
}

func TestCaptureResponseWriter_Flush_Delegation(t *testing.T) {
	flushed := false
	flushedFlag = &flushed
	crw := &captureResponseWriter{ResponseWriter: &testFlusher{}}
	crw.Flush()
	if !flushed {
		t.Error("Flush was not delegated")
	}
	flushedFlag = nil
}

func TestCaptureResponseWriter_Hijack_Supported(t *testing.T) {
	called := false
	crw := &captureResponseWriter{ResponseWriter: &hijacker{called: &called}}
	_, _, err := crw.Hijack()
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if !called {
		t.Error("Hijack was not delegated")
	}
}

func TestCaptureResponseWriter_Push_Supported(t *testing.T) {
	called := false
	crw := &captureResponseWriter{ResponseWriter: &pusher{called: &called}}
	err := crw.Push("/foo", nil)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if !called {
		t.Error("Push was not delegated")
	}
}

func TestCaptureResponseWriter_Hijack_Unsupported(t *testing.T) {
	rw := httptest.NewRecorder()
	crw := &captureResponseWriter{ResponseWriter: rw}
	conn, buf, err := crw.Hijack()
	if err == nil || conn != nil || buf != nil {
		t.Errorf("expected error and nils, got: %v, %v, %v", conn, buf, err)
	}
}

func TestCaptureResponseWriter_Push_Unsupported(t *testing.T) {
	rw := httptest.NewRecorder()
	crw := &captureResponseWriter{ResponseWriter: rw}
	err := crw.Push("/foo", nil)
	if !errors.Is(err, http.ErrNotSupported) {
		t.Errorf("expected http.ErrNotSupported, got: %v", err)
	}
}

func TestCloneHeader(t *testing.T) {
	h := http.Header{"Foo": {"bar", "baz"}}
	cloned := cloneHeader(h)
	if len(cloned) != 1 || len(cloned["Foo"]) != 2 || cloned["Foo"][0] != "bar" || cloned["Foo"][1] != "baz" {
		t.Errorf("cloneHeader did not clone correctly: %v", cloned)
	}
	cloned["Foo"][0] = "changed"
	if h["Foo"][0] == "changed" {
		t.Error("cloneHeader did not deep copy slice")
	}
}

func TestCaptureResponseWriter_Flush(t *testing.T) {
	rw := httptest.NewRecorder()
	crw := &captureResponseWriter{ResponseWriter: rw}
	// Should not panic
	crw.Flush()
}
