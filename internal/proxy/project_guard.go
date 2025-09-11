package proxy

import (
	"context"
	"encoding/json"
	"net/http"

	"go.uber.org/zap"
)

// ProjectActiveGuardMiddleware creates middleware that enforces project active status
// If enforceActive is false, the middleware passes through all requests without checking
// If enforceActive is true, inactive projects receive a 403 Forbidden response
func ProjectActiveGuardMiddleware(enforceActive bool, checker ProjectActiveChecker) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// If enforcement is disabled, pass through immediately
			if !enforceActive {
				next.ServeHTTP(w, r)
				return
			}

			// Get project ID from context (should be set by token validation middleware)
			projectIDValue := r.Context().Value(ctxKeyProjectID)
			if projectIDValue == nil {
				writeErrorResponse(w, http.StatusInternalServerError, ErrorResponse{
					Error:       "Internal server error",
					Code:        "internal_error",
					Description: "missing project ID in request context",
				})
				return
			}

			projectID, ok := projectIDValue.(string)
			if !ok || projectID == "" {
				writeErrorResponse(w, http.StatusInternalServerError, ErrorResponse{
					Error:       "Internal server error", 
					Code:        "internal_error",
					Description: "invalid project ID in request context",
				})
				return
			}

			// Check if project is active
			isActive, err := checker.GetProjectActive(r.Context(), projectID)
			if err != nil {
				// Log the error but don't expose internal details to client
				if logger := getLoggerFromContext(r.Context()); logger != nil {
					logger.Error("Failed to check project active status",
						zap.String("project_id", projectID),
						zap.Error(err))
				}
				
				writeErrorResponse(w, http.StatusServiceUnavailable, ErrorResponse{
					Error: "Service temporarily unavailable",
					Code:  "service_unavailable",
				})
				return
			}

			// If project is inactive, deny the request
			if !isActive {
				writeErrorResponse(w, http.StatusForbidden, ErrorResponse{
					Error: "Project is inactive",
					Code:  "project_inactive",
				})
				return
			}

			// Project is active, continue with request
			next.ServeHTTP(w, r)
		})
	}
}

// writeErrorResponse writes a JSON error response
func writeErrorResponse(w http.ResponseWriter, statusCode int, errorResp ErrorResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(errorResp); err != nil {
		// If we can't encode the error response, at least log it
		// (but we can't do much else since headers are already sent)
	}
}

// getLoggerFromContext tries to get a logger from context 
// This is a helper function that works with the existing logging context
func getLoggerFromContext(ctx context.Context) *zap.Logger {
	// Try to get logger from context if available
	// If not available, return nil and let caller handle gracefully
	if loggerValue := ctx.Value("logger"); loggerValue != nil {
		if logger, ok := loggerValue.(*zap.Logger); ok {
			return logger
		}
	}
	return nil
}