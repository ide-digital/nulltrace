package app

import (
	"bufio"
	"errors"
	"strings"
	"testing"
)

func TestClosedStdinDoesNotConfirm(t *testing.T) {
	confirmed := false
	run := func() (err error) {
		defer quitOnEOF(&err)
		in := bufio.NewReader(strings.NewReader(""))
		confirmed = promptYes(in, "send", true)
		return errors.New("prompt returned")
	}
	if err := run(); err != nil {
		t.Fatalf("expected a clean exit, got %v", err)
	}
	if confirmed {
		t.Fatal("EOF counted as yes")
	}
}

func TestLastLineWithoutNewline(t *testing.T) {
	in := bufio.NewReader(strings.NewReader("y"))
	if !promptYes(in, "send", false) {
		t.Fatal("y without a trailing newline should still count")
	}
}

func TestNavChoices(t *testing.T) {
	if !isHomeChoice("m") || !isHomeChoice("HOME") || !isHomeChoice(" menu ") {
		t.Fatal("home aliases")
	}
	if isHomeChoice("0") || isHomeChoice("n") {
		t.Fatal("0/n are not home")
	}
	if !isBackChoice("0") || !isBackChoice("back") || !isBackChoice("B") {
		t.Fatal("back aliases")
	}
	if isBackChoice("m") {
		t.Fatal("m is home, not back")
	}
}

func TestScrubAppPass(t *testing.T) {
	got := string(scrubAppPass([]byte("abcd efgh ijkl mnop")))
	if got != "abcdefghijklmnop" {
		t.Fatalf("got %q", got)
	}
}

func TestMailBrandLabel(t *testing.T) {
	if got := mailBrandLabel("smtp.gmail.com"); got != "Gmail" {
		t.Fatalf("got %q", got)
	}
	if got := mailBrandLabel("smtp.mail.yahoo.com"); got != "Yahoo" {
		t.Fatalf("got %q", got)
	}
	if got := mailBrandLabel("mail.example.com"); got != "Custom" {
		t.Fatalf("got %q", got)
	}
}
