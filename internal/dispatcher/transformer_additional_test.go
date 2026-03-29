package dispatcher

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/andybalholm/brotli"

	"github.com/sofatutor/llm-proxy/internal/eventbus"
)

// Covers gzip Content-Encoding with non-gzipped but valid JSON payload: should fall back to raw JSON
func TestSafeRawMessageOrBase64_GzipHeaderButPlainJSON(t *testing.T) {
	headers := map[string][]string{
		"Content-Encoding": {"gzip"},
	}
	data := []byte(`{"foo":"bar"}`)
	js, b64 := safeRawMessageOrBase64(data, headers)
	if b64 != "" || js == nil {
		t.Fatalf("expected JSON fallback when decompress fails and data is valid JSON")
	}
	var m map[string]any
	if err := json.Unmarshal(js, &m); err != nil || m["foo"] != "bar" {
		t.Fatalf("unexpected JSON: %s err=%v", string(js), err)
	}
}

func TestSafeRawMessageOrBase64_BrotliJSON(t *testing.T) {
	// Prepare brotli-compressed JSON
	var buf bytes.Buffer
	w := brotli.NewWriter(&buf)
	_, _ = w.Write([]byte(`{"a":1}`))
	_ = w.Close()
	headers := map[string][]string{
		"Content-Encoding": {"br"},
		"Content-Type":     {"application/json"},
	}
	js, b64 := safeRawMessageOrBase64(buf.Bytes(), headers)
	if b64 != "" || js == nil {
		t.Fatalf("expected JSON for brotli-compressed input")
	}
	var m map[string]any
	if err := json.Unmarshal(js, &m); err != nil || m["a"].(float64) != 1 {
		t.Fatalf("unexpected JSON: %s err=%v", string(js), err)
	}
}

// Covers thread.run streaming merge branch
func TestSafeRawMessageOrBase64_ThreadStreamingMerge(t *testing.T) {
	stream := "event: thread.run.step.completed\n" +
		"data: {\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":2,\"total_tokens\":3}}\n\n" +
		"event: thread.message.delta\n" +
		"data: {\"delta\":{\"content\":[{\"type\":\"output_text\",\"text\":{\"value\":\"hi\"}}]}}\n\n"
	js, b64 := safeRawMessageOrBase64([]byte(stream), nil)
	if b64 != "" || js == nil {
		t.Fatalf("expected merged JSON for thread streaming input")
	}
}

// Ensure verbose response headers inclusion in Transform
func TestDefaultEventTransformer_Transform_VerboseHeaders(t *testing.T) {
	tr := NewDefaultEventTransformer(true)
	evt := eventbus.Event{
		RequestID: "id",
		Method:    "POST",
		// Use non-OpenAI path so Transform doesn't early-return before adding headers
		Path:         "/debug/echo",
		Status:       200,
		Duration:     5 * time.Millisecond,
		RequestBody:  []byte(`{"x":1}`),
		ResponseBody: []byte(`{"choices":[]}`),
		ResponseHeaders: http.Header{
			"Content-Type": {"application/json"},
			"X-Custom":     {"a", "b"},
		},
	}
	payload, err := tr.Transform(evt)
	if err != nil {
		t.Fatalf("Transform err: %v", err)
	}
	if payload == nil {
		t.Fatalf("nil payload")
	}
	headersAny, ok := payload.Metadata["response_headers"]
	if !ok {
		t.Fatalf("expected response_headers in metadata")
	}
	headers := headersAny.(map[string]any)
	if v, ok := headers["X-Custom"].([]string); !ok || len(v) != 2 {
		t.Fatalf("expected multi-value header preserved, got %T %v", headers["X-Custom"], headers["X-Custom"])
	}
}

