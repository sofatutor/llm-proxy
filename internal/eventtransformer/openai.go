package eventtransformer

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

// IsOpenAIStreaming detects if the response body is a sequence of OpenAI streaming chunks (data: ... lines).
func IsOpenAIStreaming(body string) bool {
	lines := strings.Split(body, "\n")
	count := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") && !strings.HasPrefix(line, "data: [DONE]") {
			count++
		}
	}
	return count > 1 // at least two chunks
}

// MergeOpenAIStreamingChunks parses and merges OpenAI streaming chunks into a single response object.
func MergeOpenAIStreamingChunks(body string) (map[string]any, error) {
	type Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	}
	var (
		id      string
		object  string
		created int
		model   string
		content strings.Builder
		finish  string
		usage   Usage
	)
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") || line == "data: [DONE]" {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		var chunk map[string]any
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if v, ok := chunk["id"].(string); ok && id == "" {
			id = v
		}
		if v, ok := chunk["object"].(string); ok && object == "" {
			object = v
		}
		if v, ok := chunk["created"].(float64); ok && created == 0 {
			created = int(v)
		}
		if v, ok := chunk["model"].(string); ok && model == "" {
			model = v
		}
		// Merge choices
		if choices, ok := chunk["choices"].([]any); ok && len(choices) > 0 {
			choice := choices[0].(map[string]any)
			if delta, ok := choice["delta"].(map[string]any); ok {
				if c, ok := delta["content"].(string); ok {
					content.WriteString(c)
				}
			}
			if fr, ok := choice["finish_reason"].(string); ok && fr != "" {
				finish = fr
			}
		}
		// Merge usage if present (usually only in last chunk)
		if u, ok := chunk["usage"].(map[string]any); ok {
			if v, ok := u["prompt_tokens"].(float64); ok {
				usage.PromptTokens = int(v)
			}
			if v, ok := u["completion_tokens"].(float64); ok {
				usage.CompletionTokens = int(v)
			}
			if v, ok := u["total_tokens"].(float64); ok {
				usage.TotalTokens = int(v)
			}
		}
	}
	merged := map[string]any{
		"id":      id,
		"object":  object,
		"created": created,
		"model":   model,
		"choices": []map[string]any{{
			"index": 0,
			"message": map[string]any{
				"role":    "assistant",
				"content": content.String(),
			},
			"finish_reason": finish,
		}},
	}
	if usage.PromptTokens > 0 || usage.CompletionTokens > 0 || usage.TotalTokens > 0 {
		merged["usage"] = map[string]any{
			"prompt_tokens":     usage.PromptTokens,
			"completion_tokens": usage.CompletionTokens,
			"total_tokens":      usage.TotalTokens,
		}
	}
	log.Printf("[eventtransformer] [streaming] Merged OpenAI chunks (len=%d)", content.Len())
	return merged, nil
}

