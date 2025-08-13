package eventtransformer

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"testing"
)

func TestDecompressAndDecode_GzipJSONAndFallback(t *testing.T) {
	// Build gzip JSON
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, _ = zw.Write([]byte(`{"x":1}`))
	_ = zw.Close()

	headers := map[string]any{
		"Content-Encoding": "gzip",
		"Content-Type":     "application/json",
	}
	out, ok := DecompressAndDecode(base64.StdEncoding.EncodeToString(buf.Bytes()), headers)
	if !ok || out != "{\"x\":1}" {
		t.Fatalf("expected gzip json decode, got ok=%v out=%q", ok, out)
	}

	// Fallback: non-decodable binary
	_, ok = DecompressAndDecode(base64.StdEncoding.EncodeToString([]byte{0xff, 0xfe}), map[string]any{"Content-Type": "application/octet-stream"})
	if ok {
		t.Fatalf("expected not ok for binary content")
	}
}
