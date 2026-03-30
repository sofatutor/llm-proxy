package dispatcher

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/andybalholm/brotli"

	"github.com/sofatutor/llm-proxy/internal/eventbus"
)

func TestCloneStringAnyMap_DeepClone(t *testing.T) {
	original := map[string]any{
		"message": "hello",
		"nested": map[string]any{
			"count": 3,
		},
		"items": []any{
			map[string]any{"kind": "text", "value": "alpha"},
			"tail",
		},
	}

	cloned := cloneStringAnyMap(original)
	if cloned == nil {
		t.Fatal("expected cloned map")
	}
	if reflect.DeepEqual(cloned, original) == false {
		t.Fatalf("expected equal content, got %#v want %#v", cloned, original)
	}

	cloned["message"] = "changed"
	clonedNested := cloned["nested"].(map[string]any)
	clonedNested["count"] = 9
	clonedItems := cloned["items"].([]any)
	clonedItems[0].(map[string]any)["value"] = "beta"

	if original["message"] != "hello" {
		t.Fatalf("expected top-level value to remain unchanged, got %v", original["message"])
	}
	if original["nested"].(map[string]any)["count"] != 3 {
		t.Fatalf("expected nested map to remain unchanged, got %v", original["nested"])
	}
	if original["items"].([]any)[0].(map[string]any)["value"] != "alpha" {
		t.Fatalf("expected nested slice map to remain unchanged, got %v", original["items"])
	}

	if cloneStringAnyMap(map[string]any{}) != nil {
		t.Fatal("expected empty source to return nil")
	}
}

func TestFirstMapValues(t *testing.T) {
	intValues := map[string]int{"prompt_tokens": 7}
	if got := firstIntMapValue(intValues, "input_tokens", "prompt_tokens"); got != 7 {
		t.Fatalf("expected int fallback value 7, got %d", got)
	}
	if got := firstIntMapValue(intValues, "missing"); got != 0 {
		t.Fatalf("expected missing int value 0, got %d", got)
	}

	floatValues := map[string]float64{"completion_tokens": 4.9}
	if got := firstFloatMapValue(floatValues, "output_tokens", "completion_tokens"); got != 4 {
		t.Fatalf("expected float fallback value 4, got %d", got)
	}
	if got := firstFloatMapValue(floatValues, "missing"); got != 0 {
		t.Fatalf("expected missing float value 0, got %d", got)
	}
}

func TestFloatToIntAndFirstUsageValue(t *testing.T) {
	tests := []struct {
		name   string
		value  any
		want   int
		wantOK bool
	}{
		{name: "int", value: int(3), want: 3, wantOK: true},
		{name: "int32", value: int32(4), want: 4, wantOK: true},
		{name: "int64", value: int64(5), want: 5, wantOK: true},
		{name: "float32", value: float32(6.9), want: 6, wantOK: true},
		{name: "float64", value: float64(7.2), want: 7, wantOK: true},
		{name: "unsupported", value: "8", want: 0, wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := floatToInt(tt.value)
			if ok != tt.wantOK {
				t.Fatalf("expected ok=%v, got %v", tt.wantOK, ok)
			}
			if got != tt.want {
				t.Fatalf("expected %d, got %d", tt.want, got)
			}
		})
	}

	usage := map[string]any{"completion_tokens": float64(9), "total_tokens": int64(12)}
	if got, ok := firstUsageValue(usage, "output_tokens", "completion_tokens"); !ok || got != 9 {
		t.Fatalf("expected completion token fallback 9, got %d ok=%v", got, ok)
	}
	if got, ok := firstUsageValue(usage, "total_tokens"); !ok || got != 12 {
		t.Fatalf("expected total token value 12, got %d ok=%v", got, ok)
	}
	if _, ok := firstUsageValue(usage, "missing"); ok {
		t.Fatal("expected missing usage keys to return ok=false")
	}
}

func TestPromptInputSource(t *testing.T) {
	if got := promptInputSource("hello"); got != "hello" {
		t.Fatalf("expected string input source, got %q", got)
	}
	if got := promptInputSource(nil); got != "" {
		t.Fatalf("expected nil input source to be empty, got %q", got)
	}
	if got := promptInputSource([]any{"alpha", map[string]any{"beta": true}}); got != `["alpha",{"beta":true}]` {
		t.Fatalf("expected structured input to be marshaled, got %q", got)
	}
}

func TestAssistantReplyContentFromResponseBody(t *testing.T) {
	if content, ok := assistantReplyContentFromResponseBody([]byte(`{"choices":[{"message":{"content":"hello"}}]}`)); !ok || content != "hello" {
		t.Fatalf("expected chat choices content, got %q ok=%v", content, ok)
	}

	responseObject := map[string]any{
		"choices": []map[string]any{{
			"message": map[string]any{"content": "typed"},
		}},
	}
	responseBytes, err := json.Marshal(responseObject)
	if err != nil {
		t.Fatalf("marshal response err: %v", err)
	}
	if content, ok := assistantReplyContentFromResponseBody(responseBytes); !ok || content != "typed" {
		t.Fatalf("expected typed choices content, got %q ok=%v", content, ok)
	}

	if content, ok := assistantReplyContentFromResponseBody([]byte(`{"output":[{"content":[{"output_text":"fallback"}]}]}`)); !ok || content != "fallback" {
		t.Fatalf("expected responses output fallback, got %q ok=%v", content, ok)
	}

	if _, ok := assistantReplyContentFromResponseBody([]byte(`{"choices":[]}`)); ok {
		t.Fatal("expected false when no assistant content is present")
	}
}