// MergeThreadStreamingChunks parses and merges assistant thread.run streaming events.
func MergeThreadStreamingChunks(body string) (map[string]any, error) {
	type Usage struct {
		PromptTokens     int
		CompletionTokens int
		TotalTokens      int
	}
	var (
		id           string
		assistantID  string
		threadID     string
		status       string
		created      int
		model        string
		contentB     strings.Builder
		usage        Usage
		finalContent string // for thread.message.completed
	)
	lines := strings.Split(body, "\n")
	foundCompleted := false
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "event: ") {
			eventType := strings.TrimPrefix(line, "event: ")
			i++
			if i >= len(lines) {
				break
			}
			dataLine := strings.TrimSpace(lines[i])
			if !strings.HasPrefix(dataLine, "data: ") || dataLine == "data: [DONE]" {
				continue
			}
			var msg map[string]any
			if err := json.Unmarshal([]byte(strings.TrimPrefix(dataLine, "data: ")), &msg); err != nil {
				continue
			}
			switch eventType {
			case "thread.run.created", "thread.run.queued":
				if v, ok := msg["id"].(string); ok && id == "" {
					id = v
				}
				if v, ok := msg["assistant_id"].(string); ok && assistantID == "" {
					assistantID = v
				}
				if v, ok := msg["thread_id"].(string); ok && threadID == "" {
					threadID = v
				}
				if v, ok := msg["status"].(string); ok && status == "" {
					status = v
				}
				if v, ok := msg["created_at"].(float64); ok && created == 0 {
					created = int(v)
				}
			case "thread.message.delta":
				if !foundCompleted {
					if delta, ok := msg["delta"].(map[string]any); ok {
						if segs, ok := delta["content"].([]any); ok {
							for _, seg := range segs {
								segMap := seg.(map[string]any)
								if txt, ok := segMap["text"].(map[string]any); ok {
									contentB.WriteString(txt["value"].(string))
								}
							}
						}
					}
				}
			case "thread.message.completed":
				// If this event is found, ignore all previous deltas and use only this content
				if contentArr, ok := msg["content"].([]any); ok {
					var sb strings.Builder
					for _, seg := range contentArr {
						segMap, ok := seg.(map[string]any)
						if !ok {
							continue
						}
						if segMap["type"] == "text" {
							if txt, ok := segMap["text"].(map[string]any); ok {
								if val, ok := txt["value"].(string); ok {
									sb.WriteString(val)
								}
							}
						}
					}
					finalContent = sb.String()
					foundCompleted = true
				}
			case "thread.run.step.completed", "thread.run.completed":
				if u, ok := msg["usage"].(map[string]any); ok {
					if v, ok := u["prompt_tokens"].(float64); ok {
						usage.PromptTokens = int(v)
					}
					if v, ok := u["completion_tokens"].(float64); ok {
						usage.CompletionTokens = int(v)
					}
					if v, ok := u["total_tokens"].(float64); ok {
						usage.TotalTokens = int(v)
					}
				}
			}
		}
	}
	// If thread.message.completed was found, use only its content
	if foundCompleted {
		contentB.Reset()
		contentB.WriteString(finalContent)
	}
	merged := map[string]any{
		"id":           id,
		"object":       "thread.run",
		"created_at":   created,
		"assistant_id": assistantID,
		"thread_id":    threadID,
		"status":       status,
		"model":        model,
		"choices": []map[string]any{{
			"index": 0,
			"message": map[string]any{
				"role":    "assistant",
				"content": contentB.String(),
			},
			"finish_reason": "",
		}},
	}
	if usage.PromptTokens+usage.CompletionTokens+usage.TotalTokens > 0 {
		merged["usage"] = map[string]any{
			"prompt_tokens":     usage.PromptTokens,
			"completion_tokens": usage.CompletionTokens,
			"total_tokens":      usage.TotalTokens,
		}
	}
	log.Printf("[eventtransformer] [streaming] Merged thread.run chunks (len=%d, used_completed=%v)", contentB.Len(), foundCompleted)
	return merged, nil
}

