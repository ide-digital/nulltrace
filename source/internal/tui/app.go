package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/4x3/nulltrace/pkg/ipc"
)

type tab int

const (
	tabDash tab = iota
	tabDossier
	tabMatrix
	tabStream
	tabPlay
	tabVault
	tabCount
)

var tabNames = []string{"dashboard", "dossier", "matrix", "stream", "playbooks", "vault"}

type snapshotFunc func(context.Context) (ipc.Snapshot, error)

type model struct {
	ctx    context.Context
	load   snapshotFunc
	snap   ipc.Snapshot
	err    error
	tab    tab
	cursor int
	width  int
	height int
	detail string
	vault  vaultForm
}

type snapMsg struct {
	snap ipc.Snapshot
	err  error
}

func Run(ctx context.Context, load snapshotFunc) error {
	defer RestoreCookedConsole()
	m := model{
		ctx:    ctx,
		load:   load,
		vault:  newVaultForm(),
		width:  100,
		height: 32,
	}
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithContext(ctx))
	_, err := p.Run()
	return err
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.refresh(), tick())
}

func tick() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return tickMsg{} })
}

type tickMsg struct{}

func (m model) refresh() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.ctx, 8*time.Second)
		defer cancel()
		s, err := m.load(ctx)
		return snapMsg{snap: s, err: err}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = fitTerm(msg.Width, msg.Height)
		return m, nil
	case snapMsg:
		m.snap = msg.snap
		m.err = msg.err
		m.clamp()
		return m, nil
	case tickMsg:
		return m, tea.Batch(m.refresh(), tick())
	case tea.KeyMsg:
		if m.tab == tabVault {
			var cmd tea.Cmd
			m.vault, cmd = m.vault.Update(msg)
			if msg.String() == "tab" || msg.String() == "esc" {
				m.tab = tabDash
				return m, cmd
			}
			if msg.String() == "ctrl+c" || msg.String() == "q" {
				return m, tea.Quit
			}
			return m, cmd
		}
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			if m.detail != "" {
				m.detail = ""
				return m, nil
			}
			return m, tea.Quit
		case "tab":
			m.tab = (m.tab + 1) % tabCount
			m.cursor = 0
			m.detail = ""
		case "shift+tab":
			m.tab = (m.tab - 1 + tabCount) % tabCount
			m.cursor = 0
			m.detail = ""
		case "1":
			m.tab, m.cursor = tabDash, 0
		case "2":
			m.tab, m.cursor = tabDossier, 0
		case "3":
			m.tab, m.cursor = tabMatrix, 0
		case "4":
			m.tab, m.cursor = tabStream, 0
		case "5":
			m.tab, m.cursor = tabPlay, 0
		case "6":
			m.tab, m.cursor = tabVault, 0
		case "j", "down":
			m.cursor++
			m.clamp()
		case "k", "up":
			m.cursor--
			m.clamp()
		case "g":
			m.cursor = 0
		case "G":
			m.cursor = m.maxCursor()
		case "enter":
			m.openDetail()
		case "r":
			return m, m.refresh()
		}
	}
	return m, nil
}

func (m *model) maxCursor() int {
	n := 0
	switch m.tab {
	case tabMatrix:
		n = len(m.snap.Exposed) - 1
	case tabStream:
		n = len(m.snap.Audit) - 1
	case tabPlay:
		n = len(m.snap.Playbooks) - 1
	}
	if n < 0 {
		return 0
	}
	return n
}

func (m *model) clamp() {
	if m.cursor < 0 {
		m.cursor = 0
	}
	if mx := m.maxCursor(); m.cursor > mx {
		m.cursor = mx
	}
}

func (m *model) openDetail() {
	switch m.tab {
	case tabMatrix:
		if m.cursor >= 0 && m.cursor < len(m.snap.Exposed) {
			r := m.snap.Exposed[m.cursor]
			m.detail = fmt.Sprintf("%s\n%s\nstatus=%s risk=%s conf=%.2f\n%s", r.BrokerName, r.BrokerID, r.Status, r.RiskTier, r.Confidence, r.ProfileURL)
		}
	case tabPlay:
		if m.cursor >= 0 && m.cursor < len(m.snap.Playbooks) {
			m.detail = m.snap.Playbooks[m.cursor]
		}
	case tabStream:
		if m.cursor >= 0 && m.cursor < len(m.snap.Audit) {
			a := m.snap.Audit[m.cursor]
			m.detail = a.Timestamp + "\n" + a.EventType + "\n" + a.BrokerID + "\n" + a.Details
		}
	}
}

func (m model) View() string {
	var b strings.Builder
	b.WriteString(Banner())
	b.WriteString("\n")
	b.WriteString(m.tabs())
	b.WriteString("\n")
	if m.err != nil {
		b.WriteString(styleBad.Render("  " + m.err.Error()))
		b.WriteString("\n")
	}
	bodyH := m.height - 16
	if bodyH < 8 {
		bodyH = 8
	}
	switch m.tab {
	case tabDash:
		b.WriteString(renderDashboard(m.snap, m.width))
	case tabDossier:
		b.WriteString(renderDossier(m.snap))
	case tabMatrix:
		b.WriteString(renderMatrix(m.snap, m.cursor, bodyH-8))
		b.WriteString(renderActions(m.snap, 0, bodyH))
	case tabStream:
		b.WriteString(renderStreamer(m.snap, m.cursor, bodyH))
	case tabPlay:
		b.WriteString(renderPlaybooks(m.snap, m.cursor, bodyH))
	case tabVault:
		b.WriteString(m.vault.View())
	}
	if m.detail != "" {
		b.WriteString("\n")
		b.WriteString(styleBox.Width(min(m.width-4, 80)).Render(m.detail))
	}
	b.WriteString("\n")
	b.WriteString(styleHelp.Render("  j/k g/G  tab  1-6  enter  r refresh  esc/q quit"))
	return b.String()
}

func (m model) tabs() string {
	var parts []string
	for i, name := range tabNames {
		if tab(i) == m.tab {
			parts = append(parts, styleTabOn.Render(name))
		} else {
			parts = append(parts, styleTab.Render(name))
		}
	}
	return strings.Join(parts, "")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
