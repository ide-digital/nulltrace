package opsec

import (
	"bytes"
	"testing"
)

func TestStripJPEGDropsAPP1(t *testing.T) {
	// Minimal SOI + APP1 + SOS-like trailer.
	var b bytes.Buffer
	b.Write([]byte{0xff, 0xd8})
	// APP1, length 8 (2 length + 6 dummy)
	b.Write([]byte{0xff, 0xe1, 0x00, 0x08, 'E', 'x', 'i', 'f', 0x00, 0x00})
	// COM comment kept
	b.Write([]byte{0xff, 0xfe, 0x00, 0x04, 'h', 'i'})
	// SOS + bytes + EOI
	b.Write([]byte{0xff, 0xda, 0x00, 0x02, 0xff, 0xd9})
	out, err := Strip(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out, []byte("Exif")) {
		t.Fatalf("EXIF still present")
	}
	if !bytes.Contains(out, []byte{0xff, 0xfe}) {
		t.Fatalf("comment marker dropped")
	}
}