// TransformEvent transforms an OpenAI event for logging/analytics.
// It handles OPTIONS skipping, header filtering, decoding, chunk merging, token counting, and snake_case normalization.
func (t *OpenAITransformer) TransformEvent(evt map[string]any) (map[string]any, error) {
	// Skip OPTIONS requests
	if method, _ := evt["Method"].(string); strings.ToUpper(method) == "OPTIONS" {
		return nil, nil
	}

	// Filter out set-cookie from response_headers
	if headers, ok := evt["ResponseHeaders"].(map[string]any); ok {
		for k := range headers {
			if strings.ToLower(k) == "set-cookie" {
				delete(headers, k)
			}
		}
	}

	// Set request_id
	requestID := ""
	if headers, ok := evt["RequestHeaders"].(map[string]any); ok {
		for k, v := range headers {
			if strings.ToLower(k) == "x-request-id" {
				if s, ok := v.(string); ok && s != "" {
					requestID = s
					break
				}
			}
		}
	}
	if requestID == "" {
		requestID = uuid.NewString()
	}
	evt["request_id"] = requestID

	// Decode request_body to UTF-8 or compact JSON
	for _, key := range []string{"RequestBody", "request_body"} {
		if v, ok := evt[key]; ok {
			switch val := v.(type) {
			case string:
				if val != "" {
					decoded := tryBase64DecodeWithLog(val)
					if compact, _, ok := normalizeToCompactJSON(decoded); ok {
						evt["request_body"] = compact
						// promptTokenSource is set below if request_body is present
					} else if isValidUTF8(decoded) {
						evt["request_body"] = decoded
					} else {
						evt["request_body"] = "[binary or undecodable data]"
					}
					break
				}
			case []byte:
				if len(val) > 0 {
					decoded := tryBase64DecodeWithLog(string(val))
					evt["request_body"] = decoded
					break
				}
			}
		}
	}

	// Handle ResponseBody
	contentType := ""
	hdrs, ok := evt["ResponseHeaders"]
	var hdrMap map[string]any
	if ok {
		hdrMap, _ = hdrs.(map[string]any)
	} else {
		hdrMap = map[string]any{}
	}
	if hdrMap == nil {
		hdrMap = map[string]any{}
	}
	if len(hdrMap) > 0 {
		for k, v := range hdrMap {
			if strings.ToLower(strings.ReplaceAll(k, "-", "_")) == "content_type" {
				switch arr := v.(type) {
				case []any:
					if len(arr) > 0 {
						if s, ok := arr[0].(string); ok {
							contentType = strings.ToLower(s)
						}
					}
				case string:
					contentType = strings.ToLower(arr)
				}
			}
		}
	}

	if respBody, ok := evt["ResponseBody"].(string); ok && respBody != "" {
		decoded, okDecoded := DecompressAndDecode(respBody, hdrMap)
		if !okDecoded {
			decoded = tryBase64DecodeWithLog(respBody)
		}

		if strings.HasPrefix(contentType, "audio/") || strings.HasPrefix(contentType, "image/") || contentType == "application/octet-stream" {
			if os.Getenv("LOG_BINARY_RESPONSES") == "1" {
				evt["response_body"] = respBody
			} else {
				evt["response_body"] = "[binary or undecodable data]"
			}
			evt["response_body_binary"] = true
		} else {
			if IsOpenAIStreaming(decoded) {
				var merged map[string]any
				var err error
				if strings.Contains(decoded, "event: thread.run") {
					merged, err = MergeThreadStreamingChunks(decoded)
				} else {
					merged, err = MergeOpenAIStreamingChunks(decoded)
				}
				if err == nil {
					comp, _ := json.Marshal(merged)
					evt["response_body"] = string(comp)
					if usage, ok := merged["usage"].(map[string]any); ok {
						evt["TokenUsage"] = usage
					}
				} else {
					log.Printf("[eventtransformer] merge stream error: %v", err)
					evt["response_body"] = decoded
				}
			} else {
				if compact, _, ok := normalizeToCompactJSON(decoded); ok {
					evt["response_body"] = compact
				} else if isValidUTF8(decoded) {
					evt["response_body"] = decoded
				} else {
					evt["response_body"] = "[binary or undecodable data]"
				}
				// Extract usage from non-streaming completion
				if resp, ok := evt["response_body"].(string); ok && json.Valid([]byte(resp)) {
					var respObj map[string]any
					if err := json.Unmarshal([]byte(resp), &respObj); err == nil {
						if usage, ok := respObj["usage"].(map[string]any); ok {
							evt["TokenUsage"] = usage
							delete(respObj, "usage")
							b, _ := json.Marshal(respObj)
							evt["response_body"] = string(b)
						}
					}
				}
			}
		}
	}

	// Token usage fallback
	if _, has := evt["TokenUsage"]; !has {
		if resp, ok := evt["response_body"].(string); ok && json.Valid([]byte(resp)) {
			pt, ct := 0, 0
			// Compose prompt token source: messages + instructions if present
			var promptTokenSource string
			if req, ok := evt["request_body"].(string); ok && req != "" {
				var reqObj map[string]any
				if err := json.Unmarshal([]byte(req), &reqObj); err == nil {
					if msgs, ok := reqObj["messages"]; ok {
						b, _ := json.Marshal(msgs)
						promptTokenSource = string(b)
					}
					if instr, ok := reqObj["instructions"].(string); ok && instr != "" {
						if promptTokenSource != "" {
							promptTokenSource += instr
						} else {
							promptTokenSource = instr
						}
					}
				}
			}
			if promptTokenSource != "" {
				modelName := ""
				if req, ok := evt["request_body"].(string); ok && req != "" {
					var reqObj map[string]any
					_ = json.Unmarshal([]byte(req), &reqObj)
					if m, ok := reqObj["model"].(string); ok {
						modelName = m
					}
				}
				t, _ := CountOpenAITokensForModel(promptTokenSource, modelName)
				pt = t
			}
			cnt, _ := extractAssistantReplyContent(resp)
			if cnt != "" {
				// Try to read model from the parsed response JSON
				modelName := ""
				var respObj map[string]any
				if err := json.Unmarshal([]byte(resp), &respObj); err == nil {
					if m, ok := respObj["model"].(string); ok {
						modelName = m
					}
				}
				tk, _ := CountOpenAITokensForModel(cnt, modelName)
				ct = tk
			}
			evt["TokenUsage"] = map[string]int{"prompt_tokens": pt, "completion_tokens": ct, "total_tokens": pt + ct}
		}
	}

	// Clean up and normalize
	delete(evt, "RequestBody")
	delete(evt, "ResponseBody")
	delete(evt, "response_body_streamed")
	return ToSnakeCaseMap(evt), nil
}

