package eventtransformer

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestIsOpenAIStreaming(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{"not streaming", "data: {\"foo\":1}\n", false},
		{"one chunk", "data: {\"foo\":1}\n", false},
		{"two chunks", "data: {\"foo\":1}\ndata: {\"bar\":2}\n", true},
		{"done chunk", "data: {\"foo\":1}\ndata: [DONE]\n", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := IsOpenAIStreaming(c.input)
			if got != c.want {
				t.Errorf("IsOpenAIStreaming(%q) = %v, want %v", c.input, got, c.want)
			}
		})
	}
}

func TestMergeOpenAIStreamingChunks(t *testing.T) {
	input := "data: {\"id\":\"abc\",\"object\":\"chat.completion.chunk\",\"created\":123,\"model\":\"gpt\",\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\ndata: {\"choices\":[{\"delta\":{\"content\":\" world\"},\"finish_reason\":\"stop\"}]}\n"
	merged, err := MergeOpenAIStreamingChunks(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	choices, ok := merged["choices"].([]map[string]interface{})
	if !ok || len(choices) == 0 {
		t.Fatalf("expected choices in merged result")
	}
	msg := choices[0]["message"].(map[string]interface{})
	if msg["content"] != "Hello world" {
		t.Errorf("expected content 'Hello world', got %q", msg["content"])
	}
}

func TestMergeThreadStreamingChunks(t *testing.T) {
	input := "event: thread.run.created\ndata: {\"id\":\"run1\",\"assistant_id\":\"asst1\",\"thread_id\":\"th1\",\"status\":\"queued\",\"created_at\":123}\nevent: thread.message.delta\ndata: {\"delta\":{\"content\":[{\"type\":\"text\",\"text\":{\"value\":\"Hello\"}}]}}\nevent: thread.message.completed\ndata: {\"content\":[{\"type\":\"text\",\"text\":{\"value\":\"Final\"}}]}\n"
	merged, err := MergeThreadStreamingChunks(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	choices, ok := merged["choices"].([]map[string]interface{})
	if !ok || len(choices) == 0 {
		t.Fatalf("expected choices in merged result")
	}
	msg := choices[0]["message"].(map[string]interface{})
	if msg["content"] != "Final" {
		t.Errorf("expected content 'Final', got %q", msg["content"])
	}
}

func TestOpenAITransformer_TransformEvent_OPTIONS(t *testing.T) {
	tr := &OpenAITransformer{}
	evt := map[string]interface{}{"Method": "OPTIONS"}
	out, err := tr.TransformEvent(evt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != nil {
		t.Errorf("expected nil for OPTIONS, got %#v", out)
	}
}

func TestOpenAITransformer_TransformEvent_Basic(t *testing.T) {
	tr := &OpenAITransformer{}
	evt := map[string]interface{}{
		"Method":          "POST",
		"ResponseHeaders": map[string]interface{}{"Content-Type": "application/json"},
		"ResponseBody":    base64.StdEncoding.EncodeToString([]byte(`{"foo":"bar"}`)),
	}
	out, err := tr.TransformEvent(evt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == nil {
		t.Fatal("expected non-nil output")
	}
	if _, ok := out["request_id"]; !ok {
		t.Error("expected request_id to be set")
	}
	if _, ok := out["response_body"]; !ok {
		t.Error("expected response_body to be set")
	}
}

func TestTryBase64DecodeWithLog(t *testing.T) {
	plain := "hello"
	plainB64 := base64.StdEncoding.EncodeToString([]byte(plain))
	plainB64URL := base64.URLEncoding.EncodeToString([]byte(plain))
	jsonStr := `{"foo":1}`

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"plain", plain, plain},
		{"b64", plainB64, plain},
		{"b64url", plainB64URL, plain},
		{"json", jsonStr, jsonStr},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := tryBase64DecodeWithLog(c.input)
			if got != c.want {
				t.Errorf("tryBase64DecodeWithLog(%q) = %q, want %q", c.input, got, c.want)
			}
		})
	}
}

