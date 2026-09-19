package tui

import "github.com/charmbracelet/lipgloss"

const Version = "0.3.1"

func Key(k string) string   { return styleNum.Render("[" + k + "]") }
func Item(s string) string  { return styleItem.Render(s) }
func Name(s string) string  { return styleName.Render(s) }
func Muted(s string) string { return styleMuted.Render(s) }
func Label(s string) string { return styleLabel.Render(s) }
func OK(s string) string    { return styleOK.Render(s) }
func Bad(s string) string   { return styleBad.Render(s) }
func Warn(s string) string  { return styleWarn.Render(s) }
func Title(s string) string { return styleTitle.Render(s) }
func Prompt() string        { return stylePrompt.Render(">") }
func RuleLine(w int) string { return rule(w) }

// NavLine is the standard nested-screen footer: one step back, or jump home.
func NavLine() string {
	return Key("0") + "  back      " + Key("m") + "  main menu"
}

var (
	colorBanner = lipgloss.Color("250")
	colorMuted  = lipgloss.Color("240")
	colorLabel  = lipgloss.Color("243")
	colorItem   = lipgloss.Color("249")
	colorName   = lipgloss.Color("255")
	colorTitle  = lipgloss.Color("247")
	colorNum    = lipgloss.Color("246")
	colorRule   = lipgloss.Color("237")
	colorWarn   = lipgloss.Color("247")
	colorBad    = lipgloss.Color("174")
	colorOK     = lipgloss.Color("108")
	colorSel    = lipgloss.Color("238")
)

var (
	styleBanner = lipgloss.NewStyle().Foreground(colorBanner).Bold(true)
	styleMuted  = lipgloss.NewStyle().Foreground(colorMuted)
	styleLabel  = lipgloss.NewStyle().Foreground(colorLabel)
	styleTitle  = lipgloss.NewStyle().Foreground(colorTitle)
	styleOK     = lipgloss.NewStyle().Foreground(colorOK)
	styleWarn   = lipgloss.NewStyle().Foreground(colorWarn)
	styleBad    = lipgloss.NewStyle().Foreground(colorBad)
	styleSel    = lipgloss.NewStyle().Foreground(colorName).Background(colorSel)
	styleTab    = lipgloss.NewStyle().Padding(0, 2)
	styleTabOn  = lipgloss.NewStyle().Padding(0, 2).Foreground(colorName).Background(colorSel)
	styleHelp   = lipgloss.NewStyle().Foreground(colorMuted).MarginTop(1)
	styleBox    = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(colorRule).Padding(0, 1)
	styleNum    = lipgloss.NewStyle().Foreground(colorNum)
	styleItem   = lipgloss.NewStyle().Foreground(colorItem)
	styleName   = lipgloss.NewStyle().Foreground(colorName).Bold(true)
	styleRule   = lipgloss.NewStyle().Foreground(colorRule)
	stylePrompt = lipgloss.NewStyle().Foreground(colorItem)
)