// -- helper functions --

func tryBase64DecodeWithLog(val string) string {
	clean := strings.ReplaceAll(val, "", "")
	if json.Valid([]byte(clean)) {
		return clean
	}
	if b, err := base64.StdEncoding.DecodeString(clean); err == nil {
		return string(b)
	}
	if b, err := base64.URLEncoding.DecodeString(clean); err == nil {
		return string(b)
	}
	return clean
}

func normalizeToCompactJSON(input string) (string, string, bool) {
	var obj any
	if err := json.Unmarshal([]byte(input), &obj); err != nil {
		return input, "", false
	}
	if s, ok := obj.(string); ok {
		var inner any
		if err := json.Unmarshal([]byte(s), &inner); err == nil {
			b, _ := json.Marshal(inner)
			str := string(b)
			if m, ok := inner.(map[string]any); ok {
				if msgs, ok := m["messages"]; ok {
					mj, _ := json.Marshal(msgs)
					return str, string(mj), true
				}
			}
			return str, "", true
		}
		return s, "", true
	}
	b, err := json.Marshal(obj)
	if err != nil {
		return input, "", false
	}
	str := string(b)
	if m, ok := obj.(map[string]any); ok {
		if msgs, ok := m["messages"]; ok {
			mj, _ := json.Marshal(msgs)
			return str, string(mj), true
		}
	}
	return str, "", true
}

func isValidUTF8(s string) bool { return utf8.ValidString(s) }

func extractAssistantReplyContent(resp string) (string, error) {
	var obj map[string]any
	if err := json.Unmarshal([]byte(resp), &obj); err != nil {
		return "", err
	}
	if ch, ok := obj["choices"].([]any); ok && len(ch) > 0 {
		if c0, ok := ch[0].(map[string]any); ok {
			if msg, ok := c0["message"].(map[string]any); ok {
				if content, ok := msg["content"].(string); ok {
					return content, nil
				}
			}
		}
	}
	return "", nil
}
