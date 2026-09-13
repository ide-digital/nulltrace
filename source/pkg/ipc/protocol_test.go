package ipc

import (
	"bytes"
	"testing"
)

func TestEncodeRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	env := Envelope{V: 1, ID: "abc", Cmd: CmdPing, Token: "tok"}
	if err := Encode(&buf, env); err != nil {
		t.Fatal(err)
	}
	got, err := Decode[Envelope](&buf)
	if err != nil {
		t.Fatal(err)
	}
	if got.Cmd != CmdPing || got.ID != "abc" || got.Token != "tok" {
		t.Fatalf("%+v", got)
	}
}
