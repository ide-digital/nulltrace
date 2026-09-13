package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/4x3/nulltrace/internal/paths"
	"github.com/4x3/nulltrace/internal/tui"
	"github.com/4x3/nulltrace/pkg/ipc"
)

const listingsPage = 12

func (s *session) doListings() {
	page := 0
	filter := ""
	for {
		if s.gone() {
			return
		}
		snap, err := s.snapshot()
		if err != nil {
			clearScreen()
			printHead("Listings", "")
			fmt.Println("  " + tui.Bad(err.Error()))
			waitEnter(s.in)
			return
		}
		rows := filterListings(sortListings(snap.Exposed), filter)
		if len(snap.Exposed) == 0 {
			clearScreen()
			printHead("Listings", "Nothing stored yet. Run [1] Scan to map brokers to your identity.")
			printNav()
			fmt.Print("\n  " + tui.Prompt() + " ")
			choice := strings.TrimSpace(s.readChoice())
			if isHomeChoice(choice) {
				s.goHome = true
			}
			return
		}
		if len(rows) == 0 {
			clearScreen()
			printHead("Listings", "No sites match  "+filter)
			fmt.Println("  " + tui.Key("c") + "  clear filter")
			fmt.Println()
			printNav()
			fmt.Print("\n  " + tui.Prompt() + " ")
			choice := strings.ToLower(strings.TrimSpace(s.readChoice()))
			switch {
			case isHomeChoice(choice):
				s.goHome = true
				return
			case isBackChoice(choice), choice == "":
				return
			case choice == "c", choice == "/":
				filter = ""
				page = 0
			}
			continue
		}
		pages := (len(rows) + listingsPage - 1) / listingsPage
		if page >= pages {
			page = pages - 1
		}
		if page < 0 {
			page = 0
		}
		start := page * listingsPage
		end := start + listingsPage
		if end > len(rows) {
			end = len(rows)
		}
		chunk := rows[start:end]

		clearScreen()
		printHead("Listings", "sites that have (or likely have) data on "+nz(snap.Status.IdentityName, "unknown"))
		if filter != "" {
			fmt.Println("  " + tui.Label("filter") + "   " + tui.Name(filter) + "  " + tui.Muted("c clears"))
			fmt.Println()
		}
		fmt.Println("  " + tui.Spread([]string{
			tui.Muted("showing") + "  " + tui.Item(fmt.Sprintf("%d–%d of %d", start+1, end, len(rows))),
			tui.Muted("page") + "  " + tui.Item(fmt.Sprintf("%d/%d", page+1, pages)),
		}, 100))
		fmt.Println()
		fmt.Println("  " + tui.Spread([]string{
			tui.Key("n") + " next",
			tui.Key("p") + " prev",
			tui.Key("f") + " first",
			tui.Key("l") + " last",
			tui.Key("g") + " page",
			tui.Key("/") + " find",
			tui.Key("e") + " export",
			tui.Key("s") + " status",
		}, 100))
		fmt.Println("  " + tui.Muted("type a row number for the full record"))
		fmt.Println()
		fmt.Print(renderListingsTable(chunk))
		fmt.Println()
		printNav()
		fmt.Print("\n  " + tui.Prompt() + " ")
		choice := strings.ToLower(strings.TrimSpace(s.readChoice()))
		switch {
		case isHomeChoice(choice), choice == "q":
			s.goHome = true
			return
		case isBackChoice(choice), choice == "":
			return
		case choice == "n", choice == "+", choice == "]", choice == "d":
			if page+1 < pages {
				page++
			}
		case choice == "p", choice == "-", choice == "[", choice == "u":
			if page > 0 {
				page--
			}
		case choice == "f":
			page = 0
		case choice == "l":
			page = pages - 1
		case choice == "g":
			want := promptInt(s.in, "  Page", page+1)
			page = want - 1
		case choice == "c":
			filter = ""
			page = 0
		case strings.HasPrefix(choice, "/"):
			filter = strings.TrimSpace(strings.TrimPrefix(choice, "/"))
			if filter == "" {
				filter = strings.ToLower(strings.TrimSpace(promptLine(s.in, "  Find")))
			}
			page = 0
		case choice == "e":
			s.exportListings(snap)
		case choice == "s":
			s.doProgress()
		default:
			idx, err := strconv.Atoi(choice)
			if err != nil || idx < 1 || idx > len(chunk) {
				continue
			}
			s.showListing(chunk[idx-1])
		}
	}
}

