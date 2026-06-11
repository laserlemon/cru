package main

import (
	"os"
	"testing"
)

// mustPipeReader returns an *os.File whose Stat reports a non-character
// device (== isPipe true). Used to exercise the stdin-batch path
// without an actual subshell.
func mustPipeReader(t *testing.T, contents string) *os.File {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	go func() {
		_, _ = w.WriteString(contents)
		_ = w.Close()
	}()
	return r
}
