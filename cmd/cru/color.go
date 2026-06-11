package main

import "os"

// colorize wraps s in the ANSI bright-black (gray) escape, then resets.
// Returns s unchanged when NO_COLOR is set, so the only place that needs
// to think about color is here.
//
// 90 is bright black (gray) in the standard 16-color palette, which every
// terminal emulator built since 1995 supports natively without needing
// 256-color or truecolor extensions.
func colorize(s string) string {
	if os.Getenv("NO_COLOR") != "" {
		return s
	}
	return "\x1b[90m" + s + "\x1b[0m"
}

// bold wraps s in the ANSI bold escape, then resets. Honors NO_COLOR
// (bold is a style attribute, not a color, but the NO_COLOR spec
// explicitly covers bold/dim/italic/etc. as part of "decoration").
func bold(s string) string {
	if os.Getenv("NO_COLOR") != "" {
		return s
	}
	return "\x1b[1m" + s + "\x1b[0m"
}
