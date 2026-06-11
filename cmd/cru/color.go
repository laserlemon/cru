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
