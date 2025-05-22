package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/sofatutor/llm-proxy/internal/token"
)

var (
	// ErrTokenNotFound is returned when a token is not found.
	ErrTokenNotFound = errors.New("token not found")
	// ErrTokenExists is returned when a token already exists.
	ErrTokenExists = errors.New("token already exists")
)

// CreateToken creates a new token in the database.
func (d *DB) CreateToken(ctx context.Context, token Token) error {
	query := `
	INSERT INTO tokens (token, project_id, expires_at, is_active, request_count, max_requests, created_at, last_used_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := d.db.ExecContext(
		ctx,
		query,
		token.Token,
		token.ProjectID,
		token.ExpiresAt,
		token.IsActive,
		token.RequestCount,
		token.MaxRequests,
		token.CreatedAt,
		token.LastUsedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create token: %w", err)
	}

	return nil
}

// GetTokenByID retrieves a token by ID.
func (d *DB) GetTokenByID(ctx context.Context, tokenID string) (Token, error) {
	query := `
	SELECT token, project_id, expires_at, is_active, request_count, max_requests, created_at, last_used_at
	FROM tokens
	WHERE token = ?
	`

	var token Token
	var expiresAt, lastUsedAt sql.NullTime
	var maxRequests sql.NullInt32

	err := d.db.QueryRowContext(ctx, query, tokenID).Scan(
		&token.Token,
		&token.ProjectID,
		&expiresAt,
		&token.IsActive,
		&token.RequestCount,
		&maxRequests,
		&token.CreatedAt,
		&lastUsedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Token{}, ErrTokenNotFound
		}
		return Token{}, fmt.Errorf("failed to get token: %w", err)
	}

	if expiresAt.Valid {
		token.ExpiresAt = &expiresAt.Time
	}
	if lastUsedAt.Valid {
		token.LastUsedAt = &lastUsedAt.Time
	}
	if maxRequests.Valid {
		maxReq := int(maxRequests.Int32)
		token.MaxRequests = &maxReq
	}

	return token, nil
}

// UpdateToken updates a token in the database.
func (d *DB) UpdateToken(ctx context.Context, token Token) error {
	query := `
	UPDATE tokens
	SET project_id = ?, expires_at = ?, is_active = ?, request_count = ?, max_requests = ?, last_used_at = ?
	WHERE token = ?
	`

	result, err := d.db.ExecContext(
		ctx,
		query,
		token.ProjectID,
		token.ExpiresAt,
		token.IsActive,
		token.RequestCount,
		token.MaxRequests,
		token.LastUsedAt,
		token.Token,
	)
	if err != nil {
		return fmt.Errorf("failed to update token: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrTokenNotFound
	}

	return nil
}

// DeleteToken deletes a token from the database.
func (d *DB) DeleteToken(ctx context.Context, tokenID string) error {
	query := `
	DELETE FROM tokens
	WHERE token = ?
	`

	result, err := d.db.ExecContext(ctx, query, tokenID)
	if err != nil {
		return fmt.Errorf("failed to delete token: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrTokenNotFound
	}

	return nil
}

// ListTokens retrieves all tokens from the database.
func (d *DB) ListTokens(ctx context.Context) ([]Token, error) {
	query := `
	SELECT token, project_id, expires_at, is_active, request_count, max_requests, created_at, last_used_at
	FROM tokens
	ORDER BY created_at DESC
	`

	return d.queryTokens(ctx, query)
}

// GetTokensByProjectID retrieves all tokens for a project.
func (d *DB) GetTokensByProjectID(ctx context.Context, projectID string) ([]Token, error) {
	query := `
	SELECT token, project_id, expires_at, is_active, request_count, max_requests, created_at, last_used_at
	FROM tokens
	WHERE project_id = ?
	ORDER BY created_at DESC
	`

	return d.queryTokens(ctx, query, projectID)
}

// IncrementTokenUsage increments the request count and updates the last_used_at timestamp.
func (d *DB) IncrementTokenUsage(ctx context.Context, tokenID string) error {
	now := time.Now()
	query := `
	UPDATE tokens
	SET request_count = request_count + 1, last_used_at = ?
	WHERE token = ?
	`

	result, err := d.db.ExecContext(ctx, query, now, tokenID)
	if err != nil {
		return fmt.Errorf("failed to increment token usage: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrTokenNotFound
	}

	return nil
}

// CleanExpiredTokens deletes expired tokens from the database.
func (d *DB) CleanExpiredTokens(ctx context.Context) (int64, error) {
	now := time.Now()
	query := `
	DELETE FROM tokens
	WHERE expires_at IS NOT NULL AND expires_at < ?
	`

	result, err := d.db.ExecContext(ctx, query, now)
	if err != nil {
		return 0, fmt.Errorf("failed to clean expired tokens: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rowsAffected, nil
}

// queryTokens is a helper function to query tokens.
func (d *DB) queryTokens(ctx context.Context, query string, args ...interface{}) ([]Token, error) {
	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query tokens: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var tokens []Token
	for rows.Next() {
		var token Token
		var expiresAt, lastUsedAt sql.NullTime
		var maxRequests sql.NullInt32

		if err := rows.Scan(
			&token.Token,
			&token.ProjectID,
			&expiresAt,
			&token.IsActive,
			&token.RequestCount,
			&maxRequests,
			&token.CreatedAt,
			&lastUsedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan token: %w", err)
		}

		if expiresAt.Valid {
			token.ExpiresAt = &expiresAt.Time
		}
		if lastUsedAt.Valid {
			token.LastUsedAt = &lastUsedAt.Time
		}
		if maxRequests.Valid {
			maxReq := int(maxRequests.Int32)
			token.MaxRequests = &maxReq
		}

		tokens = append(tokens, token)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tokens: %w", err)
	}

	return tokens, nil
}

// --- token.TokenStore interface adapter for *DB ---
type DBTokenStoreAdapter struct {
	db *DB
}

func NewDBTokenStoreAdapter(db *DB) *DBTokenStoreAdapter {
	return &DBTokenStoreAdapter{db: db}
}

// Stub implementations for build success
func (a *DBTokenStoreAdapter) GetTokenByID(ctx context.Context, tokenID string) (token.TokenData, error) {
	return token.TokenData{}, nil
}
func (a *DBTokenStoreAdapter) IncrementTokenUsage(ctx context.Context, tokenID string) error {
	return nil
}
func (a *DBTokenStoreAdapter) CreateToken(ctx context.Context, td token.TokenData) error {
	return nil
}
func (a *DBTokenStoreAdapter) ListTokens(ctx context.Context) ([]token.TokenData, error) {
	return nil, nil
}
func (a *DBTokenStoreAdapter) GetTokensByProjectID(ctx context.Context, projectID string) ([]token.TokenData, error) {
	return nil, nil
}
