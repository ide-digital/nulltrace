package app

import (
	"fmt"
	"strings"

	ncrypto "github.com/4x3/nulltrace/internal/crypto"
	"github.com/4x3/nulltrace/internal/paths"
	"github.com/4x3/nulltrace/internal/tui"
	"github.com/4x3/nulltrace/pkg/ipc"
)

func (s *session) doSMTP() {
	if !s.needLocal("Email setup") {
		return
	}
	brands := mailBrands()
	for {
		if s.gone() {
			return
		}
		clearScreen()
		printHead("Email", "Deletion letters are sent from a mailbox you control.")
		cfg := s.cfg()
		if brand, addr, ok := s.linkedMail(); ok {
			fmt.Println("  " + tui.Label("linked") + "     " + tui.OK(brand))
			fmt.Println("  " + tui.Label("address") + "    " + tui.Name(addr))
			fmt.Println("  " + tui.Label("server") + "     " + tui.Muted(fmt.Sprintf("%s:%d", cfg.SMTP.Host, cfg.SMTP.Port)))
			if cfg.IMAP.Host != "" && s.rt != nil && s.rt.hasSecret(s.ctx, SecretIMAPPassword) {
				fmt.Println("  " + tui.Label("inbox") + "      " + tui.OK("watching  "+nz(cfg.IMAP.Username, addr)))
			} else {
				fmt.Println("  " + tui.Label("inbox") + "      " + tui.Muted("not watching"))
			}
		} else if cfg.SMTP.Host != "" {
			fmt.Println("  " + tui.Warn("server saved, but no app password yet"))
			fmt.Println("  " + tui.Muted(fmt.Sprintf("%s  %s:%d", nz(cfg.SMTP.From, cfg.SMTP.Username), cfg.SMTP.Host, cfg.SMTP.Port)))
		} else {
			fmt.Println("  " + tui.Muted("Nothing linked. Pick a provider — it opens in your browser."))
		}
		fmt.Println()
		fmt.Println("  " + tui.Key("1") + "  Link Gmail")
		fmt.Println("  " + tui.Key("2") + "  Link Outlook / Hotmail / Live")
		fmt.Println("  " + tui.Key("3") + "  Link Yahoo")
		fmt.Println("  " + tui.Key("4") + "  Custom server")
		fmt.Println("  " + tui.Key("5") + "  Watch inbox (same account)")
		fmt.Println("  " + tui.Key("6") + "  Unlink")
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
			s.linkMail(brands["gmail"])
		case choice == "2":
			s.linkMail(brands["outlook"])
		case choice == "3":
			s.linkMail(brands["yahoo"])
		case choice == "4":
			s.linkCustomMail()
		case choice == "5":
			s.enableInboxFromLinked()
		case choice == "6":
			s.unlinkMail()
		}
	}
}

func (s *session) doIMAP() {
	if !s.needLocal("Inbox setup") {
		return
	}
	for {
		if s.gone() {
			return
		}
		clearScreen()
		printHead("Inbox", "Optional. Watch for “click to confirm” mail from brokers.")
		cfg := s.cfg()
		if cfg.IMAP.Host != "" {
			watch := tui.Muted("no password")
			if s.rt != nil && s.rt.hasSecret(s.ctx, SecretIMAPPassword) {
				watch = tui.OK("watching")
			}
			fmt.Println("  " + tui.Label("status") + "    " + watch)
			fmt.Println("  " + tui.Label("mailbox") + "   " + tui.Name(nz(cfg.IMAP.Username, "—")))
			fmt.Println("  " + tui.Label("server") + "    " + tui.Muted(fmt.Sprintf("%s:%d  %s", cfg.IMAP.Host, cfg.IMAP.Port, cfg.IMAP.Mailbox)))
		} else {
			fmt.Println("  " + tui.Muted("Inbox watching is off."))
			if _, addr, ok := s.linkedMail(); ok {
				fmt.Println("  " + tui.Muted("Linked mail: "+addr+" — [1] reuses that app password."))
			}
		}
		fmt.Println()
		fmt.Println("  " + tui.Key("1") + "  Use the linked email account")
		fmt.Println("  " + tui.Key("2") + "  Custom IMAP")
		fmt.Println("  " + tui.Key("3") + "  Stop watching")
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
			s.enableInboxFromLinked()
		case choice == "2":
			s.doIMAPCustom()
		case choice == "3":
			s.clearIMAP()
			s.pause()
		}
	}
}

