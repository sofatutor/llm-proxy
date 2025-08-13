package dispatcher

import (
	"bytes"
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
