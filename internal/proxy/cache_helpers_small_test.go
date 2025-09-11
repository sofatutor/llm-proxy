package proxy

import (
    "net/http"
    "testing"
)

func TestWantsRevalidation_Unit(t *testing.T) {
    if wantsRevalidation(nil) {
        t.Fatalf("nil request should not want revalidation")
    }
    r, _ := http.NewRequest(http.MethodGet, "http://x", nil)
    if wantsRevalidation(r) {
        t.Fatalf("empty Cache-Control should not want revalidation")
    }
    r.Header.Set("Cache-Control", "no-cache")
    if !wantsRevalidation(r) {
        t.Fatalf("no-cache should want revalidation")
    }
    r.Header.Set("Cache-Control", "max-age=0")
    if !wantsRevalidation(r) {
        t.Fatalf("max-age=0 should want revalidation")
    }
}

func TestCanServeCachedForRequest_WithAuthorizationRules(t *testing.T) {
    r, _ := http.NewRequest(http.MethodGet, "http://x", nil)
    // No Authorization: always allowed
    if !canServeCachedForRequest(r, http.Header{}) {
        t.Fatalf("expected allowed when no Authorization header")
    }

    // With Authorization requires public or s-maxage>0
    r.Header.Set("Authorization", "Bearer x")
    // private => not allowed
    if canServeCachedForRequest(r, http.Header{"Cache-Control": {"private"}}) {
        t.Fatalf("expected not allowed for private cache with Authorization")
    }
    // public => allowed
    if !canServeCachedForRequest(r, http.Header{"Cache-Control": {"public"}}) {
        t.Fatalf("expected allowed for public cache with Authorization")
    }
    // s-maxage>0 => allowed
    if !canServeCachedForRequest(r, http.Header{"Cache-Control": {"s-maxage=60"}}) {
        t.Fatalf("expected allowed for s-maxage with Authorization")
    }
}


