package app

import "testing"

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
