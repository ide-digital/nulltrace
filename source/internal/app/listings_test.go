package app

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/4x3/nulltrace/pkg/ipc"
)

func TestRenderListingsExport(t *testing.T) {
	rows := []ipc.ExposedView{
		{
			BrokerName:   "Spokeo",
			Domain:       "spokeo.com",
			Category:     "PEOPLE_SEARCH",
			Mechanism:    "WEB_FORM",
			RiskTier:     "HIGH",
			Status:       "DISCOVERED",
			ProfileURL:   "https://www.spokeo.com/search",
			OptOutURL:    "https://www.spokeo.com/optout",
			ContactEmail: "privacy@spokeo.com",
			Detected:     "2026-09-12T00:00:00Z",
		},
	}
	md := renderListingsExport("md", "Jane Doe", rows)
	for _, need := range []string{"# NullTrace listings", "Jane Doe", "Spokeo", "spokeo.com", "privacy@spokeo.com"} {
		if !strings.Contains(md, need) {
			t.Fatalf("md missing %q", need)
		}
	}
	txt := renderListingsExport("txt", "Jane Doe", rows)
	for _, need := range []string{"NULLTRACE LISTINGS", "Jane Doe", "Spokeo", "spokeo.com"} {
		if !strings.Contains(txt, need) {
			t.Fatalf("txt missing %q", need)
		}
	}
}

func TestFilterListings(t *testing.T) {
	rows := []ipc.ExposedView{
		{BrokerName: "Spokeo", Domain: "spokeo.com", Category: "PEOPLE_SEARCH"},
		{BrokerName: "Whitepages", Domain: "whitepages.com", Category: "PEOPLE_SEARCH"},
	}
	got := filterListings(rows, "spoke")
	if len(got) != 1 || got[0].BrokerName != "Spokeo" {
		t.Fatalf("got %#v", got)
	}
	if n := len(filterListings(rows, "")); n != 2 {
		t.Fatalf("empty filter should keep all, got %d", n)
	}
}

func TestListingsTableRowsAlign(t *testing.T) {
	rows := make([]ipc.ExposedView, 11)
	for i := range rows {
		rows[i] = ipc.ExposedView{
			BrokerName: "Whitepages",
			Domain:     "whitepages.com",
			RiskTier:   "HIGH",
			Status:     "DISCOVERED",
		}
	}
	rows[0].RiskTier = "CRITICAL"
	s := renderListingsTable(rows)
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	if len(lines) != 12 {
		t.Fatalf("header + 11 rows, got %d", len(lines))
	}
	want := lipgloss.Width(lines[0])
	for i, line := range lines {
		if got := lipgloss.Width(line); got != want {
			t.Fatalf("line %d width %d want %d", i, got, want)
		}
	}
}
