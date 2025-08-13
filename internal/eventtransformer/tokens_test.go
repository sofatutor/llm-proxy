package eventtransformer

import (
	"strings"
	"testing"
)

func TestCountOpenAITokens(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantMin int
		wantErr bool
	}{
		{"normal", "Hello, world!", 1, false},
		{"empty", "", 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			n, err := CountOpenAITokens(c.input)

			// Handle network connectivity issues gracefully
			if err != nil && isNetworkError(err) {
				t.Skipf("Skipping test due to network connectivity issue: %v", err)
				return
			}

			if (err != nil) != c.wantErr {
				t.Errorf("CountOpenAITokens() error = %v, wantErr %v", err, c.wantErr)
			}
			if err == nil && n < c.wantMin {
				t.Errorf("CountOpenAITokens() = %d, want at least %d", n, c.wantMin)
			}
		})
	}
}

func TestCountOpenAITokensForModel(t *testing.T) {
	cases := []struct {
		name  string
		text  string
		model string
		min   int
	}{
		{"known_model_mapping", "hello world", "gpt-3.5-turbo", 1},
		{"o_family_heuristic", "hello world", "gpt-4o-mini", 1},
		{"omni_family_heuristic", "hello world", "omni-moderation-latest", 1},
		{"default_cl100k_base", "hello world", "totally-unknown-model", 1},
		{"empty_text", "", "gpt-3.5-turbo", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			n, err := CountOpenAITokensForModel(c.text, c.model)
			// This should not require network; if it does, keep parity with other test
			if err != nil && isNetworkError(err) {
				t.Skipf("Skipping due to network error: %v", err)
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if n < c.min {
				t.Fatalf("tokens = %d, want >= %d", n, c.min)
			}
		})
	}
}

// isNetworkError checks if the error is related to network connectivity
func isNetworkError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return containsAny(errStr, []string{
		"dial tcp",
		"server misbehaving",
		"connection refused",
		"network is unreachable",
		"openaipublic.blob.core.windows.net",
	})
}

// containsAny checks if the string contains any of the substrings
func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

// NOTE: tiktoken.EncodingForModel returns error only if model is unknown. We cannot easily mock it without changing the code to allow injection. If needed, this branch is covered by design for all supported models.
