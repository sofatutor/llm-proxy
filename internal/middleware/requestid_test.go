package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sofatutor/llm-proxy/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestIDMiddleware(t *testing.T) {
	tests := []struct {
		name                  string
		existingRequestID     string // X-Request-ID header in request
		existingCorrelationID string // X-Correlation-ID header in request
		expectRequestID       bool
		expectCorrelationID   bool
		expectResponseHeaders bool
	}{
		{
			name:                  "no existing headers - generates new IDs",
			existingRequestID:     "",
			existingCorrelationID: "",
			expectRequestID:       true,
			expectCorrelationID:   true,
			expectResponseHeaders: true,
		},
		{
			name:                  "existing request ID - uses it",
			existingRequestID:     "existing-req-123",
			existingCorrelationID: "",
			expectRequestID:       true,
			expectCorrelationID:   true,
			expectResponseHeaders: true,
		},
		{
			name:                  "existing correlation ID - uses it",
			existingRequestID:     "",
			existingCorrelationID: "existing-corr-456",
			expectRequestID:       true,
			expectCorrelationID:   true,
			expectResponseHeaders: true,
		},
		{
			name:                  "both existing headers - uses them",
			existingRequestID:     "existing-req-123",
			existingCorrelationID: "existing-corr-456",
			expectRequestID:       true,
			expectCorrelationID:   true,
			expectResponseHeaders: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create middleware
			middleware := NewRequestIDMiddleware()

			// Create test handler that checks context
			var contextRequestID, contextCorrelationID string
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if id, ok := logging.GetRequestID(r.Context()); ok {
					contextRequestID = id
				}
				if id, ok := logging.GetCorrelationID(r.Context()); ok {
					contextCorrelationID = id
				}
				w.WriteHeader(http.StatusOK)
			})

			// Create request
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.existingRequestID != "" {
				req.Header.Set("X-Request-ID", tt.existingRequestID)
			}
			if tt.existingCorrelationID != "" {
				req.Header.Set("X-Correlation-ID", tt.existingCorrelationID)
			}

			// Create response recorder
			rr := httptest.NewRecorder()

			// Execute middleware
			middleware(handler).ServeHTTP(rr, req)

			// Check context was populated
			if tt.expectRequestID {
				assert.NotEmpty(t, contextRequestID, "request ID should be in context")
				if tt.existingRequestID != "" {
					assert.Equal(t, tt.existingRequestID, contextRequestID, "should use existing request ID")
				} else {
					assert.Regexp(t, `^[a-f0-9-]{36}$`, contextRequestID, "generated request ID should be UUID format")
				}
			}

			if tt.expectCorrelationID {
				assert.NotEmpty(t, contextCorrelationID, "correlation ID should be in context")
				if tt.existingCorrelationID != "" {
					assert.Equal(t, tt.existingCorrelationID, contextCorrelationID, "should use existing correlation ID")
				} else {
					assert.Regexp(t, `^[a-f0-9-]{36}$`, contextCorrelationID, "generated correlation ID should be UUID format")
				}
			}

			// Check response headers were set
			if tt.expectResponseHeaders {
				assert.NotEmpty(t, rr.Header().Get("X-Request-ID"), "response should have X-Request-ID header")
				assert.NotEmpty(t, rr.Header().Get("X-Correlation-ID"), "response should have X-Correlation-ID header")

				if tt.existingRequestID != "" {
					assert.Equal(t, tt.existingRequestID, rr.Header().Get("X-Request-ID"))
				}
				if tt.existingCorrelationID != "" {
					assert.Equal(t, tt.existingCorrelationID, rr.Header().Get("X-Correlation-ID"))
				}
			}
		})
	}
}

func TestRequestIDMiddleware_GenerateUniqueIDs(t *testing.T) {
	middleware := NewRequestIDMiddleware()

	// Track generated IDs
	generatedRequestIDs := make(map[string]bool)
	generatedCorrelationIDs := make(map[string]bool)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requestID, ok := logging.GetRequestID(r.Context()); ok {
			generatedRequestIDs[requestID] = true
		}
		if correlationID, ok := logging.GetCorrelationID(r.Context()); ok {
			generatedCorrelationIDs[correlationID] = true
		}
	})

	// Generate multiple requests
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()
		middleware(handler).ServeHTTP(rr, req)
	}

	// Check uniqueness
	assert.Len(t, generatedRequestIDs, 10, "should generate unique request IDs")
	assert.Len(t, generatedCorrelationIDs, 10, "should generate unique correlation IDs")
}

func TestRequestIDMiddleware_Integration(t *testing.T) {
	// Test that the middleware integrates properly with the logging context helpers
	middleware := NewRequestIDMiddleware()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify context can be used with logging helpers
		requestID, hasRequestID := logging.GetRequestID(r.Context())
		correlationID, hasCorrelationID := logging.GetCorrelationID(r.Context())

		require.True(t, hasRequestID, "context should have request ID")
		require.True(t, hasCorrelationID, "context should have correlation ID")
		require.NotEmpty(t, requestID, "request ID should not be empty")
		require.NotEmpty(t, correlationID, "correlation ID should not be empty")

		// Test that context values are accessible
		ctxWithBoth := logging.WithRequestID(logging.WithCorrelationID(context.Background(), correlationID), requestID)
		retrievedRequestID, _ := logging.GetRequestID(ctxWithBoth)
		retrievedCorrelationID, _ := logging.GetCorrelationID(ctxWithBoth)

		assert.Equal(t, requestID, retrievedRequestID)
		assert.Equal(t, correlationID, retrievedCorrelationID)

		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	middleware(handler).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestRequestIDMiddleware_HeaderValidation(t *testing.T) {
	tests := []struct {
		name        string
		headerValue string
		expectUsed  bool // whether the header value should be used or a new one generated
	}{
		{
			name:        "valid UUID",
			headerValue: "550e8400-e29b-41d4-a716-446655440000",
			expectUsed:  true,
		},
		{
			name:        "valid short ID",
			headerValue: "req-12345",
			expectUsed:  true,
		},
		{
			name:        "empty string",
			headerValue: "",
			expectUsed:  false, // should generate new
		},
		{
			name:        "whitespace only",
			headerValue: "   ",
			expectUsed:  false, // should generate new
		},
		{
			name:        "very long ID",
			headerValue: "this-is-a-very-long-request-id-that-might-be-problematic-for-some-systems-but-should-still-work",
			expectUsed:  true, // should use as-is for now
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := NewRequestIDMiddleware()

			var contextRequestID string
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if id, ok := logging.GetRequestID(r.Context()); ok {
					contextRequestID = id
				}
			})

			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("X-Request-ID", tt.headerValue)
			rr := httptest.NewRecorder()

			middleware(handler).ServeHTTP(rr, req)

			if tt.expectUsed {
				assert.Equal(t, tt.headerValue, contextRequestID, "should use provided header value")
			} else {
				assert.NotEqual(t, tt.headerValue, contextRequestID, "should generate new ID instead of using invalid header")
				assert.NotEmpty(t, contextRequestID, "should have generated a new ID")
			}
		})
	}
}
