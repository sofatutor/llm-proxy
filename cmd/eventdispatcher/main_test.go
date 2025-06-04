package main

import (
	"os"
	"testing"
)

func TestRun_InvalidArg(t *testing.T) {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()
	os.Args = []string{"eventdispatcher", "--notarealflag"}
	_ = run() // Should not panic
}