func filterListings(rows []ipc.ExposedView, q string) []ipc.ExposedView {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return rows
	}
	var out []ipc.ExposedView
	for _, r := range rows {
		blob := strings.ToLower(strings.Join([]string{
			r.BrokerName, r.BrokerID, r.Domain, r.Category, r.Status, r.ContactEmail, r.Notes,
		}, " "))
		if strings.Contains(blob, q) {
			out = append(out, r)
		}
	}
	return out
}

func (s *session) showListing(r ipc.ExposedView) {
	clearScreen()
	printHead(nz(r.BrokerName, r.BrokerID), r.Domain)
	kv := func(k, v string) {
		if strings.TrimSpace(v) == "" {
			v = "—"
		}
		fmt.Printf("  %s  %s\n", tui.Label(fmt.Sprintf("%-12s", k)), v)
	}
	kv("category", humanCategory(r.Category))
	kv("how", humanMechanism(r.Mechanism))
	kv("risk", humanRisk(r.RiskTier))
	kv("status", humanStatus(r.Status))
	if r.ActionState != "" {
		kv("queue", humanStatus(r.ActionState)+"  "+humanMechanism(r.ActionStrategy))
	}
	kv("first seen", prettyTime(r.Detected))
	if r.LastVerified != "" {
		kv("verified", prettyTime(r.LastVerified))
	}
	if r.Confidence > 0 {
		kv("confidence", fmt.Sprintf("%.0f%%", r.Confidence*100))
	}
	kv("profile", r.ProfileURL)
	kv("opt-out", r.OptOutURL)
	kv("email", r.ContactEmail)
	if r.Notes != "" {
		fmt.Println()
		fmt.Println("  " + tui.Label("notes"))
		fmt.Println("  " + tui.Muted(wrapNotes(r.Notes, 72)))
	}
	fmt.Println()
	fmt.Println("  " + tui.Key("1") + "  Open profile in browser")
	fmt.Println("  " + tui.Key("2") + "  Open opt-out page")
	fmt.Println("  " + tui.Key("3") + "  Mark this listing verified gone")
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
		if r.ProfileURL == "" {
			fmt.Println("  " + tui.Muted("no profile URL on file"))
			s.pause()
			return
		}
		_ = openURL(r.ProfileURL)
	case choice == "2":
		if r.OptOutURL == "" {
			fmt.Println("  " + tui.Muted("no opt-out URL on file"))
			s.pause()
			return
		}
		_ = openURL(r.OptOutURL)
	case choice == "3":
		if err := s.verifyRecord(r.ID); err != nil {
			fmt.Println("  " + tui.Bad(err.Error()))
		} else {
			fmt.Println("  " + tui.OK("marked verified gone"))
		}
		s.pause()
	}
}

func (s *session) exportListings(snap ipc.Snapshot) {
	fmt.Println()
	fmt.Println("  " + tui.Label("Export listings"))
	fmt.Println("  " + tui.Key("1") + "  Markdown   (.md)")
	fmt.Println("  " + tui.Key("2") + "  Plain text (.txt)")
	fmt.Println()
	printNav()
	fmt.Print("\n  " + tui.Prompt() + " ")
	format := ""
	choice := strings.TrimSpace(s.readChoice())
	switch {
	case isHomeChoice(choice):
		s.goHome = true
		return
	case isBackChoice(choice), choice == "":
		return
	case choice == "1":
		format = "md"
	case choice == "2":
		format = "txt"
	default:
		return
	}
	who := strings.TrimSpace(snap.Identity.First + " " + snap.Identity.Last)
	if who == "" {
		who = snap.Status.IdentityName
	}
	body := renderListingsExport(format, who, sortListings(snap.Exposed))
	dir := listingsExportDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		fmt.Println("  " + tui.Bad(err.Error()))
		s.pause()
		return
	}
	stamp := time.Now().Format("2006-01-02")
	path := filepath.Join(dir, "nulltrace-listings-"+stamp+"."+format)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		fmt.Println("  " + tui.Bad(err.Error()))
		s.pause()
		return
	}
	fmt.Println("  " + tui.OK("saved"))
	fmt.Println("  " + tui.Muted(path))
	if promptYes(s.in, "  Open the folder", true) {
		_ = openFolder(dir)
	}
	s.pause()
}

func listingsExportDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		desk := filepath.Join(home, "Desktop")
		if st, err := os.Stat(desk); err == nil && st.IsDir() {
			return desk
		}
	}
	if dirs, err := paths.Resolve(); err == nil {
		return dirs.AuditLogs
	}
	return "."
}

func sortListings(in []ipc.ExposedView) []ipc.ExposedView {
	out := append([]ipc.ExposedView(nil), in...)
	sort.Slice(out, func(i, j int) bool {
		ai := strings.ToLower(nz(out[i].BrokerName, out[i].BrokerID))
		aj := strings.ToLower(nz(out[j].BrokerName, out[j].BrokerID))
		return ai < aj
	})
	return out
}

func renderListingsExport(format, identity string, rows []ipc.ExposedView) string {
	var b strings.Builder
	now := time.Now().Format("2006-01-02 15:04")
	if format == "md" {
		b.WriteString("# NullTrace listings\n\n")
		if identity != "" {
			b.WriteString("**Identity:** ")
			b.WriteString(identity)
			b.WriteString("\n\n")
		}
		b.WriteString("Generated ")
		b.WriteString(now)
		b.WriteString("  ·  ")
		b.WriteString(strconv.Itoa(len(rows)))
		b.WriteString(" sites\n\n")
		for i, r := range rows {
			b.WriteString("## ")
			b.WriteString(strconv.Itoa(i + 1))
			b.WriteString(". ")
			b.WriteString(nz(r.BrokerName, r.BrokerID))
			b.WriteString("\n\n")
			if r.Domain != "" {
				b.WriteString("- **Site:** ")
				b.WriteString(r.Domain)
				b.WriteString("\n")
			}
			writeMDField(&b, "Category", humanCategory(r.Category))
			writeMDField(&b, "How they take opt-outs", humanMechanism(r.Mechanism))
			writeMDField(&b, "Risk", humanRisk(r.RiskTier))
			writeMDField(&b, "Status", humanStatus(r.Status))
			if r.ActionState != "" {
				writeMDField(&b, "Queue", humanStatus(r.ActionState))
			}
			writeMDField(&b, "First seen", prettyTime(r.Detected))
			writeMDField(&b, "Profile", r.ProfileURL)
			writeMDField(&b, "Opt-out page", r.OptOutURL)
			writeMDField(&b, "Privacy email", r.ContactEmail)
			if r.Notes != "" {
				b.WriteString("\n")
				b.WriteString(r.Notes)
				b.WriteString("\n")
			}
			b.WriteString("\n")
		}
		return b.String()
	}

	b.WriteString("NULLTRACE LISTINGS\n")
	b.WriteString(strings.Repeat("=", 72) + "\n")
	if identity != "" {
		b.WriteString("Identity: ")
		b.WriteString(identity)
		b.WriteString("\n")
	}
	b.WriteString("Generated: ")
	b.WriteString(now)
	b.WriteString("\n")
	b.WriteString("Sites: ")
	b.WriteString(strconv.Itoa(len(rows)))
	b.WriteString("\n")
	for i, r := range rows {
		b.WriteString("\n")
		b.WriteString(strings.Repeat("-", 72))
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, nz(r.BrokerName, r.BrokerID)))
		writeTXTField(&b, "Site", r.Domain)
		writeTXTField(&b, "Category", humanCategory(r.Category))
		writeTXTField(&b, "How", humanMechanism(r.Mechanism))
		writeTXTField(&b, "Risk", humanRisk(r.RiskTier))
		writeTXTField(&b, "Status", humanStatus(r.Status))
		if r.ActionState != "" {
			writeTXTField(&b, "Queue", humanStatus(r.ActionState))
		}
		writeTXTField(&b, "First seen", prettyTime(r.Detected))
		writeTXTField(&b, "Profile", r.ProfileURL)
		writeTXTField(&b, "Opt-out", r.OptOutURL)
		writeTXTField(&b, "Email", r.ContactEmail)
		if r.Notes != "" {
			b.WriteString("  Notes: ")
			b.WriteString(r.Notes)
			b.WriteString("\n")
		}
	}
	return b.String()
}

