package proxy

import (
	"net/http"
	"testing"
)

// Ensure Chain preserves order and applies all middleware
func TestChain_OrderAndApplication(t *testing.T) {
	var seq []string
	mw1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { seq = append(seq, "mw1"); next.ServeHTTP(w, r) })
	}
	mw2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { seq = append(seq, "mw2"); next.ServeHTTP(w, r) })
	}
	finalCalled := false
	final := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { finalCalled = true; seq = append(seq, "final") })

	h := Chain(final, mw1, mw2)
	h.ServeHTTP(http.ResponseWriter(http.ResponseWriter(nil)), (*http.Request)(nil))
	if !finalCalled {
		t.Fatalf("final handler not called")
	}
	// Chain applies middleware in reverse order of arguments, but execution order
	// is outermost first: mw1 wraps mw2 wraps final → mw1, mw2, final.
	if len(seq) != 3 || seq[0] != "mw1" || seq[1] != "mw2" || seq[2] != "final" {
		t.Fatalf("unexpected sequence: %v", seq)
	}
}
