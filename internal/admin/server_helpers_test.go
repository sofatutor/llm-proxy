package admin

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestParsePositiveIntAndQueryHelpers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/test", func(c *gin.Context) {
		// page and size helpers fallback
		p := getPageFromQuery(c, 3)
		if p != 3 {
			t.Fatalf("default page: got %d, want 3", p)
		}
		s := getPageSizeFromQuery(c, 25)
		if s != 25 {
			t.Fatalf("default size: got %d, want 25", s)
		}
		// invalid numbers
		if _, err := parsePositiveInt("notnum"); err == nil {
			t.Fatal("expected error for invalid int")
		}
		c.Status(200)
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)
}

func TestTemplateFuncs(t *testing.T) {
	s := &Server{}
	f := s.templateFuncs()
	// arithmetic
	if f["add"].(func(int, int) int)(2, 3) != 5 {
		t.Fatal("add failed")
	}
	if f["sub"].(func(int, int) int)(5, 2) != 3 {
		t.Fatal("sub failed")
	}
	if f["inc"].(func(int) int)(1) != 2 {
		t.Fatal("inc failed")
	}
	if f["dec"].(func(int) int)(2) != 1 {
		t.Fatal("dec failed")
	}
	seq := f["seq"].(func(int, int) []int)(2, 4)
	if len(seq) != 3 || seq[0] != 2 || seq[2] != 4 {
		t.Fatalf("seq failed: %v", seq)
	}
	// comparisons
	now := time.Now()
	if !f["lt"].(func(any, any) bool)(1, 2) {
		t.Fatal("lt int failed")
	}
	if !f["gt"].(func(any, any) bool)(2, 1) {
		t.Fatal("gt int failed")
	}
	if !f["le"].(func(any, any) bool)(2, 2) {
		t.Fatal("le int failed")
	}
	if !f["ge"].(func(any, any) bool)(2, 2) {
		t.Fatal("ge int failed")
	}
	if !f["lt"].(func(any, any) bool)(now.Add(-time.Second), now) {
		t.Fatal("lt time failed")
	}
	if !f["gt"].(func(any, any) bool)(now.Add(time.Second), now) {
		t.Fatal("gt time failed")
	}
	if !f["le"].(func(any, any) bool)(now, now) {
		t.Fatal("le time failed")
	}
	if !f["ge"].(func(any, any) bool)(now, now) {
		t.Fatal("ge time failed")
	}
	if !f["and"].(func(bool, bool) bool)(true, true) {
		t.Fatal("and failed")
	}
	if f["or"].(func(bool, bool) bool)(false, false) {
		t.Fatal("or failed")
	}
	if f["not"].(func(bool) bool)(true) {
		t.Fatal("not failed")
	}
	// strings helpers
	if !f["contains"].(func(string, string) bool)("hello", "ell") {
		t.Fatal("contains failed")
	}
	if got := f["obfuscateAPIKey"].(func(string) string)("sk-1234567890abcdef"); got == "" {
		t.Fatal("obfuscateAPIKey empty")
	}
	if got := f["obfuscateToken"].(func(string) string)("tok-1234567890"); got == "" {
		t.Fatal("obfuscateToken empty")
	}
}
