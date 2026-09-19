package app

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/4x3/nulltrace/internal/broker"
	"github.com/4x3/nulltrace/internal/config"
	"github.com/4x3/nulltrace/internal/engine/playbook"
	"github.com/4x3/nulltrace/internal/paths"
	"github.com/4x3/nulltrace/internal/tui"
	"github.com/4x3/nulltrace/pkg/ipc"
	"github.com/4x3/nulltrace/pkg/opsec"
)

type session struct {
	ctx    context.Context
	rt     *Runtime
	cl     *ipc.Client
	in     *bufio.Reader
	hint   string
	goHome bool
}

func runInteractive(ctx context.Context, rt *Runtime, cl *ipc.Client) error {
	tui.RestoreCookedConsole()
	s := &session{ctx: ctx, rt: rt, cl: cl, in: bufio.NewReader(os.Stdin)}
	if rt == nil && cl == nil {
		return fmt.Errorf("vault is not available")
	}
	s.hint = "First time?  [5] Identity  →  [1] Scan  →  [14] Leaks  →  [4] Listings"
	for {
		if err := s.ctx.Err(); err != nil {
			return err
		}
		s.goHome = false
		s.drawHome()
		choice := strings.TrimSpace(s.readChoice())
		if choice == "" {
			continue
		}
		switch choice {
		case "0", "q", "quit", "exit":
			return nil
		case "1":
			s.doScan()
		case "2":
			s.doScrub()
		case "3":
			s.doSend()
		case "4":
			s.doListings()
		case "5":
			s.doIdentity()
		case "6":
			s.doFootprint()
		case "7":
			s.doSMTP()
		case "8":
			s.doIMAP()
		case "9":
			s.doSettings()
		case "10":
			s.doPlaybooks()
		case "11":
			s.doExport()
		case "12":
			s.doExif()
		case "13":
			s.doGuide()
		case "14":
			s.doLeaks()
		default:
			s.hint = "Unknown option " + choice + " — type a number from the menu"
		}
	}
}

func (s *session) readChoice() string {
	line, _ := s.in.ReadString('\n')
	return strings.TrimSpace(line)
}

func (s *session) drawHome() {
	clearScreen()
	info := tui.HomeInfo{Vault: tui.Bad("locked"), Hint: s.hint}
	if snap, err := s.snapshot(); err == nil {
		st := snap.Status
		if st.Unlocked {
			info.Vault = tui.OK("unlocked")
		}
		info.Identity = st.IdentityName
		info.Listings = st.ExposedRecords
		info.Queued = st.ActiveRemovals
		info.Verified = st.VerifiedRemoved
		info.Manual = st.ManualPending
	}
	cfg := s.cfg()
	if cfg.Proxy != "" {
		info.Proxy = tui.Item("on")
	}
	if brand, addr, ok := s.linkedMail(); ok {
		info.Mail = tui.OK(brand) + "  " + tui.Name(addr)
		info.EmailHint = "linked  " + addr
	} else if cfg.SMTP.Host != "" {
		info.Mail = tui.Warn("needs app password")
		info.EmailHint = "finish linking"
	} else {
		info.Mail = tui.Muted("not linked")
		info.EmailHint = "Gmail / Outlook / Yahoo"
	}
	if cfg.IMAP.Host != "" && s.rt != nil && s.rt.hasSecret(s.ctx, SecretIMAPPassword) {
		info.InboxHint = "watching  " + nz(cfg.IMAP.Username, cfg.IMAP.Host)
	} else {
		info.InboxHint = "watch for confirmations"
	}
	fmt.Print(tui.RenderHome(info))
	s.hint = ""
}

func (s *session) cfg() config.File {
	if s.rt != nil {
		return s.rt.Cfg
	}
	dirs, err := paths.Resolve()
	if err != nil {
		return config.Default()
	}
	cfg, err := config.Load(dirs.ConfigFile)
	if err != nil {
		return config.Default()
	}
	return cfg
}

func (s *session) saveCfg(cfg config.File) error {
	dirs, err := paths.Resolve()
	if err != nil {
		return err
	}
	if err := config.Save(dirs.ConfigFile, cfg); err != nil {
		return err
	}
	if s.rt != nil {
		s.rt.Cfg = cfg
	}
	return nil
}

