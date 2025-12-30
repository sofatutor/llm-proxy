package proxy

import (
	"os"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewTransparentProxy_RedisCacheClientOptionsAreTuned(t *testing.T) {
	// Save and restore env to avoid leaking state across tests.
	origPool := os.Getenv("REDIS_CACHE_POOL_SIZE")
	origTimeout := os.Getenv("REDIS_CACHE_TIMEOUT")
	defer func() {
		_ = os.Setenv("REDIS_CACHE_POOL_SIZE", origPool)
		_ = os.Setenv("REDIS_CACHE_TIMEOUT", origTimeout)
	}()

	_ = os.Setenv("REDIS_CACHE_POOL_SIZE", "77")
	_ = os.Setenv("REDIS_CACHE_TIMEOUT", "250ms")

	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	cfg := ProxyConfig{
		TargetBaseURL:           "http://example.invalid",
		AllowedMethods:          []string{"GET"},
		HTTPCacheEnabled:        true,
		HTTPCacheDefaultTTL:     5 * time.Second,
		HTTPCacheMaxObjectBytes: 1024,
		RedisCacheURL:           "redis://" + mr.Addr() + "/0",
	}

	p, err := NewTransparentProxyWithLogger(cfg, &MockTokenValidator{}, &MockProjectStore{}, zap.NewNop())
	require.NoError(t, err)

	rc, ok := p.cache.(*redisCache)
	require.True(t, ok, "expected redis cache backend")
	require.Equal(t, 77, rc.client.Options().PoolSize)
	require.Equal(t, 250*time.Millisecond, rc.client.Options().DialTimeout)
	require.Equal(t, 250*time.Millisecond, rc.client.Options().ReadTimeout)
	require.Equal(t, 250*time.Millisecond, rc.client.Options().WriteTimeout)
}
