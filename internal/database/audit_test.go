package database

import (
	"context"
	"testing"

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
    // Insert multiple events with different fields to exercise search and lookups
    e1 := audit.NewEvent(audit.ActionProjectCreate, audit.ActorManagement, audit.ResultSuccess).
        WithProjectID("p1").WithRequestID("req-123").WithDetail("http_method", "POST").WithDetail("endpoint", "/manage/projects").WithClientIP("198.51.100.1")
    e2 := audit.NewEvent(audit.ActionTokenCreate, audit.ActorManagement, audit.ResultFailure).
        WithProjectID("p2").WithRequestID("req-456").WithDetail("http_method", "POST").WithDetail("endpoint", "/manage/tokens").WithClientIP("198.51.100.2").WithDetail("error", "boom")
    if err := db.StoreAuditEvent(ctx, e1); err != nil { t.Fatalf("store e1: %v", err) }
    if err := db.StoreAuditEvent(ctx, e2); err != nil { t.Fatalf("store e2: %v", err) }

    // Count without filters
    n, err := db.CountAuditEvents(ctx, AuditEventFilters{})
    if err != nil || n < 2 {
        t.Fatalf("CountAuditEvents all: n=%d err=%v", n, err)
    }

    // Count with search matching request_id and path
    n2, err := db.CountAuditEvents(ctx, AuditEventFilters{Search: "req-123"})
    if err != nil || n2 < 1 {
        t.Fatalf("CountAuditEvents search req-123: n=%d err=%v", n2, err)
    }
    n3, err := db.CountAuditEvents(ctx, AuditEventFilters{Search: "/manage/tokens"})
    if err != nil || n3 < 1 {
        t.Fatalf("CountAuditEvents search path: n=%d err=%v", n3, err)
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
