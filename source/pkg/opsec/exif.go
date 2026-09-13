package opsec

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
)

// StripFile removes metadata from JPEG (APP1 EXIF / XMP, APP13 IPTC) and PNG
// (tEXt / iTXt / zTXt / eXIf) images. Other formats are copied unchanged.
func StripFile(src, dst string) error {
	raw, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	out, err := Strip(raw)
	if err != nil {
		return err
	}
	if dst == "" {
		dst = src
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil && !os.IsExist(err) {
		return err
	}
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

func Strip(data []byte) ([]byte, error) {
	if len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff {
		return stripJPEG(data)
	}
	if len(data) >= 8 && bytes.Equal(data[:8], []byte{137, 80, 78, 71, 13, 10, 26, 10}) {
		return stripPNG(data)
	}
	return data, nil
}

func stripJPEG(data []byte) ([]byte, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("jpeg: truncated")
	}
	var out bytes.Buffer
	out.Write(data[:2]) // SOI
	i := 2
	for i < len(data) {
		if data[i] != 0xff {
			out.Write(data[i:])
			break
		}
		for i < len(data) && data[i] == 0xff {
			i++
		}
		if i >= len(data) {
			break
		}
		marker := data[i]
		i++
		if marker == 0xd9 { // EOI
			out.Write([]byte{0xff, marker})
			break
		}
		if marker == 0xda { // SOS — remainder is entropy-coded
			out.Write([]byte{0xff, marker})
			out.Write(data[i:])
			break
		}
		// Standalone markers (RST, TEM) have no length.
		if marker >= 0xd0 && marker <= 0xd8 {
			out.Write([]byte{0xff, marker})
			continue
		}
		if i+1 >= len(data) {
			return nil, fmt.Errorf("jpeg: truncated segment")
		}
		seglen := int(binary.BigEndian.Uint16(data[i : i+2]))
		if seglen < 2 || i+seglen > len(data) {
			return nil, fmt.Errorf("jpeg: bad segment length")
		}
		seg := data[i-2 : i+seglen] // includes ff marker
		i += seglen
		// Drop APP1 (EXIF/XMP) and APP13 (IPTC/Photoshop IRB).
		if marker == 0xe1 || marker == 0xed {
			continue
		}
		out.Write(seg)
	}
	return out.Bytes(), nil
}

func stripPNG(data []byte) ([]byte, error) {
	var out bytes.Buffer
	out.Write(data[:8])
	i := 8
	drop := map[string]struct{}{
		"tEXt": {}, "iTXt": {}, "zTXt": {}, "eXIf": {}, "tIME": {},
	}
	for i+12 <= len(data) {
		n := binary.BigEndian.Uint32(data[i : i+4])
		typ := string(data[i+4 : i+8])
		end := i + 12 + int(n)
		if end > len(data) {
			return nil, fmt.Errorf("png: truncated chunk %s", typ)
		}
		chunk := data[i:end]
		i = end
		if _, skip := drop[typ]; skip {
			continue
		}
		out.Write(chunk)
		if typ == "IEND" {
			break
		}
	}
	return out.Bytes(), nil
}