func TestResponseObjectFromBody(t *testing.T) {
	if responseObject, ok := responseObjectFromBody([]byte(`{"id":"resp_1"}`)); !ok || responseObject["id"] != "resp_1" {
		t.Fatalf("expected JSON response body to decode, got %#v ok=%v", responseObject, ok)
	}

	chatStream := "data: {\"id\":\"chatcmpl_1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\" world\"},\"finish_reason\":\"stop\"}]}\n\n" +
		"data: [DONE]\n\n"
	if responseObject, ok := responseObjectFromBody([]byte(chatStream)); !ok || responseObject["id"] != "chatcmpl_1" {
		t.Fatalf("expected streaming response body to merge, got %#v ok=%v", responseObject, ok)
	}

	threadStream := "event: thread.message.delta\n" +
		"data: {\"delta\":{\"content\":[{\"type\":\"text\",\"text\":{\"value\":\"hello\"}}]}}\n\n" +
		"event: thread.run.completed\n" +
		"data: {\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":2,\"total_tokens\":3}}\n\n"
	if responseObject, ok := responseObjectFromBody([]byte(threadStream)); !ok || responseObject["usage"] == nil {
		t.Fatalf("expected thread stream body to merge, got %#v ok=%v", responseObject, ok)
	}

	if _, ok := responseObjectFromBody([]byte{0xff, 0xfe, 0xfd}); ok {
		t.Fatal("expected invalid UTF-8 response body to fail")
	}
	if _, ok := responseObjectFromBody(nil); ok {
		t.Fatal("expected empty response body to fail")
	}
}

func TestResponseOutputText(t *testing.T) {
	response := map[string]any{
		"output": []any{
			map[string]any{
				"content": []any{
					map[string]any{"text": "Hello"},
					map[string]any{"text": map[string]any{"value": " there"}},
					map[string]any{"output_text": "!"},
					map[string]any{"ignored": true},
				},
			},
			map[string]any{"content": []any{"skip-non-map"}},
			"skip-non-map",
		},
	}

	content, ok := responseOutputText(response)
	if !ok {
		t.Fatal("expected response output text")
	}
	if content != "Hello there!" {
		t.Fatalf("expected concatenated content, got %q", content)
	}

	if _, ok := responseOutputText(map[string]any{"output": []any{map[string]any{"content": []any{map[string]any{"ignored": true}}}}}); ok {
		t.Fatal("expected false when no text segments are present")
	}
	if _, ok := responseOutputText(map[string]any{"output": "invalid"}); ok {
		t.Fatal("expected false for invalid output shape")
	}
}

func TestResponseOutputSegmentText(t *testing.T) {
	tests := []struct {
		name    string
		segment map[string]any
		want    string
		wantOK  bool
	}{
		{
			name:    "plain text",
			segment: map[string]any{"text": "hello"},
			want:    "hello",
			wantOK:  true,
		},
		{
			name:    "nested value",
			segment: map[string]any{"text": map[string]any{"value": "nested"}},
			want:    "nested",
			wantOK:  true,
		},
		{
			name:    "output text fallback",
			segment: map[string]any{"output_text": "fallback"},
			want:    "fallback",
			wantOK:  true,
		},
		{
			name:    "missing content",
			segment: map[string]any{"text": map[string]any{"value": ""}},
			want:    "",
			wantOK:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := responseOutputSegmentText(tt.segment)
			if ok != tt.wantOK {
				t.Fatalf("expected ok=%v, got %v", tt.wantOK, ok)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

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

func TestDefaultEventTransformer_Transform_EmbeddingsUsageNormalization(t *testing.T) {
	tr := NewDefaultEventTransformer(false)
	evt := eventbus.Event{
		RequestID:    "req-embeddings-1",
		Method:       "POST",
		Path:         "/v1/embeddings",
		Status:       200,
		Duration:     8 * time.Millisecond,
		RequestBody:  []byte(`{"model":"text-embedding-3-small","input":"hello embeddings"}`),
		ResponseBody: []byte(`{"object":"list","data":[{"object":"embedding","embedding":[0.1,0.2],"index":0}],"model":"text-embedding-3-small","usage":{"prompt_tokens":2,"total_tokens":2}}`),
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
	if payload.Metadata["model"] != "text-embedding-3-small" {
		t.Fatalf("expected embeddings model metadata, got %v", payload.Metadata["model"])
	}
	if payload.TokensUsage == nil {
		t.Fatalf("expected embeddings token usage")
	}
	if payload.TokensUsage.Input != 2 || payload.TokensUsage.Output != 0 || payload.TokensUsage.Total != 2 {
		t.Fatalf("unexpected embeddings token usage: %#v", payload.TokensUsage)
	}
	if len(payload.Output) == 0 {
		t.Fatalf("expected embeddings output to be preserved")
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
