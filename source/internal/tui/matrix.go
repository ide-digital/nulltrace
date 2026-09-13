package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/4x3/nulltrace/pkg/ipc"
)

func renderMatrix(s ipc.Snapshot, cursor, height int) string {
	var b strings.Builder
	b.WriteString(styleTitle.Render("  EXPOSURE MATRIX"))
	b.WriteString("\n")
	b.WriteString(styleMuted.Render("  broker                        risk     conf   status"))
	b.WriteString("\n")
	if len(s.Exposed) == 0 {
		b.WriteString(styleMuted.Render("  no findings yet — run nulltrace scan"))
		b.WriteString("\n")
		return b.String()
	}
	start, end := window(cursor, len(s.Exposed), height)
	for i := start; i < end; i++ {
		r := s.Exposed[i]
		name := truncate(r.BrokerName, 28)
		line := fmt.Sprintf("  %-28s %-8s %5.2f  %s", name, r.RiskTier, r.Confidence, r.Status)
		if i == cursor {
			b.WriteString(styleSel.Render(line))
		} else {
			b.WriteString(riskStyle(r.RiskTier).Render(line))
		}
		b.WriteString("\n")
	}
	b.WriteString(styleMuted.Render(fmt.Sprintf("  %d/%d  enter: playbook url", cursor+1, len(s.Exposed))))
	return b.String()
}

func renderActions(s ipc.Snapshot, cursor, height int) string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(styleTitle.Render("  ACTIONS"))
	b.WriteString("\n")
	if len(s.Actions) == 0 {
		b.WriteString(styleMuted.Render("  none queued"))
		return b.String()
	}
	start, end := window(cursor, len(s.Actions), max(4, height/3))
	for i := start; i < end; i++ {
		a := s.Actions[i]
		line := fmt.Sprintf("  %-22s %-16s %-22s %s", truncate(a.BrokerName, 22), a.Strategy, a.State, shortID(a.ID))
		if i == cursor {
			b.WriteString(styleSel.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func riskStyle(tier string) lipgloss.Style {
	switch strings.ToUpper(tier) {
	case "CRITICAL":
		return styleBad
	case "HIGH":
		return styleWarn
	case "LOW":
		return styleMuted
	default:
		return styleOK
	}
}

func window(cursor, n, height int) (int, int) {
	if height <= 0 {
		height = 12
	}
	if n == 0 {
		return 0, 0
	}
	start := cursor - height/2
	if start < 0 {
		start = 0
	}
	end := start + height
	if end > n {
		end = n
		start = max(0, end-height)
	}
	return start, end
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}

func shortID(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
