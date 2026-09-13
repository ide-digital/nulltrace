package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	ncrypto "github.com/4x3/nulltrace/internal/crypto"
	"github.com/4x3/nulltrace/internal/engine/exposure"
	"github.com/4x3/nulltrace/internal/tui"
)

func (s *session) doLeaks() {
	if !s.needLocal("Leak checks") {
		return
	}
	for {
		if s.gone() {
			return
		}
		clearScreen()
		printHead("Leaks", "Password strength and public dump hits. No account, no API key.")
		fmt.Println("  " + tui.Muted("The password is never stored or sent in full. Only a 5-character"))
		fmt.Println("  " + tui.Muted("SHA-1 prefix is checked against Have I Been Pwned’s dump index."))
		fmt.Println()
		fmt.Println("  " + tui.Key("1") + "  Check a password     strength, common-list, dump hits, replacements")
		fmt.Println("  " + tui.Key("2") + "  Generate a password  passphrase + random")
		fmt.Println()
		printNav()
		fmt.Print("\n  " + tui.Prompt() + " ")
		choice := strings.TrimSpace(s.readChoice())
		switch {
		case isHomeChoice(choice):
			s.goHome = true
			return
		case isBackChoice(choice), choice == "":
			return
		case choice == "1":
			s.checkPassword()
		case choice == "2":
			s.showGeneratedPasswords()
		}
	}
}

func (s *session) identityHints() []string {
	snap, err := s.snapshot()
	if err != nil {
		return nil
	}
	return exposure.IdentityHints(snap.Identity.First, snap.Identity.Last, firstEmail(snap.Identity.Emails), snap.Identity.Emails)
}

func firstEmail(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	return ss[0]
}

func (s *session) checkPassword() {
	fmt.Println()
	fmt.Println("  " + tui.Muted("Typed text is hidden. Nothing is stored."))
	pw, err := ReadSecret("  Password to check: ")
	if err != nil {
		fmt.Println("  " + tui.Bad(err.Error()))
		s.pause()
		return
	}
	if len(strings.TrimSpace(string(pw))) == 0 {
		ncrypto.Zeroize(pw)
		return
	}
	fmt.Println("  " + tui.Muted("checking strength and dump index…"))
	ctx, cancel := context.WithTimeout(s.ctx, 25*time.Second)
	rep := exposure.CheckPassword(ctx, s.rt.HTTP, string(pw), s.identityHints())
	cancel()
	ncrypto.Zeroize(pw)
	s.showPasswordReport(rep)
}

func (s *session) showPasswordReport(rep exposure.PasswordReport) {
	clearScreen()
	printHead("Password check", "The password itself is not shown.")
	risk := tui.Item(rep.Risk)
	switch rep.Risk {
	case "critical", "high":
		risk = tui.Bad(rep.Risk)
	case "ok":
		risk = tui.OK(rep.Risk)
	case "medium":
		risk = tui.Warn(rep.Risk)
	}
	kv := func(k, v string) {
		fmt.Printf("  %s  %s\n", tui.Label(fmt.Sprintf("%-12s", k)), v)
	}
	kv("risk", risk)
	kv("strength", tui.Item(rep.Strength))
	kv("length", fmt.Sprintf("%d characters", rep.Length))
	kv("entropy", fmt.Sprintf("~%d bits  (%s)", rep.EntropyBits, nz(rep.Charset, "—")))
	if rep.PwnedOK {
		if rep.PwnedCount == 0 {
			kv("dump index", tui.OK("not found in public dumps"))
		} else {
			kv("dump index", tui.Bad("seen "+exposure.Comma(rep.PwnedCount)+" times"))
		}
	} else {
		kv("dump index", tui.Warn("offline / error"))
	}
	fmt.Println()
	fmt.Println("  " + tui.Item(rep.Summary))
	if len(rep.Issues) > 0 {
		fmt.Println()
		fmt.Println("  " + tui.Title("Why"))
		for _, i := range rep.Issues {
			fmt.Println("  •  " + i)
		}
	}
	if len(rep.Advice) > 0 {
		fmt.Println()
		fmt.Println("  " + tui.Title("Do this"))
		for _, a := range rep.Advice {
			fmt.Println("  •  " + a)
		}
	}
	if len(rep.Suggestions) > 0 {
		fmt.Println()
		fmt.Println("  " + tui.Title("Use one of these instead"))
		fmt.Println("  " + tui.Muted("Unique per site. A password manager is the easy way to keep them."))
		for i, sug := range rep.Suggestions {
			fmt.Printf("  %s  %s\n", tui.Key(strconv.Itoa(i+1)), tui.Name(sug))
		}
	}
	fmt.Println()
	fmt.Println("  " + tui.Muted("The dump index counts how often this password appeared in stolen"))
	fmt.Println("  " + tui.Muted("compilations. It does not name each database — that kind of email"))
	fmt.Println("  " + tui.Muted("lookup needs a paid/registered API, so NullTrace does not offer it."))
	fmt.Println()
	printNav()
	fmt.Print("\n  " + tui.Prompt() + " ")
	choice := strings.TrimSpace(s.readChoice())
	switch {
	case isHomeChoice(choice):
		s.goHome = true
	case choice == "1", choice == "2", choice == "3":
		idx, _ := strconv.Atoi(choice)
		if idx >= 1 && idx <= len(rep.Suggestions) {
			fmt.Println("  " + tui.Muted("checking the suggestion (still never stored)…"))
			ctx, cancel := context.WithTimeout(s.ctx, 25*time.Second)
			next := exposure.CheckPassword(ctx, s.rt.HTTP, rep.Suggestions[idx-1], nil)
			cancel()
			s.showPasswordReport(next)
		}
	}
}

func (s *session) showGeneratedPasswords() {
	clearScreen()
	printHead("Generate a password", "Created on this PC with crypto/rand. Nothing is uploaded.")
	sugs := exposure.Suggest(3)
	fmt.Println("  " + tui.Key("1") + "  passphrase     " + tui.Name(sugs[0]))
	fmt.Println("  " + tui.Key("2") + "  20-char        " + tui.Name(sugs[1]))
	fmt.Println("  " + tui.Key("3") + "  16-char+sym    " + tui.Name(sugs[2]))
	fmt.Println()
	fmt.Println("  " + tui.Muted("Type 1–3 to run a leak check on that suggestion.  r  new set."))
	fmt.Println()
	printNav()
	fmt.Print("\n  " + tui.Prompt() + " ")
	choice := strings.ToLower(strings.TrimSpace(s.readChoice()))
	switch {
	case isHomeChoice(choice):
		s.goHome = true
	case choice == "r":
		s.showGeneratedPasswords()
	case choice == "1", choice == "2", choice == "3":
		idx, _ := strconv.Atoi(choice)
		fmt.Println("  " + tui.Muted("checking…"))
		ctx, cancel := context.WithTimeout(s.ctx, 25*time.Second)
		rep := exposure.CheckPassword(ctx, s.rt.HTTP, sugs[idx-1], nil)
		cancel()
		s.showPasswordReport(rep)
	}
}
