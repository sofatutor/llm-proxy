package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
)

const fakeMySQLDriverName = "llmproxytest-fake-mysql"

var registerFakeMySQLDriverOnce sync.Once

func registerFakeMySQLDriver(t *testing.T) {
	t.Helper()
	registerFakeMySQLDriverOnce.Do(func() {
		sql.Register(fakeMySQLDriverName, &fakeMySQLDriver{})
	})
}

// fakeMySQLDriver is a tiny driver used only to unit test MySQL-specific branches
// without requiring a real MySQL server in CI.
type fakeMySQLDriver struct{}

func (d *fakeMySQLDriver) Open(name string) (driver.Conn, error) {
	return &fakeMySQLConn{scenario: name}, nil
}

type fakeMySQLConn struct {
	scenario string
}

func (c *fakeMySQLConn) Prepare(query string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}
func (c *fakeMySQLConn) Close() error              { return nil }
func (c *fakeMySQLConn) Begin() (driver.Tx, error) { return nil, errors.New("not implemented") }

func (c *fakeMySQLConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	switch c.scenario {
	case "maintain_ok":
		if strings.HasPrefix(query, "ANALYZE TABLE ") {
			return driver.RowsAffected(0), nil
		}
		return nil, fmt.Errorf("unexpected exec query: %s", query)
	case "maintain_analyze_error":
		if strings.HasPrefix(query, "ANALYZE TABLE ") {
			return nil, errors.New("analyze failed")
		}
		return nil, fmt.Errorf("unexpected exec query: %s", query)
	default:
		return nil, fmt.Errorf("unexpected exec for scenario %q: %s", c.scenario, query)
	}
}

func (c *fakeMySQLConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	switch c.scenario {
	case "maintain_ok":
		if strings.HasPrefix(query, "SELECT table_name FROM information_schema.tables") {
			return &fakeRows{
				cols: []string{"table_name"},
				data: [][]driver.Value{
					{"projects"},
					{"tokens"},
				},
			}, nil
		}
		return nil, fmt.Errorf("unexpected query: %s", query)

	case "maintain_query_error":
		if strings.HasPrefix(query, "SELECT table_name FROM information_schema.tables") {
			return nil, errors.New("boom")
		}
		return nil, fmt.Errorf("unexpected query: %s", query)

	case "maintain_rows_err":
		if strings.HasPrefix(query, "SELECT table_name FROM information_schema.tables") {
			return &fakeRows{
				cols: []string{"table_name"},
				data: [][]driver.Value{
					{"projects"},
				},
				finalErr: errors.New("rows boom"),
			}, nil
		}
		return nil, fmt.Errorf("unexpected query: %s", query)

	case "maintain_scan_error":
		if strings.HasPrefix(query, "SELECT table_name FROM information_schema.tables") {
			return &fakeRows{
				cols: []string{"table_name"},
				data: [][]driver.Value{
					{struct{}{}},
				},
			}, nil
		}
		return nil, fmt.Errorf("unexpected query: %s", query)

	case "maintain_analyze_error":
		if strings.HasPrefix(query, "SELECT table_name FROM information_schema.tables") {
			return &fakeRows{
				cols: []string{"table_name"},
				data: [][]driver.Value{
					{"projects"},
				},
			}, nil
		}
		return nil, fmt.Errorf("unexpected query: %s", query)

	case "getstats_ok":
		switch {
		case strings.HasPrefix(query, "SELECT COALESCE(SUM(data_length + index_length), 0) FROM information_schema.tables"):
			return &fakeRows{cols: []string{"db_size"}, data: [][]driver.Value{{int64(1234)}}}, nil
		case query == "SELECT COUNT(*) FROM projects":
			return &fakeRows{cols: []string{"count"}, data: [][]driver.Value{{int64(0)}}}, nil
		case strings.HasPrefix(query, "SELECT COUNT(*) FROM tokens WHERE is_active = ?"):
			return &fakeRows{cols: []string{"count"}, data: [][]driver.Value{{int64(0)}}}, nil
		case strings.HasPrefix(query, "SELECT COUNT(*) FROM tokens WHERE expires_at IS NOT NULL"):
			return &fakeRows{cols: []string{"count"}, data: [][]driver.Value{{int64(0)}}}, nil
		case query == "SELECT SUM(request_count) FROM tokens":
			// Return NULL to exercise sql.NullInt64 handling.
			return &fakeRows{cols: []string{"sum"}, data: [][]driver.Value{{nil}}}, nil
		default:
			return nil, fmt.Errorf("unexpected query: %s", query)
		}

	case "getstats_total_requests_nonnull":
		switch {
		case strings.HasPrefix(query, "SELECT COALESCE(SUM(data_length + index_length), 0) FROM information_schema.tables"):
			return &fakeRows{cols: []string{"db_size"}, data: [][]driver.Value{{int64(0)}}}, nil
		case query == "SELECT COUNT(*) FROM projects":
			return &fakeRows{cols: []string{"count"}, data: [][]driver.Value{{int64(0)}}}, nil
		case strings.HasPrefix(query, "SELECT COUNT(*) FROM tokens WHERE is_active = ?"):
			return &fakeRows{cols: []string{"count"}, data: [][]driver.Value{{int64(0)}}}, nil
		case strings.HasPrefix(query, "SELECT COUNT(*) FROM tokens WHERE expires_at IS NOT NULL"):
			return &fakeRows{cols: []string{"count"}, data: [][]driver.Value{{int64(0)}}}, nil
		case query == "SELECT SUM(request_count) FROM tokens":
			return &fakeRows{cols: []string{"sum"}, data: [][]driver.Value{{int64(7)}}}, nil
		default:
			return nil, fmt.Errorf("unexpected query: %s", query)
		}

	case "getstats_size_error":
		if strings.HasPrefix(query, "SELECT COALESCE(SUM(data_length + index_length), 0) FROM information_schema.tables") {
			return nil, errors.New("size query failed")
		}
		return nil, fmt.Errorf("unexpected query: %s", query)

	default:
		return nil, fmt.Errorf("unexpected query for scenario %q: %s", c.scenario, query)
	}
}

