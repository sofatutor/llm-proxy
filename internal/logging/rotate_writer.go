package logging

import (
	"fmt"
	"os"
	"sync"
)

type rotateWriter struct {
	path        string
	maxSize     int64
	maxBackups  int
	mu          sync.Mutex
	file        *os.File
	currentSize int64 // Track current file size in memory
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
	// Update currentSize to match the file's actual size
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	rw.currentSize = fi.Size()
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
	if rw.currentSize+int64(len(p)) > rw.maxSize {
		errClose := rw.file.Close()
		if errClose != nil {
			return 0, errClose
		}
		if err := rw.rotate(); err != nil {
			// Return the error to the caller
			return 0, err
		}
		if err := rw.open(); err != nil {
			return 0, err
		}
	}
	n, err := rw.file.Write(p)
	rw.currentSize += int64(n)
	return n, err
}

func (rw *rotateWriter) rotate() error {
	var rotateErr error
	for i := rw.maxBackups - 1; i >= 1; i-- {
		old := fmt.Sprintf("%s.%d", rw.path, i)
		new := fmt.Sprintf("%s.%d", rw.path, i+1)
		if _, err := os.Stat(old); err == nil {
			if err := os.Rename(old, new); err != nil {
				fmt.Printf("log rotation rename error: %v\n", err)
				rotateErr = err
			}
		}
	}
	if err := os.Rename(rw.path, fmt.Sprintf("%s.1", rw.path)); err != nil {
		fmt.Printf("log rotation rename error: %v\n", err)
		rotateErr = err
	}
	return rotateErr
}

func (rw *rotateWriter) Sync() error {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	if rw.file != nil {
		return rw.file.Sync()
	}
	return nil
}