func TestDefaultEventTransformer_Transform_ResponsesUsageAndTokenMetadata(t *testing.T) {
	tr := NewDefaultEventTransformer(false)
	evt := eventbus.Event{
		RequestID:     "req-123",
		Method:        "POST",
		Path:          "/v1/responses",
		ProjectID:     "project-1",
		TokenID:       "token-uuid-1",
		TokenMetadata: map[string]string{"feature": "sofabuddy", "user_id": "42"},
		Status:        200,
		Duration:      5 * time.Millisecond,
		RequestBody:   []byte(`{"model":"gpt-4.1-mini","input":"hello"}`),
		ResponseBody:  []byte(`{"id":"resp_1","model":"gpt-4.1-mini","usage":{"input_tokens":11,"output_tokens":7,"total_tokens":18},"output":[]}`),
		ResponseHeaders: http.Header{
			"Content-Type": {"application/json"},
		},
	}

	payload, err := tr.Transform(evt)
	if err != nil {
		t.Fatalf("Transform err: %v", err)
	}
	if payload == nil {
		t.Fatalf("expected payload")
	}
	if payload.UserID == nil || *payload.UserID != "42" {
		t.Fatalf("expected user id 42, got %v", payload.UserID)
	}
	if payload.TokensUsage == nil || payload.TokensUsage.Input != 11 || payload.TokensUsage.Output != 7 || payload.TokensUsage.Total != 18 {
		t.Fatalf("unexpected token usage: %#v", payload.TokensUsage)
	}
	if payload.Metadata["project_id"] != "project-1" {
		t.Fatalf("expected project_id metadata, got %v", payload.Metadata["project_id"])
	}
	if payload.Metadata["token_id"] != "token-uuid-1" {
		t.Fatalf("expected token_id metadata, got %v", payload.Metadata["token_id"])
	}
	if payload.Metadata["model"] != "gpt-4.1-mini" {
		t.Fatalf("expected model metadata, got %v", payload.Metadata["model"])
	}
	tokenMetadata, ok := payload.Metadata["token_metadata"].(map[string]string)
	if !ok {
		t.Fatalf("expected token metadata map, got %T", payload.Metadata["token_metadata"])
	}
	if tokenMetadata["feature"] != "sofabuddy" {
		t.Fatalf("expected feature metadata, got %v", tokenMetadata)
	}
}

func TestDefaultEventTransformer_Transform_ResponsesTokenUsageFallbackFromOutputText(t *testing.T) {
	tr := NewDefaultEventTransformer(false)
	evt := eventbus.Event{
		RequestID:    "req-responses-1",
		Method:       "POST",
		Path:         "/v1/responses",
		Status:       200,
		Duration:     12 * time.Millisecond,
		RequestBody:  []byte(`{"model":"gpt-4.1-mini","input":"hello"}`),
		ResponseBody: []byte(`{"id":"resp_1","model":"gpt-4.1-mini","output":[{"type":"message","content":[{"type":"output_text","text":"hi there"}]}]}`),
		ResponseHeaders: http.Header{
			"Content-Type": {"application/json"},
		},
	}

	payload, err := tr.Transform(evt)
	if err != nil {
		t.Fatalf("Transform err: %v", err)
	}
	if payload == nil {
		t.Fatalf("expected payload")
	}
	if payload.TokensUsage == nil {
		t.Fatalf("expected computed token usage")
	}
	if payload.TokensUsage.Output <= 0 {
		t.Fatalf("expected responses completion tokens > 0, got %#v", payload.TokensUsage)
	}
}