type fakeRows struct {
	cols     []string
	data     [][]driver.Value
	i        int
	finalErr error
}

func (r *fakeRows) Columns() []string { return r.cols }
func (r *fakeRows) Close() error      { return nil }
func (r *fakeRows) Next(dest []driver.Value) error {
	if r.i >= len(r.data) {
		if r.finalErr != nil {
			return r.finalErr
		}
		return io.EOF
	}
	copy(dest, r.data[r.i])
	r.i++
	return nil
}

func TestMaintainDatabase_MySQLBranch_NoRealDB(t *testing.T) {
	registerFakeMySQLDriver(t)
	sqlDB, err := sql.Open(fakeMySQLDriverName, "maintain_ok")
	if err != nil {
		t.Fatalf("failed to open fake db: %v", err)
	}
	defer func() { _ = sqlDB.Close() }()

	db := &DB{driver: DriverMySQL, db: sqlDB}
	if err := db.MaintainDatabase(context.Background()); err != nil {
		t.Fatalf("MaintainDatabase failed: %v", err)
	}
}

func TestMaintainDatabase_MySQLBranch_TableQueryError(t *testing.T) {
	registerFakeMySQLDriver(t)
	sqlDB, err := sql.Open(fakeMySQLDriverName, "maintain_query_error")
	if err != nil {
		t.Fatalf("failed to open fake db: %v", err)
	}
	defer func() { _ = sqlDB.Close() }()

	db := &DB{driver: DriverMySQL, db: sqlDB}
	err = db.MaintainDatabase(context.Background())
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "failed to query table names") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMaintainDatabase_MySQLBranch_RowsErr(t *testing.T) {
	registerFakeMySQLDriver(t)
	sqlDB, err := sql.Open(fakeMySQLDriverName, "maintain_rows_err")
	if err != nil {
		t.Fatalf("failed to open fake db: %v", err)
	}
	defer func() { _ = sqlDB.Close() }()

	db := &DB{driver: DriverMySQL, db: sqlDB}
	err = db.MaintainDatabase(context.Background())
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "error iterating table names") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMaintainDatabase_MySQLBranch_ScanError(t *testing.T) {
	registerFakeMySQLDriver(t)
	sqlDB, err := sql.Open(fakeMySQLDriverName, "maintain_scan_error")
	if err != nil {
		t.Fatalf("failed to open fake db: %v", err)
	}
	defer func() { _ = sqlDB.Close() }()

	db := &DB{driver: DriverMySQL, db: sqlDB}
	err = db.MaintainDatabase(context.Background())
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "failed to scan table name") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMaintainDatabase_MySQLBranch_AnalyzeError(t *testing.T) {
	registerFakeMySQLDriver(t)
	sqlDB, err := sql.Open(fakeMySQLDriverName, "maintain_analyze_error")
	if err != nil {
		t.Fatalf("failed to open fake db: %v", err)
	}
	defer func() { _ = sqlDB.Close() }()

	db := &DB{driver: DriverMySQL, db: sqlDB}
	err = db.MaintainDatabase(context.Background())
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "failed to analyze table") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetStats_MySQLBranch_NoRealDB(t *testing.T) {
	registerFakeMySQLDriver(t)
	sqlDB, err := sql.Open(fakeMySQLDriverName, "getstats_ok")
	if err != nil {
		t.Fatalf("failed to open fake db: %v", err)
	}
	defer func() { _ = sqlDB.Close() }()

	db := &DB{driver: DriverMySQL, db: sqlDB}
	stats, err := db.GetStats(context.Background())
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}
	if stats["database_size_bytes"].(int64) != 1234 {
		t.Fatalf("expected database_size_bytes=1234, got %v", stats["database_size_bytes"])
	}
	if stats["project_count"].(int) != 0 {
		t.Fatalf("expected project_count=0, got %v", stats["project_count"])
	}
	if stats["total_request_count"].(int64) != 0 {
		t.Fatalf("expected total_request_count=0, got %v", stats["total_request_count"])
	}
}

func TestGetStats_MySQLBranch_TotalRequestsNonNull(t *testing.T) {
	registerFakeMySQLDriver(t)
	sqlDB, err := sql.Open(fakeMySQLDriverName, "getstats_total_requests_nonnull")
	if err != nil {
		t.Fatalf("failed to open fake db: %v", err)
	}
	defer func() { _ = sqlDB.Close() }()

	db := &DB{driver: DriverMySQL, db: sqlDB}
	stats, err := db.GetStats(context.Background())
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}
	if stats["total_request_count"].(int64) != 7 {
		t.Fatalf("expected total_request_count=7, got %v", stats["total_request_count"])
	}
}

func TestGetStats_MySQLBranch_SizeQueryError(t *testing.T) {
	registerFakeMySQLDriver(t)
	sqlDB, err := sql.Open(fakeMySQLDriverName, "getstats_size_error")
	if err != nil {
		t.Fatalf("failed to open fake db: %v", err)
	}
	defer func() { _ = sqlDB.Close() }()

	db := &DB{driver: DriverMySQL, db: sqlDB}
	_, err = db.GetStats(context.Background())
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "failed to get database size") {
		t.Fatalf("unexpected error: %v", err)
	}
}
