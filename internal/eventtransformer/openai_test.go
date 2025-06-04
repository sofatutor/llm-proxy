package eventtransformer

import (
	"encoding/base64"
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
