package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/sofatutor/llm-proxy/internal/audit"
	"github.com/sofatutor/llm-proxy/internal/logging"
	"go.uber.org/zap"
)

// shouldAllowProject determines whether the request should proceed based on project active status.
// It returns (allowed, statusCode, errorResponse). When allowed is true, statusCode and errorResponse are ignored.
// It also emits audit events for denied or error cases.
func shouldAllowProject(ctx context.Context, enforceActive bool, checker ProjectActiveChecker, projectID string, auditLogger *audit.Logger, r *http.Request) (bool, int, ErrorResponse) {
	if !enforceActive {
		return true, 0, ErrorResponse{}
	}
	
	// Get request metadata for audit events
	requestID, _ := logging.GetRequestID(ctx)
	clientIP := getClientIP(r)
	userAgent := r.UserAgent()
	
	isActive, err := checker.GetProjectActive(ctx, projectID)
	if err != nil {
		if logger := getLoggerFromContext(ctx); logger != nil {
			logger.Error("Failed to check project active status",
				zap.String("project_id", projectID),
				zap.Error(err))
		}
		
		// Emit audit event for service unavailable
		if auditLogger != nil {
			auditEvent := audit.NewEvent(audit.ActionProxyRequest, audit.ActorSystem, audit.ResultError).
				WithProjectID(projectID).
				WithRequestID(requestID).
				WithClientIP(clientIP).
				WithUserAgent(userAgent).
				WithHTTPMethod(r.Method).
				WithEndpoint(r.URL.Path).
				WithReason("service_unavailable").
				WithError(err)
			_ = auditLogger.Log(auditEvent)
		}
		
		return false, http.StatusServiceUnavailable, ErrorResponse{Error: "Service temporarily unavailable", Code: "service_unavailable"}
	}
	
	if !isActive {
		// Emit audit event for project inactive denial
		if auditLogger != nil {
			auditEvent := audit.NewEvent(audit.ActionProxyRequest, audit.ActorSystem, audit.ResultDenied).
				WithProjectID(projectID).
				WithRequestID(requestID).
				WithClientIP(clientIP).
				WithUserAgent(userAgent).
				WithHTTPMethod(r.Method).
				WithEndpoint(r.URL.Path).
				WithReason("project_inactive")
			_ = auditLogger.Log(auditEvent)
		}
		
		return false, http.StatusForbidden, ErrorResponse{Error: "Project is inactive", Code: "project_inactive"}
	}
	
	return true, 0, ErrorResponse{}
}

// ProjectActiveGuardMiddleware creates middleware that enforces project active status
// If enforceActive is false, the middleware passes through all requests without checking
// If enforceActive is true, inactive projects receive a 403 Forbidden response
func ProjectActiveGuardMiddleware(enforceActive bool, checker ProjectActiveChecker, auditLogger *audit.Logger) Middleware {
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

			if allowed, status, er := shouldAllowProject(r.Context(), enforceActive, checker, projectID, auditLogger, r); !allowed {
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

// getClientIP extracts the client IP address from the request
// It checks X-Forwarded-For, X-Real-IP headers and falls back to RemoteAddr
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (comma-separated list)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP from the list
		if idx := strings.Index(xff, ","); idx >= 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	
	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	
	// Fallback to RemoteAddr (remove port if present)
	if idx := strings.LastIndex(r.RemoteAddr, ":"); idx >= 0 {
		return r.RemoteAddr[:idx]
	}
	return r.RemoteAddr
}
