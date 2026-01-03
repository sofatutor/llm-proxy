package main

import (
	"os"
	"testing"

	"github.com/sofatutor/llm-proxy/internal/config"
	"github.com/sofatutor/llm-proxy/internal/database"
)

func TestBuildDatabaseConfig_DBDriverMySQL(t *testing.T) {
	origDriver := os.Getenv("DB_DRIVER")
	origDatabaseURL := os.Getenv("DATABASE_URL")
	origDatabasePath := os.Getenv("DATABASE_PATH")
	defer func() {
		_ = os.Setenv("DB_DRIVER", origDriver)
		_ = os.Setenv("DATABASE_URL", origDatabaseURL)
		_ = os.Setenv("DATABASE_PATH", origDatabasePath)
	}()

	_ = os.Setenv("DB_DRIVER", "mysql")
	_ = os.Setenv("DATABASE_URL", "llmproxy:pass@tcp(mysql:3306)/llmproxy?parseTime=true")
	_ = os.Unsetenv("DATABASE_PATH")

	appConfig := &config.Config{DatabasePath: "data/llm-proxy.db"}
	dbConfig := buildDatabaseConfig(appConfig)
	if dbConfig.Driver != database.DriverMySQL {
		t.Fatalf("expected DriverMySQL, got %q", dbConfig.Driver)
	}
	if dbConfig.DatabaseURL == "" {
		t.Fatalf("expected DatabaseURL to be set")
	}
}

func TestBuildDatabaseConfig_SQLitePathFallback(t *testing.T) {
	origDriver := os.Getenv("DB_DRIVER")
	origDatabasePath := os.Getenv("DATABASE_PATH")
	defer func() {
		_ = os.Setenv("DB_DRIVER", origDriver)
		_ = os.Setenv("DATABASE_PATH", origDatabasePath)
	}()

	_ = os.Unsetenv("DB_DRIVER")
	_ = os.Unsetenv("DATABASE_PATH")

	appConfig := &config.Config{DatabasePath: "/tmp/llm-proxy-test.db"}
	dbConfig := buildDatabaseConfig(appConfig)
	if dbConfig.Driver != database.DriverSQLite {
		t.Fatalf("expected DriverSQLite, got %q", dbConfig.Driver)
	}
	if dbConfig.Path != appConfig.DatabasePath {
		t.Fatalf("expected Path %q, got %q", appConfig.DatabasePath, dbConfig.Path)
	}
}
