package proxy

import (
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestRequestForcedCacheTTL(t *testing.T) {
	r := &http.Request{Header: http.Header{}}
	if d := requestForcedCacheTTL(r); d != 0 {
		t.Fatalf("expected 0, got %v", d)
	}
	// Not public -> 0
	r.Header.Set("Cache-Control", "max-age=10")
	if d := requestForcedCacheTTL(r); d != 0 {
		t.Fatalf("expected 0 without public, got %v", d)
	}
	// public + s-maxage
	r.Header.Set("Cache-Control", "public, s-maxage=3")
	if d := requestForcedCacheTTL(r); d != 3*time.Second {
		t.Fatalf("expected 3s, got %v", d)
	}
	// public + max-age
	r.Header.Set("Cache-Control", "public, max-age=7")
	if d := requestForcedCacheTTL(r); d != 7*time.Second {
		t.Fatalf("expected 7s, got %v", d)
	}
}

func TestCacheTTLFromHeaders(t *testing.T) {
	res := &http.Response{Header: http.Header{}}
	// no-store wins
	res.Header.Set("Cache-Control", "no-store, max-age=10")
	if ttl := cacheTTLFromHeaders(res, 5*time.Second); ttl != 0 {
		t.Fatalf("expected 0, got %v", ttl)
	}

	// s-maxage preferred
	res.Header.Set("Cache-Control", "public, s-maxage=4, max-age=9")
	if ttl := cacheTTLFromHeaders(res, 0); ttl != 4*time.Second {
		t.Fatalf("expected 4s, got %v", ttl)
	}

	// fallback to max-age
	res.Header.Set("Cache-Control", "public, max-age=6")
	if ttl := cacheTTLFromHeaders(res, 0); ttl != 6*time.Second {
		t.Fatalf("expected 6s, got %v", ttl)
	}

	// default TTL when allowed and no explicit age
	res.Header.Set("Cache-Control", "public")
	if ttl := cacheTTLFromHeaders(res, 2*time.Second); ttl != 2*time.Second {
		t.Fatalf("expected 2s, got %v", ttl)
	}

	// private should not take default TTL
	res.Header.Set("Cache-Control", "private")
	if ttl := cacheTTLFromHeaders(res, 2*time.Second); ttl != 0 {
		t.Fatalf("expected 0 for private, got %v", ttl)
	}
}

func TestCacheKeyFromRequest_VaryAndBodyHash(t *testing.T) {
	// Base GET request
	r1 := &http.Request{Method: http.MethodGet, Header: http.Header{}, URL: &url.URL{Path: "/v1/x", RawQuery: "b=2&a=1"}}
	k1 := cacheKeyFromRequest(r1)

	// Different Accept should change key
	r2 := &http.Request{Method: http.MethodGet, Header: http.Header{"Accept": {"text/plain"}}, URL: &url.URL{Path: "/v1/x", RawQuery: "a=1&b=2"}}
	k2 := cacheKeyFromRequest(r2)
	if k1 == k2 {
		t.Fatalf("expected vary Accept to change key")
	}

	// POST with body hash should change key vs without
	r3 := &http.Request{Method: http.MethodPost, Header: http.Header{"X-Body-Hash": {"abc123"}}, URL: &url.URL{Path: "/v1/x"}}
	r4 := &http.Request{Method: http.MethodPost, Header: http.Header{}, URL: &url.URL{Path: "/v1/x"}}
	if cacheKeyFromRequest(r3) == cacheKeyFromRequest(r4) {
		t.Fatalf("expected X-Body-Hash to affect key")
	}

	// Client-forced TTL on POST adds a ttl component so different TTLs do not collide
	r5 := &http.Request{Method: http.MethodPost, Header: http.Header{"Cache-Control": {"public, max-age=60"}}, URL: &url.URL{Path: "/v1/x"}}
	r6 := &http.Request{Method: http.MethodPost, Header: http.Header{"Cache-Control": {"public, max-age=10"}}, URL: &url.URL{Path: "/v1/x"}}
	if cacheKeyFromRequest(r5) == cacheKeyFromRequest(r6) {
		t.Fatalf("expected cache-forced TTL to influence key for POST")
	}
}

func TestIsResponseCacheable_Policy(t *testing.T) {
	// Helper to build response
	mk := func(status int, cc string, ct string, withAuth bool) *http.Response {
		req := &http.Request{Header: http.Header{}}
		if withAuth {
			req.Header.Set("Authorization", "Bearer x")
		}
		return &http.Response{StatusCode: status, Header: http.Header{"Cache-Control": {cc}, "Content-Type": {ct}}, Request: req}
	}

	// Non-cacheable: no-store
	if isResponseCacheable(mk(200, "no-store", "application/json", false)) {
		t.Fatalf("no-store should not be cacheable")
	}
	// Non-cacheable: private
	if isResponseCacheable(mk(200, "private, max-age=10", "application/json", false)) {
		t.Fatalf("private should not be cacheable")
	}
	// Non-cacheable: SSE
	if isResponseCacheable(mk(200, "public, max-age=10", "text/event-stream", false)) {
		t.Fatalf("SSE should not be cacheable")
	}
	// Non-cacheable: Vary: *
	r := mk(200, "public, max-age=10", "application/json", false)
	r.Header.Set("Vary", "*")
	if isResponseCacheable(r) {
		t.Fatalf("Vary * should not be cacheable")
	}
	// Cacheable status variants
	for _, st := range []int{200, 203, 301, 308, 404, 410} {
		if !isResponseCacheable(mk(st, "public, max-age=1", "application/json", false)) {
			t.Fatalf("status %d should be cacheable with public+max-age", st)
		}
	}
	// With Authorization requires explicit shared cache directives
	if isResponseCacheable(mk(200, "max-age=10", "application/json", true)) {
		t.Fatalf("auth without public/s-maxage should not be cacheable for shared cache")
	}
	if !isResponseCacheable(mk(200, "public, max-age=10", "application/json", true)) {
		t.Fatalf("auth with public should be cacheable")
	}
}

func TestConditionalRequestMatches_Edges(t *testing.T) {
	h := http.Header{}
	// ETag exact
	h.Set("ETag", "\"abc\"")
	r := &http.Request{Header: http.Header{"If-None-Match": {"abc"}}}
	if !conditionalRequestMatches(r, h) {
		t.Fatalf("If-None-Match exact should match")
	}
	// ETag wildcard
	r = &http.Request{Header: http.Header{"If-None-Match": {"*"}}}
	if !conditionalRequestMatches(r, h) {
		t.Fatalf("If-None-Match * should match")
	}
	// IMS valid date: not modified
	lm := time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat)
	ims := time.Now().UTC().Format(http.TimeFormat)
	h.Set("Last-Modified", lm)
	r = &http.Request{Header: http.Header{"If-Modified-Since": {ims}}}
	if !conditionalRequestMatches(r, h) {
		t.Fatalf("IMS newer than LM should be not-modified")
	}
	// IMS parse error or missing LM should yield false
	h.Set("Last-Modified", "bad")
	if conditionalRequestMatches(r, h) {
		t.Fatalf("bad LM parse should not match")
	}
}

