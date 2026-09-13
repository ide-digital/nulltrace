package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestRenderHomeFitsTwoColumns(t *testing.T) {
	s := RenderHome(HomeInfo{
		Identity: "Ada Lovelace",
		Vault:    "unlocked",
		Mail:     "linked  ada@example.com",
		Listings: 12,
		Queued:   3,
		Verified: 1,
	})
	if !strings.HasPrefix(s, "\n\n") {
		t.Fatal("home should leave a little space above the banner")
	}
	if strings.HasPrefix(s, "\n\n\n\n") {
		t.Fatal("too much space above the banner")
	}
	for _, need := range []string{
		"Ada Lovelace", "Scan", "Listings", "Email", "Leaks", "Exit",
		"Work", "Account", "Tools", "privacy toolkit", "Yahoo",
	} {
		if !strings.Contains(s, need) {
			t.Fatalf("home missing %q", need)
		}
	}
}

func TestHomeMenuRowsAreSymmetric(t *testing.T) {
	s := RenderHome(HomeInfo{
		Identity: "Ada Lovelace",
		Vault:    "unlocked",
		Mail:     "not linked",
		Listings: 12,
		Queued:   3,
		Verified: 1,
		Manual:   2,
	})
	const want = 2 + 48 + 4 + 48
	lines := strings.Split(s, "\n")
	idx := -1
	for i, line := range lines {
		if strings.Contains(line, "Work") && strings.Contains(line, "Account") {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatal("Work / Account header missing")
	}
	for j, label := range []string{"header", "rules", "row1", "row2", "row3", "row4", "row5"} {
		line := lines[idx+j]
		if got := lipgloss.Width(line); got != want {
			t.Fatalf("%s width %d want %d: %q", label, got, want, line)
		}
	}
}

func TestSpreadEvenGaps(t *testing.T) {
	s := Spread([]string{"a", "b", "c"}, 9)
	if s != "a   b   c" {
		t.Fatalf("%q", s)
	}
}

func TestPadRightVisibleWidth(t *testing.T) {
	s := PadRight(Label("vault"), 10)
	if got := lipgloss.Width(s); got != 10 {
		t.Fatalf("width %d", got)
	}
}