func TestDefaultEventTransformer_Transform_ResponsesStreamUsesCompletedEvent(t *testing.T) {
	tr := NewDefaultEventTransformer(false)
	stream := "event: response.created\n" +
		"data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_1\",\"object\":\"response\",\"created_at\":123,\"status\":\"in_progress\",\"model\":\"gpt-4.1-mini\",\"output\":[]}}\n\n" +
		"event: response.output_text.delta\n" +
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"Hallo Welt\"}\n\n" +
		"event: response.completed\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\",\"object\":\"response\",\"created_at\":123,\"status\":\"completed\",\"model\":\"gpt-4.1-mini\",\"usage\":{\"input_tokens\":40,\"output_tokens\":12,\"total_tokens\":52},\"output\":[{\"type\":\"message\",\"content\":[{\"type\":\"output_text\",\"text\":\"Hallo Welt\"}]}]}}\n\n" +
		"data: [DONE]\n\n"
	vt := eventbus.Event{
		RequestID:    "req-responses-stream-1",
		Method:       "POST",
		Path:         "/v1/responses",
		Status:       200,
		Duration:     12 * time.Millisecond,
		RequestBody:  []byte(`{"model":"gpt-4.1-mini","input":"hello"}`),
		ResponseBody: []byte(stream),
		ResponseHeaders: http.Header{
			"Content-Type": {"text/event-stream"},
		},
	}

	payload, err := tr.Transform(vt)
	if err != nil {
		t.Fatalf("Transform err: %v", err)
	}
	if payload == nil {
		t.Fatalf("expected payload")
	}
	if payload.TokensUsage == nil {
		t.Fatalf("expected token usage from response.completed")
	}
	if payload.TokensUsage.Input != 40 || payload.TokensUsage.Output != 12 || payload.TokensUsage.Total != 52 {
		t.Fatalf("expected final stream usage 40/12, got %#v", payload.TokensUsage)
	}
	if payload.Metadata["model"] != "gpt-4.1-mini" {
		t.Fatalf("expected model metadata, got %v", payload.Metadata["model"])
	}
	var output map[string]any
	if err := json.Unmarshal(payload.Output, &output); err != nil {
		t.Fatalf("failed to unmarshal payload output: %v", err)
	}
	if output["status"] != "completed" {
		t.Fatalf("expected completed merged response status, got %v", output["status"])
	}
}

func TestDefaultEventTransformer_Transform_ModelFallbackFromRequestBody(t *testing.T) {
	tr := NewDefaultEventTransformer(false)
	evt := eventbus.Event{
		RequestID:    "req-chat-1",
		Method:       "POST",
		Path:         "/v1/chat/completions",
		Status:       200,
		Duration:     12 * time.Millisecond,
		RequestBody:  []byte(`{"model":"gpt-4.1-mini","messages":[{"role":"user","content":"hello"}]}`),
		ResponseBody: []byte(`{"id":"chatcmpl_1","choices":[{"index":0,"message":{"role":"assistant","content":"hi there"},"finish_reason":"stop"}]}`),
		ResponseHeaders: http.Header{
			"Content-Type": {"application/json"},
		},
	}

	payload, err := tr.Transform(evt)
	if err != nil {
		t.Fatalf("Transform err: %v", err)
	}
	if payload == nil {
		t.Fatalf("expected payload")
	}
	if payload.Metadata["model"] != "gpt-4.1-mini" {
		t.Fatalf("expected request model fallback, got %v", payload.Metadata["model"])
	}
}

func TestDefaultEventTransformer_Transform_TokenUsageFallbackFromComputedUsage(t *testing.T) {
	tr := NewDefaultEventTransformer(false)
	evt := eventbus.Event{
		RequestID:    "req-chat-2",
		Method:       "POST",
		Path:         "/v1/chat/completions",
		Status:       200,
		Duration:     12 * time.Millisecond,
		RequestBody:  []byte(`{"model":"gpt-4.1-mini","messages":[{"role":"user","content":"hello"}]}`),
		ResponseBody: []byte(`{"id":"chatcmpl_1","choices":[{"index":0,"message":{"role":"assistant","content":"hi there"},"finish_reason":"stop"}]}`),
		ResponseHeaders: http.Header{
			"Content-Type": {"application/json"},
		},
	}

	payload, err := tr.Transform(evt)
	if err != nil {
		t.Fatalf("Transform err: %v", err)
	}
	if payload == nil {
		t.Fatalf("expected payload")
	}
	if payload.TokensUsage == nil {
		t.Fatalf("expected computed token usage")
	}
	if payload.TokensUsage.Input <= 0 {
		t.Fatalf("expected prompt tokens > 0, got %#v", payload.TokensUsage)
	}
	if payload.TokensUsage.Output <= 0 {
		t.Fatalf("expected completion tokens > 0, got %#v", payload.TokensUsage)
	}
}

