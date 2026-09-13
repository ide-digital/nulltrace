//go:build !windows

package main

import (
	"fmt"
	"os"
)

func engineName() string { return "nulltrace" }

func launchEngine(root, engine string, extra []string) error {
	cmd := engineCmd(root, engine, extra)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func showError(msg string) {
	fmt.Fprintln(os.Stderr, msg)
}
