package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/4x3/nulltrace/internal/config"
	ncrypto "github.com/4x3/nulltrace/internal/crypto"
	"github.com/4x3/nulltrace/internal/engine/mailer"
	"github.com/4x3/nulltrace/internal/tui"
)

type mailBrand struct {
	Label    string
	SMTP     string
	SMTPPort int
	IMAP     string
	IMAPPort int
	HelpURL  string
	Hint     string
}

func mailBrands() map[string]mailBrand {
	return map[string]mailBrand{
		"gmail": {
			Label:    "Gmail",
			SMTP:     "smtp.gmail.com",
			SMTPPort: 587,
			IMAP:     "imap.gmail.com",
			IMAPPort: 993,
			HelpURL:  "https://myaccount.google.com/apppasswords",
			Hint:     "Google opens in your browser. Turn on 2-step verification if asked, then create an App Password named NullTrace.",
		},
		"outlook": {
			Label:    "Outlook",
			SMTP:     "smtp.office365.com",
			SMTPPort: 587,
			IMAP:     "outlook.office365.com",
			IMAPPort: 993,
			HelpURL:  "https://account.live.com/proofs/AppPassword",
			Hint:     "Microsoft opens in your browser. With 2-step verification on, create an App Password.",
		},
		"yahoo": {
			Label:    "Yahoo",
			SMTP:     "smtp.mail.yahoo.com",
			SMTPPort: 587,
			IMAP:     "imap.mail.yahoo.com",
			IMAPPort: 993,
			HelpURL:  "https://login.yahoo.com/account/security",
			Hint:     "Yahoo opens in your browser. Under security, generate an App Password.",
		},
	}
}

func mailBrandLabel(host string) string {
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "smtp.gmail.com":
		return "Gmail"
	case "smtp.office365.com", "smtp-mail.outlook.com":
		return "Outlook"
	case "smtp.mail.yahoo.com":
		return "Yahoo"
	case "":
		return ""
	default:
		return "Custom"
	}
}

func scrubAppPass(pw []byte) []byte {
	n := 0
	for _, c := range pw {
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			pw[n] = c
			n++
		}
	}
	for i := n; i < len(pw); i++ {
		pw[i] = 0
	}
	return pw[:n]
}

func (s *session) defaultMailUser() string {
	cfg := s.cfg()
	if u := strings.TrimSpace(cfg.SMTP.Username); u != "" {
		return u
	}
	if u := strings.TrimSpace(cfg.SMTP.From); u != "" {
		return u
	}
	if snap, err := s.snapshot(); err == nil {
		for _, e := range snap.Identity.Emails {
			if strings.TrimSpace(e) != "" {
				return e
			}
		}
	}
	return ""
}

func (s *session) linkedMail() (brand, addr string, ok bool) {
	cfg := s.cfg()
	addr = strings.TrimSpace(nz(cfg.SMTP.From, cfg.SMTP.Username))
	if cfg.SMTP.Host == "" || addr == "" {
		return "", addr, false
	}
	return mailBrandLabel(cfg.SMTP.Host), addr, s.hasMailPassword()
}

func (s *session) linkMail(b mailBrand) {
	clearScreen()
	printHead("Link "+b.Label, b.Hint)
	fmt.Println("  Opening " + b.Label + " in your browser…")
	if err := openURL(b.HelpURL); err != nil {
		fmt.Println("  " + tui.Warn("could not open a browser — visit this page:"))
		fmt.Println("  " + tui.Muted(b.HelpURL))
	} else {
		fmt.Println("  " + tui.Muted(b.HelpURL))
	}
	fmt.Println()
	fmt.Println("  " + tui.Item("1.") + "  Sign in if asked")
	fmt.Println("  " + tui.Item("2.") + "  Create a 16-character App Password")
	fmt.Println("  " + tui.Item("3.") + "  Paste it below (spaces are stripped)")
	fmt.Println()
	fmt.Println("  " + tui.Muted("Your normal "+b.Label+" login password will not work here."))
	fmt.Println()

	user := promptDefault(s.in, "  Email", s.defaultMailUser())
	if strings.TrimSpace(user) == "" {
		fmt.Println("  " + tui.Bad("email is required"))
		s.pause()
		return
	}
	pw, err := ReadSecret("  App password: ")
	if err != nil {
		fmt.Println("  " + tui.Bad(err.Error()))
		s.pause()
		return
	}
	pw = scrubAppPass(pw)
	if len(pw) == 0 {
		fmt.Println("  " + tui.Bad("app password cannot be empty"))
		s.pause()
		return
	}

	cfg := s.cfg()
	cfg.SMTP = config.SMTPConfig{
		Host:     b.SMTP,
		Port:     b.SMTPPort,
		Username: user,
		From:     user,
		STARTTLS: true,
	}
	if err := s.saveMail(cfg, pw, b, true); err != nil {
		ncrypto.Zeroize(pw)
		fmt.Println("  " + tui.Bad(err.Error()))
		s.pause()
		return
	}
	ncrypto.Zeroize(pw)
}