func TestDefaultEventTransformer_Transform_TokenUsageFallbackFromGzippedChatResponse(t *testing.T) {
	tr := NewDefaultEventTransformer(false)

	var compressed bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressed)
	_, err := gzipWriter.Write([]byte(`{"id":"chatcmpl_1","choices":[{"index":0,"message":{"role":"assistant","content":"{\"suggestions\":[\"A\",\"B\",\"C\"]}"},"finish_reason":"stop"}]}`))
	if err != nil {
		t.Fatalf("gzip write err: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("gzip close err: %v", err)
	}

	evt := eventbus.Event{
		RequestID:    "req-chat-gzip-1",
		Method:       "POST",
		Path:         "/v1/chat/completions",
		Status:       200,
		Duration:     12 * time.Millisecond,
		RequestBody:  []byte(`{"model":"gpt-4.1-mini","messages":[{"role":"user","content":"hello"}]}`),
		ResponseBody: compressed.Bytes(),
		ResponseHeaders: http.Header{
			"Content-Type":     {"application/json"},
			"Content-Encoding": {"gzip"},
		},
	}

	payload, err := tr.Transform(evt)
	if err != nil {
		t.Fatalf("Transform err: %v", err)
	}
	if payload == nil {
		t.Fatalf("expected payload")
	}
	if payload.TokensUsage == nil {
		t.Fatalf("expected token usage")
	}
	if payload.TokensUsage.Output <= 0 {
		t.Fatalf("expected completion tokens > 0 for gzipped chat response, got %#v", payload.TokensUsage)
	}
	if payload.Metadata["model"] != "gpt-4.1-mini" {
		t.Fatalf("expected model metadata, got %v", payload.Metadata["model"])
	}
}

func TestDefaultEventTransformer_Transform_TokenUsageFallbackFromChatStream(t *testing.T) {
	tr := NewDefaultEventTransformer(false)
	stream := "data: {\"id\":\"chatcmpl_1\",\"object\":\"chat.completion.chunk\",\"created\":123,\"model\":\"gpt-4.1-mini\",\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\" there\"},\"finish_reason\":\"stop\"}]}\n\n" +
		"data: [DONE]\n\n"
	evt := eventbus.Event{
		RequestID:    "req-chat-stream-1",
		Method:       "POST",
		Path:         "/v1/chat/completions",
		Status:       200,
		Duration:     12 * time.Millisecond,
		RequestBody:  []byte(`{"model":"gpt-4.1-mini","messages":[{"role":"user","content":"hello"}]}`),
		ResponseBody: []byte(stream),
		ResponseHeaders: http.Header{
			"Content-Type": {"text/event-stream"},
		},
	}

	payload, err := tr.Transform(evt)
	if err != nil {
		t.Fatalf("Transform err: %v", err)
	}
	if payload == nil {
		t.Fatalf("expected payload")
	}
	if payload.TokensUsage == nil {
		t.Fatalf("expected computed token usage from stream")
	}
	if payload.TokensUsage.Output <= 0 {
		t.Fatalf("expected stream completion tokens > 0, got %#v", payload.TokensUsage)
	}
	if payload.Metadata["model"] != "gpt-4.1-mini" {
		t.Fatalf("expected model metadata from request body, got %v", payload.Metadata["model"])
	}
}

