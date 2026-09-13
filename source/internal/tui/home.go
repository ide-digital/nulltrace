package tui

import (
	"fmt"
	"strings"
)

type HomeInfo struct {
	Identity  string
	Proxy     string
	Mail      string
	Vault     string
	Hint      string
	EmailHint string
	InboxHint string
	Listings  int
	Queued    int
	Verified  int
	Manual    int
}

func RenderHome(info HomeInfo) string {
	const (
		indent = 2
		inner  = 100
		gap    = 4
		colW   = 48
		keyW   = 4
		nameW  = 11
	)
	width := indent + inner
	ind := strings.Repeat(" ", indent)

	var b strings.Builder
	b.WriteString("\n\n")
	b.WriteString(center(Banner(), width))
	b.WriteString("\n")
	meta := Muted("v"+Version) + "   " + Item("all-in-one privacy toolkit")
	b.WriteString(center(meta, width))
	b.WriteString("\n")
	b.WriteString(ind + styleRule.Render(strings.Repeat("━", inner)))
	b.WriteString("\n\n")

	ident := info.Identity
	if ident == "" {
		ident = "(none yet)"
	}
	mail := info.Mail
	if strings.TrimSpace(mail) == "" {
		mail = Muted("not linked")
	}

	b.WriteString(ind + trio(
		Label("vault")+"  "+info.Vault,
		Label("identity")+"  "+Name(ident),
		Label("mail")+"  "+mail,
		inner,
	) + "\n")

	found := Item(fmt.Sprintf("%d", info.Listings)) + Muted(" found")
	queued := Item(fmt.Sprintf("%d", info.Queued)) + Muted(" queued")
	tail := Item(fmt.Sprintf("%d", info.Verified)) + Muted(" gone")
	if info.Manual > 0 {
		tail += "    " + Item(fmt.Sprintf("%d", info.Manual)) + Muted(" web-forms")
	}
	if info.Proxy != "" {
		tail += "    " + Label("proxy") + "  " + info.Proxy
	}
	b.WriteString(ind + trio(found, queued, tail, inner))
	b.WriteString("\n\n")

	emailHint := info.EmailHint
	if emailHint == "" {
		emailHint = "Gmail / Outlook / Yahoo"
	}
	inboxHint := info.InboxHint
	if inboxHint == "" {
		inboxHint = "watch for confirmations"
	}

	left := []struct{ k, name, hint string }{
		{"1", "Scan", "find broker listings"},
		{"2", "Scrub", "queue deletion requests"},
		{"3", "Send", "deliver queued emails"},
		{"4", "Listings", "browse / export"},
		{"14", "Leaks", "password strength + dumps"},
	}
	right := []struct{ k, name, hint string }{
		{"5", "Identity", "name, email, phone"},
		{"7", "Email", emailHint},
		{"8", "Inbox", inboxHint},
		{"6", "Footprint", "username lookup"},
		{"9", "Settings", "proxy, keys, law"},
	}

	colRule := styleRule.Render(strings.Repeat("━", colW))
	b.WriteString(ind + PadRight(Title("Work"), colW) + strings.Repeat(" ", gap) + PadRight(Title("Account"), colW) + "\n")
	b.WriteString(ind + PadRight(colRule, colW) + strings.Repeat(" ", gap) + colRule + "\n")

	n := max(len(left), len(right))
	for i := 0; i < n; i++ {
		l, r := PadRight("", colW), PadRight("", colW)
		if i < len(left) {
			l = menuRow(left[i].k, left[i].name, left[i].hint, colW, keyW, nameW)
		}
		if i < len(right) {
			r = menuRow(right[i].k, right[i].name, right[i].hint, colW, keyW, nameW)
		}
		b.WriteString(ind + l + strings.Repeat(" ", gap) + r + "\n")
	}

	b.WriteString("\n")
	b.WriteString(ind + Title("Tools") + "\n")
	b.WriteString(ind + styleRule.Render(strings.Repeat("━", inner)) + "\n")
	tools := []string{
		Key("10") + "  " + Item("Playbooks"),
		Key("11") + "  " + Item("Export"),
		Key("12") + "  " + Item("Photos"),
		Key("13") + "  " + Item("Guide"),
		Key("0") + "  " + Item("Exit"),
	}
	b.WriteString(ind + Spread(tools, inner) + "\n")

	if info.Hint != "" {
		b.WriteString("\n" + ind + Muted(info.Hint) + "\n")
	}
	b.WriteString("\n" + ind + Prompt() + " ")
	return b.String()
}

func trio(a, b, c string, inner int) string {
	slot := inner / 3
	last := inner - 2*slot
	return PadRight(a, slot) + PadRight(b, slot) + PadRight(c, last)
}

func menuRow(k, name, hint string, colW, keyW, nameW int) string {
	key := PadRight(Key(k), keyW)
	nm := PadRight(Item(name), nameW)
	hintW := colW - keyW - nameW - 4
	if hintW < 0 {
		hintW = 0
	}
	h := ""
	if hint != "" && hintW > 0 {
		h = Muted(Clip(hint, hintW))
	}
	return PadRight(key+"  "+nm+"  "+h, colW)
}
