package dispatcher

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/andybalholm/brotli"
	"github.com/google/uuid"
	"github.com/sofatutor/llm-proxy/internal/eventbus"
	"github.com/sofatutor/llm-proxy/internal/eventtransformer"
)

// DefaultEventTransformer provides a basic transformation from eventbus.Event to EventPayload
// Verbose: if true, includes response_headers in metadata
// Use NewDefaultEventTransformer(verbose) to construct
type DefaultEventTransformer struct {
	Verbose bool
}

// NewDefaultEventTransformer creates a transformer with the given verbosity
func NewDefaultEventTransformer(verbose bool) *DefaultEventTransformer {
	return &DefaultEventTransformer{Verbose: verbose}
}

// cleanJSONBinary recursively replaces binary fields in a JSON object with a placeholder
func cleanJSONBinary(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		for k, v2 := range val {
			val[k] = cleanJSONBinary(v2)
		}
		return val
	case []interface{}:
		for i, v2 := range val {
			val[i] = cleanJSONBinary(v2)
		}
		return val
	case string:
		if !utf8.ValidString(val) {
			return "<binary omitted>"
		}
		return val
	case []byte:
		if !utf8.Valid(val) {
			return "<binary omitted>"
		}
		return string(val)
	default:
		return val
	}
}

// safeRawMessageOrBase64 tries to decode data as JSON, decompressing with gzip or brotli if needed, then as UTF-8 string, else returns base64 string
// If Content-Type is JSON, always return cleaned JSON (with binary fields replaced)
func safeRawMessageOrBase64(data []byte, headers map[string][]string) (json.RawMessage, string) {
	if len(data) == 0 {
		return nil, ""
	}
	var js json.RawMessage
	// Check for Content-Encoding
	encoding := ""
	contentType := ""
	if headers != nil {
		if v, ok := headers["Content-Encoding"]; ok && len(v) > 0 {
			encoding = v[0]
		}
		if v, ok := headers["Content-Type"]; ok && len(v) > 0 {
			contentType = v[0]
		}
	}
	// If Content-Type is multipart, return a placeholder
	if strings.Contains(contentType, "multipart") {
		return []byte(strconv.Quote("<multipart response omitted>")), ""
	}
	decompressed := data
	var decompressErr error
	switch encoding {
	case "gzip":
		zr, err := gzip.NewReader(bytes.NewReader(data))
		if err == nil {
			decompressed, decompressErr = io.ReadAll(zr)
			_ = zr.Close()
		} else {
			decompressErr = err
		}
	case "br":
		br := brotli.NewReader(bytes.NewReader(data))
		var err error
		decompressed, err = io.ReadAll(br)
		if err != nil {
			decompressErr = err
		}
	}
	// If Content-Type is JSON, always return cleaned JSON
	if strings.Contains(contentType, "json") {
		var obj interface{}
		if json.Unmarshal(decompressed, &obj) == nil {
			cleaned := cleanJSONBinary(obj)
			if jsBytes, err := json.Marshal(cleaned); err == nil {
				return jsBytes, ""
			}
		}
	}
	if decompressErr == nil && json.Unmarshal(decompressed, &js) == nil {
		return js, ""
	} else if decompressErr != nil {
		fmt.Printf("[transformer] Decompression failed: %v\n", decompressErr)
	} else if json.Unmarshal(decompressed, &js) != nil {
		if strings.Contains(contentType, "json") {
			fmt.Printf("[transformer] JSON unmarshal after decompress failed: %v\nFirst 64 bytes: %x\n", decompressErr, decompressed[:min(64, len(decompressed))])
		}
	}
	// Try direct JSON unmarshal if not already tried
	if decompressErr != nil && json.Unmarshal(data, &js) == nil {
		return js, ""
	} else if decompressErr != nil {
		fmt.Printf("[transformer] JSON unmarshal failed: %v\nFirst 64 bytes: %x\n", decompressErr, data[:min(64, len(data))])
	}
	// If valid UTF-8, try to parse as JSON string or OpenAI event stream
	if utf8.Valid(decompressed) {
		str := string(decompressed)
		trim := strings.TrimSpace(str)
		if (strings.HasPrefix(trim, "{") && strings.HasSuffix(trim, "}")) ||
			(strings.HasPrefix(trim, "[") && strings.HasSuffix(trim, "]")) {
			// Looks like JSON object/array in a string
			if json.Unmarshal([]byte(trim), &js) == nil {
				return js, ""
			}
		}
		// OpenAI streaming or event lines
		if eventtransformer.IsOpenAIStreaming(str) {
			if merged, err := eventtransformer.MergeOpenAIStreamingChunks(str); err == nil {
				if js, err := json.Marshal(merged); err == nil {
					return js, ""
				}
			}
		}
		if strings.Contains(str, "event: ") && strings.Contains(str, "data: ") {
			if merged, err := eventtransformer.MergeThreadStreamingChunks(str); err == nil {
				if js, err := json.Marshal(merged); err == nil {
					return js, ""
				}
			}
		}
		// Fallback: log as JSON string
		quoted := []byte(strconv.Quote(str))
		return quoted, ""
	}
	// For binary data, return a placeholder string instead of base64
	return []byte(strconv.Quote("<binary response omitted>")), ""
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Transform converts an eventbus.Event to an EventPayload
func (t *DefaultEventTransformer) Transform(evt eventbus.Event) (*EventPayload, error) {
	// Skip non-POST requests (like OPTIONS, GET)
	if evt.Method != "POST" {
		return nil, nil
	}

	// Generate a unique run ID for this event
	runID := uuid.New().String()

	// Basic transformation
	payload := &EventPayload{
		Type:      "llm",
		Event:     "start", // For now, all events are considered "start" events
		RunID:     runID,
		Timestamp: time.Now(),
		LogID:     evt.LogID,
		Metadata: map[string]any{
			"method":      evt.Method,
			"path":        evt.Path,
			"status":      evt.Status,
			"duration_ms": evt.Duration.Milliseconds(),
			"request_id":  evt.RequestID,
		},
	}

	// Add request body as input (JSON or base64)
	if len(evt.RequestBody) > 0 {
		if js, b64 := safeRawMessageOrBase64(evt.RequestBody, nil); js != nil {
			payload.Input = js
		} else {
			payload.InputBase64 = b64
		}
	}

	// --- OpenAI-specific output transformation ---
	isOpenAI := strings.HasPrefix(evt.Path, "/v1/completions") ||
		strings.HasPrefix(evt.Path, "/v1/chat/completions") ||
		strings.HasPrefix(evt.Path, "/v1/threads/")

	if isOpenAI && len(evt.ResponseBody) > 0 {
		// Only use OpenAI transformer if response is valid JSON
		if js := json.Valid(evt.ResponseBody); js {
			openaiTransformer := &eventtransformer.OpenAITransformer{}
			parsed, err := openaiTransformer.TransformEvent(map[string]any{
				"response_body": string(evt.ResponseBody),
				"path":          evt.Path,
			})
			if err == nil && parsed != nil {
				if js, err := json.Marshal(parsed); err == nil {
					payload.Output = js
					// Optionally extract token usage if present
					if usage, ok := parsed["usage"].(map[string]any); ok {
						payload.TokensUsage = &TokensUsage{
							Prompt:     int(usage["prompt_tokens"].(float64)),
							Completion: int(usage["completion_tokens"].(float64)),
						}
					}
					return payload, nil
				}
			}
			// If parsing fails, fall through to generic logic
		}
	}

	// Add response body as output (JSON or base64)
	if len(evt.ResponseBody) > 0 {
		if js, b64 := safeRawMessageOrBase64(evt.ResponseBody, evt.ResponseHeaders); js != nil {
			payload.Output = js
		} else {
			payload.OutputBase64 = b64
		}
	}

	// Add response headers to metadata only if Verbose is true
	if t.Verbose && evt.ResponseHeaders != nil {
		headers := make(map[string]any)
		for k, v := range evt.ResponseHeaders {
			if len(v) == 1 {
				headers[k] = v[0]
			} else {
				headers[k] = v
			}
		}
		payload.Metadata["response_headers"] = headers
	}

	return payload, nil
}