func TestDefaultEventTransformer_Transform_TokenUsageFallbackFromThreadStream(t *testing.T) {
	tr := NewDefaultEventTransformer(false)
	stream := "event: thread.run.created\n" +
		"data: {\"id\":\"run_1\",\"assistant_id\":\"asst_1\",\"thread_id\":\"thread_1\",\"status\":\"queued\",\"created_at\":123}\n\n" +
		"event: thread.message.delta\n" +
		"data: {\"delta\":{\"content\":[{\"type\":\"text\",\"text\":{\"value\":\"hello\"}}]}}\n\n" +
		"event: thread.message.completed\n" +
		"data: {\"content\":[{\"type\":\"text\",\"text\":{\"value\":\"hello there\"}}]}\n\n" +
		"event: thread.run.completed\n" +
		"data: {\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":2,\"total_tokens\":3}}\n\n"
	evt := eventbus.Event{
		RequestID:    "req-thread-stream-1",
		Method:       "POST",
		Path:         "/v1/threads/runs",
		Status:       200,
		Duration:     12 * time.Millisecond,
		RequestBody:  []byte(`{"model":"gpt-4.1-mini","input":"hello"}`),
		ResponseBody: []byte(stream),
		ResponseHeaders: http.Header{
			"Content-Type": {"text/event-stream"},
		},
	}

	payload, err := tr.Transform(evt)
	if err != nil {
		t.Fatalf("Transform err: %v", err)
	}
	if payload == nil {
		t.Fatalf("expected payload")
	}
	if payload.TokensUsage == nil {
		t.Fatalf("expected token usage from thread stream")
	}
	if payload.TokensUsage.Output <= 0 {
		t.Fatalf("expected thread stream completion tokens > 0, got %#v", payload.TokensUsage)
	}
}

func TestDefaultEventTransformer_Transform_ChatErrorStillKeepsModelAndPromptTokens(t *testing.T) {
	tr := NewDefaultEventTransformer(false)
	evt := eventbus.Event{
		RequestID:    "req-chat-404",
		Method:       "POST",
		Path:         "/v1/chat/completions",
		Status:       404,
		Duration:     600 * time.Microsecond,
		RequestBody:  []byte(`{"model":"gpt-4.1-mini","messages":[{"role":"user","content":"hello"}]}`),
		ResponseBody: []byte(`not found`),
		ResponseHeaders: http.Header{
			"Content-Type": {"text/plain"},
		},
	}

	payload, err := tr.Transform(evt)
	if err != nil {
		t.Fatalf("Transform err: %v", err)
	}
	if payload == nil {
		t.Fatalf("expected payload")
	}
	if payload.Metadata["model"] != "gpt-4.1-mini" {
		t.Fatalf("expected request model on error response, got %v", payload.Metadata["model"])
	}
	if payload.TokensUsage == nil || payload.TokensUsage.Input <= 0 || payload.TokensUsage.Output != 0 {
		t.Fatalf("expected prompt-only token usage on error response, got %#v", payload.TokensUsage)
	}
	if payload.Metadata["duration_ms"] != int64(1) {
		t.Fatalf("expected sub-millisecond duration to round up to 1ms, got %v", payload.Metadata["duration_ms"])
	}
}

func TestDefaultEventTransformer_Transform_ModerationsModelFallback(t *testing.T) {
	tr := NewDefaultEventTransformer(false)
	evt := eventbus.Event{
		RequestID:    "req-mod-1",
		Method:       "POST",
		Path:         "/v1/moderations",
		Status:       200,
		Duration:     2 * time.Millisecond,
		RequestBody:  []byte(`{"model":"omni-moderation-latest","input":"hello"}`),
		ResponseBody: []byte(`{"id":"modr-1","results":[{"flagged":false}]}`),
		ResponseHeaders: http.Header{
			"Content-Type": {"application/json"},
		},
	}

	payload, err := tr.Transform(evt)
	if err != nil {
		t.Fatalf("Transform err: %v", err)
	}
	if payload == nil {
		t.Fatalf("expected payload")
	}
	if payload.Metadata["model"] != "omni-moderation-latest" {
		t.Fatalf("expected moderation model from request body, got %v", payload.Metadata["model"])
	}
	if payload.TokensUsage == nil || payload.TokensUsage.Input <= 0 {
		t.Fatalf("expected moderation prompt tokens from input fallback, got %#v", payload.TokensUsage)
	}
}
