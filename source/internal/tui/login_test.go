package tui

import (
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestMachineName(t *testing.T) {
	name := MachineName()
	if name == "" {
		t.Fatal("machine name is empty")
	}
	if os.Getenv("COMPUTERNAME") != "" && name != os.Getenv("COMPUTERNAME") {
		t.Fatalf("got %q want COMPUTERNAME", name)
	}
}

func TestLoginFirstRunAsksToSetPassword(t *testing.T) {
	m := newLoginModel(true)
	v := m.View()
	if !strings.Contains(v, "set a password") {
		t.Fatalf("first-run login missing set-password copy:\n%s", v)
	}
	if !strings.Contains(v, "locked") {
		t.Fatal("username should be marked locked")
	}
	if !strings.Contains(v, m.user) {
		t.Fatalf("username %q not shown", m.user)
	}
}

func TestLoginSignInDoesNotAskToSetPassword(t *testing.T) {
	m := newLoginModel(false)
	v := m.View()
	if strings.Contains(v, "set a password") {
		t.Fatal("returning login should not say set a password")
	}
	if !strings.Contains(v, "sign in") {
		t.Fatalf("returning login missing sign-in title:\n%s", v)
	}
	if n := strings.Count(strings.ToLower(v), "password"); n > 2 {
		t.Fatalf("password label/placeholder repeated %d times:\n%s", n, v)
	}
	if strings.Contains(v, "••••") || strings.Contains(v, "********") {
		t.Fatalf("empty password field should not show placeholder bullets:\n%s", v)
	}
}

func TestLoginTypingStaysOneField(t *testing.T) {
	var model tea.Model = newLoginModel(false)
	model, _ = model.Update(tea.WindowSizeMsg{Width: 100, Height: 36})
	for _, r := range "secret" {
		var cmd tea.Cmd
		model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		_ = cmd
	}
	lm := model.(loginModel)
	if len(lm.pass) != 6 {
		t.Fatalf("got %d runes want 6", len(lm.pass))
	}
	v := lm.View()
	if n := strings.Count(strings.ToLower(v), "password"); n != 1 {
		t.Fatalf("password label count %d, want 1:\n%s", n, v)
	}
	if got := strings.Count(v, "•"); got != 6 {
		t.Fatalf("bullet count %d, want 6:\n%s", got, v)
	}
	if strings.Contains(v, "*") {
		t.Fatalf("password should use • not *:\n%s", v)
	}
}

func TestLoginViewDoesNotGrowWhenTyping(t *testing.T) {
	m := newLoginModel(true)
	m.width, m.height = 100, 36
	before := strings.Count(m.View(), "\n")
	for _, r := range "supersecret" {
		m = m.edit([]rune{r}, false)
	}
	after := m.View()
	if strings.Count(after, "\n") != before {
		t.Fatalf("view grew from %d to %d lines", before, strings.Count(after, "\n"))
	}
	if n := strings.Count(strings.ToLower(after), "set a password"); n != 1 {
		t.Fatalf("set a password count %d:\n%s", n, after)
	}
}

func TestLoginIgnoresWindowsBufferHeight(t *testing.T) {
	var model tea.Model = newLoginModel(false)
	model, _ = model.Update(tea.WindowSizeMsg{Width: 120, Height: 3000})
	lm := model.(loginModel)
	if lm.height > 50 {
		t.Fatalf("stored height %d, Windows buffer size was not clamped", lm.height)
	}
	v := lm.View()
	if n := strings.Count(v, "\n"); n > 80 {
		t.Fatalf("view has %d newlines; a tall buffer would stack password rows", n)
	}
	for _, r := range "abcd" {
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	v = model.(loginModel).View()
	if n := strings.Count(strings.ToLower(v), "password"); n != 1 {
		t.Fatalf("password label count %d after typing:\n%s", n, v)
	}
}

func TestFitTermClampsBufferHeight(t *testing.T) {
	_, h := fitTerm(120, 3000)
	if h > 50 {
		t.Fatalf("fitTerm height %d", h)
	}
}

func TestCenterDoesNotReflowBox(t *testing.T) {
	box := "┌──────┐\n│ hi   │\n└──────┘"
	out := center(box, 40)
	if strings.Count(out, "\n") != strings.Count(box, "\n") {
		t.Fatalf("center changed line count:\n%s", out)
	}
	if !strings.Contains(out, "┌──────┐") || !strings.Contains(out, "└──────┘") {
		t.Fatalf("center broke box drawing:\n%s", out)
	}
}
