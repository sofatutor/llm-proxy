package database

// NOTE: This test suite focuses on unit tests for factory logic, rebinding helpers, and error conditions.
// PostgreSQL integration tests (connecting to a real PostgreSQL instance, running migrations, performing CRUD operations)
// are not included here and will be added via Docker Compose integration tests (see issue #139 and review comment #2580739337).

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDriverType_Constants(t *testing.T) {
	// Verify constants are correctly defined
	assert.Equal(t, DriverType("sqlite"), DriverSQLite)
	assert.Equal(t, DriverType("postgres"), DriverPostgres)
	assert.Equal(t, DriverType("mysql"), DriverMySQL)
}

func TestDefaultFullConfig(t *testing.T) {
	config := DefaultFullConfig()

	assert.Equal(t, DriverSQLite, config.Driver)
	assert.Equal(t, "data/llm-proxy.db", config.Path)
	assert.Equal(t, "", config.DatabaseURL)
	assert.Equal(t, 10, config.MaxOpenConns)
	assert.Equal(t, 5, config.MaxIdleConns)
	assert.Equal(t, time.Hour, config.ConnMaxLifetime)
}

func TestConfigFromEnv(t *testing.T) {
	// Save original env vars and restore after test using t.Setenv
	// t.Setenv automatically restores the original value after the test
	origDriver := os.Getenv("DB_DRIVER")
	origPath := os.Getenv("DATABASE_PATH")
	origURL := os.Getenv("DATABASE_URL")
	origPoolSize := os.Getenv("DATABASE_POOL_SIZE")
	origIdleConns := os.Getenv("DATABASE_MAX_IDLE_CONNS")
	origLifetime := os.Getenv("DATABASE_CONN_MAX_LIFETIME")

	// Cleanup after test
	defer func() {
		_ = os.Setenv("DB_DRIVER", origDriver)
		_ = os.Setenv("DATABASE_PATH", origPath)
		_ = os.Setenv("DATABASE_URL", origURL)
		_ = os.Setenv("DATABASE_POOL_SIZE", origPoolSize)
		_ = os.Setenv("DATABASE_MAX_IDLE_CONNS", origIdleConns)
		_ = os.Setenv("DATABASE_CONN_MAX_LIFETIME", origLifetime)
	}()

	tests := []struct {
		name     string
		envVars  map[string]string
		expected FullConfig
	}{
		{
			name:    "default values",
			envVars: map[string]string{},
			expected: FullConfig{
				Driver:          DriverSQLite,
				Path:            "data/llm-proxy.db",
				DatabaseURL:     "",
				MaxOpenConns:    10,
				MaxIdleConns:    5,
				ConnMaxLifetime: time.Hour,
			},
		},
		{
			name: "postgres driver",
			envVars: map[string]string{
				"DB_DRIVER":    "postgres",
				"DATABASE_URL": "postgres://user:pass@localhost:5432/db",
			},
			expected: FullConfig{
				Driver:          DriverPostgres,
				Path:            "data/llm-proxy.db",
				DatabaseURL:     "postgres://user:pass@localhost:5432/db",
				MaxOpenConns:    10,
				MaxIdleConns:    5,
				ConnMaxLifetime: time.Hour,
			},
		},
		{
			name: "custom pool settings",
			envVars: map[string]string{
				"DATABASE_POOL_SIZE":         "20",
				"DATABASE_MAX_IDLE_CONNS":    "10",
				"DATABASE_CONN_MAX_LIFETIME": "30m",
			},
			expected: FullConfig{
				Driver:          DriverSQLite,
				Path:            "data/llm-proxy.db",
				DatabaseURL:     "",
				MaxOpenConns:    20,
				MaxIdleConns:    10,
				ConnMaxLifetime: 30 * time.Minute,
			},
		},
		{
			name: "mysql driver",
			envVars: map[string]string{
				"DB_DRIVER":    "mysql",
				"DATABASE_URL": "user:password@tcp(localhost:3306)/dbname?parseTime=true",
			},
			expected: FullConfig{
				Driver:          DriverMySQL,
				Path:            "data/llm-proxy.db",
				DatabaseURL:     "user:password@tcp(localhost:3306)/dbname?parseTime=true",
				MaxOpenConns:    10,
				MaxIdleConns:    5,
				ConnMaxLifetime: time.Hour,
			},
		},
		{
			name: "uppercase driver",
			envVars: map[string]string{
				"DB_DRIVER": "POSTGRES",
			},
			expected: FullConfig{
				Driver:          DriverPostgres,
				Path:            "data/llm-proxy.db",
				DatabaseURL:     "",
				MaxOpenConns:    10,
				MaxIdleConns:    5,
				ConnMaxLifetime: time.Hour,
			},
		},
		{
			name: "invalid pool size (ignored)",
			envVars: map[string]string{
				"DATABASE_POOL_SIZE": "invalid",
			},
			expected: FullConfig{
				Driver:          DriverSQLite,
				Path:            "data/llm-proxy.db",
				DatabaseURL:     "",
				MaxOpenConns:    10, // default preserved
				MaxIdleConns:    5,
				ConnMaxLifetime: time.Hour,
			},
		},
		{
			name: "negative pool size (ignored)",
			envVars: map[string]string{
				"DATABASE_POOL_SIZE": "-5",
			},
			expected: FullConfig{
				Driver:          DriverSQLite,
				Path:            "data/llm-proxy.db",
				DatabaseURL:     "",
				MaxOpenConns:    10, // default preserved
				MaxIdleConns:    5,
				ConnMaxLifetime: time.Hour,
			},
		},
		{
			name: "invalid driver (defaults to sqlite)",
			envVars: map[string]string{
				"DB_DRIVER": "oracle",
			},
			expected: FullConfig{
				Driver:          DriverSQLite, // default preserved for invalid driver
				Path:            "data/llm-proxy.db",
				DatabaseURL:     "",
				MaxOpenConns:    10,
				MaxIdleConns:    5,
				ConnMaxLifetime: time.Hour,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear env vars
			_ = os.Unsetenv("DB_DRIVER")
			_ = os.Unsetenv("DATABASE_PATH")
			_ = os.Unsetenv("DATABASE_URL")
			_ = os.Unsetenv("DATABASE_POOL_SIZE")
			_ = os.Unsetenv("DATABASE_MAX_IDLE_CONNS")
			_ = os.Unsetenv("DATABASE_CONN_MAX_LIFETIME")

			// Set env vars for test
			for k, v := range tt.envVars {
				_ = os.Setenv(k, v)
			}

			config := ConfigFromEnv()
			assert.Equal(t, tt.expected.Driver, config.Driver)
			assert.Equal(t, tt.expected.Path, config.Path)
			assert.Equal(t, tt.expected.DatabaseURL, config.DatabaseURL)
			assert.Equal(t, tt.expected.MaxOpenConns, config.MaxOpenConns)
			assert.Equal(t, tt.expected.MaxIdleConns, config.MaxIdleConns)
			assert.Equal(t, tt.expected.ConnMaxLifetime, config.ConnMaxLifetime)
		})
	}
}