func (s *session) snapshot() (ipc.Snapshot, error) {
	if s.cl != nil {
		rep, err := s.cl.Call(s.ctx, ipc.CmdSnapshot, nil)
		if err != nil {
			return ipc.Snapshot{}, err
		}
		return ipc.UnmarshalPayload[ipc.Snapshot](rep)
	}
	return s.rt.Snapshot(s.ctx)
}

func (s *session) hasMailPassword() bool {
	if s.rt != nil {
		return s.rt.hasSecret(s.ctx, SecretSMTPPassword)
	}
	return false
}

func (s *session) needLocal(what string) bool {
	if s.rt != nil {
		return true
	}
	fmt.Println()
	fmt.Println("  " + tui.Warn(what+" needs this window to hold the vault open."))
	fmt.Println("  " + tui.Muted("Stop the background daemon (if you started one) and open NullTrace again."))
	waitEnter(s.in)
	return false
}

func (s *session) doScan() {
	clearScreen()
	printHead("Scan", "Maps the identity in your vault onto the broker list. This does not log into sites.")
	snap, err := s.snapshot()
	if err == nil && snap.Identity.ID == "" {
		fmt.Println("  " + tui.Bad("No identity yet. Use [5] Identity first."))
		waitEnter(s.in)
		return
	}
	hibp := false
	fmt.Println()
	fmt.Println("  " + tui.Muted("scanning…"))
	var res ipc.ScanResult
	if s.cl != nil {
		rep, e := s.cl.Call(s.ctx, ipc.CmdScan, ipc.ScanRequest{EnableHIBP: hibp})
		if e != nil {
			fmt.Println("  " + tui.Bad(e.Error()))
			waitEnter(s.in)
			return
		}
		res, err = ipc.UnmarshalPayload[ipc.ScanResult](rep)
	} else {
		_, res, err = s.rt.Scan(s.ctx, hibp)
	}
	if err != nil {
		fmt.Println("  " + tui.Bad(err.Error()))
		waitEnter(s.in)
		return
	}
	fmt.Println()
	fmt.Printf("  %s  listings stored: %d\n", tui.OK("done"), res.Persisted)
	fmt.Printf("  findings %d    breaches %d\n", res.FindingCount, res.BreachCount)
	fmt.Println()
	fmt.Println("  Next: [2] Scrub to queue deletion requests.")
	s.hint = "Scan finished. [4] Listings to browse them, then [2] Scrub."
	waitEnter(s.in)
}

func (s *session) doScrub() {
	clearScreen()
	printHead("Scrub", "Queues a legal deletion email for every broker that publishes one, and a web-form playbook for the rest. Nothing is sent until [3] Send.")
	if !promptYes(s.in, "  Queue deletion requests now", true) {
		return
	}
	var res ipc.ScrubResult
	var err error
	if s.cl != nil {
		rep, e := s.cl.Call(s.ctx, ipc.CmdScrub, ipc.ScrubRequest{All: true})
		if e != nil {
			fmt.Println("  " + tui.Bad(e.Error()))
			waitEnter(s.in)
			return
		}
		res, err = ipc.UnmarshalPayload[ipc.ScrubResult](rep)
	} else {
		res, err = s.rt.Scrub(s.ctx)
	}
	if err != nil {
		fmt.Println("  " + tui.Bad(err.Error()))
		waitEnter(s.in)
		return
	}
	fmt.Println()
	fmt.Printf("  %s  email queue %d    web-forms %d    skipped %d\n",
		tui.OK("queued"), res.QueuedSMTP, res.QueuedManual, res.Skipped)
	cfg := s.cfg()
	if cfg.SMTP.Host == "" || !s.hasMailPassword() {
		fmt.Println("  " + tui.Warn("Email is not set up yet — [7] Email, then [3] Send."))
	} else {
		fmt.Println("  Next: [3] Send to deliver the queued mail.")
	}
	if res.QueuedManual > 0 {
		fmt.Println("  Web-form sites: [10] Playbooks.")
	}
	waitEnter(s.in)
}

