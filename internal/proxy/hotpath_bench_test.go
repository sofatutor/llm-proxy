package proxy

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sofatutor/llm-proxy/internal/eventbus"
	"github.com/sofatutor/llm-proxy/internal/middleware"
	"go.uber.org/zap"
)

type benchTokenValidator struct{}

func (benchTokenValidator) ValidateToken(ctx context.Context, tokenID string) (string, error) {
	return "p1", nil
}
func (benchTokenValidator) ValidateTokenWithTracking(ctx context.Context, tokenID string) (string, error) {
	return "p1", nil
}

type benchProjectStore struct{}

func (benchProjectStore) GetAPIKeyForProject(ctx context.Context, projectID string) (string, error) {
	return "sk-test", nil
}
func (benchProjectStore) GetProjectActive(ctx context.Context, projectID string) (bool, error) {
	return true, nil
}
func (benchProjectStore) ListProjects(ctx context.Context) ([]Project, error) { return nil, nil }
func (benchProjectStore) CreateProject(ctx context.Context, project Project) error {
	return nil
}
func (benchProjectStore) GetProjectByID(ctx context.Context, projectID string) (Project, error) {
	return Project{}, nil
}
func (benchProjectStore) UpdateProject(ctx context.Context, project Project) error  { return nil }
func (benchProjectStore) DeleteProject(ctx context.Context, projectID string) error { return nil }

type noopEventBus struct{}

func (noopEventBus) Publish(ctx context.Context, evt eventbus.Event) {}
func (noopEventBus) Subscribe() <-chan eventbus.Event {
	ch := make(chan eventbus.Event)
	close(ch)
	return ch
}
func (noopEventBus) Stop() {}

func BenchmarkProxy_HotPath_NoObservability(b *testing.B) {
	b.ReportAllocs()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"model":"gpt-4","usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`)
	}))
	defer upstream.Close()

	p := newTestProxyWithConfig(ProxyConfig{
		TargetBaseURL:         upstream.URL,
		AllowedEndpoints:      []string{"/v1/chat/completions"},
		AllowedMethods:        []string{http.MethodPost},
		EnforceProjectActive:  false,
		MaxIdleConns:          512,
		MaxIdleConnsPerHost:   512,
		IdleConnTimeout:       0,
		ResponseHeaderTimeout: 0,
	})
	p.tokenValidator = benchTokenValidator{}
	p.projectStore = benchProjectStore{}
	p.obsMiddleware = nil

	h := p.Handler()
	body := `{"model":"gpt-4.1-nano","messages":[{"role":"user","content":"Hi"}]}`

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
			req.Header.Set("Authorization", "Bearer tok")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			_ = w.Result().Body.Close()
		}
	})
}

func BenchmarkProxy_HotPath_WithObservability(b *testing.B) {
	b.ReportAllocs()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"model":"gpt-4","usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`)
	}))
	defer upstream.Close()

	p := newTestProxyWithConfig(ProxyConfig{
		TargetBaseURL:         upstream.URL,
		AllowedEndpoints:      []string{"/v1/chat/completions"},
		AllowedMethods:        []string{http.MethodPost},
		EnforceProjectActive:  false,
		MaxIdleConns:          512,
		MaxIdleConnsPerHost:   512,
		IdleConnTimeout:       0,
		ResponseHeaderTimeout: 0,
	})
	p.tokenValidator = benchTokenValidator{}
	p.projectStore = benchProjectStore{}
	p.obsMiddleware = middleware.NewObservabilityMiddleware(middleware.ObservabilityConfig{
		Enabled:              true,
		EventBus:             noopEventBus{},
		MaxRequestBodyBytes:  64 * 1024,
		MaxResponseBodyBytes: 256 * 1024,
	}, zap.NewNop())

	h := p.Handler()
	body := `{"model":"gpt-4.1-nano","messages":[{"role":"user","content":"Hi"}]}`

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
			req.Header.Set("Authorization", "Bearer tok")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			_ = w.Result().Body.Close()
		}
	})
}