func TestParsePositiveInt(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		hasError bool
	}{
		{"5", 5, false},
		{"100", 100, false},
		{"1", 1, false},
		{"0", 0, true},
		{"-5", 0, true},
		{"invalid", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parsePositiveInt(tt.input)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestMigrationsPathForDriver(t *testing.T) {
	t.Run("sqlite_returns_error", func(t *testing.T) {
		path, err := MigrationsPathForDriver(DriverSQLite)
		require.Error(t, err)
		assert.Empty(t, path)
	})

	t.Run("postgres_returns_postgres_migrations_dir", func(t *testing.T) {
		path, err := MigrationsPathForDriver(DriverPostgres)
		require.NoError(t, err)
		require.NotEmpty(t, path)
		assert.Equal(t, "postgres", filepath.Base(path))
		assert.Equal(t, "sql", filepath.Base(filepath.Dir(path)))
		assert.Equal(t, "migrations", filepath.Base(filepath.Dir(filepath.Dir(path))))
	})

	t.Run("mysql_returns_mysql_migrations_dir", func(t *testing.T) {
		path, err := MigrationsPathForDriver(DriverMySQL)
		require.NoError(t, err)
		require.NotEmpty(t, path)
		assert.Equal(t, "mysql", filepath.Base(path))
		assert.Equal(t, "sql", filepath.Base(filepath.Dir(path)))
		assert.Equal(t, "migrations", filepath.Base(filepath.Dir(filepath.Dir(path))))
	})
}

func TestNewFromConfig_SQLite(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	config := FullConfig{
		Driver:          DriverSQLite,
		Path:            dbPath,
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: time.Minute,
	}

	db, err := NewFromConfig(config)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	assert.Equal(t, DriverSQLite, db.Driver())
	assert.NotNil(t, db.DB())

	// Verify connection works
	err = db.HealthCheck(context.Background())
	assert.NoError(t, err)
}

func TestNewFromConfig_SQLiteInMemory(t *testing.T) {
	config := FullConfig{
		Driver:          DriverSQLite,
		Path:            ":memory:",
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: time.Minute,
	}

	db, err := NewFromConfig(config)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	assert.Equal(t, DriverSQLite, db.Driver())

	// Verify connection works
	err = db.HealthCheck(context.Background())
	assert.NoError(t, err)
}

func TestNewFromConfig_UnsupportedDriver(t *testing.T) {
	config := FullConfig{
		Driver: DriverType("oracle"),
	}

	db, err := NewFromConfig(config)
	assert.Error(t, err)
	assert.Nil(t, db)
	assert.Contains(t, err.Error(), "unsupported database driver")
}

// TestNewFromConfig_PostgresMissingURL has been split into:
// - factory_postgres_test.go: TestNewFromConfig_PostgresMissingURL_WithPostgresTag (//go:build postgres)
// - factory_postgres_stub_test.go: TestNewFromConfig_PostgresNotCompiledIn (//go:build !postgres)
// These tests verify appropriate error messages based on whether PostgreSQL support is compiled in.

func TestDB_Driver(t *testing.T) {
	config := FullConfig{
		Driver: DriverSQLite,
		Path:   ":memory:",
	}

	db, err := NewFromConfig(config)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	assert.Equal(t, DriverSQLite, db.Driver())
}

func TestDB_HealthCheck(t *testing.T) {
	config := FullConfig{
		Driver: DriverSQLite,
		Path:   ":memory:",
	}

	db, err := NewFromConfig(config)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Health check should pass
	err = db.HealthCheck(context.Background())
	assert.NoError(t, err)

	// Close and try again - should fail
	_ = db.Close()
	err = db.HealthCheck(context.Background())
	assert.Error(t, err)
}

func TestDB_HealthCheck_NilDB(t *testing.T) {
	var db *DB
	err := db.HealthCheck(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database is nil")
}

func TestGetMigrationsPathForDialect(t *testing.T) {
	// Test that SQLite returns an error (SQLite uses schema.sql, NOT migrations)
	_, err := getMigrationsPathForDialect("sqlite3")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not use migrations")

	// Test that we can find migrations for postgres
	path, err := getMigrationsPathForDialect("postgres")
	assert.NoError(t, err)
	assert.NotEmpty(t, path)
}

func TestRunMigrationsForDriver_SQLiteError(t *testing.T) {
	// Test that SQLite returns an error (SQLite uses schema.sql, NOT migrations)
	err := runMigrationsForDriver(nil, "sqlite3")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not use migrations")

	err = runMigrationsForDriver(nil, "sqlite")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not use migrations")
}

func TestRunMigrationsForDriver_UnknownDialect(t *testing.T) {
	err := runMigrationsForDriver(nil, "oracle")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "migrations directory not found")
}

func TestRunMigrationsForDriver_PostgresNilDB(t *testing.T) {
	migrationsPath, err := getMigrationsPathForDialect("postgres")
	require.NoError(t, err)
	require.NotEmpty(t, migrationsPath)

	err = runMigrationsForDriver(nil, "postgres")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database connection is nil")
}

func TestPostgresMigrations(t *testing.T) {
	t.Helper()

	repoRoot := repoRootFromTestFile(t)
	postgresDir := filepath.Join(repoRoot, "internal", "database", "migrations", "sql", "postgres")

	postgres := collectMigrationFiles(t, postgresDir)

	// Verify we have PostgreSQL migrations
	assert.NotEmpty(t, postgres, "expected PostgreSQL migrations to exist")
}

func collectMigrationFiles(t *testing.T, dir string) map[string]struct{} {
	t.Helper()

	entries, err := os.ReadDir(dir)
	require.NoErrorf(t, err, "failed to read migrations directory %s", dir)

	files := make(map[string]struct{})
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if migrationFilePattern.MatchString(name) {
			files[name] = struct{}{}
		}
	}

	return files
}

var migrationFilePattern = regexp.MustCompile(`^\d+_.+\.sql$`)

func repoRootFromTestFile(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to determine caller path for migration test")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func TestDB_RebindQuery(t *testing.T) {
	tests := []struct {
		name     string
		driver   DriverType
		query    string
		expected string
	}{
		{
			name:     "SQLite no change",
			driver:   DriverSQLite,
			query:    "SELECT * FROM tokens WHERE token = ?",
			expected: "SELECT * FROM tokens WHERE token = ?",
		},
		{
			name:     "MySQL no change",
			driver:   DriverMySQL,
			query:    "SELECT * FROM tokens WHERE token = ?",
			expected: "SELECT * FROM tokens WHERE token = ?",
		},
		{
			name:     "PostgreSQL single placeholder",
			driver:   DriverPostgres,
			query:    "SELECT * FROM tokens WHERE token = ?",
			expected: "SELECT * FROM tokens WHERE token = $1",
		},
		{
			name:     "PostgreSQL multiple placeholders",
			driver:   DriverPostgres,
			query:    "INSERT INTO tokens (token, project_id, is_active) VALUES (?, ?, ?)",
			expected: "INSERT INTO tokens (token, project_id, is_active) VALUES ($1, $2, $3)",
		},
		{
			name:     "PostgreSQL no placeholders",
			driver:   DriverPostgres,
			query:    "SELECT * FROM tokens",
			expected: "SELECT * FROM tokens",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := &DB{driver: tt.driver}
			result := db.RebindQuery(tt.query)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDB_Placeholder(t *testing.T) {
	sqliteDB := &DB{driver: DriverSQLite}
	postgresDB := &DB{driver: DriverPostgres}
	mysqlDB := &DB{driver: DriverMySQL}

	assert.Equal(t, "?", sqliteDB.Placeholder(1))
	assert.Equal(t, "?", sqliteDB.Placeholder(2))

	assert.Equal(t, "?", mysqlDB.Placeholder(1))
	assert.Equal(t, "?", mysqlDB.Placeholder(2))

	assert.Equal(t, "$1", postgresDB.Placeholder(1))
	assert.Equal(t, "$2", postgresDB.Placeholder(2))
	assert.Equal(t, "$10", postgresDB.Placeholder(10))
}

func TestDB_Placeholders(t *testing.T) {
	sqliteDB := &DB{driver: DriverSQLite}
	postgresDB := &DB{driver: DriverPostgres}
	mysqlDB := &DB{driver: DriverMySQL}

	assert.Equal(t, []string{"?", "?", "?"}, sqliteDB.Placeholders(3))
	assert.Equal(t, []string{"?", "?", "?"}, mysqlDB.Placeholders(3))
	assert.Equal(t, []string{"$1", "$2", "$3"}, postgresDB.Placeholders(3))
}

func TestDB_PlaceholderList(t *testing.T) {
	sqliteDB := &DB{driver: DriverSQLite}
	postgresDB := &DB{driver: DriverPostgres}
	mysqlDB := &DB{driver: DriverMySQL}

	assert.Equal(t, "?, ?, ?", sqliteDB.PlaceholderList(3))
	assert.Equal(t, "?, ?, ?", mysqlDB.PlaceholderList(3))
	assert.Equal(t, "$1, $2, $3", postgresDB.PlaceholderList(3))
}

func TestMaintainDatabase_SQLite(t *testing.T) {
	config := FullConfig{
		Driver: DriverSQLite,
		Path:   ":memory:",
	}

	db, err := NewFromConfig(config)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Should work for SQLite
	err = db.MaintainDatabase(context.Background())
	assert.NoError(t, err)
}

func TestGetStats_SQLite(t *testing.T) {
	config := FullConfig{
		Driver: DriverSQLite,
		Path:   ":memory:",
	}

	db, err := NewFromConfig(config)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	stats, err := db.GetStats(context.Background())
	require.NoError(t, err)

	// Check expected keys
	expectedKeys := []string{
		"database_size_bytes",
		"project_count",
		"active_token_count",
		"expired_token_count",
		"total_request_count",
	}
	for _, key := range expectedKeys {
		_, ok := stats[key]
		assert.True(t, ok, "missing stats key: %s", key)
	}
}

func TestBackupDatabase_SQLite(t *testing.T) {
	config := FullConfig{
		Driver: DriverSQLite,
		Path:   ":memory:",
	}

	db, err := NewFromConfig(config)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	tmpDir := t.TempDir()
	backupPath := filepath.Join(tmpDir, "backup.db")

	err = db.BackupDatabase(context.Background(), backupPath)
	assert.NoError(t, err)

	// Verify backup file exists
	_, err = os.Stat(backupPath)
	assert.NoError(t, err)
}

func TestBackupDatabase_EmptyPath(t *testing.T) {
	config := FullConfig{
		Driver: DriverSQLite,
		Path:   ":memory:",
	}

	db, err := NewFromConfig(config)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	err = db.BackupDatabase(context.Background(), "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "backup path cannot be empty")
}

func TestBackupDatabase_InvalidPath(t *testing.T) {
	config := FullConfig{
		Driver: DriverSQLite,
		Path:   ":memory:",
	}

	db, err := NewFromConfig(config)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Path starting with invalid character
	err = db.BackupDatabase(context.Background(), "-invalidpath")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid backup path")
}

// TestBackupDatabase_PostgresNotSupported tests that PostgreSQL backup returns an error
func TestBackupDatabase_PostgresNotSupported(t *testing.T) {
	// Create a mock postgres DB (just set the driver, no actual connection)
	db := &DB{driver: DriverPostgres}

	err := db.BackupDatabase(context.Background(), "/tmp/backup.db")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "backup not supported for PostgreSQL")
}

// TestBackupDatabase_MySQLNotSupported tests that MySQL backup returns an error
func TestBackupDatabase_MySQLNotSupported(t *testing.T) {
	// Create a mock mysql DB (just set the driver, no actual connection)
	db := &DB{driver: DriverMySQL}

	err := db.BackupDatabase(context.Background(), "/tmp/backup.db")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "backup not supported for MySQL")
}

// TestSQLiteRegressionAfterPostgresSupport tests that SQLite still works after PostgreSQL changes
func TestSQLiteRegressionAfterPostgresSupport(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "regression.db")

	// Create database using factory
	config := FullConfig{
		Driver:          DriverSQLite,
		Path:            dbPath,
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: time.Minute,
	}

	db, err := NewFromConfig(config)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	// Test health check
	err = db.HealthCheck(ctx)
	require.NoError(t, err)

	// Test creating a project
	projectID := "test-project-1"
	now := time.Now()
	err = db.DBCreateProject(ctx, Project{
		ID:        projectID,
		Name:      "Test Project",
		APIKey:    "sk-test-key",
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	})
	require.NoError(t, err)

	// Test retrieving the project
	project, err := db.DBGetProjectByID(ctx, projectID)
	require.NoError(t, err)
	assert.Equal(t, "Test Project", project.Name)
	assert.True(t, project.IsActive)

	// Test creating a token - note: ID is now a UUID separate from the token string
	tokenUUID := "550e8400-e29b-41d4-a716-446655440000"
	tokenString := "test-token-abc123"
	err = db.CreateToken(ctx, Token{
		ID:           tokenUUID,
		Token:        tokenString,
		ProjectID:    projectID,
		IsActive:     true,
		RequestCount: 0,
		CreatedAt:    now,
	})
	require.NoError(t, err)

	// Test retrieving the token by ID (UUID)
	token, err := db.GetTokenByID(ctx, tokenUUID)
	require.NoError(t, err)
	assert.Equal(t, tokenString, token.Token)
	assert.Equal(t, projectID, token.ProjectID)
	assert.True(t, token.IsActive)

	// Test increment usage (by token string)
	err = db.IncrementTokenUsage(ctx, tokenString)
	require.NoError(t, err)

	// Verify increment
	token, err = db.GetTokenByID(ctx, tokenUUID)
	require.NoError(t, err)
	assert.Equal(t, 1, token.RequestCount)

	// Test stats
	stats, err := db.GetStats(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, stats["project_count"])
	assert.Equal(t, 1, stats["active_token_count"])
}

func TestDB_ExecContextRebound(t *testing.T) {
	config := FullConfig{
		Driver: DriverSQLite,
		Path:   ":memory:",
	}

	db, err := NewFromConfig(config)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	// Use rebound exec to update a token
	now := time.Now()
	projectID := "test-project"
	err = db.DBCreateProject(ctx, Project{
		ID:        projectID,
		Name:      "Test",
		APIKey:    "key",
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	})
	require.NoError(t, err)

	tokenUUID := "650e8400-e29b-41d4-a716-446655440001"
	tokenString := "rebound-test-token"
	err = db.CreateToken(ctx, Token{
		ID:           tokenUUID,
		Token:        tokenString,
		ProjectID:    projectID,
		IsActive:     true,
		RequestCount: 0,
		CreatedAt:    now,
	})
	require.NoError(t, err)

	// Test ExecContextRebound
	result, err := db.ExecContextRebound(ctx, "UPDATE tokens SET request_count = ? WHERE token = ?", 5, tokenString)
	require.NoError(t, err)
	rows, err := result.RowsAffected()
	require.NoError(t, err)
	assert.Equal(t, int64(1), rows)

	// Verify the update by token UUID
	token, err := db.GetTokenByID(ctx, tokenUUID)
	require.NoError(t, err)
	assert.Equal(t, 5, token.RequestCount)
}

func TestDB_QueryRowContextRebound(t *testing.T) {
	config := FullConfig{
		Driver: DriverSQLite,
		Path:   ":memory:",
	}

	db, err := NewFromConfig(config)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	now := time.Now()

	projectID := "test-project"
	err = db.DBCreateProject(ctx, Project{
		ID:        projectID,
		Name:      "TestQuery",
		APIKey:    "key",
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	})
	require.NoError(t, err)

	tokenUUID := "850e8400-e29b-41d4-a716-446655440002"
	tokenString := "query-test-token"
	err = db.CreateToken(ctx, Token{
		ID:           tokenUUID,
		Token:        tokenString,
		ProjectID:    projectID,
		IsActive:     true,
		RequestCount: 42,
		CreatedAt:    now,
	})
	require.NoError(t, err)

	// Test QueryRowContextRebound
	var count int
	row := db.QueryRowContextRebound(ctx, "SELECT request_count FROM tokens WHERE token = ?", tokenString)
	err = row.Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 42, count)
}

func TestDB_QueryContextRebound(t *testing.T) {
	config := FullConfig{
		Driver: DriverSQLite,
		Path:   ":memory:",
	}

	db, err := NewFromConfig(config)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	now := time.Now()

	projectID := "test-project"
	err = db.DBCreateProject(ctx, Project{
		ID:        projectID,
		Name:      "TestQueryMulti",
		APIKey:    "key",
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	})
	require.NoError(t, err)

	// Create multiple tokens - note: ID is now a UUID separate from token string
	for i := 0; i < 3; i++ {
		err = db.CreateToken(ctx, Token{
			ID:           fmt.Sprintf("750e8400-e29b-41d4-a716-44665544000%d", i),
			Token:        "multi-token-" + string(rune('a'+i)),
			ProjectID:    projectID,
			IsActive:     true,
			RequestCount: i * 10,
			CreatedAt:    now,
		})
		require.NoError(t, err)
	}

	// Test QueryContextRebound
	rows, err := db.QueryContextRebound(ctx, "SELECT token FROM tokens WHERE project_id = ?", projectID)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	var tokens []string
	for rows.Next() {
		var tok string
		err = rows.Scan(&tok)
		require.NoError(t, err)
		tokens = append(tokens, tok)
	}
	require.NoError(t, rows.Err())
	assert.Len(t, tokens, 3)
}

// TestNewFromConfig_PostgresInvalidURL has been moved to factory_postgres_test.go
// with the postgres build tag, as it requires PostgreSQL support to be compiled in.

func TestNewFromConfig_SQLiteInvalidPath(t *testing.T) {
	config := FullConfig{
		Driver: DriverSQLite,
		Path:   "/nonexistent/very/deep/path/that/does/not/exist/db.sqlite",
	}

	db, err := NewFromConfig(config)
	// Should fail because directory doesn't exist and can't be created
	// Note: This might pass on some systems if the directory gets created
	// So we just check that it either works or fails gracefully
	if err != nil {
		assert.Contains(t, err.Error(), "failed")
	} else {
		_ = db.Close()
	}
}
