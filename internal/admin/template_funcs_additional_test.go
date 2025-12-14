package admin

import (
	"strings"
	"testing"
	"time"
)

func TestTemplateFuncs_FormatRFC3339UTC(t *testing.T) {
	s := &Server{}
	f := s.templateFuncs()

	formatRFC3339UTC := f["formatRFC3339UTC"].(func(time.Time) string)
	formatRFC3339UTCPtr := f["formatRFC3339UTCPtr"].(func(*time.Time) string)

	// Test non-UTC time gets converted to UTC
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("Failed to load timezone: %v", err)
	}
	nyTime := time.Date(2024, 1, 15, 14, 30, 45, 123456789, loc)
	result := formatRFC3339UTC(nyTime)

	// Should end with Z (UTC marker)
	if !strings.HasSuffix(result, "Z") {
		t.Fatalf("formatRFC3339UTC should return UTC time ending with Z, got: %s", result)
	}

	// Should be parseable as RFC3339
	parsed, err := time.Parse(time.RFC3339Nano, result)
	if err != nil {
		t.Fatalf("formatRFC3339UTC result should be RFC3339 parseable: %v", err)
	}

	// Should represent the same instant in time
	if !parsed.Equal(nyTime) {
		t.Fatalf("formatRFC3339UTC should preserve instant: want %v, got %v", nyTime, parsed)
	}

	// Test nil pointer returns empty string
	if got := formatRFC3339UTCPtr(nil); got != "" {
		t.Fatalf("formatRFC3339UTCPtr with nil should return empty string, got: %q", got)
	}

	// Test non-nil pointer
	result = formatRFC3339UTCPtr(&nyTime)
	if !strings.HasSuffix(result, "Z") {
		t.Fatalf("formatRFC3339UTCPtr should return UTC time ending with Z, got: %s", result)
	}
}

func TestTemplateFuncs_StringOr_And_TimeComparators(t *testing.T) {
	s := &Server{}
	f := s.templateFuncs()

	stringOr := f["stringOr"].(func(any, string) string)
	var p *string
	if got := stringOr(p, "fb"); got != "fb" {
		t.Fatalf("stringOr nil ptr: got %q", got)
	}
	empty := ""
	if got := stringOr(empty, "fb"); got != "fb" {
		t.Fatalf("stringOr empty: got %q", got)
	}
	val := "x"
	if got := stringOr(val, "fb"); got != "x" {
		t.Fatalf("stringOr value: got %q", got)
	}

	now := time.Now()
	past := now.Add(-time.Minute)
	future := now.Add(time.Minute)
	lt := f["lt"].(func(any, any) bool)
	gt := f["gt"].(func(any, any) bool)
	le := f["le"].(func(any, any) bool)
	ge := f["ge"].(func(any, any) bool)

	if !lt(past, now) || lt(now, past) {
		t.Fatalf("lt time failed")
	}
	if !gt(future, now) || gt(now, future) {
		t.Fatalf("gt time failed")
	}
	if !le(now, now) || !le(past, now) || le(future, now) {
		t.Fatalf("le time failed")
	}
	if !ge(now, now) || !ge(future, now) || ge(past, now) {
		t.Fatalf("ge time failed")
	}
}

func TestTemplateFuncs_PageRange_Adjustments(t *testing.T) {
	s := &Server{}
	f := s.templateFuncs()
	pageRange := f["pageRange"].(func(int, int) []int)

	// total fewer than 7 pages -> 1..total
	if got := pageRange(3, 5); len(got) != 5 || got[0] != 1 || got[4] != 5 {
		t.Fatalf("pageRange small total failed: %v", got)
	}

	// current near start with larger total -> shifts end window
	if got := pageRange(1, 20); len(got) != 7 || got[0] != 1 || got[6] != 7 {
		t.Fatalf("pageRange start window failed: %v", got)
	}

	// current near end with larger total -> shifts start window
	if got := pageRange(20, 20); len(got) != 7 || got[0] != 14 || got[6] != 20 {
		t.Fatalf("pageRange end window failed: %v", got)
	}
}
