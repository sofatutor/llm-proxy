package proxy

import (
	"net/http"
	"time"
)

// RetryMiddleware returns a middleware that retries the handler on transient network errors.
// Retries are only performed for clear, transient network errors (timeouts, connection resets),
// with conservative backoff and a low retry limit. No retries for application-level errors.
func RetryMiddleware(maxRetries int, baseBackoff time.Duration) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for attempt := 0; attempt <= maxRetries; attempt++ {
				rec := newBufferedResponseRecorder()
				next.ServeHTTP(rec, r)

				if !isTransientStatusCode(rec.statusCode) {
					rec.CopyTo(w)
					return
				}
				if attempt < maxRetries {
					time.Sleep(baseBackoff * (1 << attempt))
				}
			}
			// If we exhausted retries, write the last error
			w.WriteHeader(http.StatusBadGateway)
			w.Write([]byte("{\"error\":\"Upstream unavailable after retries\"}"))
		})
	}
}

// errorCapturingWriter wraps a ResponseWriter and captures the last error from Write.
type errorCapturingWriter struct {
	http.ResponseWriter
	errPtr *error
}

func (w *errorCapturingWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	if w.errPtr != nil {
		*w.errPtr = err
	}
	return n, err
}

// isTransientStatusCode returns true if the status code is a transient network error (502, 503, 504).
func isTransientStatusCode(statusCode int) bool {
	return statusCode == http.StatusBadGateway || statusCode == http.StatusServiceUnavailable || statusCode == http.StatusGatewayTimeout
}

// bufferedResponseRecorder buffers status and body in memory for retry logic
// It does not write to the real ResponseWriter until explicitly copied.
type bufferedResponseRecorder struct {
	statusCode int
	headers    http.Header
	body       []byte
}

func newBufferedResponseRecorder() *bufferedResponseRecorder {
	return &bufferedResponseRecorder{
		statusCode: http.StatusOK,
		headers:    make(http.Header),
	}
}

func (b *bufferedResponseRecorder) Header() http.Header {
	return b.headers
}

func (b *bufferedResponseRecorder) WriteHeader(statusCode int) {
	b.statusCode = statusCode
}

func (b *bufferedResponseRecorder) Write(data []byte) (int, error) {
	b.body = append(b.body, data...)
	return len(data), nil
}

func (b *bufferedResponseRecorder) CopyTo(w http.ResponseWriter) {
	for k, vv := range b.headers {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(b.statusCode)
	w.Write(b.body)
}
