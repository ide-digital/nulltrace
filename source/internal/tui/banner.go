package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const logo = `
 ███╗   ██╗██╗   ██╗██╗     ██╗  ████████╗██████╗  █████╗  ██████╗███████╗
 ████╗  ██║██║   ██║██║     ██║  ╚══██╔══╝██╔══██╗██╔══██╗██╔════╝██╔════╝
 ██╔██╗ ██║██║   ██║██║     ██║     ██║   ██████╔╝███████║██║     █████╗
 ██║╚██╗██║██║   ██║██║     ██║     ██║   ██╔══██╗██╔══██║██║     ██╔══╝
 ██║ ╚████║╚██████╔╝███████╗███████╗██║   ██║  ██║██║  ██║╚██████╗███████╗
 ╚═╝  ╚═══╝ ╚═════╝ ╚══════╝╚══════╝╚═╝   ╚═╝  ╚═╝╚═╝  ╚═╝ ╚═════╝╚══════╝`

func Banner() string {
	return styleBanner.Render(strings.Trim(logo, "\n"))
}

func rule(width int) string {
	if width < 20 {
		width = 72
	}
	dash := width - 2
	if dash < 8 {
		dash = 8
	}
	return styleRule.Render("  " + strings.Repeat("━", dash))
}

// fitTerm clamps a console size. Windows reports the *screen buffer*
// (often thousands of rows) as a resize event; using that height pads
// the view with blank lines and each keystroke leaves a leftover row.
func fitTerm(w, h int) (int, int) {
	if w < 80 {
		w = 80
	}
	if h < 24 {
		h = 24
	}
	if w > 160 {
		w = 120
	}
	if h > 50 {
		h = 40
	}
	return w, h
}

// drop pushes body down a few rows so the banner is not clipped by the
// window chrome, without painting a full-height frame (that scrolls the
// ASCII off the top of a Windows console).
func drop(body string, h int) string {
	bh := lipgloss.Height(body)
	pad := (h - bh) / 2
	if pad < 2 {
		pad = 2
	}
	if pad > 5 {
		pad = 5
	}
	return strings.Repeat("\n", pad) + body
}

func center(s string, width int) string {
	if width <= 0 {
		return s
	}
	// Never use Style.Width — it reflows newlines and shreds bordered boxes.
	var b strings.Builder
	for i, line := range strings.Split(s, "\n") {
		if i > 0 {
			b.WriteByte('\n')
		}
		lw := lipgloss.Width(line)
		if lw > 0 && lw < width {
			b.WriteString(strings.Repeat(" ", (width-lw)/2))
		}
		b.WriteString(line)
	}
	return b.String()
}
