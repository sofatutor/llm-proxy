package proxy

import (
	"container/list"
	"context"
	"sync"
	"time"
)

// CachedProjectStore wraps a ProjectStore with an in-memory TTL+LRU cache for GetAPIKeyForProject.
//
// Rationale: GetAPIKeyForProject is on the hot path for cache misses and currently performs a DB query.
// Caching avoids per-request DB round-trips in steady state.
//
// Security note: The API key is stored in memory in plaintext (same as other in-memory config); this does
// not change persistence characteristics and is scoped to the process lifetime.
type CachedProjectStore struct {
	underlying ProjectStore
	cache      *apiKeyCache
}

type CachedProjectStoreConfig struct {
	TTL time.Duration
	Max int
}

func NewCachedProjectStore(underlying ProjectStore, cfg CachedProjectStoreConfig) *CachedProjectStore {
	if cfg.TTL <= 0 {
		cfg.TTL = 30 * time.Second
	}
	if cfg.Max <= 0 {
		cfg.Max = 10000
	}
	return &CachedProjectStore{
		underlying: underlying,
		cache:      newAPIKeyCache(cfg.TTL, cfg.Max),
	}
}

func (s *CachedProjectStore) GetAPIKeyForProject(ctx context.Context, projectID string) (string, error) {
	if projectID != "" {
		if v, ok := s.cache.Get(projectID); ok {
			return v, nil
		}
	}

	apiKey, err := s.underlying.GetAPIKeyForProject(ctx, projectID)
	if err != nil {
		return "", err
	}
	if projectID != "" && apiKey != "" {
		s.cache.Set(projectID, apiKey)
	}
	return apiKey, nil
}

func (s *CachedProjectStore) GetProjectActive(ctx context.Context, projectID string) (bool, error) {
	return s.underlying.GetProjectActive(ctx, projectID)
}

func (s *CachedProjectStore) ListProjects(ctx context.Context) ([]Project, error) {
	return s.underlying.ListProjects(ctx)
}

func (s *CachedProjectStore) CreateProject(ctx context.Context, project Project) error {
	if err := s.underlying.CreateProject(ctx, project); err != nil {
		return err
	}
	if project.ID != "" {
		// Defensive purge: ensures we never serve a stale API key for a re-created project ID
		// (e.g., delete+recreate with same ID, or out-of-band DB changes).
		s.cache.Purge(project.ID)
	}
	return nil
}

func (s *CachedProjectStore) GetProjectByID(ctx context.Context, projectID string) (Project, error) {
	return s.underlying.GetProjectByID(ctx, projectID)
}

func (s *CachedProjectStore) UpdateProject(ctx context.Context, project Project) error {
	if err := s.underlying.UpdateProject(ctx, project); err != nil {
		return err
	}
	if project.ID != "" {
		s.cache.Purge(project.ID)
	}
	return nil
}

func (s *CachedProjectStore) DeleteProject(ctx context.Context, projectID string) error {
	if err := s.underlying.DeleteProject(ctx, projectID); err != nil {
		return err
	}
	if projectID != "" {
		s.cache.Purge(projectID)
	}
	return nil
}

type apiKeyCacheEntry struct {
	key       string
	value     string
	expiresAt time.Time
	elem      *list.Element
}

type apiKeyCache struct {
	mu  sync.Mutex
	ll  *list.List
	m   map[string]*apiKeyCacheEntry
	ttl time.Duration
	max int
}

func newAPIKeyCache(ttl time.Duration, max int) *apiKeyCache {
	return &apiKeyCache{
		ll:  list.New(),
		m:   make(map[string]*apiKeyCacheEntry, max),
		ttl: ttl,
		max: max,
	}
}

func (c *apiKeyCache) Get(key string) (string, bool) {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()

	ent := c.m[key]
	if ent == nil {
		return "", false
	}
	if now.After(ent.expiresAt) {
		c.removeLocked(ent)
		return "", false
	}

	c.ll.MoveToFront(ent.elem)
	return ent.value, true
}

func (c *apiKeyCache) Set(key, value string) {
	if key == "" {
		return
	}
	now := time.Now()
	exp := now.Add(c.ttl)

	c.mu.Lock()
	defer c.mu.Unlock()

	if ent := c.m[key]; ent != nil {
		ent.value = value
		ent.expiresAt = exp
		c.ll.MoveToFront(ent.elem)
		return
	}

	elem := c.ll.PushFront(key)
	ent := &apiKeyCacheEntry{key: key, value: value, expiresAt: exp, elem: elem}
	c.m[key] = ent

	if c.max > 0 && c.ll.Len() > c.max {
		c.evictOldestLocked()
	}
}

func (c *apiKeyCache) Purge(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ent := c.m[key]; ent != nil {
		c.removeLocked(ent)
	}
}

func (c *apiKeyCache) evictOldestLocked() {
	elem := c.ll.Back()
	if elem == nil {
		return
	}
	key, _ := elem.Value.(string)
	ent := c.m[key]
	if ent != nil {
		c.removeLocked(ent)
		return
	}
	// Shouldn't happen, but be defensive.
	c.ll.Remove(elem)
}

func (c *apiKeyCache) removeLocked(ent *apiKeyCacheEntry) {
	delete(c.m, ent.key)
	if ent.elem != nil {
		c.ll.Remove(ent.elem)
	}
}
