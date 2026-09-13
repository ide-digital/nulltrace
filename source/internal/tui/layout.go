package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// PadRight pads or clips s to a visible cell width, ignoring ANSI.
func PadRight(s string, width int) string {
	if width <= 0 {
		return ""
	}
	n := lipgloss.Width(s)
	if n == width {
		return s
	}
	if n > width {
		return Clip(s, width)
	}
	return s + strings.Repeat(" ", width-n)
}

// Clip shortens s to a visible cell width, adding an ellipsis when needed.
func Clip(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	if width == 1 {
		return "…"
	}
	keep := width - 1
	var b strings.Builder
	w := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if w+rw > keep {
			break
		}
		b.WriteRune(r)
		w += rw
	}
	b.WriteRune('…')
	for lipgloss.Width(b.String()) < width {
		b.WriteByte(' ')
	}
	return b.String()
}

// Spread distributes parts across width with even gaps.
func Spread(parts []string, width int) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return PadRight(parts[0], width)
	}
	vis := 0
	for _, p := range parts {
		vis += lipgloss.Width(p)
	}
	gaps := len(parts) - 1
	extra := width - vis
	if extra < gaps {
		return strings.Join(parts, " ")
	}
	base := extra / gaps
	rem := extra % gaps
	var b strings.Builder
	for i, p := range parts {
		b.WriteString(p)
		if i == gaps {
			break
		}
		n := base
		if i < rem {
			n++
		}
		b.WriteString(strings.Repeat(" ", n))
	}
	return b.String()
}
