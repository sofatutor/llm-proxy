package middleware

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// addOpenAIResponseMetadataHeaders extracts a small set of OpenAI response metadata from a JSON
// response body and attaches it to the provided headers as X-OpenAI-*.
//
// This is intentionally tolerant: if fields are missing or have unexpected types, it simply
// skips them rather than returning an error.
func addOpenAIResponseMetadataHeaders(h http.Header, body []byte) {
	if h == nil || len(body) == 0 {
		return
	}

	// Only parse as JSON if Content-Type is application/json and not compressed.
	contentType := h.Get("Content-Type")
	contentEncoding := h.Get("Content-Encoding")
	if !strings.Contains(contentType, "application/json") || (contentEncoding != "" && contentEncoding != "identity") {
		return
	}

	type resp struct {
		Usage   json.RawMessage `json:"usage"`
		Model   json.RawMessage `json:"model"`
		ID      json.RawMessage `json:"id"`
		Created json.RawMessage `json:"created"`
	}
	var r resp
	if err := json.Unmarshal(body, &r); err != nil {
		return
	}

	if len(r.Usage) > 0 {
		var u map[string]json.RawMessage
		if err := json.Unmarshal(r.Usage, &u); err == nil {
			var v int
			if raw := u["prompt_tokens"]; len(raw) > 0 && json.Unmarshal(raw, &v) == nil && v > 0 {
				h.Set("X-OpenAI-Prompt-Tokens", strconv.Itoa(v))
			}
			if raw := u["completion_tokens"]; len(raw) > 0 && json.Unmarshal(raw, &v) == nil && v > 0 {
				h.Set("X-OpenAI-Completion-Tokens", strconv.Itoa(v))
			}
			if raw := u["total_tokens"]; len(raw) > 0 && json.Unmarshal(raw, &v) == nil && v > 0 {
				h.Set("X-OpenAI-Total-Tokens", strconv.Itoa(v))
			}
		}
	}

	if len(r.Model) > 0 {
		var s string
		if err := json.Unmarshal(r.Model, &s); err == nil && s != "" {
			h.Set("X-OpenAI-Model", s)
		}
	}

	if len(r.ID) > 0 {
		var s string
		if err := json.Unmarshal(r.ID, &s); err == nil && s != "" {
			h.Set("X-OpenAI-ID", s)
		}
	}

	if len(r.Created) > 0 {
		var n int64
		if err := json.Unmarshal(r.Created, &n); err == nil && n != 0 {
			h.Set("X-OpenAI-Created", strconv.FormatInt(n, 10))
		}
	}
}