func (s *session) doIMAPCustom() {
	cfg := s.cfg()
	fmt.Println()
	fmt.Println("  " + tui.Key("1") + "  Gmail          imap.gmail.com:993")
	fmt.Println("  " + tui.Key("2") + "  Outlook        outlook.office365.com:993")
	fmt.Println("  " + tui.Key("3") + "  Yahoo          imap.mail.yahoo.com:993")
	fmt.Println("  " + tui.Key("4") + "  Other host")
	fmt.Print("\n  " + tui.Prompt() + " ")
	host, port := "", 993
	switch strings.TrimSpace(s.readChoice()) {
	case "1":
		host = "imap.gmail.com"
	case "2":
		host = "outlook.office365.com"
	case "3":
		host = "imap.mail.yahoo.com"
	case "4":
		host = promptLine(s.in, "  IMAP host")
		port = promptInt(s.in, "  Port", 993)
	default:
		return
	}
	if host == "" {
		return
	}
	user := promptDefault(s.in, "  Username", nz(cfg.IMAP.Username, cfg.SMTP.Username))
	box := promptDefault(s.in, "  Mailbox", nz(cfg.IMAP.Mailbox, "INBOX"))
	auto := promptYes(s.in, "  Auto-open confirmation links (plain GET)", cfg.IMAP.AutoFetch)
	pw, err := ReadSecret("  IMAP password / app password: ")
	if err != nil {
		fmt.Println("  " + tui.Bad(err.Error()))
		s.pause()
		return
	}
	pw = scrubAppPass(pw)
	cfg.IMAP.Host = host
	cfg.IMAP.Port = port
	cfg.IMAP.Username = user
	cfg.IMAP.Mailbox = box
	cfg.IMAP.TLS = true
	cfg.IMAP.AutoFetch = auto
	if err := s.saveCfg(cfg); err != nil {
		ncrypto.Zeroize(pw)
		fmt.Println("  " + tui.Bad(err.Error()))
		s.pause()
		return
	}
	if len(pw) > 0 {
		if err := s.rt.Store.PutSecret(s.ctx, SecretIMAPPassword, pw); err != nil {
			ncrypto.Zeroize(pw)
			fmt.Println("  " + tui.Bad(err.Error()))
			s.pause()
			return
		}
	}
	ncrypto.Zeroize(pw)
	fmt.Println("  " + tui.OK("inbox saved  "+user))
	s.pause()
}

func (s *session) doSettings() {
	for {
		if s.gone() {
			return
		}
		clearScreen()
		printHead("Settings", "")
		cfg := s.cfg()
		proxy := cfg.Proxy
		if proxy == "" {
			proxy = "off"
		}
		cap := "no"
		if s.rt != nil && s.rt.hasSecret(s.ctx, SecretCapSolverKey) {
			cap = "stored"
		}
		browser := "off"
		if cfg.Browser.Enabled {
			browser = "on"
		}
		fmt.Printf("  proxy %s    law %s    CapSolver %s    browser %s\n",
			proxy, cfg.Jurisdiction, cap, browser)
		fmt.Printf("  send rate %d/min    listen %s\n", cfg.Mailer.PerMinute, cfg.ListenAddr)
		fmt.Println()
		fmt.Println("  " + tui.Key("1") + "  Proxy URL")
		fmt.Println("  " + tui.Key("2") + "  Privacy law (CCPA / GDPR / US state)")
		fmt.Println("  " + tui.Key("3") + "  CapSolver API key")
		fmt.Println("  " + tui.Key("4") + "  Browser form-fill")
		fmt.Println("  " + tui.Key("5") + "  Send speed (emails per minute)")
		fmt.Println("  " + tui.Key("6") + "  Show folders")
		fmt.Println("  " + tui.Key("7") + "  Open config folder")
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
			s.setProxy()
		case choice == "2":
			s.setLaw()
		case choice == "3":
			s.setSecret(SecretCapSolverKey, "CapSolver API key")
		case choice == "4":
			s.setBrowser()
		case choice == "5":
			cfg.Mailer.PerMinute = promptInt(s.in, "  Emails per minute", cfg.Mailer.PerMinute)
			if cfg.Mailer.PerMinute <= 0 {
				cfg.Mailer.PerMinute = 5
			}
			if err := s.saveCfg(cfg); err != nil {
				fmt.Println("  " + tui.Bad(err.Error()))
				s.pause()
			}
		case choice == "6":
			dirs, err := paths.Resolve()
			if err != nil {
				fmt.Println("  " + tui.Bad(err.Error()))
			} else {
				fmt.Println("  config  " + dirs.ConfigFile)
				fmt.Println("  data    " + dirs.Data)
				fmt.Println("  vault   " + dirs.VaultDB)
				fmt.Println("  audit   " + dirs.AuditLogs)
			}
			s.pause()
		case choice == "7":
			dirs, err := paths.Resolve()
			if err != nil {
				fmt.Println("  " + tui.Bad(err.Error()))
				s.pause()
				continue
			}
			if err := openFolder(dirs.Config); err != nil {
				fmt.Println("  " + tui.Bad(err.Error()))
				s.pause()
			}
		}
	}
}

