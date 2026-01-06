package proxy

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type countingActiveStore struct {
	mu      sync.Mutex
	activeN int
	active  bool
	err     error
}

func (s *countingActiveStore) GetAPIKeyForProject(ctx context.Context, projectID string) (string, error) {
	return "sk-test", nil
}
func (s *countingActiveStore) GetProjectActive(ctx context.Context, projectID string) (bool, error) {
	s.mu.Lock()
	s.activeN++
	s.mu.Unlock()
	if s.err != nil {
		return false, s.err
	}
	return s.active, nil
}
func (s *countingActiveStore) ListProjects(ctx context.Context) ([]Project, error) { return nil, nil }
func (s *countingActiveStore) CreateProject(ctx context.Context, project Project) error {
	return nil
}
func (s *countingActiveStore) GetProjectByID(ctx context.Context, projectID string) (Project, error) {
	return Project{}, nil
}
func (s *countingActiveStore) UpdateProject(ctx context.Context, project Project) error { return nil }
func (s *countingActiveStore) DeleteProject(ctx context.Context, projectID string) error {
	return nil
}

func TestCachedProjectActiveStore_CachesTrueAndFalse(t *testing.T) {
	ctx := context.Background()

	under := &countingActiveStore{active: true}
	c := NewCachedProjectActiveStore(under, CachedProjectActiveStoreConfig{TTL: time.Minute, Max: 10})

	v1, err := c.GetProjectActive(ctx, "p1")
	require.NoError(t, err)
	v2, err := c.GetProjectActive(ctx, "p1")
	require.NoError(t, err)
	require.True(t, v1)
	require.True(t, v2)

	under.mu.Lock()
	require.Equal(t, 1, under.activeN)
	under.mu.Unlock()

	under.active = false
	v3, err := c.GetProjectActive(ctx, "p2")
	require.NoError(t, err)
	v4, err := c.GetProjectActive(ctx, "p2")
	require.NoError(t, err)
	require.False(t, v3)
	require.False(t, v4)

	under.mu.Lock()
	require.Equal(t, 2, under.activeN) // p1 once + p2 once
	under.mu.Unlock()
}

func TestCachedProjectActiveStore_TTLExpires(t *testing.T) {
	ctx := context.Background()

	under := &countingActiveStore{active: true}
	c := NewCachedProjectActiveStore(under, CachedProjectActiveStoreConfig{TTL: 20 * time.Millisecond, Max: 10})

	_, _ = c.GetProjectActive(ctx, "p1")
	time.Sleep(30 * time.Millisecond)
	_, _ = c.GetProjectActive(ctx, "p1")

	under.mu.Lock()
	defer under.mu.Unlock()
	require.Equal(t, 2, under.activeN)
}

func TestCachedProjectActiveStore_DoesNotCacheErrors(t *testing.T) {
	ctx := context.Background()

	under := &countingActiveStore{err: errors.New("db down")}
	c := NewCachedProjectActiveStore(under, CachedProjectActiveStoreConfig{TTL: time.Minute, Max: 10})

	_, err1 := c.GetProjectActive(ctx, "p1")
	_, err2 := c.GetProjectActive(ctx, "p1")
	require.Error(t, err1)
	require.Error(t, err2)

	under.mu.Lock()
	defer under.mu.Unlock()
	require.Equal(t, 2, under.activeN)
}
