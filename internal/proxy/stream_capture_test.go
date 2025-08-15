package proxy

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

// helper to create a ReadCloser from bytes
type nopReadCloser struct{ *bytes.Reader }

func (n nopReadCloser) Close() error { return nil }

func TestStreamingCapture_Read_FinalizeOnEOF(t *testing.T) {
	src := nopReadCloser{bytes.NewReader([]byte("hello world"))}
	var got []byte
	sc := newStreamingCapture(src, 0, func(b []byte) { got = append([]byte(nil), b...) })

	buf := make([]byte, 64)
	for {
		_, err := sc.Read(buf)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("unexpected read error: %v", err)
		}
	}
	if string(got) != "hello world" {
		t.Fatalf("unexpected captured: %q", string(got))
	}
}

func TestStreamingCapture_Close_FinalizeOnce(t *testing.T) {
	src := nopReadCloser{bytes.NewReader([]byte("abcdef"))}
	calls := 0
	sc := newStreamingCapture(src, 0, func(b []byte) { calls++ })

	// Call Close before EOF; finalize should run once
	if err := sc.Close(); err != nil {
		t.Fatalf("close error: %v", err)
	}
	// Further reads should still return from underlying reader
	_, _ = sc.Read(make([]byte, 1))
	if calls != 1 {
		t.Fatalf("finalize calls = %d, want 1", calls)
	}
	// Second Close should not increase calls
	_ = sc.Close()
	if calls != 1 {
		t.Fatalf("finalize called multiple times: %d", calls)
	}
}

func TestStreamingCapture_MaxBytesLimit(t *testing.T) {
	payload := []byte("0123456789")
	src := nopReadCloser{bytes.NewReader(payload)}
	var got []byte
	sc := newStreamingCapture(src, 4, func(b []byte) { got = append([]byte(nil), b...) })

	buf := make([]byte, 3)
	for {
		_, err := sc.Read(buf)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("read error: %v", err)
		}
	}
	if string(got) != "0123" {
		t.Fatalf("captured = %q, want '0123'", string(got))
	}
}
