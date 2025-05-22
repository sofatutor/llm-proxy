package proxy

import (
	"net/http"
	"sync"
	"time"
)

// CircuitBreakerMiddleware returns a middleware that opens the circuit after N consecutive failures.
// While open, it returns 503 immediately. After a cooldown, it closes and allows requests again.
func CircuitBreakerMiddleware(failureThreshold int, cooldown time.Duration, isTransient func(status int) bool) Middleware {
	cb := &circuitBreaker{
		failureThreshold: failureThreshold,
		cooldown:         cooldown,
		isTransient:      isTransient,
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cb.mu.Lock()
			if cb.open {
				// Allow test override of cooldown via context
				if override, ok := r.Context().Value("circuitbreaker_cooldown_override").(time.Duration); ok {
					cb.cooldown = override
				}
				if time.Since(cb.openedAt) < cb.cooldown {
					cb.mu.Unlock()
					w.WriteHeader(http.StatusServiceUnavailable)
					w.Write([]byte("{\"error\":\"Upstream unavailable (circuit breaker open)\"}"))
					return
				}
				// Cooldown expired, close circuit
				cb.open = false
				cb.failureCount = 0
			}
			cb.mu.Unlock()

			rec := &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(rec, r)

			if cb.isTransient(rec.statusCode) {
				cb.mu.Lock()
				cb.failureCount++
				if cb.failureCount >= cb.failureThreshold {
					cb.open = true
					cb.openedAt = time.Now()
				}
				cb.mu.Unlock()
			} else {
				cb.mu.Lock()
				cb.failureCount = 0
				cb.mu.Unlock()
			}
		})
	}
}

type circuitBreaker struct {
	mu              sync.Mutex
	open            bool
	failureThreshold int
	failureCount     int
	openedAt        time.Time
	cooldown        time.Duration
	isTransient     func(status int) bool
}
