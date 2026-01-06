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
	// MaxRequestBodyBytes limits request body capture for observability events. 0 means "use default".
	MaxRequestBodyBytes int64
	// MaxResponseBodyBytes limits response body capture for observability events. 0 means "use default".
	MaxResponseBodyBytes int64
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
			maxReq := m.cfg.MaxRequestBodyBytes
			if maxReq <= 0 {
				maxReq = 64 * 1024 // default 64KB
			}
			maxResp := m.cfg.MaxResponseBodyBytes
			if maxResp <= 0 {
				maxResp = 256 * 1024 // default 256KB
			}

			crw := &captureResponseWriter{ResponseWriter: w, statusCode: http.StatusOK, maxBodyBytes: maxResp}

			var reqBody []byte
			if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
				if r.Body != nil {
					// Capture only up to maxReq bytes without consuming the downstream body.
					// We read maxReq+1 bytes so we can detect truncation.
					originalBody := r.Body
					limitedReader := io.LimitReader(originalBody, maxReq+1)
					bodyBytes, err := io.ReadAll(limitedReader)
					if err == nil && len(bodyBytes) > 0 {
						if int64(len(bodyBytes)) > maxReq {
							reqBody = bodyBytes[:maxReq]
						} else {
							reqBody = bodyBytes
						}
						// Restore the body for downstream handlers (including any unread bytes from originalBody).
						r.Body = &readerWithCloser{
							r: io.MultiReader(bytes.NewReader(bodyBytes), originalBody),
							c: originalBody,
						}
					} else {
						// Restore the body even on read errors.
						r.Body = originalBody
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
				RequestID: reqID,
				Method:    r.Method,
				Path:      r.URL.Path,
				Status:    crw.statusCode,
				Duration:  time.Since(start),
				// Avoid cloning here; we clone inside the goroutine before mutating/enriching.
				ResponseHeaders: crw.Header(),
				ResponseBody:    crw.body.Bytes(),
				RequestBody:     reqBody,
			}

			// Off-hot-path enrichment: parse OpenAI response metadata from the already-captured body.
			// This avoids buffering/parsing response bodies in the proxy ModifyResponse path.
			go func(e eventbus.Event) {
				// ResponseHeaders is a map type; make a defensive copy so the async mutation is fully isolated.
				e.ResponseHeaders = cloneHeader(e.ResponseHeaders)
				addOpenAIResponseMetadataHeaders(e.ResponseHeaders, e.ResponseBody)
				m.cfg.EventBus.Publish(context.Background(), e)
			}(evt)
		})
	}
}

// captureResponseWriter wraps http.ResponseWriter to capture status and body while supporting streaming.
type captureResponseWriter struct {
	http.ResponseWriter
	statusCode    int
	body          bytes.Buffer
	maxBodyBytes  int64
	capturedBytes int64
}

func (w *captureResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *captureResponseWriter) Write(b []byte) (int, error) {
	if w.maxBodyBytes <= 0 || w.capturedBytes < w.maxBodyBytes {
		remaining := int64(len(b))
		if w.maxBodyBytes > 0 {
			remaining = w.maxBodyBytes - w.capturedBytes
		}
		if remaining > 0 {
			toWrite := b
			if int64(len(b)) > remaining {
				toWrite = b[:remaining]
			}
			_, _ = w.body.Write(toWrite)
			w.capturedBytes += int64(len(toWrite))
		}
	}
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

type readerWithCloser struct {
	r io.Reader
	c io.Closer
}

func (rc *readerWithCloser) Read(p []byte) (int, error) { return rc.r.Read(p) }
func (rc *readerWithCloser) Close() error               { return rc.c.Close() }
