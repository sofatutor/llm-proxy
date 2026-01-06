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
	createN int
	updateN int
	deleteN int
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
	s.mu.Lock()
	s.createN++
	s.mu.Unlock()
	return nil
}
func (s *countingActiveStore) GetProjectByID(ctx context.Context, projectID string) (Project, error) {
	return Project{}, nil
}
func (s *countingActiveStore) UpdateProject(ctx context.Context, project Project) error {
	s.mu.Lock()
	s.updateN++
	s.mu.Unlock()
	return nil
}
func (s *countingActiveStore) DeleteProject(ctx context.Context, projectID string) error {
	s.mu.Lock()
	s.deleteN++
	s.mu.Unlock()
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

func TestCachedProjectActiveStore_EmptyProjectID_IsNotCached(t *testing.T) {
	ctx := context.Background()

	under := &countingActiveStore{active: true}
	c := NewCachedProjectActiveStore(under, CachedProjectActiveStoreConfig{TTL: time.Minute, Max: 10})

	_, err1 := c.GetProjectActive(ctx, "")
	_, err2 := c.GetProjectActive(ctx, "")
	require.NoError(t, err1)
	require.NoError(t, err2)

	under.mu.Lock()
	defer under.mu.Unlock()
	require.Equal(t, 2, under.activeN, "empty project ID should not be cached")
}

func TestCachedProjectActiveStore_UpdateProject_Invalidate(t *testing.T) {
	ctx := context.Background()

	under := &countingActiveStore{active: true}
	c := NewCachedProjectActiveStore(under, CachedProjectActiveStoreConfig{TTL: time.Minute, Max: 10})

	_, _ = c.GetProjectActive(ctx, "p1")
	require.NoError(t, c.UpdateProject(ctx, Project{ID: "p1"}))
	_, _ = c.GetProjectActive(ctx, "p1")

	under.mu.Lock()
	defer under.mu.Unlock()
	require.Equal(t, 2, under.activeN, "expected cache to be invalidated on UpdateProject")
	require.Equal(t, 1, under.updateN)
}

func TestCachedProjectActiveStore_DeleteProject_Invalidate(t *testing.T) {
	ctx := context.Background()

	under := &countingActiveStore{active: true}
	c := NewCachedProjectActiveStore(under, CachedProjectActiveStoreConfig{TTL: time.Minute, Max: 10})

	_, _ = c.GetProjectActive(ctx, "p1")
	require.NoError(t, c.DeleteProject(ctx, "p1"))
	_, _ = c.GetProjectActive(ctx, "p1")

	under.mu.Lock()
	defer under.mu.Unlock()
	require.Equal(t, 2, under.activeN, "expected cache to be invalidated on DeleteProject")
	require.Equal(t, 1, under.deleteN)
}

func TestCachedProjectActiveStore_CreateProject_Invalidate(t *testing.T) {
	ctx := context.Background()

	under := &countingActiveStore{active: true}
	c := NewCachedProjectActiveStore(under, CachedProjectActiveStoreConfig{TTL: time.Minute, Max: 10})

	_, _ = c.GetProjectActive(ctx, "p1")
	require.NoError(t, c.CreateProject(ctx, Project{ID: "p1"}))
	_, _ = c.GetProjectActive(ctx, "p1")

	under.mu.Lock()
	defer under.mu.Unlock()
	require.Equal(t, 2, under.activeN, "expected cache to be invalidated on CreateProject")
	require.Equal(t, 1, under.createN)
}

func TestCachedProjectActiveStore_GetProjectActive_LRUEviction(t *testing.T) {
	ctx := context.Background()

	under := &countingActiveStore{active: true}
	c := NewCachedProjectActiveStore(under, CachedProjectActiveStoreConfig{TTL: time.Minute, Max: 2})

	_, _ = c.GetProjectActive(ctx, "p1") // miss
	_, _ = c.GetProjectActive(ctx, "p2") // miss
	_, _ = c.GetProjectActive(ctx, "p1") // hit => p1 MRU, p2 LRU
	_, _ = c.GetProjectActive(ctx, "p3") // miss => should evict p2
	_, _ = c.GetProjectActive(ctx, "p1") // hit
	_, _ = c.GetProjectActive(ctx, "p2") // miss after eviction

	under.mu.Lock()
	defer under.mu.Unlock()
	// Underlying calls: p1 (1), p2 (2), p3 (3), p2 again (4). p1 hits.
	require.Equal(t, 4, under.activeN, "expected LRU eviction of p2 when inserting p3")
}

func TestCachedProjectActiveStore_ConcurrentAccess(t *testing.T) {
	under := &countingActiveStore{active: true}
	c := NewCachedProjectActiveStore(under, CachedProjectActiveStoreConfig{TTL: time.Minute, Max: 50})

	ctx := context.Background()
	const goroutines = 50
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(idx int) {
			defer wg.Done()
			pid := string(rune('a' + (idx % 10)))
			for i := 0; i < iterations; i++ {
				v, err := c.GetProjectActive(ctx, pid)
				require.NoError(t, err)
				require.True(t, v)
				if i%10 == 0 {
					_ = c.UpdateProject(ctx, Project{ID: pid})
				}
				if i%25 == 0 {
					_ = c.DeleteProject(ctx, pid)
				}
				if i%40 == 0 {
					_ = c.CreateProject(ctx, Project{ID: pid})
				}
			}
		}(g)
	}
	wg.Wait()

	// Sanity: caching should reduce underlying lookups below raw request count.
	totalCalls := goroutines * iterations
	under.mu.Lock()
	defer under.mu.Unlock()
	require.Greater(t, under.activeN, 0)
	require.Less(t, under.activeN, totalCalls)
}
