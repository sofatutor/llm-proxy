package eventtransformer

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/andybalholm/brotli"
	"github.com/sofatutor/llm-proxy/internal/logging"
	"go.uber.org/zap"
)

// Package-level logger for event transformer
var logger *zap.Logger

func init() {
	// Initialize with a default logger, can be overridden with SetLogger
	var err error
	logger, err = logging.NewComponentLogger("info", "json", "", logging.ComponentEventBus)
	if err != nil {
		// Fallback to a no-op logger if initialization fails
		logger = zap.NewNop()
	}
}

// SetLogger allows setting a custom logger for the package
func SetLogger(l *zap.Logger) {
	logger = l.With(zap.String(logging.FieldComponent, logging.ComponentEventBus))
}

// DecompressAndDecode attempts to decompress (gzip, brotli) if needed, then base64 decode if needed, and returns the decoded string and true if decoding was successful.
func DecompressAndDecode(val string, headers map[string]interface{}) (string, bool) {
	// Only log errors, binary skipping, and major state changes
	data := []byte(val)
	encoding := ""
	contentType := ""
	for k, v := range headers {
		key := strings.ToLower(strings.ReplaceAll(k, "-", "_"))
		if key == "content_encoding" {
			if arr, ok := v.([]interface{}); ok && len(arr) > 0 {
				if s, ok := arr[0].(string); ok {
					encoding = strings.ToLower(s)
				}
			} else if s, ok := v.(string); ok {
				encoding = strings.ToLower(s)
			}
		}
		if key == "content_type" {
			if arr, ok := v.([]interface{}); ok && len(arr) > 0 {
				if s, ok := arr[0].(string); ok {
					contentType = strings.ToLower(s)
				}
			} else if s, ok := v.(string); ok {
				contentType = strings.ToLower(s)
			}
		}
	}
	// Only log binary skipping
	if strings.HasPrefix(contentType, "audio/") || strings.HasPrefix(contentType, "image/") || contentType == "application/octet-stream" {
		logger.Debug("Skipping decode for binary content-type", zap.String("content_type", contentType))
		return val, false
	}

	// 1. Try base64 decode (standard)
	decoded, err := base64.StdEncoding.DecodeString(string(data))
	if err == nil {
		data = decoded
		// Use tagged switch for encoding
		switch encoding {
		case "gzip":
			zr, err := gzip.NewReader(bytes.NewReader(data))
			if err == nil {
				decompressed, err := io.ReadAll(zr)
				_ = zr.Close()
				if err == nil {
					data = decompressed
				}
			}
		case "br":
			br := brotli.NewReader(bytes.NewReader(data))
			decompressed, err := io.ReadAll(br)
			if err == nil {
				data = decompressed
			}
		}
		if json.Valid(data) {
			return string(data), true
		}
		if utf8.Valid(data) {
			return string(data), true
		}
	}

	// 2. Try base64 decode (URL-safe)
	decoded, err = base64.URLEncoding.DecodeString(string(data))
	if err == nil {
		data = decoded
		// Use tagged switch for encoding
		switch encoding {
		case "gzip":
			zr, err := gzip.NewReader(bytes.NewReader(data))
			if err == nil {
				decompressed, err := io.ReadAll(zr)
				_ = zr.Close()
				if err == nil {
					data = decompressed
				}
			}
		case "br":
			br := brotli.NewReader(bytes.NewReader(data))
			decompressed, err := io.ReadAll(br)
			if err == nil {
				data = decompressed
			}
		}
		if json.Valid(data) {
			return string(data), true
		}
		if utf8.Valid(data) {
			return string(data), true
		}
	}

	// 3. If base64 fails, try decompressing original data (legacy case)
	switch encoding {
	case "gzip":
		zr, err := gzip.NewReader(bytes.NewReader([]byte(val)))
		if err == nil {
			decompressed, err := io.ReadAll(zr)
			_ = zr.Close()
			if err == nil {
				if json.Valid(decompressed) {
					return string(decompressed), true
				}
				if utf8.Valid(decompressed) {
					return string(decompressed), true
				}
			}
		}
	case "br":
		br := brotli.NewReader(bytes.NewReader([]byte(val)))
		decompressed, err := io.ReadAll(br)
		if err == nil {
			if json.Valid(decompressed) {
				return string(decompressed), true
			}
			if utf8.Valid(decompressed) {
				return string(decompressed), true
			}
		}
	}

	// 4. Fallback: check if input is JSON or UTF-8
	if json.Valid([]byte(val)) {
		return val, true
	}
	if utf8.Valid([]byte(val)) {
			return val, true
	}
	logger.Debug("Fallback: returning original input, could not decode", zap.String("input_length", string(rune(len(val)))))
	return val, false
}
