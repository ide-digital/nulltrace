package ipc

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTokenRoundTrip(t *testing.T) {
	tok, err := RandomToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(tok) != 64 {
		t.Fatalf("token len %d", len(tok))
	}
	path := filepath.Join(t.TempDir(), "daemon.token")
	if err := WriteToken(path, tok); err != nil {
		t.Fatal(err)
	}
	// Editors love adding a trailing newline or CRLF.
	if err := os.WriteFile(path, []byte(tok+"\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadToken(path)
	if err != nil || got != tok {
		t.Fatalf("got %q err=%v", got, err)
	}
}

func TestUnmarshalEmptyPayload(t *testing.T) {
	v, err := UnmarshalPayload[StatusPayload](Reply{OK: true})
	if err != nil || v.Unlocked {
		t.Fatalf("v=%+v err=%v", v, err)
	}
}
