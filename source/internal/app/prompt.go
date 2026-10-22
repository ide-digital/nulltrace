package app

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	ncrypto "github.com/ide-digital/nulltrace/internal/crypto"
	"github.com/ide-digital/nulltrace/internal/tui"
	"golang.org/x/term"
)

// stdinGone is panicked by readLine once stdin is closed. Without it every
// prompt loop spins on "" and promptYes happily returns its default.
type stdinGone struct{}

func readLine(in *bufio.Reader) string {
	s, err := in.ReadString('\n')
	if err != nil && s == "" {
		panic(stdinGone{})
	}
	return strings.TrimSpace(s)
}

func (s *session) readChoice() string { return readLine(s.in) }

func quitOnEOF(err *error) {
	r := recover()
	if r == nil {
		return
	}
	if _, ok := r.(stdinGone); !ok {
		panic(r)
	}
	fmt.Println()
	*err = nil
}

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
	if isHomeChoice(readLine(s.in)) {
		s.goHome = true
	}
}

func (s *session) gone() bool { return s.goHome }

func promptLine(in *bufio.Reader, label string) string {
	fmt.Print(label + ": ")
	return readLine(in)
}

func promptDefault(in *bufio.Reader, label, def string) string {
	if def != "" {
		fmt.Printf("%s [%s]: ", label, def)
	} else {
		fmt.Print(label + ": ")
	}
	if s := readLine(in); s != "" {
		return s
	}
	return def
}

func promptYes(in *bufio.Reader, label string, defYes bool) bool {
	hint := "Y/n"
	if !defYes {
		hint = "y/N"
	}
	fmt.Printf("%s [%s]: ", label, hint)
	s := strings.ToLower(readLine(in))
	if s == "" {
		return defYes
	}
	return s == "y" || s == "yes"
}

func waitEnter(in *bufio.Reader) {
	fmt.Print("\n  Press Enter to return to the menu...")
	readLine(in)
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

func (rt *Runtime) MailLinked(ctx context.Context) bool {
	return rt.Cfg.SMTP.Host != "" && rt.hasSecret(ctx, SecretSMTPPassword)
}