func (s *session) doSend() {
	if !s.needLocal("Sending mail") {
		return
	}
	clearScreen()
	printHead("Send queued emails", "Sends legal deletion demands through the linked mailbox. Keep this window open. Ctrl+C stops between messages.")
	cfg := s.cfg()
	if cfg.SMTP.Host == "" || !s.hasMailPassword() {
		fmt.Println("  " + tui.Bad("Email is not linked. Use [7] Email first."))
		waitEnter(s.in)
		return
	}
	if !promptYes(s.in, "  Send now", true) {
		return
	}
	fmt.Println()
	sent, fail, err := s.rt.SendQueuedSMTP(s.ctx, func(sent, fail int, name string) {
		fmt.Printf("  sent %d  failed %d  %s\n", sent, fail, name)
	})
	fmt.Println()
	if err != nil && err != context.Canceled {
		fmt.Println("  " + tui.Bad(err.Error()))
	}
	fmt.Printf("  %s  delivered %d    failed %d\n", tui.OK("finished"), sent, fail)
	waitEnter(s.in)
}

func (s *session) doProgress() {
	for {
		if s.gone() {
			return
		}
		clearScreen()
		printHead("Progress", "")
		snap, err := s.snapshot()
		if err != nil {
			fmt.Println("  " + tui.Bad(err.Error()))
			s.pause()
			return
		}
		st := snap.Status
		fmt.Printf("  identity %s\n", nz(st.IdentityName, "(none)"))
		fmt.Printf("  listings %d    queued/active %d    verified gone %d\n", st.ExposedRecords, st.ActiveRemovals, st.VerifiedRemoved)
		fmt.Printf("  awaiting confirm %d    web-forms %d    failed %d    overdue %d\n",
			st.AwaitingConfirm, st.ManualPending, st.FailedActions, st.Overdue)
		fmt.Println()
		fmt.Println("  " + tui.Title("Recent actions"))
		n := len(snap.Actions)
		start := 0
		if n > 18 {
			start = n - 18
		}
		if n == 0 {
			fmt.Println("  " + tui.Muted("(none yet — Scan then Scrub)"))
		}
		for _, a := range snap.Actions[start:] {
			short := a.ID
			if len(short) > 8 {
				short = short[:8]
			}
			errbit := ""
			if a.LastError != "" {
				errbit = "  " + tui.Bad(trimErr(a.LastError))
			}
			fmt.Printf("  %s  %-22s  %-22s%s\n", tui.Muted(short), a.BrokerName, a.State, errbit)
		}
		fmt.Println()
		fmt.Println("  " + tui.Key("1") + "  Mark an action completed")
		fmt.Println("  " + tui.Key("2") + "  Mark a listing verified gone")
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
			id := promptLine(s.in, "  Action id (first 8 characters is enough)")
			if full, err := s.resolveAction(snap, id); err != nil {
				fmt.Println("  " + tui.Bad(err.Error()))
			} else if err := s.completeAction(full); err != nil {
				fmt.Println("  " + tui.Bad(err.Error()))
			} else {
				fmt.Println("  " + tui.OK("marked completed"))
			}
			s.pause()
		case choice == "2":
			id := promptLine(s.in, "  Exposed-record id")
			if id == "" {
				continue
			}
			if err := s.verifyRecord(id); err != nil {
				fmt.Println("  " + tui.Bad(err.Error()))
			} else {
				fmt.Println("  " + tui.OK("marked verified gone"))
			}
			s.pause()
		}
	}
}

func (s *session) resolveAction(snap ipc.Snapshot, prefix string) (string, error) {
	prefix = strings.TrimSpace(strings.ToLower(prefix))
	if prefix == "" {
		return "", fmt.Errorf("no id")
	}
	var hits []string
	for _, a := range snap.Actions {
		if strings.HasPrefix(strings.ToLower(a.ID), prefix) || a.ID == prefix {
			hits = append(hits, a.ID)
		}
	}
	if len(hits) == 1 {
		return hits[0], nil
	}
	if len(hits) == 0 {
		return "", fmt.Errorf("no action starts with %q", prefix)
	}
	return "", fmt.Errorf("ambiguous — type more of the id")
}

func (s *session) completeAction(id string) error {
	if s.cl != nil {
		_, err := s.cl.Call(s.ctx, ipc.CmdActionComplete, ipc.ActionIDRequest{ActionID: id})
		return err
	}
	return s.rt.CompleteAction(s.ctx, id)
}

func (s *session) verifyRecord(id string) error {
	if s.cl != nil {
		_, err := s.cl.Call(s.ctx, ipc.CmdActionVerify, ipc.ActionIDRequest{RecordID: id})
		return err
	}
	return s.rt.VerifyRemoved(s.ctx, id)
}

