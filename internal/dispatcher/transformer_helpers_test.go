package dispatcher

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"testing"
)

func TestMin(t *testing.T) {
	if min(1, 2) != 1 || min(2, 1) != 1 || min(5, 5) != 5 {
		t.Fatalf("min incorrect")
	}
}

func TestCleanJSONBinary(t *testing.T) {
	// Includes invalid UTF-8 in string and []byte
	in := map[string]any{
		"ok":    "text",
		"s":     string([]byte{0xff, 0xfe}),
		"bytes": []byte{0xff, 0xfe},
		"arr":   []any{"x", []byte{0xff}},
	}
	out := cleanJSONBinary(in).(map[string]any)
	if out["ok"].(string) != "text" {
		t.Fatalf("expected ok field unchanged")
	}
	if out["s"].(string) != "<binary omitted>" {
		t.Fatalf("expected invalid utf8 string replaced")
	}
	if out["bytes"].(string) != "<binary omitted>" {
		t.Fatalf("expected invalid utf8 bytes replaced")
	}
}

func TestSafeRawMessageOrBase64_MultipartAndPlain(t *testing.T) {
	// Multipart should return placeholder JSON string
	js, b64 := safeRawMessageOrBase64([]byte("ignored"), map[string][]string{"Content-Type": {"multipart/form-data"}})
	if b64 != "" || js == nil {
		// Expect JSON string with placeholder; ensure it decodes back to string
		var s string
		if err := json.Unmarshal(js, &s); err != nil || s != "<multipart response omitted>" {
			t.Fatalf("expected multipart placeholder, got %s, err=%v", string(js), err)
		}
	}

	// Plain UTF-8 non-JSON should be returned as JSON-quoted string
	js, b64 = safeRawMessageOrBase64([]byte("hello world"), map[string][]string{})
	if b64 != "" {
		t.Fatalf("expected no base64 for plain utf-8")
	}
	var s string
	if err := json.Unmarshal(js, &s); err != nil || s != "hello world" {
		t.Fatalf("expected quoted plain text, got %q err=%v", string(js), err)
	}
}

func TestSafeRawMessageOrBase64_GzipJSON(t *testing.T) {
	// Prepare gzipped JSON
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, _ = zw.Write([]byte(`{"foo":"bar","n":1}`))
	_ = zw.Close()

	headers := map[string][]string{
		"Content-Encoding": {"gzip"},
		"Content-Type":     {"application/json"},
	}
	js, b64 := safeRawMessageOrBase64(buf.Bytes(), headers)
	if b64 != "" || js == nil {
		t.Fatalf("expected JSON, got b64=%q js=%v", b64, js)
	}
	// Should unmarshal back to object
	var m map[string]any
	if err := json.Unmarshal(js, &m); err != nil || m["foo"] != "bar" {
		t.Fatalf("unexpected JSON: %s err=%v", string(js), err)
	}
}

func TestSafeRawMessageOrBase64_GzipInvalidJSONWithJSONContentType(t *testing.T) {
	// Corrupt gzip data
	headers := map[string][]string{
		"Content-Encoding": {"gzip"},
		"Content-Type":     {"application/json"},
	}
	js, b64 := safeRawMessageOrBase64([]byte{0x1f, 0x8b, 0x08, 0x00, 0xde, 0xad}, headers)
	if b64 != "" || js == nil {
		t.Fatalf("expected placeholder JSON when gzip fails, got b64=%q js=%v", b64, js)
	}
	var s string
	if err := json.Unmarshal(js, &s); err != nil || s != "<binary response omitted>" {
		t.Fatalf("expected binary placeholder, got %q err=%v", string(js), err)
	}
}

func TestSafeRawMessageOrBase64_OpenAIStreamingMerge(t *testing.T) {
	// Minimal streaming lines that look like OpenAI stream
	stream := "data: {\"choices\":[{\"delta\":{\"content\":\"he\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"llo\"}}]}\n\n" +
		"data: [DONE]\n\n"
	js, b64 := safeRawMessageOrBase64([]byte(stream), nil)
	if b64 != "" || js == nil {
		t.Fatalf("expected merged JSON, got b64=%q js=%v", b64, js)
	}
}

func TestSafeRawMessageOrBase64_BinaryNonUTF8(t *testing.T) {
	js, b64 := safeRawMessageOrBase64([]byte{0xff, 0xfe, 0xfd}, nil)
	if b64 != "" || js == nil {
		t.Fatalf("expected placeholder JSON for binary data")
	}
	var s string
	if err := json.Unmarshal(js, &s); err != nil || s != "<binary response omitted>" {
		t.Fatalf("expected binary placeholder, got %q err=%v", string(js), err)
	}
}
