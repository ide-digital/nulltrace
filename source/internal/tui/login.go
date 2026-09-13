package tui

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// MachineName is the local computer name. Used as the locked login username.
func MachineName() string {
	for _, k := range []string{"COMPUTERNAME", "HOSTNAME"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	h, err := os.Hostname()
	if err != nil || strings.TrimSpace(h) == "" {
		return "this-pc"
	}
	return h
}

type LoginResult struct {
	Password []byte
	Canceled bool
}

type secretBuf []rune

type loginModel struct {
	first   bool
	user    string
	pass    secretBuf
	confirm secretBuf
	focus   int
	err     string
	width   int
	height  int
	quit    bool
	ok      bool
}

func RunLogin(ctx context.Context, first bool, flash string) (LoginResult, error) {
	defer RestoreCookedConsole()
	m := newLoginModel(first)
	m.err = flash
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithContext(ctx))
	final, err := p.Run()
	if err != nil {
		return LoginResult{}, err
	}
	lm, ok := final.(loginModel)
	if !ok {
		return LoginResult{}, fmt.Errorf("login: unexpected model")
	}
	if lm.quit && !lm.ok {
		return LoginResult{Canceled: true}, nil
	}
	return LoginResult{Password: []byte(string(lm.pass))}, nil
}

func newLoginModel(first bool) loginModel {
	return loginModel{
		first:  first,
		user:   MachineName(),
		width:  100,
		height: 36,
	}
}

// RenderLoginDemo is the sign-in screen with a stand-in machine name.
// Used for README screenshots so a real hostname never appears in docs.
func RenderLoginDemo(username string, nTyped int) string {
	if nTyped < 0 {
		nTyped = 0
	}
	m := loginModel{
		user:   username,
		width:  102,
		height: 28,
		pass:   make(secretBuf, nTyped),
	}
	for i := range m.pass {
		m.pass[i] = 'x'
	}
	return m.View()
}

func (m loginModel) Init() tea.Cmd { return tea.HideCursor }

func (m loginModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	redraw := func(next loginModel) (tea.Model, tea.Cmd) {
		return next, tea.ClearScreen
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = fitTerm(msg.Width, msg.Height)
		return redraw(m)
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quit = true
			return m, tea.Quit
		case tea.KeyEnter:
			return m.submit()
		case tea.KeyTab, tea.KeyShiftTab, tea.KeyDown, tea.KeyUp:
			if m.first {
				if m.focus == 0 {
					m.focus = 1
				} else {
					m.focus = 0
				}
			}
			return redraw(m)
		case tea.KeyBackspace, tea.KeyCtrlH, tea.KeyDelete:
			return redraw(m.edit(nil, true))
		case tea.KeyCtrlU:
			if m.focus == 1 {
				m.confirm = nil
			} else {
				m.pass = nil
			}
			return redraw(m)
		case tea.KeySpace:
			return redraw(m.edit([]rune{' '}, false))
		case tea.KeyRunes:
			return redraw(m.edit(msg.Runes, false))
		}
	}
	return m, nil
}

func (m loginModel) edit(add []rune, del bool) loginModel {
	buf := m.pass
	if m.first && m.focus == 1 {
		buf = m.confirm
	}
	if del {
		if len(buf) > 0 {
			buf = buf[:len(buf)-1]
		}
	} else {
		for _, r := range add {
			if r < 32 || r == 127 {
				continue
			}
			if len(buf) >= 256 {
				break
			}
			buf = append(buf, r)
		}
	}
	if m.first && m.focus == 1 {
		m.confirm = buf
	} else {
		m.pass = buf
	}
	return m
}

func (m loginModel) submit() (tea.Model, tea.Cmd) {
	if len(strings.TrimSpace(string(m.pass))) == 0 {
		m.err = "password cannot be empty"
		return m, nil
	}
	if m.first {
		if m.focus == 0 && len(m.confirm) == 0 {
			m.focus = 1
			return m, nil
		}
		if string(m.confirm) != string(m.pass) {
			m.err = "passwords did not match"
			m.confirm = nil
			m.focus = 1
			return m, nil
		}
	}
	m.ok = true
	m.quit = true
	m.err = ""
	return m, tea.Quit
}

func (m loginModel) View() string {
	w, h := fitTerm(m.width, m.height)
	if w < 72 {
		w = 72
	}
	title := "sign in"
	hint := "enter to continue"
	passLabel := "password"
	if m.first {
		title = "create login"
		hint = "this password encrypts your vault on this PC"
		passLabel = "set a password"
	}

	userVal := lipgloss.NewStyle().
		Foreground(colorName).
		Background(lipgloss.Color("236")).
		Padding(0, 1).
		Render(m.user)
	lab := func(s string) string { return Label(fmt.Sprintf("%-14s", s)) }
	userLine := fmt.Sprintf("%s  %s  %s",
		lab("username"),
		userVal,
		Muted("locked"))

	field := func(label string, buf secretBuf, focused bool) string {
		// One line, fixed width. Empty field is a cursor only — placeholder
		// bullets made the caret look like it started at the end of a password.
		n := len(buf)
		if n > 48 {
			n = 48
		}
		body := strings.Repeat("•", n)
		if focused {
			body += "█"
		}
		line := lab(label) + "  " + body
		for lipgloss.Width(line) < lipgloss.Width(userLine) {
			line += " "
		}
		return line
	}

	rows := []string{userLine, "", field(passLabel, m.pass, m.focus == 0)}
	if m.first {
		rows = append(rows, "", field("confirm", m.confirm, m.focus == 1))
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorRule).
		Padding(1, 3).
		Render(lipgloss.JoinVertical(lipgloss.Left, rows...))

	errLine := ""
	if m.err != "" {
		errLine = Bad(m.err)
	}

	body := strings.Join([]string{
		center(Banner(), w),
		"",
		center(Title(title), w),
		"",
		center(box, w),
		"",
		center(errLine, w),
		center(Muted(hint), w),
	}, "\n")
	pad := (h-lipgloss.Height(body)-2)/2 + 1
	if pad < 1 {
		pad = 1
	}
	return strings.Repeat("\n", pad) + body
}
