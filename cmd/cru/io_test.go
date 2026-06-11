package main

import (
	"bytes"
	"os"
	"testing"
)

func TestIsTTY(t *testing.T) {
	// non-File writer never a TTY.
	if isTTY(&bytes.Buffer{}) {
		t.Error("buffer should not be a TTY")
	}

	t.Run("NO_COLOR wins", func(t *testing.T) {
		t.Setenv("NO_COLOR", "1")
		t.Setenv("FORCE_COLOR", "1")
		if isTTY(os.Stdout) {
			t.Error("NO_COLOR=1 should force false even with FORCE_COLOR=1")
		}
	})

	t.Run("FORCE_COLOR forces true on non-File", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")
		t.Setenv("FORCE_COLOR", "1")
		if !isTTY(&bytes.Buffer{}) {
			t.Error("FORCE_COLOR=1 should force true even on a non-File writer")
		}
	})

	t.Run("regular file is not TTY", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")
		t.Setenv("FORCE_COLOR", "")
		f, err := os.CreateTemp(t.TempDir(), "tty-test")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if isTTY(f) {
			t.Error("regular file should not be a TTY")
		}
	})
}

func TestIsPipe(t *testing.T) {
	// non-File reader is not a pipe.
	if isPipe(&bytes.Buffer{}) {
		t.Error("buffer should not be a pipe")
	}
	// os.Pipe IS a pipe.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	if !isPipe(r) {
		t.Error("os.Pipe reader should be a pipe")
	}
}

func TestColorize(t *testing.T) {
	t.Run("default colors", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")
		got := colorize("foo")
		if got != "\x1b[90mfoo\x1b[0m" {
			t.Errorf("colorize = %q, want ANSI-wrapped", got)
		}
	})
	t.Run("NO_COLOR strips", func(t *testing.T) {
		t.Setenv("NO_COLOR", "1")
		got := colorize("foo")
		if got != "foo" {
			t.Errorf("colorize with NO_COLOR = %q, want plain", got)
		}
	})
}

func TestBold(t *testing.T) {
	t.Run("default bold", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")
		got := bold("foo")
		if got != "\x1b[1mfoo\x1b[0m" {
			t.Errorf("bold = %q, want ANSI-wrapped", got)
		}
	})
	t.Run("NO_COLOR strips", func(t *testing.T) {
		t.Setenv("NO_COLOR", "1")
		got := bold("foo")
		if got != "foo" {
			t.Errorf("bold with NO_COLOR = %q, want plain", got)
		}
	})
}

func TestBoldGray(t *testing.T) {
	t.Run("default bold gray", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")
		got := boldGray("foo")
		if got != "\x1b[1;90mfoo\x1b[0m" {
			t.Errorf("boldGray = %q, want ANSI-wrapped", got)
		}
	})
	t.Run("NO_COLOR strips", func(t *testing.T) {
		t.Setenv("NO_COLOR", "1")
		got := boldGray("foo")
		if got != "foo" {
			t.Errorf("boldGray with NO_COLOR = %q, want plain", got)
		}
	})
}
