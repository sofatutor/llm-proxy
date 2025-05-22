package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProxyConfig_Validate(t *testing.T) {
	// All fields valid
	cfg := &ProxyConfig{
		TargetBaseURL:    "https://api.example.com",
		AllowedMethods:   []string{"GET"},
		AllowedEndpoints: []string{"/foo"},
	}
	assert.NoError(t, cfg.Validate())

	// Missing TargetBaseURL
	cfg = &ProxyConfig{
		AllowedMethods:   []string{"GET"},
		AllowedEndpoints: []string{"/foo"},
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "TargetBaseURL")

	// Missing AllowedMethods
	cfg = &ProxyConfig{
		TargetBaseURL:    "https://api.example.com",
		AllowedEndpoints: []string{"/foo"},
	}
	err = cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "AllowedMethods")

	// Missing AllowedEndpoints
	cfg = &ProxyConfig{
		TargetBaseURL:  "https://api.example.com",
		AllowedMethods: []string{"GET"},
	}
	err = cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "AllowedEndpoints")
}

func TestChain(t *testing.T) {
	order := []string{}
	mw1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "mw1")
			next.ServeHTTP(w, r)
		})
	}
	mw2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "mw2")
			next.ServeHTTP(w, r)
		})
	}
	handlerCalled := false
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		order = append(order, "handler")
	})

	chained := Chain(h, mw1, mw2)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	chained.ServeHTTP(rec, req)

	if !handlerCalled {
		t.Errorf("handler was not called")
	}
	if len(order) != 3 || order[0] != "mw1" || order[1] != "mw2" || order[2] != "handler" {
		t.Errorf("unexpected middleware order: %v", order)
	}
}
