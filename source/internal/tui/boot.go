package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type bootResultMsg struct {
	inited bool
	err    error
}

type bootModel struct {
	work   func() (bool, error)
	pct    int
	width  int
	height int
	inited bool
	err    error
	ready  bool
	start  time.Time
}

func RunBoot(ctx context.Context, work func() (bool, error)) (bool, error) {
	defer RestoreCookedConsole()
	m := bootModel{work: work, width: 100, height: 36, start: time.Now()}
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithContext(ctx))
	final, err := p.Run()
	if err != nil {
		return false, err
	}
	b, ok := final.(bootModel)
	if !ok {
		return false, fmt.Errorf("boot: unexpected model")
	}
	return b.inited, b.err
}

func (m bootModel) Init() tea.Cmd {
	return tea.Batch(
		tea.Tick(40*time.Millisecond, func(time.Time) tea.Msg { return bootTick{} }),
		func() tea.Msg {
			ok, err := m.work()
			return bootResultMsg{inited: ok, err: err}
		},
	)
}

type bootTick struct{}

func (m bootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = fitTerm(msg.Width, msg.Height)
		return m, nil
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			m.err = context.Canceled
			return m, tea.Quit
		}
	case bootResultMsg:
		m.inited = msg.inited
		m.err = msg.err
		m.ready = true
		if m.pct < 92 {
			m.pct = 92
		}
		return m, tea.Tick(40*time.Millisecond, func(time.Time) tea.Msg { return bootTick{} })
	case bootTick:
		if m.pct < 90 && !m.ready {
			m.pct += 3
			if m.pct > 90 {
				m.pct = 90
			}
		}
		if m.ready {
			m.pct += 4
			if m.pct >= 100 && time.Since(m.start) >= 900*time.Millisecond {
				m.pct = 100
				return m, tea.Quit
			}
			if m.pct > 99 {
				m.pct = 99
			}
		}
		return m, tea.Tick(40*time.Millisecond, func(time.Time) tea.Msg { return bootTick{} })
	}
	return m, nil
}

func (m bootModel) View() string {
	w, h := fitTerm(m.width, m.height)
	if w < 60 {
		w = 60
	}
	barW := 28
	filled := barW * m.pct / 100
	if filled > barW {
		filled = barW
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barW-filled)
	body := strings.Join([]string{
		center(Banner(), w),
		"",
		center(styleItem.Render(bar), w),
		center(Muted(fmt.Sprintf("loading  %d%%", m.pct)), w),
	}, "\n")
	return drop(body, h)
}
