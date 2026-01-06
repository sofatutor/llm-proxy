package proxy

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type countingProjectStore struct {
	mu       sync.Mutex
	apiKeyN  int
	activeN  int
	updateN  int
	deleteN  int
	createN  int
	getByIDN int
	listN    int

	apiKey string
	active bool
}

func (s *countingProjectStore) GetAPIKeyForProject(ctx context.Context, projectID string) (string, error) {
	s.mu.Lock()
	s.apiKeyN++
	s.mu.Unlock()
	return s.apiKey, nil
}

func (s *countingProjectStore) GetProjectActive(ctx context.Context, projectID string) (bool, error) {
	s.mu.Lock()
	s.activeN++
	s.mu.Unlock()
	return s.active, nil
}

func (s *countingProjectStore) ListProjects(ctx context.Context) ([]Project, error) {
	s.mu.Lock()
	s.listN++
	s.mu.Unlock()
	return nil, nil
}

func (s *countingProjectStore) CreateProject(ctx context.Context, project Project) error {
	s.mu.Lock()
	s.createN++
	s.mu.Unlock()
	return nil
}

func (s *countingProjectStore) GetProjectByID(ctx context.Context, projectID string) (Project, error) {
	s.mu.Lock()
	s.getByIDN++
	s.mu.Unlock()
	return Project{}, nil
}

func (s *countingProjectStore) UpdateProject(ctx context.Context, project Project) error {
	s.mu.Lock()
	s.updateN++
	s.mu.Unlock()
	return nil
}

func (s *countingProjectStore) DeleteProject(ctx context.Context, projectID string) error {
	s.mu.Lock()
	s.deleteN++
	s.mu.Unlock()
	return nil
}

func TestCachedProjectStore_GetAPIKeyForProject_Caches(t *testing.T) {
	under := &countingProjectStore{apiKey: "sk-test"}
	c := NewCachedProjectStore(under, CachedProjectStoreConfig{TTL: time.Minute, Max: 10})

	ctx := context.Background()
	v1, err := c.GetAPIKeyForProject(ctx, "p1")
	require.NoError(t, err)
	v2, err := c.GetAPIKeyForProject(ctx, "p1")
	require.NoError(t, err)

	require.Equal(t, "sk-test", v1)
	require.Equal(t, "sk-test", v2)

	under.mu.Lock()
	defer under.mu.Unlock()
	require.Equal(t, 1, under.apiKeyN, "expected underlying store to be hit once due to caching")
}

func TestCachedProjectStore_GetAPIKeyForProject_TTLExpires(t *testing.T) {
	under := &countingProjectStore{apiKey: "sk-test"}
	c := NewCachedProjectStore(under, CachedProjectStoreConfig{TTL: 20 * time.Millisecond, Max: 10})

	ctx := context.Background()
	_, _ = c.GetAPIKeyForProject(ctx, "p1")
	time.Sleep(30 * time.Millisecond)
	_, _ = c.GetAPIKeyForProject(ctx, "p1")

	under.mu.Lock()
	defer under.mu.Unlock()
	require.Equal(t, 2, under.apiKeyN, "expected cache expiry to re-hit underlying store")
}

func TestCachedProjectStore_UpdateProject_Invalidate(t *testing.T) {
	under := &countingProjectStore{apiKey: "sk-test"}
	c := NewCachedProjectStore(under, CachedProjectStoreConfig{TTL: time.Minute, Max: 10})

	ctx := context.Background()
	_, _ = c.GetAPIKeyForProject(ctx, "p1")
	require.NoError(t, c.UpdateProject(ctx, Project{ID: "p1"}))
	_, _ = c.GetAPIKeyForProject(ctx, "p1")

	under.mu.Lock()
	defer under.mu.Unlock()
	require.Equal(t, 2, under.apiKeyN, "expected cache to be invalidated on UpdateProject")
	require.Equal(t, 1, under.updateN)
}

func TestCachedProjectStore_DeleteProject_Invalidate(t *testing.T) {
	under := &countingProjectStore{apiKey: "sk-test"}
	c := NewCachedProjectStore(under, CachedProjectStoreConfig{TTL: time.Minute, Max: 10})

	ctx := context.Background()
	_, _ = c.GetAPIKeyForProject(ctx, "p1")
	require.NoError(t, c.DeleteProject(ctx, "p1"))
	_, _ = c.GetAPIKeyForProject(ctx, "p1")

	under.mu.Lock()
	defer under.mu.Unlock()
	require.Equal(t, 2, under.apiKeyN, "expected cache to be invalidated on DeleteProject")
	require.Equal(t, 1, under.deleteN)
}

func TestCachedProjectStore_CreateProject_Invalidate(t *testing.T) {
	under := &countingProjectStore{apiKey: "sk-test"}
	c := NewCachedProjectStore(under, CachedProjectStoreConfig{TTL: time.Minute, Max: 10})

	ctx := context.Background()
	_, _ = c.GetAPIKeyForProject(ctx, "p1")
	require.NoError(t, c.CreateProject(ctx, Project{ID: "p1"}))
	_, _ = c.GetAPIKeyForProject(ctx, "p1")

	under.mu.Lock()
	defer under.mu.Unlock()
	require.Equal(t, 2, under.apiKeyN, "expected cache to be invalidated on CreateProject")
	require.Equal(t, 1, under.createN)
}

func TestCachedProjectStore_GetAPIKeyForProject_LRUEviction(t *testing.T) {
	under := &countingProjectStore{apiKey: "sk-test"}
	c := NewCachedProjectStore(under, CachedProjectStoreConfig{TTL: time.Minute, Max: 2})

	ctx := context.Background()
	_, _ = c.GetAPIKeyForProject(ctx, "p1") // miss
	_, _ = c.GetAPIKeyForProject(ctx, "p2") // miss
	_, _ = c.GetAPIKeyForProject(ctx, "p1") // hit => p1 is MRU, p2 is LRU
	_, _ = c.GetAPIKeyForProject(ctx, "p3") // miss => should evict p2
	_, _ = c.GetAPIKeyForProject(ctx, "p1") // still hit (p1 should not have been evicted by inserting p3)
	_, _ = c.GetAPIKeyForProject(ctx, "p2") // miss if evicted (note: this will reinsert p2 and may evict another key)

	under.mu.Lock()
	defer under.mu.Unlock()
	// Expected underlying calls:
	// p1 (1), p2 (2), p3 (3), p2 again after eviction (4). p1 accesses are hits.
	require.Equal(t, 4, under.apiKeyN, "expected LRU eviction of p2 when inserting p3")
}
