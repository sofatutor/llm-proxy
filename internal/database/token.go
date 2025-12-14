package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sofatutor/llm-proxy/internal/token"
)

var (
	// ErrTokenNotFound is returned when a token is not found.
	ErrTokenNotFound = errors.New("token not found")
	// ErrTokenExists is returned when a token already exists.
	ErrTokenExists = errors.New("token already exists")
)

// CreateToken creates a new token in the database.
// If token.ID is empty, a UUID will be generated automatically.
func (d *DB) CreateToken(ctx context.Context, token Token) error {
	// Auto-generate ID if not provided
	if token.ID == "" {
		token.ID = uuid.New().String()
	}

	query := `
	INSERT INTO tokens (id, token, project_id, expires_at, is_active, deactivated_at, request_count, max_requests, created_at, last_used_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := d.ExecContextRebound(
		ctx,
		query,
		token.ID,
		token.Token,
		token.ProjectID,
		token.ExpiresAt,
		token.IsActive,
		nil,
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

// GetTokenByID retrieves a token by its UUID.
func (d *DB) GetTokenByID(ctx context.Context, id string) (Token, error) {
	query := `
	SELECT id, token, project_id, expires_at, is_active, deactivated_at, request_count, max_requests, created_at, last_used_at, cache_hit_count
	FROM tokens
	WHERE id = ?
	`

	var token Token
	var expiresAt, lastUsedAt, deactivatedAt sql.NullTime
	var maxRequests sql.NullInt32

	err := d.QueryRowContextRebound(ctx, query, id).Scan(
		&token.ID,
		&token.Token,
		&token.ProjectID,
		&expiresAt,
		&token.IsActive,
		&deactivatedAt,
		&token.RequestCount,
		&maxRequests,
		&token.CreatedAt,
		&lastUsedAt,
		&token.CacheHitCount,
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

// GetTokenByToken retrieves a token by its token string (for authentication).
func (d *DB) GetTokenByToken(ctx context.Context, tokenString string) (Token, error) {
	query := `
	SELECT id, token, project_id, expires_at, is_active, deactivated_at, request_count, max_requests, created_at, last_used_at, cache_hit_count
	FROM tokens
	WHERE token = ?
	`

	var token Token
	var expiresAt, lastUsedAt, deactivatedAt sql.NullTime
	var maxRequests sql.NullInt32

	err := d.QueryRowContextRebound(ctx, query, tokenString).Scan(
		&token.ID,
		&token.Token,
		&token.ProjectID,
		&expiresAt,
		&token.IsActive,
		&deactivatedAt,
		&token.RequestCount,
		&maxRequests,
		&token.CreatedAt,
		&lastUsedAt,
		&token.CacheHitCount,
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
// It looks up the token by ID (UUID). If ID is empty, it falls back to looking up by token string
// for backward compatibility.
func (d *DB) UpdateToken(ctx context.Context, token Token) error {
	if token.ID == "" && token.Token == "" {
		return fmt.Errorf("token ID or token string required for update")
	}

	queryByID := `
	UPDATE tokens
	SET project_id = ?, expires_at = ?, is_active = ?, request_count = ?, max_requests = ?, last_used_at = ?
	WHERE id = ?
	`
	queryByToken := `
	UPDATE tokens
	SET project_id = ?, expires_at = ?, is_active = ?, request_count = ?, max_requests = ?, last_used_at = ?
	WHERE token = ?
	`

	query := queryByID
	lookupValue := token.ID
	if token.ID == "" {
		query = queryByToken
		lookupValue = token.Token
	}

	result, err := d.ExecContextRebound(
		ctx,
		query,
		token.ProjectID,
		token.ExpiresAt,
		token.IsActive,
		token.RequestCount,
		token.MaxRequests,
		token.LastUsedAt,
		lookupValue,
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

	result, err := d.ExecContextRebound(ctx, query, tokenID)
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
	SELECT id, token, project_id, expires_at, is_active, deactivated_at, request_count, max_requests, created_at, last_used_at, cache_hit_count
	FROM tokens
	ORDER BY created_at DESC
	`

	return d.queryTokens(ctx, query)
}

