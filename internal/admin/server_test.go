package admin

import (
	"context"
	"os"
	"testing"

	"github.com/sofatutor/llm-proxy/internal/config"
)

func TestGetSessionSecret(t *testing.T) {
	cfg := &config.Config{AdminUI: config.AdminUIConfig{ManagementToken: "secret-token"}}
	secret := getSessionSecret(cfg)
	want := []byte("secret-tokenllmproxy-cookie-salt")
	if string(secret) != string(want) {
		t.Errorf("getSessionSecret() = %q, want %q", secret, want)
	}
}

func TestNewServer_Minimal(t *testing.T) {
	if _, err := os.Stat("web/templates/base.html"); err != nil {
		t.Skip("Skipping: required template file not found")
	}
	cfg := &config.Config{
		AdminUI: config.AdminUIConfig{
			APIBaseURL:      "http://localhost:1234",
			ManagementToken: "token",
			ListenAddr:      ":0",
		},
		LogLevel: "info",
	}
	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	if srv == nil {
		t.Fatal("NewServer() returned nil")
	}
	if srv.config != cfg {
		t.Error("Server config not set correctly")
	}
	if srv.engine == nil {
		t.Error("Server engine not set")
	}
	if srv.server == nil {
		t.Error("Server http.Server not set")
	}
}

func TestServer_Shutdown_NoServer(t *testing.T) {
	s := &Server{}
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Shutdown panicked: %v", r)
		}
	}()
	_ = s.Shutdown(context.Background())
	// Accept both nil and non-nil error, but must not panic
}