func (s *session) doIdentity() {
	for {
		if s.gone() {
			return
		}
		clearScreen()
		printHead("Identity", "This is the person whose listings you want taken down.")
		snap, err := s.snapshot()
		if err != nil {
			fmt.Println("  " + tui.Bad(err.Error()))
			s.pause()
			return
		}
		id := snap.Identity
		if id.ID == "" {
			fmt.Println("  " + tui.Muted("No identity stored yet."))
		} else {
			fmt.Printf("  %s %s %s\n", tui.Item(id.First), id.Middle, tui.Item(id.Last))
			if id.DOB != "" {
				fmt.Println("  dob     " + id.DOB)
			}
			fmt.Println("  email   " + nz(strings.Join(id.Emails, ", "), "—"))
			fmt.Println("  phone   " + nz(strings.Join(id.Phones, ", "), "—"))
			fmt.Println("  city    " + nz(strings.Join(id.Cities, ", "), "—"))
			fmt.Println("  address " + nz(strings.Join(id.Addresses, ", "), "—"))
			fmt.Println("  user    " + nz(strings.Join(id.Usernames, ", "), "—"))
		}
		fmt.Println()
		fmt.Println("  " + tui.Key("1") + "  Set / replace name")
		fmt.Println("  " + tui.Key("2") + "  Add email")
		fmt.Println("  " + tui.Key("3") + "  Add phone")
		fmt.Println("  " + tui.Key("4") + "  Add city")
		fmt.Println("  " + tui.Key("5") + "  Add street address")
		fmt.Println("  " + tui.Key("6") + "  Add username")
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
			s.addIdentity()
		case choice == "2":
			s.addAttr(string(broker.AttrEmail), "Email")
		case choice == "3":
			s.addAttr(string(broker.AttrPhone), "Phone")
		case choice == "4":
			s.addAttr(string(broker.AttrCity), "City (no state in this field)")
		case choice == "5":
			s.addAttr(string(broker.AttrAddress), "Street address")
		case choice == "6":
			s.addAttr(string(broker.AttrUsername), "Username")
		}
	}
}

func (s *session) addIdentity() {
	first := promptChecked(s.in, "  First name", true, func(v string) error { return checkPersonName(v, false) })
	last := promptChecked(s.in, "  Last name", true, func(v string) error { return checkPersonName(v, false) })
	middle := promptChecked(s.in, "  Middle name (optional)", false, func(v string) error { return checkPersonName(v, true) })
	dob := promptChecked(s.in, "  Date of birth (optional)", false, checkDOB)
	email := promptChecked(s.in, "  Email (optional)", false, checkEmail)
	phone := promptChecked(s.in, "  Phone (optional)", false, checkPhone)
	city := promptCity(s.in, "  City (optional)")
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
	var err error
	if s.cl != nil {
		_, err = s.cl.Call(s.ctx, ipc.CmdIdentityAdd, req)
	} else if s.rt != nil {
		_, err = s.rt.AddIdentity(s.ctx, req)
	} else {
		fmt.Println("  " + tui.Bad("vault is not available"))
		s.pause()
		return
	}
	if err != nil {
		fmt.Println("  " + tui.Bad(err.Error()))
		s.pause()
		return
	}
	fmt.Println("  " + tui.OK("saved "+first+" "+last))
	s.pause()
}

func (s *session) addAttr(typ, label string) {
	var val string
	if strings.EqualFold(typ, string(broker.AttrCity)) {
		val = promptCity(s.in, "  "+label)
	} else {
		val = promptChecked(s.in, "  "+label, false, func(v string) error {
			return checkAttrValue(typ, v)
		})
	}
	if val == "" {
		return
	}
	req := ipc.AttrAddRequest{Type: typ, Value: val}
	var err error
	if s.cl != nil {
		_, err = s.cl.Call(s.ctx, ipc.CmdAttrAdd, req)
	} else {
		err = s.rt.AddAttribute(s.ctx, req)
	}
	if err != nil {
		fmt.Println("  " + tui.Bad(err.Error()))
		s.pause()
		return
	}
	fmt.Println("  " + tui.OK("added"))
	s.pause()
}

