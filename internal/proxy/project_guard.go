package proxy

import (
	"context"
	"encoding/json"
	"net/http"

	"go.uber.org/zap"
)

// shouldAllowProject determines whether the request should proceed based on project active status.
// It returns (allowed, statusCode, errorResponse). When allowed is true, statusCode and errorResponse are ignored.
func shouldAllowProject(ctx context.Context, enforceActive bool, checker ProjectActiveChecker, projectID string) (bool, int, ErrorResponse) {
	if !enforceActive {
		return true, 0, ErrorResponse{}
	}
	isActive, err := checker.GetProjectActive(ctx, projectID)
	if err != nil {
		if logger := getLoggerFromContext(ctx); logger != nil {
			logger.Error("Failed to check project active status",
				zap.String("project_id", projectID),
				zap.Error(err))
		}
		return false, http.StatusServiceUnavailable, ErrorResponse{Error: "Service temporarily unavailable", Code: "service_unavailable"}
	}
	if !isActive {
		return false, http.StatusForbidden, ErrorResponse{Error: "Project is inactive", Code: "project_inactive"}
	}
	return true, 0, ErrorResponse{}
}

// ProjectActiveGuardMiddleware creates middleware that enforces project active status
// If enforceActive is false, the middleware passes through all requests without checking
// If enforceActive is true, inactive projects receive a 403 Forbidden response
func ProjectActiveGuardMiddleware(enforceActive bool, checker ProjectActiveChecker) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

			if allowed, status, er := shouldAllowProject(r.Context(), enforceActive, checker, projectID); !allowed {
				writeErrorResponse(w, status, er)
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
		// No request context available here, so we cannot access a scoped logger safely
		// Consider hooking a global logger if needed; for now we silently ignore to avoid panics
		_ = err
	}
}

// getLoggerFromContext tries to get a logger from context
// This is a helper function that works with the existing logging context
func getLoggerFromContext(ctx context.Context) *zap.Logger {
	// Try to get logger from context if available
	// If not available, return nil and let caller handle gracefully
	if loggerValue := ctx.Value(ctxKeyLogger); loggerValue != nil {
		if logger, ok := loggerValue.(*zap.Logger); ok {
			return logger
		}
	}
	return nil
}
