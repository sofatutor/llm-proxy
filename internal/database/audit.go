package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/sofatutor/llm-proxy/internal/audit"
)

// AuditStore defines the interface for persisting audit events to database
type AuditStore interface {
	StoreAuditEvent(ctx context.Context, event *audit.Event) error
	ListAuditEvents(ctx context.Context, filters AuditEventFilters) ([]AuditEvent, error)
	CountAuditEvents(ctx context.Context, filters AuditEventFilters) (int, error)
	GetAuditEventByID(ctx context.Context, id string) (*AuditEvent, error)
}

// AuditEventFilters provides filtering options for audit event queries
type AuditEventFilters struct {
	Action        string
	ClientIP      string
	ProjectID     string
	StartTime     *string // RFC3339 format
	EndTime       *string // RFC3339 format
	Outcome       string
	Actor         string
	RequestID     string
	CorrelationID string
	Method        string
	Path          string
	Search        string // Full-text search over reason/metadata
	Limit         int
	Offset        int
}

// StoreAuditEvent persists an audit event to the database
func (d *DB) StoreAuditEvent(ctx context.Context, event *audit.Event) error {
	if d == nil || d.db == nil {
		return fmt.Errorf("database is nil")
	}
	if event == nil {
		return fmt.Errorf("audit event cannot be nil")
	}

	// Generate UUID for the audit event
	id := uuid.New().String()

	// Convert metadata to JSON string if present
	var metadataJSON *string
	if len(event.Details) > 0 {
		metadataBytes, err := json.Marshal(event.Details)
		if err != nil {
			return fmt.Errorf("failed to marshal audit event metadata: %w", err)
		}
		metadataStr := string(metadataBytes)
		metadataJSON = &metadataStr
	}

	// Extract common fields from details for first-class columns
	var method, path, userAgent, reason, tokenID *string
	if event.Details != nil {
		if v, ok := event.Details["http_method"].(string); ok {
			method = &v
		}
		if v, ok := event.Details["endpoint"].(string); ok {
			path = &v
		}
		if v, ok := event.Details["user_agent"].(string); ok {
			userAgent = &v
		}
		if v, ok := event.Details["error"].(string); ok {
			reason = &v
		}
		if v, ok := event.Details["token_id"].(string); ok {
			tokenID = &v
		}
	}

	// Convert optional string fields to pointers
	var projectID, requestID, correlationID, clientIP *string
	if event.ProjectID != "" {
		projectID = &event.ProjectID
	}
	if event.RequestID != "" {
		requestID = &event.RequestID
	}
	if event.CorrelationID != "" {
		correlationID = &event.CorrelationID
	}
	if event.ClientIP != "" {
		clientIP = &event.ClientIP
	}

	query := `
		INSERT INTO audit_events (
			id, timestamp, action, actor, project_id, request_id, correlation_id,
			client_ip, method, path, user_agent, outcome, reason, token_id, metadata
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := d.db.ExecContext(ctx, query,
		id,
		event.Timestamp,
		event.Action,
		event.Actor,
		projectID,
		requestID,
		correlationID,
		clientIP,
		method,
		path,
		userAgent,
		string(event.Result),
		reason,
		tokenID,
		metadataJSON,
	)
	if err != nil {
		return fmt.Errorf("failed to insert audit event: %w", err)
	}

	return nil
}

// ListAuditEvents retrieves audit events from the database with optional filtering
func (d *DB) ListAuditEvents(ctx context.Context, filters AuditEventFilters) ([]AuditEvent, error) {
	query := "SELECT id, timestamp, action, actor, project_id, request_id, correlation_id, client_ip, method, path, user_agent, outcome, reason, token_id, metadata FROM audit_events WHERE 1=1"
	args := []interface{}{}

	// Apply filters
	if filters.Action != "" {
		query += " AND action = ?"
		args = append(args, filters.Action)
	}
	if filters.ClientIP != "" {
		query += " AND client_ip = ?"
		args = append(args, filters.ClientIP)
	}
	if filters.ProjectID != "" {
		query += " AND project_id = ?"
		args = append(args, filters.ProjectID)
	}
	if filters.Outcome != "" {
		query += " AND outcome = ?"
		args = append(args, filters.Outcome)
	}
	if filters.Actor != "" {
		query += " AND actor = ?"
		args = append(args, filters.Actor)
	}
	if filters.RequestID != "" {
		query += " AND request_id = ?"
		args = append(args, filters.RequestID)
	}
	if filters.CorrelationID != "" {
		query += " AND correlation_id = ?"
		args = append(args, filters.CorrelationID)
	}
	if filters.Method != "" {
		query += " AND method = ?"
		args = append(args, filters.Method)
	}
	if filters.Path != "" {
		query += " AND path = ?"
		args = append(args, filters.Path)
	}
	if filters.Search != "" {
		query += " AND (request_id LIKE ? OR correlation_id LIKE ? OR client_ip LIKE ? OR action LIKE ? OR actor LIKE ? OR method LIKE ? OR path LIKE ? OR reason LIKE ? OR metadata LIKE ?)"
		searchPattern := "%" + filters.Search + "%"
		args = append(args, searchPattern, searchPattern, searchPattern, searchPattern, searchPattern, searchPattern, searchPattern, searchPattern, searchPattern)
	}
	if filters.StartTime != nil {
		query += " AND timestamp >= ?"
		args = append(args, *filters.StartTime)
	}
	if filters.EndTime != nil {
		query += " AND timestamp <= ?"
		args = append(args, *filters.EndTime)
	}

	// Order by timestamp descending
	query += " ORDER BY timestamp DESC"

	// Apply limit and offset
	if filters.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filters.Limit)
		if filters.Offset > 0 {
			query += " OFFSET ?"
			args = append(args, filters.Offset)
		}
	}

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit events: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var events []AuditEvent
	for rows.Next() {
		var event AuditEvent
		err := rows.Scan(
			&event.ID,
			&event.Timestamp,
			&event.Action,
			&event.Actor,
			&event.ProjectID,
			&event.RequestID,
			&event.CorrelationID,
			&event.ClientIP,
			&event.Method,
			&event.Path,
			&event.UserAgent,
			&event.Outcome,
			&event.Reason,
			&event.TokenID,
			&event.Metadata,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audit event: %w", err)
		}
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating audit events: %w", err)
	}

	return events, nil
}

// CountAuditEvents returns the total count of audit events matching the given filters
func (d *DB) CountAuditEvents(ctx context.Context, filters AuditEventFilters) (int, error) {
	query := "SELECT COUNT(*) FROM audit_events WHERE 1=1"
	args := []interface{}{}

	// Apply the same filters as ListAuditEvents (excluding limit/offset)
	if filters.Action != "" {
		query += " AND action = ?"
		args = append(args, filters.Action)
	}
	if filters.ClientIP != "" {
		query += " AND client_ip = ?"
		args = append(args, filters.ClientIP)
	}
	if filters.ProjectID != "" {
		query += " AND project_id = ?"
		args = append(args, filters.ProjectID)
	}
	if filters.Outcome != "" {
		query += " AND outcome = ?"
		args = append(args, filters.Outcome)
	}
	if filters.Actor != "" {
		query += " AND actor = ?"
		args = append(args, filters.Actor)
	}
	if filters.RequestID != "" {
		query += " AND request_id = ?"
		args = append(args, filters.RequestID)
	}
	if filters.CorrelationID != "" {
		query += " AND correlation_id = ?"
		args = append(args, filters.CorrelationID)
	}
	if filters.Method != "" {
		query += " AND method = ?"
		args = append(args, filters.Method)
	}
	if filters.Path != "" {
		query += " AND path = ?"
		args = append(args, filters.Path)
	}
	if filters.Search != "" {
		query += " AND (request_id LIKE ? OR correlation_id LIKE ? OR client_ip LIKE ? OR action LIKE ? OR actor LIKE ? OR method LIKE ? OR path LIKE ? OR reason LIKE ? OR metadata LIKE ?)"
		searchPattern := "%" + filters.Search + "%"
		args = append(args, searchPattern, searchPattern, searchPattern, searchPattern, searchPattern, searchPattern, searchPattern, searchPattern, searchPattern)
	}
	if filters.StartTime != nil {
		query += " AND timestamp >= ?"
		args = append(args, *filters.StartTime)
	}
	if filters.EndTime != nil {
		query += " AND timestamp <= ?"
		args = append(args, *filters.EndTime)
	}

	var count int
	err := d.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count audit events: %w", err)
	}

	return count, nil
}

// GetAuditEventByID retrieves a specific audit event by its ID
func (d *DB) GetAuditEventByID(ctx context.Context, id string) (*AuditEvent, error) {
	query := "SELECT id, timestamp, action, actor, project_id, request_id, correlation_id, client_ip, method, path, user_agent, outcome, reason, token_id, metadata FROM audit_events WHERE id = ?"

	row := d.db.QueryRowContext(ctx, query, id)

	var event AuditEvent
	err := row.Scan(
		&event.ID,
		&event.Timestamp,
		&event.Action,
		&event.Actor,
		&event.ProjectID,
		&event.RequestID,
		&event.CorrelationID,
		&event.ClientIP,
		&event.Method,
		&event.Path,
		&event.UserAgent,
		&event.Outcome,
		&event.Reason,
		&event.TokenID,
		&event.Metadata,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("audit event not found")
		}
		return nil, fmt.Errorf("failed to get audit event: %w", err)
	}

	return &event, nil
}