// GetTokensByProjectID retrieves all tokens for a project.
func (d *DB) GetTokensByProjectID(ctx context.Context, projectID string) ([]Token, error) {
	query := `
	SELECT id, token, project_id, expires_at, is_active, deactivated_at, request_count, max_requests, created_at, last_used_at, cache_hit_count
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

	result, err := d.ExecContextRebound(ctx, query, now, tokenID)
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

	result, err := d.ExecContextRebound(ctx, query, now)
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
	rows, err := d.QueryContextRebound(ctx, query, args...)
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
			&token.ID,
			&token.Token,
			&token.ProjectID,
			&expiresAt,
			&token.IsActive,
			&deactivatedAt,
			&token.RequestCount,
			&maxRequests,
			&token.CreatedAt,
			&lastUsedAt,
			&token.CacheHitCount,
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

func (a *DBTokenStoreAdapter) GetTokenByID(ctx context.Context, id string) (token.TokenData, error) {
	dbToken, err := a.db.GetTokenByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrTokenNotFound) {
			return token.TokenData{}, token.ErrTokenNotFound
		}
		return token.TokenData{}, err
	}
	return ExportTokenData(dbToken), nil
}

func (a *DBTokenStoreAdapter) GetTokenByToken(ctx context.Context, tokenString string) (token.TokenData, error) {
	dbToken, err := a.db.GetTokenByToken(ctx, tokenString)
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

func (a *DBTokenStoreAdapter) UpdateToken(ctx context.Context, td token.TokenData) error {
	dbToken := ImportTokenData(td)
	return a.db.UpdateToken(ctx, dbToken)
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
		ID:            td.ID,
		Token:         td.Token,
		ProjectID:     td.ProjectID,
		ExpiresAt:     td.ExpiresAt,
		IsActive:      td.IsActive,
		DeactivatedAt: td.DeactivatedAt,
		RequestCount:  td.RequestCount,
		MaxRequests:   td.MaxRequests,
		CreatedAt:     td.CreatedAt,
		LastUsedAt:    td.LastUsedAt,
		CacheHitCount: td.CacheHitCount,
	}
}

func ExportTokenData(t Token) token.TokenData {
	return token.TokenData{
		ID:            t.ID,
		Token:         t.Token,
		ProjectID:     t.ProjectID,
		ExpiresAt:     t.ExpiresAt,
		IsActive:      t.IsActive,
		DeactivatedAt: t.DeactivatedAt,
		RequestCount:  t.RequestCount,
		MaxRequests:   t.MaxRequests,
		CreatedAt:     t.CreatedAt,
		LastUsedAt:    t.LastUsedAt,
		CacheHitCount: t.CacheHitCount,
	}
}

// --- RevocationStore interface implementation ---

// RevokeToken disables a token by setting is_active to false and deactivated_at to current time
func (a *DBTokenStoreAdapter) RevokeToken(ctx context.Context, tokenID string) error {
	if tokenID == "" {
		return token.ErrTokenNotFound
	}

	now := time.Now()
	query := `UPDATE tokens SET is_active = ?, deactivated_at = COALESCE(deactivated_at, ?) WHERE token = ? AND is_active = ?`
	result, err := a.db.ExecContextRebound(ctx, query, false, now, tokenID, true)
	if err != nil {
		return fmt.Errorf("failed to revoke token: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}

	if rowsAffected == 0 {
		// Check if token exists at all (could be already inactive)
		var exists bool
		checkQuery := `SELECT 1 FROM tokens WHERE token = ? LIMIT 1`
		err = a.db.QueryRowContextRebound(ctx, checkQuery, tokenID).Scan(&exists)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return token.ErrTokenNotFound
			}
			return fmt.Errorf("failed to check token existence: %w", err)
		}
		// Token exists but was already inactive - this is idempotent, no error
	}

	return nil
}

// DeleteToken completely removes a token from storage
func (a *DBTokenStoreAdapter) DeleteToken(ctx context.Context, tokenID string) error {
	if tokenID == "" {
		return token.ErrTokenNotFound
	}
	result, err := a.db.ExecContextRebound(ctx, "DELETE FROM tokens WHERE token = ?", tokenID)
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
	args := make([]interface{}, len(tokenIDs)+3)
	args[0] = false
	args[1] = now
	for i, tokenID := range tokenIDs {
		placeholders[i] = "?"
		args[i+2] = tokenID
	}
	// Append active-state filter parameter
	args[len(args)-1] = true
	query := fmt.Sprintf(`UPDATE tokens SET is_active = ?, deactivated_at = COALESCE(deactivated_at, ?) WHERE token IN (%s) AND is_active = ?`, strings.Join(placeholders, ","))
	result, err := a.db.ExecContextRebound(ctx, query, args...)
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
	query := `UPDATE tokens SET is_active = ?, deactivated_at = COALESCE(deactivated_at, ?) WHERE project_id = ? AND is_active = ?`
	result, err := a.db.ExecContextRebound(ctx, query, false, now, projectID, true)
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
	query := `UPDATE tokens SET is_active = ?, deactivated_at = COALESCE(deactivated_at, ?) WHERE expires_at IS NOT NULL AND expires_at < ? AND is_active = ?`
	result, err := a.db.ExecContextRebound(ctx, query, false, now, now, true)
	if err != nil {
		return 0, fmt.Errorf("failed to revoke expired tokens: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}
	return int(rowsAffected), nil
}

// IncrementCacheHitCount increments the cache_hit_count for a single token.
func (d *DB) IncrementCacheHitCount(ctx context.Context, tokenID string, delta int) error {
	if delta <= 0 {
		return nil
	}
	query := `UPDATE tokens SET cache_hit_count = cache_hit_count + ? WHERE token = ?`
	_, err := d.ExecContextRebound(ctx, query, delta, tokenID)
	if err != nil {
		return fmt.Errorf("failed to increment cache hit count: %w", err)
	}
	return nil
}

// IncrementCacheHitCountBatch increments cache_hit_count for multiple tokens in batch.
// The deltas map has token IDs as keys and increment values as values.
func (d *DB) IncrementCacheHitCountBatch(ctx context.Context, deltas map[string]int) error {
	if len(deltas) == 0 {
		return nil
	}
	// Use a transaction for batch updates
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback() // No-op if already committed
	}()

	query := `UPDATE tokens SET cache_hit_count = cache_hit_count + ? WHERE token = ?`
	stmt, err := tx.PrepareContext(ctx, d.RebindQuery(query))
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer func() {
		_ = stmt.Close()
	}()

	for tokenID, delta := range deltas {
		if delta <= 0 {
			continue
		}
		if _, err := stmt.ExecContext(ctx, delta, tokenID); err != nil {
			return fmt.Errorf("failed to increment cache hit count for token %s: %w", tokenID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}
