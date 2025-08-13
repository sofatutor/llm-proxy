package logging

import (
	"strings"
	"testing"

	"go.uber.org/zap/zapcore"
)

func TestTokenID_FieldShapeAndValue(t *testing.T) {
	field := TokenID("sk-1234567890abcdef")
	if field.Key != "token_id" {
		t.Fatalf("field.Key = %q, want %q", field.Key, "token_id")
	}
	if field.Type != zapcore.StringType {
		t.Fatalf("field.Type = %v, want %v", field.Type, zapcore.StringType)
	}
	if field.String == "" {
		t.Fatalf("field.String should not be empty")
	}
	if !strings.Contains(field.String, "...") && len(field.String) > 12 {
		t.Fatalf("expected obfuscated token to contain ellipsis for long inputs, got %q", field.String)
	}
}