func (s *session) doFootprint() {
	clearScreen()
	printHead("Footprint", "Checks whether a username already exists on public sites.")
	user := promptLine(s.in, "  Username")
	if user == "" {
		return
	}
	persist := promptYes(s.in, "  Save hits as listings in the vault", false)
	fmt.Println("  " + tui.Warn("checking…"))
	var res ipc.FootprintResult
	var err error
	if s.cl != nil {
		rep, e := s.cl.Call(s.ctx, ipc.CmdFootprint, ipc.FootprintRequest{Username: user, Persist: persist})
		if e != nil {
			fmt.Println("  " + tui.Bad(e.Error()))
			waitEnter(s.in)
			return
		}
		res, err = ipc.UnmarshalPayload[ipc.FootprintResult](rep)
	} else {
		res, err = s.rt.Footprint(s.ctx, user, persist)
	}
	if err != nil {
		fmt.Println("  " + tui.Bad(err.Error()))
		waitEnter(s.in)
		return
	}
	fmt.Printf("\n  %s: checked %d, found %d\n\n", res.Username, res.Checked, res.Found)
	for _, r := range res.Results {
		mark := tui.Muted("-")
		switch r.Status {
		case "FOUND":
			mark = tui.OK("+")
		case "ERROR":
			mark = tui.Bad("!")
		}
		extra := ""
		if r.Detail != "" {
			extra = "  " + tui.Muted(r.Detail)
		}
		fmt.Printf("  %s  %-8s  %s%s\n", mark, r.Status, r.Name, extra)
	}
	waitEnter(s.in)
}

func (s *session) doExport() {
	clearScreen()
	printHead("Export", "Write an audit file of what NullTrace has stored.")
	fmt.Println("  " + tui.Key("1") + "  Markdown")
	fmt.Println("  " + tui.Key("2") + "  JSON")
	fmt.Println("  " + tui.Key("3") + "  CSV")
	fmt.Println()
	printNav()
	fmt.Print("\n  " + tui.Prompt() + " ")
	format := "md"
	choice := strings.TrimSpace(s.readChoice())
	switch {
	case isHomeChoice(choice):
		s.goHome = true
		return
	case isBackChoice(choice), choice == "":
		return
	case choice == "2":
		format = "json"
	case choice == "3":
		format = "csv"
	}
	var res ipc.ExportResult
	var err error
	if s.cl != nil {
		rep, e := s.cl.Call(s.ctx, ipc.CmdExport, ipc.ExportRequest{Format: format})
		if e != nil {
			fmt.Println("  " + tui.Bad(e.Error()))
			waitEnter(s.in)
			return
		}
		res, err = ipc.UnmarshalPayload[ipc.ExportResult](rep)
	} else {
		res, err = s.rt.Export(s.ctx, format)
	}
	if err != nil {
		fmt.Println("  " + tui.Bad(err.Error()))
		waitEnter(s.in)
		return
	}
	fmt.Println("  " + tui.OK("wrote "+res.Path))
	if promptYes(s.in, "  Open the folder", true) {
		_ = openFolder(filepath.Dir(res.Path))
	}
	waitEnter(s.in)
}

func (s *session) doExif() {
	clearScreen()
	printHead("Strip photo metadata", "Removes GPS and camera tags from a JPEG or PNG. Leave the output blank to overwrite the same file.")
	src := promptLine(s.in, "  File path")
	if src == "" {
		return
	}
	dst := promptLine(s.in, "  Output path (optional)")
	if dst == "" {
		dst = src
	}
	if err := opsec.StripFile(src, dst); err != nil {
		fmt.Println("  " + tui.Bad(err.Error()))
		waitEnter(s.in)
		return
	}
	fmt.Println("  " + tui.OK("stripped "+dst))
	waitEnter(s.in)
}

