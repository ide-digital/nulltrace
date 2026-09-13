package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/4x3/nulltrace/pkg/ipc"
)

type MenuHooks struct {
	Snapshot  func(context.Context) (ipc.Snapshot, error)
	Scan      func(context.Context) (string, error)
	Scrub     func(context.Context) (string, error)
	Dashboard func(context.Context) error
	Identity  func(context.Context) (string, error)
	Export    func(context.Context) (string, error)
	Config    func(context.Context) (string, error)
}

type menuItem struct {
	key   string
	label string
	hint  string
	group string
}

var menuItems = []menuItem{
	{"1", "Scan", "map your data onto brokers", "Actions"},
	{"2", "Scrub", "queue deletion emails + playbooks", "Actions"},
	{"3", "Dashboard", "status, dossier, logs", "Actions"},
	{"4", "Identity", "who you're scrubbing", "Actions"},
	{"5", "Export", "write the audit ledger", "Tools"},
	{"6", "Config", "paths and listen address", "Tools"},
	{"0", "Exit", "", "Tools"},
}

type menuModel struct {
	ctx    context.Context
	hooks  MenuHooks
	snap   ipc.Snapshot
	width  int
	height int
	input  string
	flash  string
	err    error
	busy   bool
	next   string
}

type menuSnapMsg struct {
	snap ipc.Snapshot
	err  error
}

type menuDoneMsg struct {
	text string
	err  error
}

func RunMenu(ctx context.Context, hooks MenuHooks) (string, error) {
	defer RestoreCookedConsole()
	m := menuModel{ctx: ctx, hooks: hooks, width: 90, height: 36}
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithContext(ctx))
	final, err := p.Run()
	if err != nil {
		return "", err
	}
	if mm, ok := final.(menuModel); ok {
		return mm.next, nil
	}
	return "", nil
}

func (m menuModel) Init() tea.Cmd {
	return m.loadSnap()
}

func (m menuModel) loadSnap() tea.Cmd {
	if m.hooks.Snapshot == nil {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.ctx, 8*time.Second)
		defer cancel()
		s, err := m.hooks.Snapshot(ctx)
		return menuSnapMsg{snap: s, err: err}
	}
}

func (m menuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = fitTerm(msg.Width, msg.Height)
		return m, nil
	case menuSnapMsg:
		m.snap = msg.snap
		if msg.err != nil {
			m.err = msg.err
		}
		return m, nil
	case menuDoneMsg:
		m.busy = false
		if msg.err != nil {
			m.err = msg.err
			m.flash = ""
		} else {
			m.err = nil
			m.flash = msg.text
		}
		return m, m.loadSnap()
	case tea.KeyMsg:
		if m.busy {
			return m, nil
		}
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEsc:
			if m.input != "" {
				m.input = ""
				return m, nil
			}
			return m, tea.Quit
		case tea.KeyBackspace:
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}
			return m, nil
		case tea.KeyEnter:
			choice := strings.TrimSpace(m.input)
			m.input = ""
			if choice == "" {
				return m, nil
			}
			if choice == "0" {
				return m, tea.Quit
			}
			if choice == "3" {
				m.next = "dashboard"
				return m, tea.Quit
			}
			m.busy = true
			m.err = nil
			m.flash = "working…"
			return m, m.dispatch(choice)
		}
		s := msg.String()
		if s == "q" && m.input == "" {
			return m, tea.Quit
		}
		if len(s) == 1 && s[0] >= '0' && s[0] <= '9' {
			m.input = s
			return m, nil
		}
	}
	return m, nil
}

func (m menuModel) dispatch(choice string) tea.Cmd {
	var fn func(context.Context) (string, error)
	switch choice {
	case "1":
		fn = m.hooks.Scan
	case "2":
		fn = m.hooks.Scrub
	case "4":
		fn = m.hooks.Identity
	case "5":
		fn = m.hooks.Export
	case "6":
		fn = m.hooks.Config
	default:
		return func() tea.Msg { return menuDoneMsg{err: fmt.Errorf("unknown option %q", choice)} }
	}
	if fn == nil {
		return func() tea.Msg { return menuDoneMsg{err: fmt.Errorf("option %s is not wired", choice)} }
	}
	return func() tea.Msg {
		text, err := fn(m.ctx)
		return menuDoneMsg{text: text, err: err}
	}
}

func (m menuModel) View() string {
	w := m.width
	if w < 72 {
		w = 72
	}
	var b strings.Builder
	b.WriteByte('\n')
	b.WriteString(rule(w))
	b.WriteString("\n")
	b.WriteString(center(Banner(), w))
	b.WriteString("\n")
	meta := styleMuted.Render("v"+Version) + "   " + styleItem.Render("all-in-one privacy toolkit")
	b.WriteString(center(meta, w))
	b.WriteString("\n")
	b.WriteString(rule(w))
	b.WriteString("\n\n")

	st := m.snap.Status
	name := st.IdentityName
	if name == "" {
		name = "(no identity)"
	}
	vault := styleBad.Render("locked")
	if st.Unlocked {
		vault = styleOK.Render("unlocked")
	}
	status := fmt.Sprintf("  vault  %s    identity  %s    brokers  %d    queued  %d    verified  %d",
		vault, styleItem.Render(name), st.BrokerCount, st.ActiveRemovals, st.VerifiedRemoved)
	b.WriteString(status)
	b.WriteString("\n\n")

	group := ""
	for _, it := range menuItems {
		if it.group != group {
			group = it.group
			b.WriteString(styleRule.Render("  " + group + " "))
			b.WriteString(styleRule.Render(strings.Repeat("━", 48)))
			b.WriteString("\n")
		}
		line := "  " + styleNum.Render("["+it.key+"]") + "  " + styleItem.Render(fmt.Sprintf("%-12s", it.label))
		if it.hint != "" {
			line += "  " + styleMuted.Render(it.hint)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	if m.busy {
		b.WriteString("  " + styleWarn.Render(m.flash))
		b.WriteString("\n")
	} else if m.err != nil {
		b.WriteString("  " + styleBad.Render(m.err.Error()))
		b.WriteString("\n")
	} else if m.flash != "" {
		b.WriteString("  " + styleOK.Render(m.flash))
		b.WriteString("\n")
	}
	b.WriteString("\n  " + stylePrompt.Render(">"))
	if m.input != "" {
		b.WriteString(" " + stylePrompt.Render("("+m.input+")"))
	} else {
		b.WriteString(" ")
	}
	b.WriteString("\n")
	return b.String()
}
