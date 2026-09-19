//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var pinnedStd [3]*os.File

func prepareConsole() {
	k32 := windows.NewLazySystemDLL("kernel32.dll")
	if alloc := k32.NewProc("AllocConsole"); alloc.Find() == nil {
		hwnd, _, _ := k32.NewProc("GetConsoleWindow").Call()
		if hwnd == 0 {
			_, _, _ = alloc.Call()
		}
	}

	if err := bindConsoleIO(); err != nil {
		bootLog("bind console: %v", err)
		showEngineError("NullTrace couldn't attach to the console.\n\n" + err.Error())
		return
	}
	bootLog("console attached stdin_terminal=%v", isStdinTerminal())

	if p := k32.NewProc("SetConsoleTitleW"); p.Find() == nil {
		if title, err := windows.UTF16PtrFromString("NULLTRACE"); err == nil {
			_, _, _ = p.Call(uintptr(unsafe.Pointer(title)))
		}
	}
	_ = windows.SetConsoleOutputCP(65001)
	_ = windows.SetConsoleCP(65001)

	out := windows.Handle(os.Stdout.Fd())
	if out != 0 {
		sizeConsole(out, k32)
		var mode uint32
		if windows.GetConsoleMode(out, &mode) == nil {
			mode |= windows.ENABLE_PROCESSED_OUTPUT |
				windows.ENABLE_WRAP_AT_EOL_OUTPUT |
				windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING
			_ = windows.SetConsoleMode(out, mode)
		}
	}

	in := windows.Handle(os.Stdin.Fd())
	if in != 0 {
		var mode uint32
		if windows.GetConsoleMode(in, &mode) == nil {
			// Keep line-input + echo so the home menu can read a line after
			// the login TUI exits. Only disable Quick Edit (mouse select
			// swallows keystrokes).
			mode |= windows.ENABLE_EXTENDED_FLAGS |
				windows.ENABLE_PROCESSED_INPUT |
				windows.ENABLE_LINE_INPUT |
				windows.ENABLE_ECHO_INPUT
			mode &^= windows.ENABLE_QUICK_EDIT_MODE
			_ = windows.SetConsoleMode(in, mode)
		}
		if p := k32.NewProc("FlushConsoleInputBuffer"); p.Find() == nil {
			_, _, _ = p.Call(uintptr(in))
		}
	}
}

// sizeConsole shrinks the screen buffer to the visible window. It does not
// move or resize the window — collapsing it to 1×1 was blinking the boot
// ASCII off and making Bubble Tea lay out against a 2-row terminal.
func sizeConsole(out windows.Handle, k32 *windows.LazyDLL) {
	var info windows.ConsoleScreenBufferInfo
	if windows.GetConsoleScreenBufferInfo(out, &info) != nil {
		return
	}
	cols := info.Window.Right - info.Window.Left + 1
	rows := info.Window.Bottom - info.Window.Top + 1
	if cols < 1 || rows < 1 {
		return
	}
	if info.Size.X == cols && info.Size.Y == rows {
		return
	}
	packed := uintptr(uint32(uint16(cols)) | uint32(uint16(rows))<<16)
	if p := k32.NewProc("SetConsoleScreenBufferSize"); p.Find() == nil {
		_, _, _ = p.Call(uintptr(out), packed)
	}
}

func bindConsoleIO() error {
	in, err := openCon("CONIN$")
	if err != nil {
		return err
	}
	out, err := openCon("CONOUT$")
	if err != nil {
		_ = windows.CloseHandle(in)
		return err
	}
	errf, err := openCon("CONOUT$")
	if err != nil {
		_ = windows.CloseHandle(in)
		_ = windows.CloseHandle(out)
		return err
	}

	inFile := os.NewFile(uintptr(in), "stdin")
	outFile := os.NewFile(uintptr(out), "stdout")
	errFile := os.NewFile(uintptr(errf), "stderr")
	pinnedStd = [3]*os.File{inFile, outFile, errFile}

	_ = windows.SetStdHandle(windows.STD_INPUT_HANDLE, in)
	_ = windows.SetStdHandle(windows.STD_OUTPUT_HANDLE, out)
	_ = windows.SetStdHandle(windows.STD_ERROR_HANDLE, errf)
	os.Stdin, os.Stdout, os.Stderr = inFile, outFile, errFile
	return nil
}

func openCon(name string) (windows.Handle, error) {
	path, err := windows.UTF16PtrFromString(`\\.\` + name)
	if err != nil {
		return 0, err
	}
	h, err := windows.CreateFile(
		path,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		0,
		0,
	)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	return h, nil
}

func showEngineError(msg string) {
	text, err1 := windows.UTF16PtrFromString(msg)
	caption, err2 := windows.UTF16PtrFromString("NULLTRACE")
	if err1 != nil || err2 != nil {
		return
	}
	_, _ = windows.MessageBox(0, text, caption, windows.MB_OK|windows.MB_ICONERROR)
}

func isStdinTerminal() bool {
	h := windows.Handle(os.Stdin.Fd())
	var mode uint32
	return windows.GetConsoleMode(h, &mode) == nil
}

func ownConsole() bool { return true }

func bootLog(msg string, args ...any) {
	dir := os.Getenv("LocalAppData")
	if dir == "" {
		return
	}
	path := filepath.Join(dir, "nulltrace", "launch.log")
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s\n", time.Now().Format(time.RFC3339), fmt.Sprintf(msg, args...))
}