func TestCalculateCacheTTL_Paths(t *testing.T) {
	// Response cacheable with s-maxage from response
	res := &http.Response{StatusCode: 200, Header: http.Header{"Cache-Control": {"public, s-maxage=5"}, "Content-Type": {"application/json"}}}
	req := &http.Request{Header: http.Header{}}
	ttl, fromResp := calculateCacheTTL(res, req, 0)
	if ttl != 5*time.Second || !fromResp {
		t.Fatalf("unexpected ttl/fromResp: %v %v", ttl, fromResp)
	}
	// Non-cacheable response despite TTL
	res = &http.Response{StatusCode: 200, Header: http.Header{"Cache-Control": {"private, s-maxage=5"}, "Content-Type": {"application/json"}}}
	ttl, fromResp = calculateCacheTTL(res, req, 0)
	if ttl != 0 || fromResp {
		t.Fatalf("private should not be cacheable")
	}
	// Fallback to client-forced TTL
	req = &http.Request{Header: http.Header{"Cache-Control": {"public, max-age=2"}}}
	res = &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}}
	ttl, fromResp = calculateCacheTTL(res, req, 0)
	if ttl != 2*time.Second || fromResp {
		t.Fatalf("expected forced ttl 2s from request; got %v %v", ttl, fromResp)
	}
}

func TestHasClientCacheOptIn(t *testing.T) {
	// nil request
	if hasClientCacheOptIn(nil) {
		t.Fatalf("nil request should not opt in")
	}
	r := &http.Request{Header: http.Header{}}
	if hasClientCacheOptIn(r) {
		t.Fatalf("empty headers should not opt in")
	}
	// public only is not enough without age values
	r.Header.Set("Cache-Control", "public")
	if hasClientCacheOptIn(r) {
		t.Fatalf("public without age should not opt in")
	}
	// s-maxage>0 opts in
	r.Header.Set("Cache-Control", "public, s-maxage=10")
	if !hasClientCacheOptIn(r) {
		t.Fatalf("expected opt-in via s-maxage")
	}
	// max-age>0 opts in too
	r.Header.Set("Cache-Control", "public, max-age=5")
	if !hasClientCacheOptIn(r) {
		t.Fatalf("expected opt-in via max-age")
	}
}
