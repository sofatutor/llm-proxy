package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
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
	INSERT INTO tokens (token, project_id, expires_at, is_active, deactivated_at, request_count, max_requests, created_at, last_used_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := d.db.ExecContext(
		ctx,
		query,
		token.Token,
		token.ProjectID,
		token.ExpiresAt,
		token.IsActive,
		token.DeactivatedAt,
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
	SELECT token, project_id, expires_at, is_active, deactivated_at, request_count, max_requests, created_at, last_used_at
	FROM tokens
	WHERE token = ?
	`

	var token Token
	var expiresAt, lastUsedAt, deactivatedAt sql.NullTime
	var maxRequests sql.NullInt32

	err := d.db.QueryRowContext(ctx, query, tokenID).Scan(
		&token.Token,
		&token.ProjectID,
		&expiresAt,
		&token.IsActive,
		&deactivatedAt,
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
	if deactivatedAt.Valid {
		token.DeactivatedAt = &deactivatedAt.Time
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
	SELECT token, project_id, expires_at, is_active, deactivated_at, request_count, max_requests, created_at, last_used_at
	FROM tokens
	ORDER BY created_at DESC
	`

	return d.queryTokens(ctx, query)
}

// GetTokensByProjectID retrieves all tokens for a project.
func (d *DB) GetTokensByProjectID(ctx context.Context, projectID string) ([]Token, error) {
	query := `
	SELECT token, project_id, expires_at, is_active, deactivated_at, request_count, max_requests, created_at, last_used_at
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
		var expiresAt, lastUsedAt, deactivatedAt sql.NullTime
		var maxRequests sql.NullInt32

		if err := rows.Scan(
			&token.Token,
			&token.ProjectID,
			&expiresAt,
			&token.IsActive,
			&deactivatedAt,
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
		if deactivatedAt.Valid {
			token.DeactivatedAt = &deactivatedAt.Time
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

func (a *DBTokenStoreAdapter) GetTokenByID(ctx context.Context, tokenID string) (token.TokenData, error) {
	dbToken, err := a.db.GetTokenByID(ctx, tokenID)
	if err != nil {
		if errors.Is(err, ErrTokenNotFound) {
			return token.TokenData{}, token.ErrTokenNotFound
		}
		return token.TokenData{}, err
	}
	return ExportTokenData(dbToken), nil
}

func (a *DBTokenStoreAdapter) IncrementTokenUsage(ctx context.Context, tokenID string) error {
	return a.db.IncrementTokenUsage(ctx, tokenID)
}

func (a *DBTokenStoreAdapter) CreateToken(ctx context.Context, td token.TokenData) error {
	dbToken := ImportTokenData(td)
	return a.db.CreateToken(ctx, dbToken)
}

func (a *DBTokenStoreAdapter) ListTokens(ctx context.Context) ([]token.TokenData, error) {
	dbTokens, err := a.db.ListTokens(ctx)
	if err != nil {
		return nil, err
	}
	tokens := make([]token.TokenData, len(dbTokens))
	for i, t := range dbTokens {
		tokens[i] = ExportTokenData(t)
	}
	return tokens, nil
}

func (a *DBTokenStoreAdapter) GetTokensByProjectID(ctx context.Context, projectID string) ([]token.TokenData, error) {
	dbTokens, err := a.db.GetTokensByProjectID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	tokens := make([]token.TokenData, len(dbTokens))
	for i, t := range dbTokens {
		tokens[i] = ExportTokenData(t)
	}
	return tokens, nil
}

// ImportTokenData and ExportTokenData helpers
func ImportTokenData(td token.TokenData) Token {
	return Token{
		Token:         td.Token,
		ProjectID:     td.ProjectID,
		ExpiresAt:     td.ExpiresAt,
		IsActive:      td.IsActive,
		DeactivatedAt: td.DeactivatedAt,
		RequestCount:  td.RequestCount,
		MaxRequests:   td.MaxRequests,
		CreatedAt:     td.CreatedAt,
		LastUsedAt:    td.LastUsedAt,
	}
}

func ExportTokenData(t Token) token.TokenData {
	return token.TokenData{
		Token:         t.Token,
		ProjectID:     t.ProjectID,
		ExpiresAt:     t.ExpiresAt,
		IsActive:      t.IsActive,
		DeactivatedAt: t.DeactivatedAt,
		RequestCount:  t.RequestCount,
		MaxRequests:   t.MaxRequests,
		CreatedAt:     t.CreatedAt,
		LastUsedAt:    t.LastUsedAt,
	}
}

// --- RevocationStore interface implementation ---

// RevokeToken disables a token by setting is_active to false and deactivated_at to current time
func (a *DBTokenStoreAdapter) RevokeToken(ctx context.Context, tokenID string) error {
	if tokenID == "" {
		return token.ErrTokenNotFound
	}
	now := time.Now()
	query := `UPDATE tokens SET is_active = 0, deactivated_at = COALESCE(deactivated_at, ?) WHERE token = ? AND is_active = 1`
	_, err := a.db.db.ExecContext(ctx, query, now, tokenID)
	if err != nil {
		return fmt.Errorf("failed to revoke token: %w", err)
	}
	return nil
}

// DeleteToken completely removes a token from storage
func (a *DBTokenStoreAdapter) DeleteToken(ctx context.Context, tokenID string) error {
	if tokenID == "" {
		return token.ErrTokenNotFound
	}
	result, err := a.db.db.ExecContext(ctx, "DELETE FROM tokens WHERE token = ?", tokenID)
	if err != nil {
		return fmt.Errorf("failed to delete token: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return token.ErrTokenNotFound
	}
	return nil
}

// RevokeBatchTokens revokes multiple tokens at once
func (a *DBTokenStoreAdapter) RevokeBatchTokens(ctx context.Context, tokenIDs []string) (int, error) {
	if len(tokenIDs) == 0 {
		return 0, nil
	}
	now := time.Now()
	placeholders := make([]string, len(tokenIDs))
	args := make([]interface{}, len(tokenIDs)+1)
	args[0] = now
	for i, tokenID := range tokenIDs {
		placeholders[i] = "?"
		args[i+1] = tokenID
	}
	query := fmt.Sprintf(`UPDATE tokens SET is_active = 0, deactivated_at = COALESCE(deactivated_at, ?) WHERE token IN (%s) AND is_active = 1`, strings.Join(placeholders, ","))
	result, err := a.db.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to revoke batch tokens: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}
	return int(rowsAffected), nil
}

// RevokeProjectTokens revokes all tokens for a project
func (a *DBTokenStoreAdapter) RevokeProjectTokens(ctx context.Context, projectID string) (int, error) {
	if projectID == "" {
		return 0, nil
	}
	now := time.Now()
	query := `UPDATE tokens SET is_active = 0, deactivated_at = COALESCE(deactivated_at, ?) WHERE project_id = ? AND is_active = 1`
	result, err := a.db.db.ExecContext(ctx, query, now, projectID)
	if err != nil {
		return 0, fmt.Errorf("failed to revoke project tokens: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}
	return int(rowsAffected), nil
}

// RevokeExpiredTokens revokes all tokens that have expired
func (a *DBTokenStoreAdapter) RevokeExpiredTokens(ctx context.Context) (int, error) {
	now := time.Now()
	query := `UPDATE tokens SET is_active = 0, deactivated_at = COALESCE(deactivated_at, ?) WHERE expires_at IS NOT NULL AND expires_at < ? AND is_active = 1`
	result, err := a.db.db.ExecContext(ctx, query, now, now)
	if err != nil {
		return 0, fmt.Errorf("failed to revoke expired tokens: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}
	return int(rowsAffected), nil
}
