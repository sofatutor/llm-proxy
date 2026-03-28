package dispatcher

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"log"
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
		log.Printf("[transformer] Decompression failed: %v", decompressErr)
	} else if json.Unmarshal(decompressed, &js) != nil {
		if strings.Contains(contentType, "json") {
			log.Printf("[transformer] JSON unmarshal after decompress failed: %v First 64 bytes: %x", decompressErr, decompressed[:min(64, len(decompressed))])
		}
	}
	// Try direct JSON unmarshal if not already tried
	if decompressErr != nil && json.Unmarshal(data, &js) == nil {
		return js, ""
	} else if decompressErr != nil {
		log.Printf("[transformer] JSON unmarshal failed: %v First 64 bytes: %x", decompressErr, data[:min(64, len(data))])
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
			"duration_ms": durationMilliseconds(evt.Duration),
			"request_id":  evt.RequestID,
		},
	}
	if model := modelFromRequestBody(evt.RequestBody); model != "" {
		payload.Metadata["model"] = model
	}
	if evt.ProjectID != "" {
		payload.Metadata["project_id"] = evt.ProjectID
	}
	if evt.TokenID != "" {
		payload.Metadata["token_id"] = evt.TokenID
	}
	if len(evt.TokenMetadata) > 0 {
		payload.Metadata["token_metadata"] = evt.TokenMetadata
		if userID := firstNonEmpty(evt.TokenMetadata["user_id"], evt.TokenMetadata["app_user_id"]); userID != "" {
			payload.UserID = &userID
		}
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
		strings.HasPrefix(evt.Path, "/v1/responses") ||
		strings.HasPrefix(evt.Path, "/v1/threads/")

	if isOpenAI && len(evt.ResponseBody) > 0 {
		// Only use OpenAI transformer if response is valid JSON
		if js := json.Valid(evt.ResponseBody); js {
			openaiTransformer := &eventtransformer.OpenAITransformer{}
			parsed, err := openaiTransformer.TransformEvent(map[string]any{
				"Method":          evt.Method,
				"Path":            evt.Path,
				"RequestBody":     string(evt.RequestBody),
				"ResponseBody":    string(evt.ResponseBody),
				"ResponseHeaders": headerToAnyMap(evt.ResponseHeaders),
			})
			if err == nil && parsed != nil {
				if js, err := json.Marshal(parsed); err == nil {
					payload.Output = js
					if model := firstNonEmpty(modelFromParsedOpenAIEvent(parsed), metadataStringValue(payload.Metadata, "model")); model != "" {
						payload.Metadata["model"] = model
					}
					payload.TokensUsage = firstTokensUsage(parsed["token_usage"], parsed["usage"])
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

	if payload.TokensUsage == nil {
		payload.TokensUsage = fallbackTokensUsage(evt.RequestBody, evt.ResponseBody, metadataStringValue(payload.Metadata, "model"))
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

func headerToAnyMap(header map[string][]string) map[string]any {
	if len(header) == 0 {
		return nil
	}

	converted := make(map[string]any, len(header))
	for key, values := range header {
		if len(values) == 1 {
			converted[key] = values[0]
			continue
		}

		items := make([]any, len(values))
		for index, value := range values {
			items[index] = value
		}
		converted[key] = items
	}

	return converted
}

func tokensUsageFromUsageMap(usage map[string]any) *TokensUsage {
	prompt, okPrompt := floatToInt(usage["prompt_tokens"])
	completion, okCompletion := floatToInt(usage["completion_tokens"])
	if !okPrompt && !okCompletion {
		return nil
	}

	return &TokensUsage{Prompt: prompt, Completion: completion}
}

func firstTokensUsage(values ...any) *TokensUsage {
	for _, value := range values {
		if usage := tokensUsageFromValue(value); usage != nil {
			return usage
		}
	}

	return nil
}

func tokensUsageFromValue(value any) *TokensUsage {
	switch typed := value.(type) {
	case map[string]any:
		return tokensUsageFromUsageMap(typed)
	case map[string]int:
		return &TokensUsage{
			Prompt:     typed["prompt_tokens"],
			Completion: typed["completion_tokens"],
		}
	case map[string]float64:
		return &TokensUsage{
			Prompt:     int(typed["prompt_tokens"]),
			Completion: int(typed["completion_tokens"]),
		}
	default:
		return nil
	}
}

func modelFromParsedOpenAIEvent(parsed map[string]any) string {
	if model, ok := parsed["model"].(string); ok && model != "" {
		return model
	}

	responseBody, ok := parsed["response_body"].(string)
	if !ok || responseBody == "" || !json.Valid([]byte(responseBody)) {
		return ""
	}

	var responseObject map[string]any
	if err := json.Unmarshal([]byte(responseBody), &responseObject); err != nil {
		return ""
	}

	if model, ok := responseObject["model"].(string); ok {
		return model
	}

	return ""
}

func modelFromRequestBody(requestBody []byte) string {
	if len(requestBody) == 0 || !json.Valid(requestBody) {
		return ""
	}

	var requestObject map[string]any
	if err := json.Unmarshal(requestBody, &requestObject); err != nil {
		return ""
	}

	if model, ok := requestObject["model"].(string); ok {
		return model
	}

	return ""
}

func floatToInt(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case float32:
		return int(typed), true
	default:
		return 0, false
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}

	return ""
}

func durationMilliseconds(duration time.Duration) int64 {
	if duration <= 0 {
		return 0
	}

	if milliseconds := duration.Milliseconds(); milliseconds > 0 {
		return milliseconds
	}

	return 1
}

func metadataStringValue(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}

	value, _ := metadata[key].(string)
	return value
}

func fallbackTokensUsage(requestBody, responseBody []byte, model string) *TokensUsage {
	promptTokens := promptTokensFromRequestBody(requestBody, model)
	completionTokens := completionTokensFromResponseBody(responseBody, model)
	if promptTokens == 0 && completionTokens == 0 {
		return nil
	}

	return &TokensUsage{Prompt: promptTokens, Completion: completionTokens}
}

func promptTokensFromRequestBody(requestBody []byte, model string) int {
	if len(requestBody) == 0 || !json.Valid(requestBody) {
		return 0
	}

	var requestObject map[string]any
	if err := json.Unmarshal(requestBody, &requestObject); err != nil {
		return 0
	}

	promptSource := ""
	if messages, ok := requestObject["messages"]; ok {
		if encodedMessages, err := json.Marshal(messages); err == nil {
			promptSource = string(encodedMessages)
		}
	}
	if instructions, ok := requestObject["instructions"].(string); ok && instructions != "" {
		promptSource += instructions
	}
	if inputSource := promptInputSource(requestObject["input"]); inputSource != "" {
		promptSource += inputSource
	}
	if promptSource == "" {
		return 0
	}

	tokens, err := eventtransformer.CountOpenAITokensForModel(promptSource, model)
	if err != nil {
		return 0
	}

	return tokens
}

func promptInputSource(input any) string {
	switch typed := input.(type) {
	case string:
		return typed
	case nil:
		return ""
	default:
		encodedInput, err := json.Marshal(typed)
		if err != nil {
			return ""
		}
		return string(encodedInput)
	}
}

func completionTokensFromResponseBody(responseBody []byte, model string) int {
	if len(responseBody) == 0 || !json.Valid(responseBody) {
		return 0
	}

	content, ok := assistantReplyContentFromResponseBody(responseBody)
	if !ok || content == "" {
		return 0
	}

	tokens, err := eventtransformer.CountOpenAITokensForModel(content, model)
	if err != nil {
		return 0
	}

	return tokens
}

func assistantReplyContentFromResponseBody(responseBody []byte) (string, bool) {
	var responseObject map[string]any
	if err := json.Unmarshal(responseBody, &responseObject); err != nil {
		return "", false
	}

	if choices, ok := responseObject["choices"].([]any); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]any); ok {
			if message, ok := choice["message"].(map[string]any); ok {
				if content, ok := message["content"].(string); ok {
					return content, true
				}
			}
		}
	}

	return "", false
}
