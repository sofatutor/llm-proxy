package proxy

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// CacheMetrics holds Prometheus metrics for cache operations.
type CacheMetrics struct {
	Hits    prometheus.Counter
	Misses  prometheus.Counter
	Bypass  prometheus.Counter
	Store   prometheus.Counter
}

// NewCacheMetrics creates and registers Prometheus counters for cache operations.
func NewCacheMetrics() *CacheMetrics {
	return &CacheMetrics{
		Hits: promauto.NewCounter(prometheus.CounterOpts{
			Name: "proxy_cache_hits_total",
			Help: "Total number of cache hits",
		}),
		Misses: promauto.NewCounter(prometheus.CounterOpts{
			Name: "proxy_cache_misses_total",
			Help: "Total number of cache misses",
		}),
		Bypass: promauto.NewCounter(prometheus.CounterOpts{
			Name: "proxy_cache_bypass_total",
			Help: "Total number of cache bypasses",
		}),
		Store: promauto.NewCounter(prometheus.CounterOpts{
			Name: "proxy_cache_store_total",
			Help: "Total number of cache stores",
		}),
	}
}