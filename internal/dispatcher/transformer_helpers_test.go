package dispatcher

import (
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
        "ok":   "text",
        "s":    string([]byte{0xff, 0xfe}),
        "bytes": []byte{0xff, 0xfe},
        "arr":  []any{"x", []byte{0xff}},
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


