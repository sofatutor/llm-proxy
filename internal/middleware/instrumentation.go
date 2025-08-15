package middleware

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/sofatutor/llm-proxy/internal/eventbus"
	"github.com/sofatutor/llm-proxy/internal/logging"
	"go.uber.org/zap"
)

// Middleware defines a function that wraps an http.Handler.
type Middleware func(http.Handler) http.Handler

// ObservabilityConfig controls the behavior of the observability middleware.
type ObservabilityConfig struct {
	Enabled  bool
	EventBus eventbus.EventBus
}

// ObservabilityMiddleware captures request/response data and forwards it to an event bus.
type ObservabilityMiddleware struct {
	cfg    ObservabilityConfig
	logger *zap.Logger
}

// NewObservabilityMiddleware creates a new ObservabilityMiddleware instance.
func NewObservabilityMiddleware(cfg ObservabilityConfig, logger *zap.Logger) *ObservabilityMiddleware {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &ObservabilityMiddleware{cfg: cfg, logger: logger}
}

// Middleware returns the http middleware function.
func (m *ObservabilityMiddleware) Middleware() Middleware {
	if !m.cfg.Enabled || m.cfg.EventBus == nil {
		return func(next http.Handler) http.Handler { return next }
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			crw := &captureResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			var reqBody []byte
			if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
				if r.Body != nil {
					// Read and buffer the request body
					bodyBytes, err := io.ReadAll(r.Body)
					if err == nil {
						reqBody = bodyBytes
						// Restore the body for downstream handlers
						r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
					}
				}
			}

			next.ServeHTTP(crw, r)

			// Resolve request ID from header, then context, then response headers
			reqID := r.Header.Get("X-Request-ID")
			if reqID == "" {
				if v, ok := logging.GetRequestID(r.Context()); ok {
					reqID = v
				}
			}
			if reqID == "" {
				reqID = crw.Header().Get("X-Request-ID")
			}

			// Skip publishing cache hits (do not incur provider cost)
			if v := strings.ToLower(crw.Header().Get("X-PROXY-CACHE")); v == "hit" || v == "conditional-hit" {
				return
			}

			evt := eventbus.Event{
				RequestID:       reqID,
				Method:          r.Method,
				Path:            r.URL.Path,
				Status:          crw.statusCode,
				Duration:        time.Since(start),
				ResponseHeaders: cloneHeader(crw.Header()),
				ResponseBody:    crw.body.Bytes(),
				RequestBody:     reqBody,
			}

			go m.cfg.EventBus.Publish(context.Background(), evt)
		})
	}
}

// captureResponseWriter wraps http.ResponseWriter to capture status and body while supporting streaming.
type captureResponseWriter struct {
	http.ResponseWriter
	statusCode int
	body       bytes.Buffer
}

func (w *captureResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *captureResponseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *captureResponseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *captureResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := w.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, fmt.Errorf("hijack not supported")
}

func (w *captureResponseWriter) Push(target string, opts *http.PushOptions) error {
	if p, ok := w.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return http.ErrNotSupported
}

func cloneHeader(h http.Header) http.Header {
	cloned := make(http.Header, len(h))
	for k, v := range h {
		vv := make([]string, len(v))
		copy(vv, v)
		cloned[k] = vv
	}
	return cloned
}