func (s *session) setProxy() {
	fmt.Println("  Examples:  http://127.0.0.1:8080")
	fmt.Println("             socks5://127.0.0.1:9050")
	fmt.Println("             http://user:pass@host:port")
	fmt.Println("  Blank disables the proxy.")
	cfg := s.cfg()
	cfg.Proxy = promptDefault(s.in, "  Proxy URL", cfg.Proxy)
	if err := s.saveCfg(cfg); err != nil {
		fmt.Println("  " + tui.Bad(err.Error()))
		s.pause()
		return
	}
	fmt.Println("  " + tui.OK("proxy saved"))
	s.pause()
}

func (s *session) setLaw() {
	fmt.Println("  " + tui.Key("1") + "  CCPA  — California (default)")
	fmt.Println("  " + tui.Key("2") + "  GDPR  — EU / UK")
	fmt.Println("  " + tui.Key("3") + "  US state privacy statutes")
	fmt.Print("  " + tui.Prompt() + " ")
	cfg := s.cfg()
	switch strings.TrimSpace(s.readChoice()) {
	case "1":
		cfg.Jurisdiction = "CCPA"
	case "2":
		cfg.Jurisdiction = "GDPR"
	case "3":
		cfg.Jurisdiction = "US_STATE"
	default:
		return
	}
	if err := s.saveCfg(cfg); err != nil {
		fmt.Println("  " + tui.Bad(err.Error()))
		s.pause()
		return
	}
	fmt.Println("  " + tui.OK("law set to "+cfg.Jurisdiction))
	s.pause()
}

func (s *session) setSecret(key, label string) {
	if !s.needLocal("Saving " + label) {
		return
	}
	pw, err := ReadSecret("  " + label + ": ")
	if err != nil {
		fmt.Println("  " + tui.Bad(err.Error()))
		s.pause()
		return
	}
	if len(pw) == 0 {
		return
	}
	if err := s.rt.Store.PutSecret(s.ctx, key, pw); err != nil {
		ncrypto.Zeroize(pw)
		fmt.Println("  " + tui.Bad(err.Error()))
		s.pause()
		return
	}
	ncrypto.Zeroize(pw)
	if key == SecretHIBPKey {
		cfg := s.cfg()
		cfg.HIBP.Enabled = true
		_ = s.saveCfg(cfg)
	}
	if key == SecretCapSolverKey {
		cfg := s.cfg()
		cfg.CapSolver.Enabled = true
		_ = s.saveCfg(cfg)
	}
	fmt.Println("  " + tui.OK("stored"))
	s.pause()
}

func (s *session) setBrowser() {
	cfg := s.cfg()
	cfg.Browser.Enabled = promptYes(s.in, "  Enable in-app form fill", cfg.Browser.Enabled)
	if cfg.Browser.Enabled {
		cfg.Browser.Headless = promptYes(s.in, "  Run the browser hidden (headless)", cfg.Browser.Headless)
		cfg.Browser.Bin = promptDefault(s.in, "  Chrome/Chromium path (blank = auto)", cfg.Browser.Bin)
	}
	if err := s.saveCfg(cfg); err != nil {
		fmt.Println("  " + tui.Bad(err.Error()))
		s.pause()
		return
	}
	fmt.Println("  " + tui.OK("browser settings saved"))
	s.pause()
}

func (s *session) putIdentityFromWizard(first, last, middle, dob, email, phone, city string) error {
	req := ipc.IdentityAddRequest{First: first, Last: last, Middle: middle, DOB: dob}
	if email != "" {
		req.Emails = []string{email}
	}
	if phone != "" {
		req.Phones = []string{phone}
	}
	if city != "" {
		req.Cities = []string{city}
	}
	_, err := s.rt.AddIdentity(s.ctx, req)
	return err
}
