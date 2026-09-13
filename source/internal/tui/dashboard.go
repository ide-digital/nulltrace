package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/4x3/nulltrace/pkg/ipc"
)

func renderDashboard(s ipc.Snapshot, width int) string {
	st := s.Status
	row := func(k, v string, sty lipgloss.Style) string {
		return fmt.Sprintf("  %-22s %s", k, sty.Render(v))
	}
	id := st.IdentityName
	if id == "" {
		id = "(none)"
	}
	unlockStyle := styleBad
	if st.Unlocked {
		unlockStyle = styleOK
	}
	failStyle := styleOK
	if st.FailedActions > 0 {
		failStyle = styleBad
	}
	overStyle := styleOK
	if st.Overdue > 0 {
		overStyle = styleBad
	}
	lines := []string{
		styleTitle.Render("  overview"),
		"",
		row("identity", id, styleName),
		row("vault", unlocked(st.Unlocked), unlockStyle),
		row("brokers seeded", fmt.Sprintf("%d", st.BrokerCount), styleMuted),
		row("exposed records", fmt.Sprintf("%d", st.ExposedRecords), styleWarn),
		row("active removals", fmt.Sprintf("%d", st.ActiveRemovals), styleWarn),
		row("awaiting confirm", fmt.Sprintf("%d", st.AwaitingConfirm), styleWarn),
		row("manual playbooks", fmt.Sprintf("%d", st.ManualPending), styleWarn),
		row("verified removed", fmt.Sprintf("%d", st.VerifiedRemoved), styleOK),
		row("failed", fmt.Sprintf("%d", st.FailedActions), failStyle),
		row("overdue", fmt.Sprintf("%d", st.Overdue), overStyle),
		row("ipc", st.ListenAddr, styleMuted),
		"",
		styleMuted.Render("  scan → broker map    scrub → queue mail    r refresh"),
	}
	_ = width
	return strings.Join(lines, "\n")
}

func unlocked(v bool) string {
	if v {
		return "unlocked"
	}
	return "LOCKED"
}
