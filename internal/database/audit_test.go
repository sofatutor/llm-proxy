package database

import (
	"context"
	"testing"
	"time"

	"github.com/sofatutor/llm-proxy/internal/audit"
)

func TestDB_StoreAuditEvent(t *testing.T) {
	// Create test database
	db, err := New(Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	tests := []struct {
		name    string
		event   *audit.Event
		wantErr bool
	}{
		{
			name: "successful event storage",
			event: audit.NewEvent(audit.ActionTokenCreate, audit.ActorManagement, audit.ResultSuccess).
				WithProjectID("test-project").
				WithRequestID("test-request").
				WithClientIP("192.168.1.1").
				WithDetail("test_key", "test_value"),
			wantErr: false,
		},
		{
			name: "event with minimal fields",
			event: audit.NewEvent(audit.ActionProjectList, audit.ActorSystem, audit.ResultFailure).
				WithClientIP("10.0.0.1"),
			wantErr: false,
		},
		{
			name:    "nil event",
			event:   nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := db.StoreAuditEvent(ctx, tt.event)
			if (err != nil) != tt.wantErr {
				t.Errorf("StoreAuditEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.event != nil {
				// Verify the event was stored by listing it
				filters := AuditEventFilters{
					Action: tt.event.Action,
					Limit:  1,
				}
				events, err := db.ListAuditEvents(ctx, filters)
				if err != nil {
					t.Errorf("Failed to list stored audit events: %v", err)
					return
				}

				if len(events) == 0 {
					t.Error("Expected at least one audit event to be stored")
					return
				}

				stored := events[0]
				if stored.Action != tt.event.Action {
					t.Errorf("Stored action = %v, want %v", stored.Action, tt.event.Action)
				}
				if stored.Actor != tt.event.Actor {
					t.Errorf("Stored actor = %v, want %v", stored.Actor, tt.event.Actor)
				}
				if stored.Outcome != string(tt.event.Result) {
					t.Errorf("Stored outcome = %v, want %v", stored.Outcome, string(tt.event.Result))
				}
			}
		})
	}
}

func TestDB_ListAuditEvents(t *testing.T) {
	// Create test database
	db, err := New(Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	// Store test events
	events := []*audit.Event{
		audit.NewEvent(audit.ActionTokenCreate, audit.ActorManagement, audit.ResultSuccess).
			WithProjectID("project-1").
			WithClientIP("192.168.1.1"),
		audit.NewEvent(audit.ActionTokenCreate, audit.ActorManagement, audit.ResultFailure).
			WithProjectID("project-2").
			WithClientIP("192.168.1.2"),
		audit.NewEvent(audit.ActionProjectList, audit.ActorAdmin, audit.ResultSuccess).
			WithClientIP("192.168.1.1"),
	}

	for _, event := range events {
		if err := db.StoreAuditEvent(ctx, event); err != nil {
			t.Fatalf("Failed to store test audit event: %v", err)
		}
	}

	tests := []struct {
		name        string
		filters     AuditEventFilters
		wantCount   int
		wantAction  string
		wantOutcome string
	}{
		{
			name:      "list all events",
			filters:   AuditEventFilters{},
			wantCount: 3,
		},
		{
			name: "filter by action",
			filters: AuditEventFilters{
				Action: audit.ActionTokenCreate,
			},
			wantCount:  2,
			wantAction: audit.ActionTokenCreate,
		},
		{
			name: "filter by client IP",
			filters: AuditEventFilters{
				ClientIP: "192.168.1.1",
			},
			wantCount: 2,
		},
		{
			name: "filter by project ID",
			filters: AuditEventFilters{
				ProjectID: "project-1",
			},
			wantCount: 1,
		},
		{
			name: "filter by outcome",
			filters: AuditEventFilters{
				Outcome: "success",
			},
			wantCount:   2,
			wantOutcome: "success",
		},
		{
			name: "limit results",
			filters: AuditEventFilters{
				Limit: 1,
			},
			wantCount: 1,
		},
		{
			name: "filter by actor",
			filters: AuditEventFilters{
				Actor: audit.ActorManagement,
			},
			wantCount: 2,
		},
		{
			name: "filter by actor admin",
			filters: AuditEventFilters{
				Actor: audit.ActorAdmin,
			},
			wantCount: 1,
		},
		{
			name: "search functionality",
			filters: AuditEventFilters{
				Search: "management",
			},
			wantCount: 2,
		},
		{
			name: "limit with offset",
			filters: AuditEventFilters{
				Limit:  1,
				Offset: 1,
			},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := db.ListAuditEvents(ctx, tt.filters)
			if err != nil {
				t.Errorf("ListAuditEvents() error = %v", err)
				return
			}

			if len(results) != tt.wantCount {
				t.Errorf("ListAuditEvents() returned %d events, want %d", len(results), tt.wantCount)
				return
			}

			// Verify filters worked correctly
			for _, result := range results {
				if tt.wantAction != "" && result.Action != tt.wantAction {
					t.Errorf("Event action = %v, want %v", result.Action, tt.wantAction)
				}
				if tt.wantOutcome != "" && result.Outcome != tt.wantOutcome {
					t.Errorf("Event outcome = %v, want %v", result.Outcome, tt.wantOutcome)
				}
			}
		})
	}
}

func TestDB_CountAuditEvents_and_GetAuditEventByID(t *testing.T) {
	db, err := New(Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	startTimeStr := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	endTimeStr := time.Now().Add(1 * time.Hour).Format(time.RFC3339)

	// Insert multiple events with different fields to exercise search and lookups
	e1 := audit.NewEvent(audit.ActionProjectCreate, audit.ActorManagement, audit.ResultSuccess).
		WithProjectID("p1").WithRequestID("req-123").WithCorrelationID("corr-abc").WithDetail("http_method", "POST").WithDetail("endpoint", "/manage/projects").WithClientIP("198.51.100.1")
	e2 := audit.NewEvent(audit.ActionTokenCreate, audit.ActorAdmin, audit.ResultFailure).
		WithProjectID("p2").WithRequestID("req-456").WithCorrelationID("corr-def").WithDetail("http_method", "POST").WithDetail("endpoint", "/manage/tokens").WithClientIP("198.51.100.2").WithDetail("error", "boom")
	e3 := audit.NewEvent(audit.ActionProjectDelete, audit.ActorSystem, audit.ResultSuccess).
		WithProjectID("p3").WithRequestID("req-789").WithCorrelationID("corr-ghi").WithDetail("http_method", "DELETE").WithDetail("endpoint", "/manage/projects/p3").WithClientIP("198.51.100.3")

	if err := db.StoreAuditEvent(ctx, e1); err != nil {
		t.Fatalf("store e1: %v", err)
	}
	if err := db.StoreAuditEvent(ctx, e2); err != nil {
		t.Fatalf("store e2: %v", err)
	}
	if err := db.StoreAuditEvent(ctx, e3); err != nil {
		t.Fatalf("store e3: %v", err)
	}

	// Count without filters
	n, err := db.CountAuditEvents(ctx, AuditEventFilters{})
	if err != nil || n < 3 {
		t.Fatalf("CountAuditEvents all: n=%d err=%v", n, err)
	}

	// Count with various filters to improve coverage
	tests := []struct {
		name        string
		filter      AuditEventFilters
		minExpected int
	}{
		{"search request_id", AuditEventFilters{Search: "req-123"}, 1},
		{"search path", AuditEventFilters{Search: "/manage/tokens"}, 1},
		{"filter by action", AuditEventFilters{Action: audit.ActionProjectCreate}, 1},
		{"filter by client IP", AuditEventFilters{ClientIP: "198.51.100.1"}, 1},
		{"filter by project ID", AuditEventFilters{ProjectID: "p2"}, 1},
		{"filter by outcome", AuditEventFilters{Outcome: "success"}, 2},
		{"filter by actor", AuditEventFilters{Actor: audit.ActorAdmin}, 1},
		{"filter by request ID", AuditEventFilters{RequestID: "req-456"}, 1},
		{"filter by correlation ID", AuditEventFilters{CorrelationID: "corr-abc"}, 1},
		{"filter by method", AuditEventFilters{Method: "POST"}, 2},
		{"filter by path", AuditEventFilters{Path: "/manage/projects"}, 1},
		{"filter by start time", AuditEventFilters{StartTime: &startTimeStr}, 3},
		{"filter by end time", AuditEventFilters{EndTime: &endTimeStr}, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, err := db.CountAuditEvents(ctx, tt.filter)
			if err != nil {
				t.Fatalf("CountAuditEvents with %s failed: %v", tt.name, err)
			}
			if count < tt.minExpected {
				t.Errorf("CountAuditEvents with %s: expected at least %d, got %d", tt.name, tt.minExpected, count)
			}
		})
	}

	// Verify we can fetch a concrete ID via List then GetAuditEventByID
	items, err := db.ListAuditEvents(ctx, AuditEventFilters{ProjectID: "p1", Limit: 1})
	if err != nil || len(items) == 0 {
		t.Fatalf("ListAuditEvents p1: items=%d err=%v", len(items), err)
	}
	got, err := db.GetAuditEventByID(ctx, items[0].ID)
	if err != nil || got == nil {
		t.Fatalf("GetAuditEventByID: err=%v", err)
	}
}

func TestAuditLogger_DatabaseIntegration(t *testing.T) {
	// Create test database
	db, err := New(Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Create temporary file for audit log
	tmpFile := t.TempDir() + "/audit.log"

	// Create audit logger with database support
	logger, err := audit.NewLogger(audit.LoggerConfig{
		FilePath:       tmpFile,
		CreateDir:      true,
		DatabaseStore:  db,
		EnableDatabase: true,
	})
	if err != nil {
		t.Fatalf("Failed to create audit logger: %v", err)
	}
	defer func() { _ = logger.Close() }()

	// Create test event with client IP
	event := audit.NewEvent(audit.ActionTokenCreate, audit.ActorManagement, audit.ResultSuccess).
		WithProjectID("test-project").
		WithRequestID("test-request").
		WithClientIP("192.168.1.100").
		WithDetail("test_key", "test_value")

	// Log the event
	if err := logger.Log(event); err != nil {
		t.Fatalf("Failed to log audit event: %v", err)
	}

	// Verify event was stored in database
	ctx := context.Background()
	events, err := db.ListAuditEvents(ctx, AuditEventFilters{
		Action: audit.ActionTokenCreate,
		Limit:  1,
	})
	if err != nil {
		t.Fatalf("Failed to list audit events from database: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Expected 1 audit event in database, got %d", len(events))
	}

	stored := events[0]
	if stored.Action != audit.ActionTokenCreate {
		t.Errorf("Stored action = %v, want %v", stored.Action, audit.ActionTokenCreate)
	}
	if stored.ClientIP == nil || *stored.ClientIP != "192.168.1.100" {
		t.Errorf("Stored client IP = %v, want 192.168.1.100", stored.ClientIP)
	}
	if stored.ProjectID == nil || *stored.ProjectID != "test-project" {
		t.Errorf("Stored project ID = %v, want test-project", stored.ProjectID)
	}
}

func TestDB_ListAuditEvents_ClosedDB(t *testing.T) {
	db, err := New(Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	_ = db.Close()

	ctx := context.Background()
	_, err = db.ListAuditEvents(ctx, AuditEventFilters{})
	if err == nil {
		t.Error("Expected error for ListAuditEvents on closed DB")
	}
}

func TestDB_CountAuditEvents_ClosedDB(t *testing.T) {
	db, err := New(Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	_ = db.Close()

	ctx := context.Background()
	_, err = db.CountAuditEvents(ctx, AuditEventFilters{})
	if err == nil {
		t.Error("Expected error for CountAuditEvents on closed DB")
	}
}

func TestDB_GetAuditEventByID_NotFound(t *testing.T) {
	db, err := New(Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	_, err = db.GetAuditEventByID(ctx, "nonexistent-id")
	if err == nil {
		t.Error("Expected error for GetAuditEventByID with non-existent ID")
	}
}

func TestDB_StoreAuditEvent_ClosedDB(t *testing.T) {
	db, err := New(Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	_ = db.Close()

	ctx := context.Background()
	event := audit.NewEvent(audit.ActionTokenCreate, audit.ActorManagement, audit.ResultSuccess)
	err = db.StoreAuditEvent(ctx, event)
	if err == nil {
		t.Error("Expected error for StoreAuditEvent on closed DB")
	}
}
