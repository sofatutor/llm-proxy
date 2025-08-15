package proxy

import (
	"bytes"
	"io"
	"sync/atomic"
)

// streamingCaptureReadCloser wraps an io.ReadCloser and captures the bytes
// read into an internal buffer. Once EOF or Close is reached, it invokes
// the provided finalize callback with the captured bytes (if any).
type streamingCaptureReadCloser struct {
	rc        io.ReadCloser
	buf       *bytes.Buffer
	maxBytes  int64
	written   int64
	finalized int32
	onDone    func([]byte)
}

func newStreamingCapture(rc io.ReadCloser, maxBytes int64, onDone func([]byte)) *streamingCaptureReadCloser {
	return &streamingCaptureReadCloser{
		rc:       rc,
		buf:      &bytes.Buffer{},
		maxBytes: maxBytes,
		onDone:   onDone,
	}
}

func (s *streamingCaptureReadCloser) Read(p []byte) (int, error) {
	n, err := s.rc.Read(p)
	if n > 0 && (s.maxBytes == 0 || s.written < s.maxBytes) {
		limit := n
		remaining := s.maxBytes - s.written
		if s.maxBytes > 0 && int64(limit) > remaining {
			limit = int(remaining)
		}
		_, _ = s.buf.Write(p[:limit])
		s.written += int64(limit)
	}
	if err == io.EOF {
		s.finalize()
	}
	return n, err
}

func (s *streamingCaptureReadCloser) Close() error {
	// Ensure finalize happens once even if Close is called before EOF
	s.finalize()
	return s.rc.Close()
}

func (s *streamingCaptureReadCloser) finalize() {
	if atomic.CompareAndSwapInt32(&s.finalized, 0, 1) {
		if s.onDone != nil {
			s.onDone(s.buf.Bytes())
		}
	}
}
