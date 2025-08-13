package middleware

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/sofatutor/llm-proxy/internal/logging"
)

// RequestIDMiddleware handles request and correlation ID context propagation
type RequestIDMiddleware struct{}

// NewRequestIDMiddleware creates a new middleware instance for request ID management
func NewRequestIDMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get or generate request ID
			requestID := getOrGenerateID(r.Header.Get("X-Request-ID"))

			// Get or generate correlation ID
			correlationID := getOrGenerateID(r.Header.Get("X-Correlation-ID"))

			// Add IDs to context
			ctx := logging.WithRequestID(r.Context(), requestID)
			ctx = logging.WithCorrelationID(ctx, correlationID)

			// Set response headers
			w.Header().Set("X-Request-ID", requestID)
			w.Header().Set("X-Correlation-ID", correlationID)

			// Continue with the request using the enriched context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// getOrGenerateID returns the provided ID if valid, otherwise generates a new UUID
func getOrGenerateID(existingID string) string {
	// Trim whitespace
	existingID = strings.TrimSpace(existingID)

	// If empty, generate new UUID
	if existingID == "" {
		return uuid.New().String()
	}

	// For now, accept any non-empty ID (could add validation later if needed)
	return existingID
}
