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

func TestTryBase64DecodeWithLog_FallbackBranch(t *testing.T) {
	input := "not_base64_but_valid_utf8"
	got := tryBase64DecodeWithLog(input)
	if got != input {
		t.Errorf("expected fallback to input, got %q", got)
	}
}

func TestTryBase64DecodeWithLog_JSONAndInvalidUTF8(t *testing.T) {
	jsonInput := `{"foo":1}`
	if got := tryBase64DecodeWithLog(jsonInput); got != jsonInput {
		t.Errorf("expected JSON passthrough, got %q", got)
	}
	invalidUTF8 := base64.StdEncoding.EncodeToString([]byte{0xff, 0xfe, 0xfd})
	got := tryBase64DecodeWithLog(invalidUTF8)
	if got != string([]byte{0xff, 0xfe, 0xfd}) {
		t.Errorf("expected decoded invalid UTF-8, got %q", got)
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

func TestNormalizeToCompactJSON_AllBranches(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantOK   bool
		wantMsgs string
	}{
		{
			name:   "not JSON",
			input:  "not json",
			wantOK: false,
		},
		{
			name:     "JSON string with valid JSON and messages",
			input:    `"{\"messages\":[{\"role\":\"user\"}]}"`,
			wantOK:   true,
			wantMsgs: `[{"role":"user"}]`,
		},
		{
			name:   "JSON string with invalid JSON",
			input:  `"not valid json"`,
			wantOK: true,
		},
		{
			name:     "JSON object with messages",
			input:    `{"messages":[{"role":"user"}]}`,
			wantOK:   true,
			wantMsgs: `[{"role":"user"}]`,
		},
		{
			name:   "JSON object without messages",
			input:  `{"foo":1}`,
			wantOK: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compact, msgs, ok := normalizeToCompactJSON(tt.input)
			if ok != tt.wantOK {
				t.Errorf("wantOK=%v, got %v", tt.wantOK, ok)
			}
			if tt.wantMsgs != "" && msgs != tt.wantMsgs {
				t.Errorf("wantMsgs=%q, got %q", tt.wantMsgs, msgs)
			}
			if compact == "" && ok {
				t.Error("expected non-empty compact for ok=true")
			}
		})
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

func TestMergeOpenAIStreamingChunks_EdgeCases(t *testing.T) {
	cases := []struct {
		name  string
		input string
		check func(t *testing.T, merged map[string]any, err error)
	}{
		{
			name:  "malformed JSON line",
			input: "data: {notjson}\ndata: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}",
			check: func(t *testing.T, merged map[string]any, err error) {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if merged["choices"] == nil {
					t.Error("expected choices in merged result")
				}
			},
		},
		{
			name:  "choices empty",
			input: "data: {\"choices\":[]}",
			check: func(t *testing.T, merged map[string]any, err error) {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				choices := merged["choices"].([]map[string]any)
				if len(choices) == 0 {
					t.Error("expected at least one choice")
				}
			},
		},
		{
			name:  "choices missing delta/content",
			input: "data: {\"choices\":[{}]}",
			check: func(t *testing.T, merged map[string]any, err error) {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			},
		},
		{
			name:  "usage all zero",
			input: "data: {\"usage\":{\"prompt_tokens\":0,\"completion_tokens\":0,\"total_tokens\":0},\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}",
			check: func(t *testing.T, merged map[string]any, err error) {
				if _, ok := merged["usage"]; ok {
					t.Error("usage should not be present if all fields zero")
				}
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			merged, err := MergeOpenAIStreamingChunks(c.input)
			c.check(t, merged, err)
		})
	}
}

func TestMergeThreadStreamingChunks_EdgeCases(t *testing.T) {
	cases := []struct {
		name  string
		input string
		check func(t *testing.T, merged map[string]any, err error)
	}{
		{
			name:  "missing data line",
			input: "event: thread.run.created",
			check: func(t *testing.T, merged map[string]any, err error) {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			},
		},
		{
			name:  "malformed data line",
			input: "event: thread.run.created\ndata: notjson",
			check: func(t *testing.T, merged map[string]any, err error) {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			},
		},
		{
			name:  "delta missing or not a slice",
			input: "event: thread.message.delta\ndata: {\"delta\":{}}",
			check: func(t *testing.T, merged map[string]any, err error) {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			},
		},
		{
			name:  "completed content not a slice",
			input: "event: thread.message.completed\ndata: {\"content\":123}",
			check: func(t *testing.T, merged map[string]any, err error) {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			},
		},
		{
			name:  "completed content missing text",
			input: "event: thread.message.completed\ndata: {\"content\":[{\"type\":\"text\"}]}",
			check: func(t *testing.T, merged map[string]any, err error) {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			},
		},
		{
			name:  "unknown event type",
			input: "event: unknown.type\ndata: {\"foo\":1}",
			check: func(t *testing.T, merged map[string]any, err error) {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			merged, err := MergeThreadStreamingChunks(c.input)
			c.check(t, merged, err)
		})
	}
}

func TestMergeThreadStreamingChunks_MoreBranches(t *testing.T) {
	cases := []struct {
		name  string
		input string
		check func(t *testing.T, merged map[string]any, err error)
	}{
		{
			name:  "event: thread.run.step.completed with usage",
			input: "event: thread.run.step.completed\ndata: {\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":2,\"total_tokens\":3}}",
			check: func(t *testing.T, merged map[string]any, err error) {
				usage, ok := merged["usage"].(map[string]any)
				if !ok || usage["prompt_tokens"] != 1 {
					t.Errorf("expected usage to be set with prompt_tokens=1, got %v", usage["prompt_tokens"])
				}
			},
		},
		{
			name:  "event: thread.message.delta with delta missing",
			input: "event: thread.message.delta\ndata: {\"foo\":1}",
			check: func(t *testing.T, merged map[string]any, err error) {
				// Should not panic, just no content
			},
		},
		{
			name:  "event: thread.message.delta with content not a slice",
			input: "event: thread.message.delta\ndata: {\"delta\":{\"content\":123}}",
			check: func(t *testing.T, merged map[string]any, err error) {
				// Should not panic, just no content
			},
		},
		{
			name:  "event: thread.message.completed with content not a slice",
			input: "event: thread.message.completed\ndata: {\"content\":123}",
			check: func(t *testing.T, merged map[string]any, err error) {
				// Should not panic, just no content
			},
		},
		{
			name:  "event: thread.run.created with missing fields",
			input: "event: thread.run.created\ndata: {\"foo\":1}",
			check: func(t *testing.T, merged map[string]any, err error) {
				// Should not panic, just missing fields
			},
		},
		{
			name:  "event: unknown type",
			input: "event: unknown.type\ndata: {\"foo\":1}",
			check: func(t *testing.T, merged map[string]any, err error) {
				// Should not panic, just no effect
			},
		},
		{
			name:  "input with no events",
			input: "just some text\nno events here",
			check: func(t *testing.T, merged map[string]any, err error) {
				// Should not panic, merged should be mostly empty
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			merged, err := MergeThreadStreamingChunks(c.input)
			c.check(t, merged, err)
		})
	}
}

func TestOpenAITransformer_TransformEvent_ExtraBranches(t *testing.T) {
	tr := &OpenAITransformer{}
	tests := []struct {
		name  string
		input map[string]interface{}
		check func(t *testing.T, out map[string]interface{}, err error)
	}{
		{
			name:  "missing ResponseBody",
			input: map[string]interface{}{"Method": "POST", "ResponseHeaders": map[string]interface{}{"Content-Type": "application/json"}},
			check: func(t *testing.T, out map[string]interface{}, err error) {
				if out != nil && out["response_body"] != nil && out["response_body"] != "" {
					t.Error("expected response_body to be absent or empty for missing ResponseBody")
				}
			},
		},
		{
			name:  "ResponseBody not string",
			input: map[string]interface{}{"Method": "POST", "ResponseHeaders": map[string]interface{}{"Content-Type": "application/json"}, "ResponseBody": 123},
			check: func(t *testing.T, out map[string]interface{}, err error) {
				if out != nil && out["response_body"] != nil && out["response_body"] != "" {
					t.Error("expected response_body to be absent or empty for non-string ResponseBody")
				}
			},
		},
		{
			name:  "binary undecodable data",
			input: map[string]interface{}{"Method": "POST", "ResponseHeaders": map[string]interface{}{"Content-Type": "audio/mpeg"}, "ResponseBody": base64.StdEncoding.EncodeToString([]byte{0xff, 0xfe, 0xfd})},
			check: func(t *testing.T, out map[string]interface{}, err error) {
				if out == nil || out["response_body"] != "[binary or undecodable data]" {
					t.Error("expected '[binary or undecodable data]' for binary undecodable")
				}
			},
		},
		{
			name:  "token usage fallback",
			input: map[string]interface{}{"Method": "POST", "ResponseHeaders": map[string]interface{}{"Content-Type": "application/json"}, "ResponseBody": base64.StdEncoding.EncodeToString([]byte(`{"choices":[{"message":{"content":"hi"}}]}`)), "RequestBody": "notbase64"},
			check: func(t *testing.T, out map[string]interface{}, err error) {
				if _, ok := out["token_usage"]; !ok {
					t.Error("expected token_usage to be set even if RequestBody is not base64")
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

func TestTryBase64DecodeWithLogAndNormalizeToCompactJSON_EdgeCases(t *testing.T) {
	bad := string([]byte{0xff, 0xfe, 0xfd})
	if got := tryBase64DecodeWithLog(bad); got != bad {
		t.Errorf("tryBase64DecodeWithLog: want %q, got %q", bad, got)
	}
	if _, _, ok := normalizeToCompactJSON(bad); ok {
		t.Error("normalizeToCompactJSON: expected not ok for invalid UTF-8/JSON")
	}
}

func TestOpenAITransformer_TransformEvent_MoreBranches(t *testing.T) {
	tr := &OpenAITransformer{}
	tests := []struct {
		name  string
		input map[string]interface{}
		check func(t *testing.T, out map[string]interface{}, err error)
	}{
		{
			name:  "RequestHeaders as non-map",
			input: map[string]interface{}{"Method": "POST", "RequestHeaders": 123, "ResponseHeaders": map[string]interface{}{"Content-Type": "application/json"}, "ResponseBody": base64.StdEncoding.EncodeToString([]byte(`{"foo":"bar"}`))},
			check: func(t *testing.T, out map[string]interface{}, err error) {
				if out == nil {
					t.Error("expected non-nil output")
				}
			},
		},
		{
			name:  "ResponseHeaders as non-map",
			input: map[string]interface{}{"Method": "POST", "ResponseHeaders": 123, "ResponseBody": base64.StdEncoding.EncodeToString([]byte(`{"foo":"bar"}`))},
			check: func(t *testing.T, out map[string]interface{}, err error) {
				if out == nil {
					t.Error("expected non-nil output")
				}
			},
		},
		{
			name:  "RequestBody as []byte",
			input: map[string]interface{}{"Method": "POST", "RequestHeaders": map[string]interface{}{}, "ResponseHeaders": map[string]interface{}{"Content-Type": "application/json"}, "RequestBody": []byte("hello")},
			check: func(t *testing.T, out map[string]interface{}, err error) {
				if out == nil || out["request_body"] != "hello" {
					t.Errorf("expected request_body to be 'hello', got %v", out["request_body"])
				}
			},
		},
		{
			name:  "ResponseBody as []byte",
			input: map[string]interface{}{"Method": "POST", "RequestHeaders": map[string]interface{}{}, "ResponseHeaders": map[string]interface{}{"Content-Type": "application/json"}, "ResponseBody": []byte("hello")},
			check: func(t *testing.T, out map[string]interface{}, err error) {
				// Should not panic, just skip response_body
			},
		},
		{
			name:  "ResponseBody not base64, not decodable, not valid UTF-8",
			input: map[string]interface{}{"Method": "POST", "RequestHeaders": map[string]interface{}{}, "ResponseHeaders": map[string]interface{}{"Content-Type": "application/json"}, "ResponseBody": string([]byte{0xff, 0xfe, 0xfd})},
			check: func(t *testing.T, out map[string]interface{}, err error) {
				if out == nil || out["response_body"] != "[binary or undecodable data]" {
					t.Errorf("expected '[binary or undecodable data]', got %v", out["response_body"])
				}
			},
		},
		{
			name:  "Token usage fallback with non-JSON response_body",
			input: map[string]interface{}{"Method": "POST", "RequestHeaders": map[string]interface{}{}, "ResponseHeaders": map[string]interface{}{"Content-Type": "application/json"}, "ResponseBody": base64.StdEncoding.EncodeToString([]byte("notjson"))},
			check: func(t *testing.T, out map[string]interface{}, err error) {
				if _, ok := out["token_usage"]; ok {
					t.Error("token_usage should not be set for non-JSON response_body")
				}
			},
		},
		{
			name:  "Token usage fallback with empty response_body",
			input: map[string]interface{}{"Method": "POST", "RequestHeaders": map[string]interface{}{}, "ResponseHeaders": map[string]interface{}{"Content-Type": "application/json"}, "ResponseBody": base64.StdEncoding.EncodeToString([]byte(""))},
			check: func(t *testing.T, out map[string]interface{}, err error) {
				if _, ok := out["token_usage"]; ok {
					t.Error("token_usage should not be set for empty response_body")
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
