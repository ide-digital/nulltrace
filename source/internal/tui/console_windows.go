//go:build windows

package tui

import (
	"os"

	"golang.org/x/sys/windows"
)

// RestoreCookedConsole puts the console back in line-input mode after a
// Bubble Tea screen. The home menu reads whole lines and needs echo + wrap.
func RestoreCookedConsole() {
	in := windows.Handle(os.Stdin.Fd())
	if in != 0 {
		var mode uint32
		if windows.GetConsoleMode(in, &mode) == nil {
			mode |= windows.ENABLE_PROCESSED_INPUT |
				windows.ENABLE_LINE_INPUT |
				windows.ENABLE_ECHO_INPUT |
				windows.ENABLE_EXTENDED_FLAGS
			mode &^= windows.ENABLE_QUICK_EDIT_MODE
			_ = windows.SetConsoleMode(in, mode)
		}
	}
	out := windows.Handle(os.Stdout.Fd())
	if out != 0 {
		var mode uint32
		if windows.GetConsoleMode(out, &mode) == nil {
			mode |= windows.ENABLE_PROCESSED_OUTPUT |
				windows.ENABLE_WRAP_AT_EOL_OUTPUT |
				windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING
			_ = windows.SetConsoleMode(out, mode)
		}
	}
}