func (s *session) doPlaybooks() {
	if s.rt == nil && s.cl == nil {
		return
	}
	for {
		if s.gone() {
			return
		}
		clearScreen()
		printHead("Web-form playbooks", "These brokers don't publish a privacy email. Open the URL and follow the steps.")
		var entries []playbook.Entry
		if s.rt != nil {
			entries = playbook.AllManual(s.rt.Reg.All())
		} else {
			snap, err := s.snapshot()
			if err != nil {
				fmt.Println("  " + tui.Bad(err.Error()))
				s.pause()
				return
			}
			fmt.Println(strings.Join(snap.Playbooks, "\n"+strings.Repeat("-", 40)+"\n"))
			s.pause()
			return
		}
		if len(entries) == 0 {
			fmt.Println("  " + tui.Muted("No web-form playbooks in the catalog."))
			s.pause()
			return
		}
		for i, e := range entries {
			fmt.Printf("  %s  %s  %s\n", tui.Key(fmt.Sprintf("%d", i+1)), tui.Item(e.BrokerName), tui.Muted(e.Domain))
		}
		fmt.Println()
		printNav()
		fmt.Print("\n  " + tui.Prompt() + " ")
		raw := strings.TrimSpace(s.readChoice())
		switch {
		case isHomeChoice(raw):
			s.goHome = true
			return
		case isBackChoice(raw), raw == "":
			return
		}
		idx := 0
		fmt.Sscanf(raw, "%d", &idx)
		if idx < 1 || idx > len(entries) {
			continue
		}
		e := entries[idx-1]
		clearScreen()
		printHead(e.BrokerName, e.Domain)
		fmt.Println(playbook.Render(e))
		fmt.Println()
		fmt.Println("  " + tui.Key("1") + "  Open opt-out page")
		fmt.Println()
		printNav()
		fmt.Print("\n  " + tui.Prompt() + " ")
		choice := strings.TrimSpace(s.readChoice())
		switch {
		case isHomeChoice(choice):
			s.goHome = true
			return
		case isBackChoice(choice), choice == "":
			continue
		case choice == "1":
			if e.OptOutURL != "" {
				_ = openURL(e.OptOutURL)
			} else {
				fmt.Println("  " + tui.Muted("no opt-out URL on file"))
				s.pause()
			}
		}
		if s.rt != nil && s.rt.Cfg.Browser.Enabled && promptYes(s.in, "  Try the in-app browser fill", false) {
			res, err := s.rt.Automate(s.ctx, e.BrokerID, !s.rt.Cfg.Browser.Headless)
			if err != nil {
				fmt.Println("  " + tui.Bad(err.Error()))
			} else {
				fmt.Printf("  %s: %s %s\n", res.BrokerName, res.Status, res.Detail)
			}
			s.pause()
		}
	}
}

func (s *session) doGuide() {
	clearScreen()
	printHead("How to clear your listings", "")
	fmt.Println("  1.  " + tui.Item("Identity") + "  — who the brokers listed (name, email, phone, city).")
	fmt.Println("  2.  " + tui.Item("Scan") + "      — match that person to ~400 people-search / broker sites.")
	fmt.Println("  3.  " + tui.Item("Listings") + "  — scroll every site, then export a .txt or .md report.")
	fmt.Println("      " + tui.Muted("m jumps to the main menu from any nested screen."))
	fmt.Println("  4.  " + tui.Item("Scrub") + "     — queue a deletion demand for each one.")
	fmt.Println("  5.  " + tui.Item("Email") + "     — pick Gmail / Outlook / Yahoo; a browser opens so you")
	fmt.Println("      can create an App Password. Paste it back here. Your normal")
	fmt.Println("      mailbox password will not work.")
	fmt.Println("  6.  " + tui.Item("Send") + "      — deliver the queue. Keep this window open.")
	fmt.Println("  7.  " + tui.Item("Playbooks") + " — the leftover sites that only have a web form.")
	fmt.Println("  8.  " + tui.Item("Leaks") + "     — check a password for strength and dump hits.")
	fmt.Println("      Nothing is stored. No API key.")
	fmt.Println()
	fmt.Println("  Optional")
	fmt.Println("  •  Proxy in Settings if you want traffic through a local or HTTP proxy.")
	fmt.Println("  •  Inbox (IMAP) to catch “click to confirm” emails.")
	fmt.Println()
	fmt.Println("  Your vault stays on this PC:")
	if dirs, err := paths.Resolve(); err == nil {
		fmt.Println("  " + tui.Muted(dirs.VaultDB))
		fmt.Println("  " + tui.Muted(dirs.ConfigFile))
	}
	fmt.Println()
	fmt.Println("  Nothing is uploaded by NullTrace itself. Emails go only to the")
	fmt.Println("  privacy addresses the brokers publish, from the mailbox you configure.")
	waitEnter(s.in)
}

func nz(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

func trimErr(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 60 {
		return s[:57] + "..."
	}
	return s
}

func openURL(raw string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("cmd", "/c", "start", "", raw).Start()
	case "darwin":
		return exec.Command("open", raw).Start()
	default:
		return exec.Command("xdg-open", raw).Start()
	}
}

func openFolder(path string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("explorer", path).Start()
	case "darwin":
		return exec.Command("open", path).Start()
	default:
		return exec.Command("xdg-open", path).Start()
	}
}
