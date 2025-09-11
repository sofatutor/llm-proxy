package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestVaryHandling_PerResponse(t *testing.T) {
	tests := []struct {
		name            string
		varyHeader      string
		request1Headers map[string]string
		request2Headers map[string]string
		expectSameKey   bool
		description     string
	}{
		{
			name:       "no_vary_header",
			varyHeader: "",
			request1Headers: map[string]string{
				"Accept": "application/json",
			},
			request2Headers: map[string]string{
				"Accept": "text/plain",
			},
			expectSameKey: true,
			description:   "Without Vary header, different Accept values should generate same key",
		},
		{
			name:       "vary_accept",
			varyHeader: "Accept",
			request1Headers: map[string]string{
				"Accept": "application/json",
			},
			request2Headers: map[string]string{
				"Accept": "text/plain",
			},
			expectSameKey: false,
			description:   "With Vary: Accept, different Accept values should generate different keys",
		},
		{
			name:       "vary_accept_same_values",
			varyHeader: "Accept",
			request1Headers: map[string]string{
				"Accept": "application/json",
			},
			request2Headers: map[string]string{
				"Accept": "application/json",
			},
			expectSameKey: true,
			description:   "With Vary: Accept, same Accept values should generate same key",
		},
		{
			name:       "vary_multiple_headers",
			varyHeader: "Accept, Accept-Language",
			request1Headers: map[string]string{
				"Accept":          "application/json",
				"Accept-Language": "en-US",
			},
			request2Headers: map[string]string{
				"Accept":          "application/json",
				"Accept-Language": "de-DE",
			},
			expectSameKey: false,
			description:   "With multiple Vary headers, different language should generate different keys",
		},
		{
			name:       "vary_missing_header",
			varyHeader: "Accept-Language",
			request1Headers: map[string]string{
				"Accept": "application/json",
			},
			request2Headers: map[string]string{
				"Accept":          "application/json",
				"Accept-Language": "en-US",
			},
			expectSameKey: false,
			description:   "With Vary header, missing vs present header should generate different keys",
		},
		{
			name:       "vary_star",
			varyHeader: "*",
			request1Headers: map[string]string{
				"Accept": "application/json",
			},
			request2Headers: map[string]string{
				"Accept": "text/plain",
			},
			expectSameKey: true,
			description:   "Vary: * should be ignored (not cacheable, but keys should be same)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create requests with different headers
			req1 := httptest.NewRequest("GET", "/v1/models", nil)
			for k, v := range tt.request1Headers {
				req1.Header.Set(k, v)
			}

			req2 := httptest.NewRequest("GET", "/v1/models", nil)
			for k, v := range tt.request2Headers {
				req2.Header.Set(k, v)
			}

			// Generate cache keys using per-response Vary
			key1 := cacheKeyFromRequestWithVary(req1, tt.varyHeader)
			key2 := cacheKeyFromRequestWithVary(req2, tt.varyHeader)

			if tt.expectSameKey {
				assert.Equal(t, key1, key2, tt.description)
			} else {
				assert.NotEqual(t, key1, key2, tt.description)
			}
		})
	}
}

func TestVaryHandling_BackwardCompatibility(t *testing.T) {
	// Test that old cache entries (without Vary) still work
	req := httptest.NewRequest("GET", "/v1/models", nil)
	req.Header.Set("Accept", "application/json")

	// Old key generation (conservative approach)
	oldKey := cacheKeyFromRequest(req)

	// New key generation with empty Vary (backward compatibility)
	newKey := cacheKeyFromRequestWithVary(req, "")

	// Keys should be different because the conservative approach includes Accept
	// but empty Vary should not include any headers
	assert.NotEqual(t, oldKey, newKey, "Old and new key generation should produce different keys")

	// However, when no Vary is specified, the new approach should be consistent
	newKey2 := cacheKeyFromRequestWithVary(req, "")
	assert.Equal(t, newKey, newKey2, "New key generation should be consistent for empty Vary")
}

func TestParseVaryHeader(t *testing.T) {
	tests := []struct {
		name     string
		vary     string
		expected []string
	}{
		{
			name:     "empty",
			vary:     "",
			expected: nil,
		},
		{
			name:     "star",
			vary:     "*",
			expected: nil,
		},
		{
			name:     "single_header",
			vary:     "Accept",
			expected: []string{"Accept"},
		},
		{
			name:     "multiple_headers",
			vary:     "Accept, Accept-Language",
			expected: []string{"Accept", "Accept-Language"},
		},
		{
			name:     "multiple_headers_with_spaces",
			vary:     "Accept,Accept-Language, Content-Type",
			expected: []string{"Accept", "Accept-Language", "Content-Type"},
		},
		{
			name:     "case_normalization",
			vary:     "accept, accept-language",
			expected: []string{"Accept", "Accept-Language"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseVaryHeader(tt.vary)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestVaryHandling_CacheValidation(t *testing.T) {
	// Test that cached responses with Vary are validated correctly
	cache := newInMemoryCache()

	// Store a response with Vary: Accept
	req := httptest.NewRequest("GET", "/v1/models", nil)
	req.Header.Set("Accept", "application/json")

	key := cacheKeyFromRequestWithVary(req, "Accept")
	t.Logf("Storage key: %s", key)
	cr := cachedResponse{
		statusCode: 200,
		headers:    http.Header{"Content-Type": []string{"application/json"}},
		body:       []byte(`{"models": []}`),
		vary:       "Accept",
		expiresAt:  time.Now().Add(time.Hour), // Set expiry time
	}
	cache.Set(key, cr)

	// Test 1: Same Accept header should hit
	req1 := httptest.NewRequest("GET", "/v1/models", nil)
	req1.Header.Set("Accept", "application/json")
	key1 := cacheKeyFromRequestWithVary(req1, "Accept")
	t.Logf("Lookup key: %s", key1)
	if cached, ok := cache.Get(key1); ok {
		assert.Equal(t, "Accept", cached.vary)
		assert.Equal(t, 200, cached.statusCode)
	} else {
		t.Error("Expected cache hit for same Accept header")
	}

	// Test 2: Different Accept header should miss
	req2 := httptest.NewRequest("GET", "/v1/models", nil)
	req2.Header.Set("Accept", "text/plain")
	key2 := cacheKeyFromRequestWithVary(req2, "Accept")
	t.Logf("Different Accept key: %s", key2)
	if _, ok := cache.Get(key2); ok {
		t.Error("Expected cache miss for different Accept header")
	}
}
