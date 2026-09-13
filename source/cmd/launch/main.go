package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	self, err := os.Executable()
	if err != nil {
		fail("Couldn't locate NullTrace.exe: " + err.Error())
	}
	root := filepath.Dir(self)
	engine := filepath.Join(root, "app", engineName())
	if _, err := os.Stat(engine); err != nil {
		fail("Missing app files.\n\nExpected:\n" + engine + "\n\nKeep NullTrace.exe next to the app folder.")
	}
	if err := launchEngine(root, engine, os.Args[1:]); err != nil {
		fail(err.Error())
	}
}

func engineCmd(root, engine string, extra []string) *exec.Cmd {
	args := append([]string{"--app"}, extra...)
	cmd := exec.Command(engine, args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "NULLTRACE_APP=1")
	return cmd
}

func fail(msg string) {
	showError(msg)
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
