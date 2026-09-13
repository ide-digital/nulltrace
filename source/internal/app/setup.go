package app

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/4x3/nulltrace/internal/config"
	"github.com/4x3/nulltrace/internal/tui"
)

func vaultInitialized(ctx context.Context) (bool, error) {
	rt, err := Open(ctx, nil, false)
	if err != nil {
		return false, err
	}
	defer rt.Close()
	return rt.Store.Initialized(ctx)
}

func runSetupThenMenu(ctx context.Context, pw []byte) error {
	if len(strings.TrimSpace(string(pw))) == 0 {
		return fmt.Errorf("password cannot be empty")
	}

	rt, err := Open(ctx, nil, false)
	if err != nil {
		return err
	}
	defer rt.Close()

	if err := rt.Store.InitVault(ctx, pw); err != nil {
		return err
	}
	if err := config.Save(rt.Dirs.ConfigFile, rt.Cfg); err != nil {
		return err
	}
	rt.Audit(ctx, "vault.initialized", "", map[string]any{"brokers": rt.Reg.Len()})

	in := bufio.NewReader(os.Stdin)
	clearScreen()
	fmt.Print("\n\n")
	fmt.Println(tui.Banner())
	fmt.Println()
	fmt.Println("  " + tui.Title("Who are we scrubbing?"))
	fmt.Println("  " + tui.Muted("Use the name and contact info the people-search sites would have."))
	fmt.Println()

	var first, last string
	for first == "" || last == "" {
		first = promptLine(in, "  First name")
		last = promptLine(in, "  Last name")
		if first == "" || last == "" {
			fmt.Println("  " + tui.Warn("first and last name are required"))
		}
	}
	middle := promptLine(in, "  Middle name (optional)")
	dob := promptLine(in, "  Date of birth (optional)")
	email := promptLine(in, "  Email (optional)")
	phone := promptLine(in, "  Phone (optional)")
	city := promptLine(in, "  City (optional)")
	s := &session{ctx: ctx, rt: rt, in: in}
	if err := s.putIdentityFromWizard(first, last, middle, dob, email, phone, city); err != nil {
		return err
	}
	fmt.Println("  " + tui.OK("saved "+first+" "+last))
	fmt.Println()

	if promptYes(in, "  Link a sending mailbox now (Gmail / Outlook / Yahoo)", true) {
		s.doSMTP()
		s.goHome = false
	}
	if promptYes(in, "  Use a proxy", false) {
		s.setProxy()
	}
	fmt.Println()
	fmt.Println("  Privacy law for the deletion letters:")
	s.setLaw()

	fmt.Println()
	fmt.Println("  " + tui.OK("Setup complete."))
	fmt.Println("  Next: [1] Scan, then [4] Listings, then [2] Scrub.")
	waitEnter(in)
	s.hint = "Setup complete. [1] Scan  →  [4] Listings  →  [2] Scrub"
	return runInteractive(ctx, rt, nil)
}
