//go:build !windows

package main

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

func prepareConsole() {}

func ownConsole() bool { return false }

func showEngineError(msg string) { fmt.Fprintln(os.Stderr, msg) }

func bootLog(string, ...any) {}

func isStdinTerminal() bool { return term.IsTerminal(int(os.Stdin.Fd())) }
