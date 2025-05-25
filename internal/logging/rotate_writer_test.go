package logging

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestRotateWriter_BasicWriteAndRotate(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")
	rw, err := newRotateWriter(logPath, 50, 2)
	if err != nil {
		t.Fatalf("failed to create rotateWriter: %v", err)
	}
	defer rw.file.Close()

	// Write below maxSize
	msg := []byte("hello world\n")
	n, err := rw.Write(msg)
	if err != nil || n != len(msg) {
		t.Errorf("Write() = %d, %v; want %d, nil", n, err, len(msg))
	}

	// Write enough to trigger rotation
	big := make([]byte, 60)
	for i := range big {
		big[i] = 'x'
	}
	n, err = rw.Write(big)
	if err != nil {
		t.Errorf("Write() after rotation error: %v", err)
	}
	// Should have .1 backup
	if _, err := os.Stat(logPath + ".1"); err != nil {
		t.Errorf("expected rotated file: %v", err)
	}
}

func TestRotateWriter_SyncAndOpenErrors(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")
	rw, err := newRotateWriter(logPath, 100, 1)
	if err != nil {
		t.Fatalf("failed to create rotateWriter: %v", err)
	}
	defer rw.file.Close()
	// Sync should succeed
	if err := rw.Sync(); err != nil {
		t.Errorf("Sync() error: %v", err)
	}
	// Close file and test Sync on closed file
	rw.file.Close()
	rw.file = nil
	if err := rw.Sync(); err != nil {
		t.Errorf("Sync() on nil file error: %v", err)
	}
}

func TestRotateWriter_RotateErrors(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")
	_ = ioutil.WriteFile(logPath, []byte("test"), 0644)
	rw, err := newRotateWriter(logPath, 10, 1)
	if err != nil {
		t.Fatalf("failed to create rotateWriter: %v", err)
	}
	defer rw.file.Close()
	// Remove file to cause rename error
	os.Remove(logPath)
	err = rw.rotate()
	// Should not panic, may return error
}