func writeMDField(b *strings.Builder, k, v string) {
	if strings.TrimSpace(v) == "" {
		return
	}
	b.WriteString("- **")
	b.WriteString(k)
	b.WriteString(":** ")
	b.WriteString(v)
	b.WriteString("\n")
}

func writeTXTField(b *strings.Builder, k, v string) {
	if strings.TrimSpace(v) == "" {
		return
	}
	b.WriteString("  ")
	b.WriteString(k)
	b.WriteString(": ")
	b.WriteString(v)
	b.WriteString("\n")
}

func humanCategory(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "_", " ")
	if s == "" {
		return ""
	}
	return strings.ToLower(s)
}

func humanMechanism(s string) string {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "EMAIL_LEGAL", "SMTP_DEMAND":
		return "deletion email"
	case "WEB_FORM", "MANUAL_WEB_FORM":
		return "web form"
	case "BROWSER_AUTOMATION":
		return "browser form"
	case "HYBRID":
		return "email + web form"
	case "API":
		return "API"
	default:
		return strings.ToLower(strings.ReplaceAll(s, "_", " "))
	}
}

func renderListingsTable(chunk []ipc.ExposedView) string {
	const numW, nameW, siteW, riskW, statW = 4, 30, 24, 10, 14
	cell := func(parts ...string) string {
		return "  " + parts[0] + "  " + parts[1] + "  " + parts[2] + "  " + parts[3] + "  " + parts[4] + "\n"
	}
	var b strings.Builder
	b.WriteString(cell(
		tui.PadRight("", numW),
		tui.PadRight(tui.Label("provider"), nameW),
		tui.PadRight(tui.Label("site"), siteW),
		tui.PadRight(tui.Label("risk"), riskW),
		tui.PadRight(tui.Label("status"), statW),
	))
	for i, r := range chunk {
		risk := humanRisk(r.RiskTier)
		st := humanStatus(r.Status)
		riskS := tui.PadRight(tui.Item(risk), riskW)
		if strings.EqualFold(r.RiskTier, "CRITICAL") || strings.EqualFold(r.RiskTier, "HIGH") {
			riskS = tui.PadRight(tui.Name(risk), riskW)
		}
		stS := tui.PadRight(tui.Muted(st), statW)
		if strings.EqualFold(r.Status, "VERIFIED_REMOVED") {
			stS = tui.PadRight(tui.OK(st), statW)
		}
		b.WriteString(cell(
			tui.PadRight(tui.Key(strconv.Itoa(i+1)), numW),
			tui.PadRight(tui.Clip(nz(r.BrokerName, r.BrokerID), nameW), nameW),
			tui.PadRight(tui.Clip(nz(r.Domain, r.BrokerID), siteW), siteW),
			riskS,
			stS,
		))
	}
	return b.String()
}

func humanRisk(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return "—"
	}
	return s
}

func humanStatus(s string) string {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "DISCOVERED":
		return "found"
	case "APPROVED_FOR_SCRUB":
		return "queued"
	case "VERIFIED_REMOVED":
		return "gone"
	case "PENDING_REVIEW":
		return "re-check"
	case "IGNORED":
		return "ignored"
	case "QUEUED":
		return "queued"
	case "SUBMITTED":
		return "sent"
	case "AWAITING_CONFIRMATION":
		return "awaiting confirm"
	case "AWAITING_MANUAL":
		return "needs web form"
	case "COMPLETED":
		return "done"
	case "FAILED":
		return "failed"
	case "REJECTED":
		return "rejected"
	default:
		return strings.ToLower(strings.ReplaceAll(s, "_", " "))
	}
}

func prettyTime(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Local().Format("2006-01-02")
	}
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if n <= 1 || len(s) <= n {
		return s
	}
	return s[:n-1] + "..."
}

func wrapNotes(s string, width int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= width {
		return s
	}
	return s[:width-1] + "..."
}