func (s *session) linkCustomMail() {
	clearScreen()
	printHead("Custom mail server", "Host, username, and the mailbox password (or app password).")
	cfg := s.cfg()
	host := promptDefault(s.in, "  SMTP host", cfg.SMTP.Host)
	if strings.TrimSpace(host) == "" {
		fmt.Println("  " + tui.Bad("host is required"))
		s.pause()
		return
	}
	port := promptInt(s.in, "  Port", nzInt(cfg.SMTP.Port, 587))
	user := promptDefault(s.in, "  Username", nz(cfg.SMTP.Username, s.defaultMailUser()))
	from := promptDefault(s.in, "  From address", nz(cfg.SMTP.From, user))
	tls := promptYes(s.in, "  STARTTLS", cfg.SMTP.STARTTLS || cfg.SMTP.Host == "")
	pw, err := ReadSecret("  Password / app password: ")
	if err != nil {
		fmt.Println("  " + tui.Bad(err.Error()))
		s.pause()
		return
	}
	pw = scrubAppPass(pw)
	cfg.SMTP = config.SMTPConfig{
		Host:     host,
		Port:     port,
		Username: user,
		From:     from,
		STARTTLS: tls,
	}
	b := mailBrand{Label: "Custom", IMAP: "", IMAPPort: 993}
	if err := s.saveMail(cfg, pw, b, false); err != nil {
		ncrypto.Zeroize(pw)
		fmt.Println("  " + tui.Bad(err.Error()))
		s.pause()
		return
	}
	ncrypto.Zeroize(pw)
}

func nzInt(n, fallback int) int {
	if n == 0 {
		return fallback
	}
	return n
}

func (s *session) saveMail(cfg config.File, pw []byte, b mailBrand, offerIMAP bool) error {
	if err := s.saveCfg(cfg); err != nil {
		return err
	}
	if s.rt == nil {
		return fmt.Errorf("vault is not available")
	}
	if len(pw) > 0 {
		if err := s.rt.Store.PutSecret(s.ctx, SecretSMTPPassword, pw); err != nil {
			return err
		}
	}
	fmt.Println()
	fmt.Println("  " + tui.Muted("checking sign-in…"))
	ctx, cancel := context.WithTimeout(s.ctx, 20*time.Second)
	err := mailer.Probe(ctx, cfg.SMTP, pw)
	cancel()
	addr := nz(cfg.SMTP.From, cfg.SMTP.Username)
	if err != nil {
		fmt.Println("  " + tui.Warn("saved, but sign-in failed"))
		fmt.Println("  " + tui.Muted(err.Error()))
		fmt.Println("  " + tui.Muted("The app password is often wrong, or 2-step verification is off."))
	} else {
		fmt.Println("  " + tui.OK("linked  "+addr))
		if offerIMAP && b.IMAP != "" && promptYes(s.in, "  Also watch this inbox for broker confirmation emails", true) {
			cfg.IMAP = config.IMAPConfig{
				Host:      b.IMAP,
				Port:      b.IMAPPort,
				Username:  cfg.SMTP.Username,
				Mailbox:   nz(cfg.IMAP.Mailbox, "INBOX"),
				TLS:       true,
				AutoFetch: true,
			}
			if err := s.saveCfg(cfg); err != nil {
				return err
			}
			if err := s.rt.Store.PutSecret(s.ctx, SecretIMAPPassword, pw); err != nil {
				return err
			}
			fmt.Println("  " + tui.OK("inbox watching  "+cfg.IMAP.Username))
		}
	}
	s.pause()
	return nil
}

func (s *session) unlinkMail() {
	cfg := s.cfg()
	cfg.SMTP = config.Default().SMTP
	if err := s.saveCfg(cfg); err != nil {
		fmt.Println("  " + tui.Bad(err.Error()))
		s.pause()
		return
	}
	if s.rt != nil {
		_ = s.rt.Store.DeleteSecret(s.ctx, SecretSMTPPassword)
	}
	fmt.Println("  " + tui.OK("email unlinked"))
	if cfg.IMAP.Host != "" && promptYes(s.in, "  Also stop watching the inbox", true) {
		s.clearIMAP()
	}
	s.pause()
}

func (s *session) clearIMAP() {
	cfg := s.cfg()
	cfg.IMAP = config.Default().IMAP
	cfg.IMAP.Host = ""
	if err := s.saveCfg(cfg); err != nil {
		fmt.Println("  " + tui.Bad(err.Error()))
		return
	}
	if s.rt != nil {
		_ = s.rt.Store.DeleteSecret(s.ctx, SecretIMAPPassword)
	}
	fmt.Println("  " + tui.OK("inbox cleared"))
}

func (s *session) enableInboxFromLinked() {
	cfg := s.cfg()
	brand := mailBrandLabel(cfg.SMTP.Host)
	b, ok := mailBrands()[strings.ToLower(brand)]
	if !ok || b.IMAP == "" {
		fmt.Println("  " + tui.Muted("Use custom IMAP from this screen, or link Gmail / Outlook / Yahoo first."))
		s.pause()
		return
	}
	if !s.hasMailPassword() {
		fmt.Println("  " + tui.Bad("link email first — the same app password is reused for the inbox."))
		s.pause()
		return
	}
	pw, err := s.rt.Store.GetSecret(s.ctx, SecretSMTPPassword)
	if err != nil {
		fmt.Println("  " + tui.Bad(err.Error()))
		s.pause()
		return
	}
	defer ncrypto.Zeroize(pw)
	cfg.IMAP = config.IMAPConfig{
		Host:      b.IMAP,
		Port:      b.IMAPPort,
		Username:  nz(cfg.SMTP.Username, cfg.SMTP.From),
		Mailbox:   "INBOX",
		TLS:       true,
		AutoFetch: true,
	}
	if err := s.saveCfg(cfg); err != nil {
		fmt.Println("  " + tui.Bad(err.Error()))
		s.pause()
		return
	}
	if err := s.rt.Store.PutSecret(s.ctx, SecretIMAPPassword, pw); err != nil {
		fmt.Println("  " + tui.Bad(err.Error()))
		s.pause()
		return
	}
	fmt.Println("  " + tui.OK("inbox watching  "+cfg.IMAP.Username))
	s.pause()
}
