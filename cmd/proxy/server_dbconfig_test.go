package main

import (
	"testing"

	"github.com/sofatutor/llm-proxy/internal/config"
	"github.com/sofatutor/llm-proxy/internal/database"
)

func TestBuildDatabaseConfig_DBDriverMySQL(t *testing.T) {
	t.Setenv("DB_DRIVER", "mysql")
	t.Setenv("DATABASE_URL", "llmproxy:pass@tcp(mysql:3306)/llmproxy?parseTime=true")
	t.Setenv("DATABASE_PATH", "")

	appConfig := &config.Config{DatabasePath: "data/llm-proxy.db"}
	dbConfig := buildDatabaseConfig(appConfig)
	if dbConfig.Driver != database.DriverMySQL {
		t.Fatalf("expected DriverMySQL, got %q", dbConfig.Driver)
	}
	if dbConfig.DatabaseURL == "" {
		t.Fatalf("expected DatabaseURL to be set")
	}
}

func TestBuildDatabaseConfig_DBDriverPostgres(t *testing.T) {
	t.Setenv("DB_DRIVER", "postgres")
	t.Setenv("DATABASE_URL", "postgresql://user:pass@localhost:5432/llmproxy?sslmode=disable")
	t.Setenv("DATABASE_PATH", "")

	appConfig := &config.Config{DatabasePath: "data/llm-proxy.db"}
	dbConfig := buildDatabaseConfig(appConfig)
	if dbConfig.Driver != database.DriverPostgres {
		t.Fatalf("expected DriverPostgres, got %q", dbConfig.Driver)
	}
	if dbConfig.DatabaseURL == "" {
		t.Fatalf("expected DatabaseURL to be set")
	}
}

func TestBuildDatabaseConfig_SQLitePathFallback(t *testing.T) {
	t.Setenv("DB_DRIVER", "")
	t.Setenv("DATABASE_PATH", "")

	appConfig := &config.Config{DatabasePath: "/tmp/llm-proxy-test.db"}
	dbConfig := buildDatabaseConfig(appConfig)
	if dbConfig.Driver != database.DriverSQLite {
		t.Fatalf("expected DriverSQLite, got %q", dbConfig.Driver)
	}
	if dbConfig.Path != appConfig.DatabasePath {
		t.Fatalf("expected Path %q, got %q", appConfig.DatabasePath, dbConfig.Path)
	}
}

func TestBuildDatabaseConfig_SQLiteExplicitDriverPathFallback(t *testing.T) {
	t.Setenv("DB_DRIVER", "sqlite")
	t.Setenv("DATABASE_PATH", "")

	appConfig := &config.Config{DatabasePath: "/tmp/llm-proxy-test-explicit-driver.db"}
	dbConfig := buildDatabaseConfig(appConfig)
	if dbConfig.Driver != database.DriverSQLite {
		t.Fatalf("expected DriverSQLite, got %q", dbConfig.Driver)
	}
	if dbConfig.Path != appConfig.DatabasePath {
		t.Fatalf("expected Path %q, got %q", appConfig.DatabasePath, dbConfig.Path)
	}
}

func TestBuildDatabaseConfig_SQLiteEnvDatabasePathOverridesAppConfig(t *testing.T) {
	t.Setenv("DB_DRIVER", "sqlite")
	t.Setenv("DATABASE_PATH", "/tmp/llm-proxy-env.db")

	appConfig := &config.Config{DatabasePath: "/tmp/llm-proxy-app.db"}
	dbConfig := buildDatabaseConfig(appConfig)
	if dbConfig.Driver != database.DriverSQLite {
		t.Fatalf("expected DriverSQLite, got %q", dbConfig.Driver)
	}
	if dbConfig.Path != "/tmp/llm-proxy-env.db" {
		t.Fatalf("expected env Path %q, got %q", "/tmp/llm-proxy-env.db", dbConfig.Path)
	}
}

func TestBuildDatabaseConfig_SQLiteNilAppConfigUsesDefaultPath(t *testing.T) {
	t.Setenv("DB_DRIVER", "sqlite")
	t.Setenv("DATABASE_PATH", "")

	dbConfig := buildDatabaseConfig(nil)
	if dbConfig.Driver != database.DriverSQLite {
		t.Fatalf("expected DriverSQLite, got %q", dbConfig.Driver)
	}
	if dbConfig.Path != database.DefaultFullConfig().Path {
		t.Fatalf("expected default Path %q, got %q", database.DefaultFullConfig().Path, dbConfig.Path)
	}
}

func TestValidateEncryptionKeyRequired(t *testing.T) {
	testCases := []struct {
		name                 string
		requireEncryptionKey string
		encryptionKey        string
		wantErr              bool
	}{
		{
			name:                 "not required, missing key",
			requireEncryptionKey: "false",
			encryptionKey:        "",
			wantErr:              false,
		},
		{
			name:                 "required, missing key",
			requireEncryptionKey: "true",
			encryptionKey:        "",
			wantErr:              true,
		},
		{
			name:                 "required, key present",
			requireEncryptionKey: "true",
			encryptionKey:        "dGVzdC1lbmNyeXB0aW9uLWtleS0zMi1ieXRlcwo=",
			wantErr:              false,
		},
		{
			name:                 "unset require var, missing key",
			requireEncryptionKey: "",
			encryptionKey:        "",
			wantErr:              false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("REQUIRE_ENCRYPTION_KEY", testCase.requireEncryptionKey)
			t.Setenv("ENCRYPTION_KEY", testCase.encryptionKey)

			err := validateEncryptionKeyRequired()
			if testCase.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
		})
	}
}