func TestNormalizeToCompactJSON(t *testing.T) {
	obj := map[string]interface{}{"foo": 1, "messages": []string{"hi"}}
	b, _ := json.Marshal(obj)
	compact, msgs, ok := normalizeToCompactJSON(string(b))
	if !ok {
		t.Error("expected ok for valid JSON")
	}
	if compact == "" || msgs == "" {
		t.Error("expected non-empty compact and msgs")
	}

	// Invalid JSON
	_, _, ok = normalizeToCompactJSON("not json")
	if ok {
		t.Error("expected not ok for invalid JSON")
	}
}

func TestIsValidUTF8(t *testing.T) {
	if !isValidUTF8("hello") {
		t.Error("expected valid UTF-8 for 'hello'")
	}
	if isValidUTF8(string([]byte{0xff, 0xfe, 0xfd})) {
		t.Error("expected invalid UTF-8 for 0xff,0xfe,0xfd")
	}
}

func TestOpenAITransformer_TransformEvent_ErrorsAndEdgeCases(t *testing.T) {
	tr := &OpenAITransformer{}
	tests := []struct {
		name  string
		input map[string]interface{}
		check func(t *testing.T, out map[string]interface{}, err error)
	}{
		{
			name: "missing ResponseHeaders",
			input: map[string]interface{}{
				"Method":       "POST",
				"ResponseBody": base64.StdEncoding.EncodeToString([]byte(`{"foo":"bar"}`)),
			},
			check: func(t *testing.T, out map[string]interface{}, err error) {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if out == nil {
					t.Error("expected non-nil output")
				}
			},
		},
		{
			name: "non-JSON response",
			input: map[string]interface{}{
				"Method":          "POST",
				"ResponseHeaders": map[string]interface{}{"Content-Type": "application/json"},
				"ResponseBody":    base64.StdEncoding.EncodeToString([]byte("notjson")),
			},
			check: func(t *testing.T, out map[string]interface{}, err error) {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if out == nil {
					t.Error("expected non-nil output")
				}
				if v, ok := out["response_body"].(string); !ok || v != "notjson" {
					t.Errorf("expected response_body to be 'notjson', got %q", v)
				}
			},
		},
		{
			name: "binary response",
			input: map[string]interface{}{
				"Method":          "POST",
				"ResponseHeaders": map[string]interface{}{"Content-Type": "audio/mpeg"},
				"ResponseBody":    base64.StdEncoding.EncodeToString([]byte("binarydata")),
			},
			check: func(t *testing.T, out map[string]interface{}, err error) {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if out == nil {
					t.Error("expected non-nil output")
				}
				if v, ok := out["response_body"].(string); !ok || v != "[binary or undecodable data]" {
					t.Errorf("expected response_body to be '[binary or undecodable data]', got %q", v)
				}
			},
		},
		{
			name: "streaming response",
			input: map[string]interface{}{
				"Method":          "POST",
				"ResponseHeaders": map[string]interface{}{"Content-Type": "application/json"},
				"ResponseBody":    base64.StdEncoding.EncodeToString([]byte("data: {\"id\":\"abc\",\"object\":\"chat.completion.chunk\",\"created\":123,\"model\":\"gpt\",\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\ndata: {\"choices\":[{\"delta\":{\"content\":\" world\"},\"finish_reason\":\"stop\"}]}\n")),
			},
			check: func(t *testing.T, out map[string]interface{}, err error) {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if out == nil {
					t.Error("expected non-nil output")
				}
				if v, ok := out["response_body"].(string); !ok || v == "" {
					t.Error("expected non-empty response_body for streaming")
				}
			},
		},
		{
			name: "token usage fallback",
			input: map[string]interface{}{
				"Method":          "POST",
				"ResponseHeaders": map[string]interface{}{"Content-Type": "application/json"},
				"ResponseBody":    base64.StdEncoding.EncodeToString([]byte(`{"choices":[{"message":{"content":"hi"}}]}`)),
				"RequestBody":     base64.StdEncoding.EncodeToString([]byte(`{"messages":[{"role":"user","content":"hello"}]}`)),
			},
			check: func(t *testing.T, out map[string]interface{}, err error) {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if out == nil {
					t.Error("expected non-nil output")
				}
				if _, ok := out["token_usage"]; !ok {
					t.Error("expected token_usage to be set")
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := tr.TransformEvent(tt.input)
			tt.check(t, out, err)
		})
	}
}
