package main

import (
	"io"
	"os"
)

// isTTY returns true when w is *os.Stdout (or similar) attached to a
// character device, with respect for NO_COLOR (always off) and
// FORCE_COLOR=1 (always on).
//
// Anything that isn't an *os.File (test buffers, pipes captured into
// io.Writers) returns false: callers wanting color in tests should pass
// FORCE_COLOR=1 in the environment, or hand-write the ANSI directly.
func isTTY(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("FORCE_COLOR") == "1" {
		return true
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// isPipe returns true when r is *os.Stdin (or any *os.File) and the
// underlying descriptor is a pipe or redirected file. Used to trigger
// stdin batch mode only when there is actually data waiting.
//
// Mirrors isTTY's logic in reverse: ModeCharDevice means an interactive
// terminal; anything else (pipe, regular file, /dev/null) qualifies.
func isPipe(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice == 0
}
