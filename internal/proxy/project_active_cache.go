package proxy

import (
	"container/list"
	"context"
	"sync"
	"time"
)

// CachedProjectActiveStore wraps a ProjectStore with an in-memory TTL+LRU cache for GetProjectActive.
//
// Rationale: GetProjectActive is on the hot path when EnforceProjectActive is enabled and can be a DB lookup.
// Caching avoids per-request DB round-trips in steady state.
type CachedProjectActiveStore struct {
	underlying ProjectStore
	cache      *projectActiveCache
}

type CachedProjectActiveStoreConfig struct {
	TTL time.Duration
	Max int
}

func NewCachedProjectActiveStore(underlying ProjectStore, cfg CachedProjectActiveStoreConfig) *CachedProjectActiveStore {
	if cfg.TTL <= 0 {
		cfg.TTL = 5 * time.Second
	}
	if cfg.Max <= 0 {
		cfg.Max = 10000
	}
	return &CachedProjectActiveStore{
		underlying: underlying,
		cache:      newProjectActiveCache(cfg.TTL, cfg.Max),
	}
}

func (s *CachedProjectActiveStore) GetProjectActive(ctx context.Context, projectID string) (bool, error) {
	if projectID != "" {
		if v, ok := s.cache.Get(projectID); ok {
			return v, nil
		}
	}

	active, err := s.underlying.GetProjectActive(ctx, projectID)
	if err != nil {
		return false, err
	}
	if projectID != "" {
		// Cache both true and false to avoid repeated DB lookups for inactive projects.
		// Invalidation happens on Update/Delete/Create; TTL bounds staleness for out-of-band changes.
		s.cache.Set(projectID, active)
	}
	return active, nil
}

func (s *CachedProjectActiveStore) GetAPIKeyForProject(ctx context.Context, projectID string) (string, error) {
	return s.underlying.GetAPIKeyForProject(ctx, projectID)
}

func (s *CachedProjectActiveStore) ListProjects(ctx context.Context) ([]Project, error) {
	return s.underlying.ListProjects(ctx)
}

func (s *CachedProjectActiveStore) CreateProject(ctx context.Context, project Project) error {
	if err := s.underlying.CreateProject(ctx, project); err != nil {
		return err
	}
	if project.ID != "" {
		s.cache.Purge(project.ID)
	}
	return nil
}

func (s *CachedProjectActiveStore) GetProjectByID(ctx context.Context, projectID string) (Project, error) {
	return s.underlying.GetProjectByID(ctx, projectID)
}

func (s *CachedProjectActiveStore) UpdateProject(ctx context.Context, project Project) error {
	if err := s.underlying.UpdateProject(ctx, project); err != nil {
		return err
	}
	if project.ID != "" {
		s.cache.Purge(project.ID)
	}
	return nil
}

func (s *CachedProjectActiveStore) DeleteProject(ctx context.Context, projectID string) error {
	if err := s.underlying.DeleteProject(ctx, projectID); err != nil {
		return err
	}
	if projectID != "" {
		s.cache.Purge(projectID)
	}
	return nil
}

type projectActiveCacheEntry struct {
	key       string
	value     bool
	expiresAt time.Time
	elem      *list.Element
}

type projectActiveCache struct {
	mu  sync.Mutex
	ll  *list.List
	m   map[string]*projectActiveCacheEntry
	ttl time.Duration
	max int
}

func newProjectActiveCache(ttl time.Duration, max int) *projectActiveCache {
	return &projectActiveCache{
		ll:  list.New(),
		m:   make(map[string]*projectActiveCacheEntry, max),
		ttl: ttl,
		max: max,
	}
}

func (c *projectActiveCache) Get(key string) (bool, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	ent := c.m[key]
	if ent == nil {
		return false, false
	}
	if time.Now().After(ent.expiresAt) {
		c.removeLocked(ent)
		return false, false
	}
	c.ll.MoveToFront(ent.elem)
	return ent.value, true
}

func (c *projectActiveCache) Set(key string, value bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ent := c.m[key]; ent != nil {
		ent.value = value
		ent.expiresAt = time.Now().Add(c.ttl)
		c.ll.MoveToFront(ent.elem)
		return
	}

	elem := c.ll.PushFront(key)
	ent := &projectActiveCacheEntry{key: key, value: value, expiresAt: time.Now().Add(c.ttl), elem: elem}
	c.m[key] = ent
	if c.max > 0 && c.ll.Len() > c.max {
		c.evictOldestLocked()
	}
}

func (c *projectActiveCache) Purge(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ent := c.m[key]; ent != nil {
		c.removeLocked(ent)
	}
}

func (c *projectActiveCache) evictOldestLocked() {
	elem := c.ll.Back()
	if elem == nil {
		return
	}
	key, ok := elem.Value.(string)
	if !ok {
		c.ll.Remove(elem)
		return
	}
	if ent := c.m[key]; ent != nil {
		c.removeLocked(ent)
		return
	}
	c.ll.Remove(elem)
}

func (c *projectActiveCache) removeLocked(ent *projectActiveCacheEntry) {
	delete(c.m, ent.key)
	if ent.elem != nil {
		c.ll.Remove(ent.elem)
	}
}
