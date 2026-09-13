package tui

import (
	"fmt"
	"strings"

	"github.com/4x3/nulltrace/pkg/ipc"
)

func renderStreamer(s ipc.Snapshot, cursor, height int) string {
	var b strings.Builder
	b.WriteString(styleTitle.Render("  AUDIT STREAM"))
	b.WriteString("\n")
	if len(s.Audit) == 0 {
		b.WriteString(styleMuted.Render("  ledger empty"))
		return b.String()
	}
	start, end := window(cursor, len(s.Audit), height)
	for i := start; i < end; i++ {
		a := s.Audit[i]
		line := fmt.Sprintf("  %s  %-22s  %s", a.Timestamp, a.EventType, a.BrokerID)
		if i == cursor {
			b.WriteString(styleSel.Render(line))
			b.WriteString("\n")
			if a.Details != "" {
				b.WriteString(styleMuted.Render("    " + truncate(a.Details, 100)))
				b.WriteString("\n")
			}
		} else {
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	return b.String()
}

func renderPlaybooks(s ipc.Snapshot, cursor, height int) string {
	var b strings.Builder
	b.WriteString(styleTitle.Render("  MANUAL WEB-FORM PLAYBOOKS"))
	b.WriteString("\n")
	b.WriteString(styleMuted.Render("  You complete captchas and forms. NullTrace only stores the recipe."))
	b.WriteString("\n\n")
	if len(s.Playbooks) == 0 {
		b.WriteString(styleMuted.Render("  no web-form brokers in registry"))
		return b.String()
	}
	start, end := window(cursor, len(s.Playbooks), 8)
	for i := start; i < end; i++ {
		head := firstLine(s.Playbooks[i])
		line := fmt.Sprintf("  %2d  %s", i+1, truncate(head, 64))
		if i == cursor {
			b.WriteString(styleSel.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}
	if cursor >= 0 && cursor < len(s.Playbooks) {
		b.WriteString("\n")
		b.WriteString(styleBox.Width(72).Render(s.Playbooks[cursor]))
	}
	_ = height
	return b.String()
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
