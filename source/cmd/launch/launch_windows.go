//go:build windows

package main

import (
	"strings"

	"golang.org/x/sys/windows"
)

func engineName() string { return "nulltrace.exe" }

func launchEngine(root, engine string, extra []string) error {
	// ShellExecute is what Explorer uses for a console .exe — a real window
	// with a keyboard. CreateProcess from a GUI parent wires stdin to NUL.
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(engine)
	if err != nil {
		return err
	}
	params, err := windows.UTF16PtrFromString(strings.TrimSpace("--app " + strings.Join(extra, " ")))
	if err != nil {
		return err
	}
	dir, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, verb, file, params, dir, windows.SW_SHOWNORMAL)
}

func showError(msg string) {
	text, err1 := windows.UTF16PtrFromString(msg)
	caption, err2 := windows.UTF16PtrFromString("NullTrace")
	if err1 != nil || err2 != nil {
		return
	}
	_, _ = windows.MessageBox(0, text, caption, windows.MB_OK|windows.MB_ICONERROR)
}
