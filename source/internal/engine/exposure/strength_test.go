package exposure

import (
	"strings"
	"testing"
)

func TestAnalyzeCommonPassword(t *testing.T) {
	r := Analyze("password", nil)
	if !r.Common {
		t.Fatal("password should be flagged common")
	}
	if r.Strength != "very weak" {
		t.Fatalf("strength %q", r.Strength)
	}
	r.PwnedOK = true
	r.PwnedCount = 9000000
	r = FinishRisk(r)
	if r.Risk != "critical" {
		t.Fatalf("risk %q", r.Risk)
	}
}

func TestAnalyzeStrongUnique(t *testing.T) {
	pw := "wX9!mQ2pL7#vN4sD0aR"
	r := Analyze(pw, []string{"jordan"})
	if r.Common {
		t.Fatal("random-looking password flagged common")
	}
	if r.Length != 19 {
		t.Fatalf("len %d", r.Length)
	}
	r.PwnedOK = true
	r.PwnedCount = 0
	r = FinishRisk(r)
	if r.Risk == "critical" || r.Risk == "high" {
		t.Fatalf("unexpected risk %q issues=%v", r.Risk, r.Issues)
	}
}

func TestAnalyzeIdentityHint(t *testing.T) {
	r := Analyze("jordan-house-2020", []string{"Jordan"})
	found := false
	for _, i := range r.Issues {
		if strings.Contains(i, "name") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected name hint, issues=%v", r.Issues)
	}
}

func TestSuggest(t *testing.T) {
	got := Suggest(3)
	if len(got) != 3 {
		t.Fatalf("len %d", len(got))
	}
	if !strings.Contains(got[0], "-") {
		t.Fatalf("first should be a passphrase: %q", got[0])
	}
	if len(got[1]) != 20 {
		t.Fatalf("second len %d", len(got[1]))
	}
}

func TestComma(t *testing.T) {
	if Comma(47312) != "47,312" {
		t.Fatalf("got %s", Comma(47312))
	}
	if Comma(12) != "12" {
		t.Fatalf("got %s", Comma(12))
	}
}
