package proxy

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestNewCacheMetrics(t *testing.T) {
	metrics := NewCacheMetrics()

	// Verify all counters are created
	if metrics.Hits == nil {
		t.Error("Expected Hits counter to be non-nil")
	}
	if metrics.Misses == nil {
		t.Error("Expected Misses counter to be non-nil")
	}
	if metrics.Bypass == nil {
		t.Error("Expected Bypass counter to be non-nil")
	}
	if metrics.Store == nil {
		t.Error("Expected Store counter to be non-nil")
	}
}

func TestCacheMetricsIncrement(t *testing.T) {
	// Create a custom registry for testing to avoid conflicts
	reg := prometheus.NewRegistry()
	metrics := &CacheMetrics{
		Hits: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "test_proxy_cache_hits_total",
			Help: "Test cache hits",
		}),
		Misses: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "test_proxy_cache_misses_total",
			Help: "Test cache misses",
		}),
		Bypass: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "test_proxy_cache_bypass_total",
			Help: "Test cache bypass",
		}),
		Store: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "test_proxy_cache_store_total",
			Help: "Test cache store",
		}),
	}

	// Register metrics
	reg.MustRegister(metrics.Hits, metrics.Misses, metrics.Bypass, metrics.Store)

	tests := []struct {
		name     string
		counter  prometheus.Counter
		expected float64
	}{
		{"hits", metrics.Hits, 1},
		{"misses", metrics.Misses, 1},
		{"bypass", metrics.Bypass, 1},
		{"store", metrics.Store, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Increment the counter
			tt.counter.Inc()

			// Read the metric value
			metric := &dto.Metric{}
			if err := tt.counter.Write(metric); err != nil {
				t.Fatalf("Failed to write metric: %v", err)
			}

			if got := metric.GetCounter().GetValue(); got != tt.expected {
				t.Errorf("Expected counter value %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestCacheMetricsMultipleIncrements(t *testing.T) {
	reg := prometheus.NewRegistry()
	hits := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "test_multiple_hits_total",
		Help: "Test multiple hits",
	})
	reg.MustRegister(hits)

	// Increment multiple times
	for i := 0; i < 5; i++ {
		hits.Inc()
	}

	metric := &dto.Metric{}
	if err := hits.Write(metric); err != nil {
		t.Fatalf("Failed to write metric: %v", err)
	}

	expected := float64(5)
	if got := metric.GetCounter().GetValue(); got != expected {
		t.Errorf("Expected counter value %v, got %v", expected, got)
	}
}