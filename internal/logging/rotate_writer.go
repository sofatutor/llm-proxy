package logging

import (
	"fmt"
	"os"
	"sync"
)

type rotateWriter struct {
	path       string
	maxSize    int64
	maxBackups int
	mu         sync.Mutex
	file       *os.File
}

func newRotateWriter(path string, maxSize int64, maxBackups int) (*rotateWriter, error) {
	if maxSize <= 0 {
		maxSize = 10 * 1024 * 1024
	}
	if maxBackups <= 0 {
		maxBackups = 5
	}
	rw := &rotateWriter{path: path, maxSize: maxSize, maxBackups: maxBackups}
	if err := rw.open(); err != nil {
		return nil, err
	}
	return rw, nil
}

func (rw *rotateWriter) open() error {
	f, err := os.OpenFile(rw.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	rw.file = f
	return nil
}

func (rw *rotateWriter) Write(p []byte) (int, error) {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	if rw.file == nil {
		if err := rw.open(); err != nil {
			return 0, err
		}
	}
	fi, err := rw.file.Stat()
	if err == nil && fi.Size()+int64(len(p)) > rw.maxSize {
		rw.file.Close()
		rw.rotate()
		if err := rw.open(); err != nil {
			return 0, err
		}
	}
	return rw.file.Write(p)
}

func (rw *rotateWriter) rotate() {
	for i := rw.maxBackups - 1; i >= 1; i-- {
		old := fmt.Sprintf("%s.%d", rw.path, i)
		new := fmt.Sprintf("%s.%d", rw.path, i+1)
		if _, err := os.Stat(old); err == nil {
			_ = os.Rename(old, new)
		}
	}
	_ = os.Rename(rw.path, fmt.Sprintf("%s.1", rw.path))
}

func (rw *rotateWriter) Sync() error {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	if rw.file != nil {
		return rw.file.Sync()
	}
	return nil
}
