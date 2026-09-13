package app

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	ncrypto "github.com/4x3/nulltrace/internal/crypto"
	"github.com/4x3/nulltrace/internal/tui"
	"golang.org/x/term"
)

func isHomeChoice(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "m", "home", "menu":
		return true
	}
	return false
}

func isBackChoice(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "0", "b", "back":
		return true
	}
	return false
}

func printHead(title, sub string) {
	fmt.Print("\n\n")
	fmt.Println("  " + tui.Title(title))
	if strings.TrimSpace(sub) != "" {
		fmt.Println("  " + tui.Muted(sub))
	}
	fmt.Println()
}

func printNav() {
	fmt.Println("  " + tui.NavLine())
}

func (s *session) pause() {
	fmt.Print("\n  " + tui.Muted("Enter") + " back      " + tui.Key("m") + "  main menu\n  " + tui.Prompt() + " ")
	line, _ := s.in.ReadString('\n')
	if isHomeChoice(line) {
		s.goHome = true
	}
}

func (s *session) gone() bool { return s.goHome }

func promptLine(in *bufio.Reader, label string) string {
	fmt.Print(label + ": ")
	s, _ := in.ReadString('\n')
	return strings.TrimSpace(s)
}

func promptDefault(in *bufio.Reader, label, def string) string {
	if def != "" {
		fmt.Printf("%s [%s]: ", label, def)
	} else {
		fmt.Print(label + ": ")
	}
	s, _ := in.ReadString('\n')
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	return s
}

func promptYes(in *bufio.Reader, label string, defYes bool) bool {
	hint := "Y/n"
	if !defYes {
		hint = "y/N"
	}
	fmt.Printf("%s [%s]: ", label, hint)
	s, _ := in.ReadString('\n')
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return defYes
	}
	return s == "y" || s == "yes"
}

func waitEnter(in *bufio.Reader) {
	fmt.Print("\n  Press Enter to return to the menu…")
	_, _ = in.ReadString('\n')
}

func clearScreen() {
	fmt.Print("\033[2J\033[H")
}

func promptInt(in *bufio.Reader, label string, def int) int {
	s := promptDefault(in, label, strconv.Itoa(def))
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

// ReadSecret prompts without echoing. Never logs the value.
func ReadSecret(prompt string) ([]byte, error) {
	fd := int(os.Stdin.Fd())
	fmt.Fprint(os.Stderr, prompt)
	if !term.IsTerminal(fd) {
		in := bufio.NewReader(os.Stdin)
		s, err := in.ReadString('\n')
		if err != nil {
			return nil, err
		}
		return trimPass([]byte(strings.TrimRight(s, "\r\n"))), nil
	}
	pw, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return nil, err
	}
	return pw, nil
}

func (rt *Runtime) hasSecret(ctx context.Context, key string) bool {
	if rt == nil || rt.Store == nil || rt.Store.Locked() {
		return false
	}
	v, err := rt.Store.GetSecret(ctx, key)
	if err != nil || len(v) == 0 {
		return false
	}
	ncrypto.Zeroize(v)
	return true
}
