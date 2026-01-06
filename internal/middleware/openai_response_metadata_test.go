package middleware

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAddOpenAIResponseMetadataHeaders_Table(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		encoding    string
		body        string
		want        map[string]string
	}{
		{
			name:        "skips_non_json",
			contentType: "text/plain",
			body:        `{"model":"gpt-4"}`,
			want:        map[string]string{},
		},
		{
			name:        "skips_compressed",
			contentType: "application/json",
			encoding:    "gzip",
			body:        `{"model":"gpt-4"}`,
			want:        map[string]string{},
		},
		{
			name:        "skips_malformed_json",
			contentType: "application/json",
			body:        `{`,
			want:        map[string]string{},
		},
		{
			name:        "model_only",
			contentType: "application/json",
			body:        `{"model":"gpt-4"}`,
			want: map[string]string{
				"X-OpenAI-Model": "gpt-4",
			},
		},
		{
			name:        "usage_zero_values_are_ignored",
			contentType: "application/json",
			body:        `{"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0},"model":"gpt-4"}`,
			want: map[string]string{
				"X-OpenAI-Model": "gpt-4",
			},
		},
		{
			name:        "full_metadata_including_id_created",
			contentType: "application/json",
			body:        `{"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3},"model":"gpt-4","id":"abc","created":123}`,
			want: map[string]string{
				"X-OpenAI-Model":             "gpt-4",
				"X-OpenAI-Prompt-Tokens":     "1",
				"X-OpenAI-Completion-Tokens": "2",
				"X-OpenAI-Total-Tokens":      "3",
				"X-OpenAI-ID":                "abc",
				"X-OpenAI-Created":           "123",
			},
		},
		{
			name:        "unexpected_types_are_tolerated",
			contentType: "application/json",
			body:        `{"usage":{"prompt_tokens":"nope"},"model":123}`,
			want:        map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := make(http.Header)
			if tt.contentType != "" {
				h.Set("Content-Type", tt.contentType)
			}
			if tt.encoding != "" {
				h.Set("Content-Encoding", tt.encoding)
			}

			addOpenAIResponseMetadataHeaders(h, []byte(tt.body))
			for k, v := range tt.want {
				require.Equal(t, v, h.Get(k))
			}

			// Ensure we don't set any of the known headers when not expected.
			if len(tt.want) == 0 {
				require.Empty(t, h.Get("X-OpenAI-Model"))
				require.Empty(t, h.Get("X-OpenAI-Prompt-Tokens"))
				require.Empty(t, h.Get("X-OpenAI-Completion-Tokens"))
				require.Empty(t, h.Get("X-OpenAI-Total-Tokens"))
				require.Empty(t, h.Get("X-OpenAI-ID"))
				require.Empty(t, h.Get("X-OpenAI-Created"))
			}
		})
	}
}
