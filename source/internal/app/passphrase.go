package app

import (
	"fmt"
	"os"

	ncrypto "github.com/4x3/nulltrace/internal/crypto"
	"golang.org/x/term"
)

// ReadPassphrase never logs the value.
//
// Order: --passphrase-file, NULLTRACE_PASSPHRASE, then a prompt (or one stdin
// line if stdin isn't a TTY).
func ReadPassphrase(fileFlag string, confirm bool) ([]byte, error) {
	if fileFlag != "" {
		raw, err := os.ReadFile(fileFlag)
		if err != nil {
			return nil, fmt.Errorf("read passphrase file: %w", err)
		}
		return trimPass(raw), nil
	}
	if v := os.Getenv("NULLTRACE_PASSPHRASE"); v != "" && os.Getenv("NULLTRACE_APP") != "1" {
		return []byte(v), nil
	}
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		buf := make([]byte, 4096)
		n, err := os.Stdin.Read(buf)
		if err != nil {
			return nil, err
		}
		return trimPass(buf[:n]), nil
	}
	fmt.Fprint(os.Stderr, "Vault passphrase: ")
	pw, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return nil, err
	}
	if len(trimPass(pw)) == 0 {
		return nil, fmt.Errorf("passphrase must not be empty")
	}
	if confirm {
		fmt.Fprint(os.Stderr, "Confirm passphrase: ")
		pw2, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return nil, err
		}
		if string(pw) != string(pw2) {
			ncrypto.Zeroize(pw)
			ncrypto.Zeroize(pw2)
			return nil, fmt.Errorf("passphrases did not match")
		}
		ncrypto.Zeroize(pw2)
	}
	return pw, nil
}

func trimPass(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r') {
		b = b[:len(b)-1]
	}
	return b
}
