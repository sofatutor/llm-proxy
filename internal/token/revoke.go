package token

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	// ErrTokenAlreadyRevoked is returned when trying to revoke an already revoked token
	ErrTokenAlreadyRevoked = errors.New("token is already revoked")
)

// RevocationStore defines the interface for token revocation
type RevocationStore interface {
	// RevokeToken disables a token by setting is_active to false
	RevokeToken(ctx context.Context, tokenID string) error

	// DeleteToken completely removes a token from storage
	DeleteToken(ctx context.Context, tokenID string) error

	// RevokeBatchTokens revokes multiple tokens at once
	RevokeBatchTokens(ctx context.Context, tokenIDs []string) (int, error)

	// RevokeProjectTokens revokes all tokens for a project
	RevokeProjectTokens(ctx context.Context, projectID string) (int, error)

	// RevokeExpiredTokens revokes all tokens that have expired
	RevokeExpiredTokens(ctx context.Context) (int, error)
}

// Revoker provides methods for token revocation
type Revoker struct {
	store RevocationStore
}

// NewRevoker creates a new token revoker with the given store
func NewRevoker(store RevocationStore) *Revoker {
	return &Revoker{
		store: store,
	}
}

// RevokeToken soft revokes a token by setting is_active to false
func (r *Revoker) RevokeToken(ctx context.Context, tokenID string) error {
	// Validate token format first
	if err := ValidateTokenFormat(tokenID); err != nil {
		return fmt.Errorf("invalid token format: %w", err)
	}

	// Attempt to revoke the token
	err := r.store.RevokeToken(ctx, tokenID)
	if err != nil {
		if errors.Is(err, ErrTokenNotFound) {
			return fmt.Errorf("cannot revoke: %w", err)
		}
		if errors.Is(err, ErrTokenAlreadyRevoked) {
			return err
		}
		return fmt.Errorf("failed to revoke token: %w", err)
	}

	return nil
}

// DeleteToken completely removes a token from storage (hard delete)
func (r *Revoker) DeleteToken(ctx context.Context, tokenID string) error {
	// Validate token format first
	if err := ValidateTokenFormat(tokenID); err != nil {
		return fmt.Errorf("invalid token format: %w", err)
	}

	// Attempt to delete the token
	err := r.store.DeleteToken(ctx, tokenID)
	if err != nil {
		if errors.Is(err, ErrTokenNotFound) {
			return fmt.Errorf("cannot delete: %w", err)
		}
		return fmt.Errorf("failed to delete token: %w", err)
	}

	return nil
}

// RevokeBatchTokens revokes multiple tokens in a single operation
func (r *Revoker) RevokeBatchTokens(ctx context.Context, tokenIDs []string) (int, error) {
	if len(tokenIDs) == 0 {
		return 0, nil
	}

	// Validate all token formats first
	for _, tokenID := range tokenIDs {
		if err := ValidateTokenFormat(tokenID); err != nil {
			return 0, fmt.Errorf("invalid token format for %s: %w", tokenID, err)
		}
	}

	// Revoke the tokens
	count, err := r.store.RevokeBatchTokens(ctx, tokenIDs)
	if err != nil {
		return 0, fmt.Errorf("failed to revoke tokens in batch: %w", err)
	}

	return count, nil
}

// RevokeProjectTokens revokes all tokens for a project
func (r *Revoker) RevokeProjectTokens(ctx context.Context, projectID string) (int, error) {
	if projectID == "" {
		return 0, errors.New("project ID cannot be empty")
	}

	count, err := r.store.RevokeProjectTokens(ctx, projectID)
	if err != nil {
		return 0, fmt.Errorf("failed to revoke project tokens: %w", err)
	}

	return count, nil
}

// RevokeExpiredTokens revokes all tokens that have expired
func (r *Revoker) RevokeExpiredTokens(ctx context.Context) (int, error) {
	count, err := r.store.RevokeExpiredTokens(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to revoke expired tokens: %w", err)
	}

	return count, nil
}

// AutomaticRevocation sets up periodic revocation of expired tokens
type AutomaticRevocation struct {
	revoker     *Revoker
	interval    time.Duration
	stopChan    chan struct{}
	stoppedChan chan struct{}
}

// NewAutomaticRevocation creates a new automatic token revocation
func NewAutomaticRevocation(revoker *Revoker, interval time.Duration) *AutomaticRevocation {
	return &AutomaticRevocation{
		revoker:     revoker,
		interval:    interval,
		stopChan:    make(chan struct{}),
		stoppedChan: make(chan struct{}),
	}
}

// Start begins the automatic revocation of expired tokens
func (a *AutomaticRevocation) Start() {
	go func() {
		ticker := time.NewTicker(a.interval)
		defer ticker.Stop()
		defer close(a.stoppedChan)

		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				count, err := a.revoker.RevokeExpiredTokens(ctx)
				if err != nil {
					// Log the error, but continue
					fmt.Printf("Failed to automatically revoke expired tokens: %v\n", err)
				} else if count > 0 {
					fmt.Printf("Automatically revoked %d expired tokens\n", count)
				}
				cancel()
			case <-a.stopChan:
				return
			}
		}
	}()
}

// Stop halts the automatic revocation
func (a *AutomaticRevocation) Stop() {
	close(a.stopChan)
	<-a.stoppedChan
}
