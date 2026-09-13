package exposure

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

type PasswordReport struct {
	Length      int
	EntropyBits int
	Charset     string
	Strength    string
	Risk        string
	PwnedCount  int
	PwnedOK     bool
	Common      bool
	Issues      []string
	Advice      []string
	Suggestions []string
	Summary     string
}

var (
	seqLower  = "abcdefghijklmnopqrstuvwxyz"
	seqUpper  = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	seqDigits = "01234567890"
	seqQwerty = "qwertyuiopasdfghjklzxcvbnm"
	yearRe    = regexp.MustCompile(`(?:19|20)\d{2}`)
)

// Analyze inspects a password locally. It never sends the value.
func Analyze(pw string, identityHints []string) PasswordReport {
	r := PasswordReport{Length: utf8.RuneCountInString(pw)}
	if r.Length == 0 {
		r.Strength = "empty"
		r.Risk = "critical"
		r.Issues = []string{"no password entered"}
		r.Summary = "Enter a password to check."
		return r
	}

	var lower, upper, digit, symbol bool
	for _, c := range pw {
		switch {
		case unicode.IsLower(c):
			lower = true
		case unicode.IsUpper(c):
			upper = true
		case unicode.IsDigit(c):
			digit = true
		default:
			symbol = true
		}
	}
	classes := 0
	var parts []string
	if lower {
		classes++
		parts = append(parts, "lower")
	}
	if upper {
		classes++
		parts = append(parts, "upper")
	}
	if digit {
		classes++
		parts = append(parts, "digits")
	}
	if symbol {
		classes++
		parts = append(parts, "symbols")
	}
	r.Charset = strings.Join(parts, "+")

	pool := 0
	if lower {
		pool += 26
	}
	if upper {
		pool += 26
	}
	if digit {
		pool += 10
	}
	if symbol {
		pool += 20
	}
	if pool < 10 {
		pool = 10
	}
	// bits ~= length * log2(pool). log2(26)~4.7, log2(62)~6, log2(82)~6.3
	bitsPer := 4
	switch {
	case pool >= 80:
		bitsPer = 6
	case pool >= 50:
		bitsPer = 5
	case pool >= 26:
		bitsPer = 4
	default:
		bitsPer = 3
	}
	r.EntropyBits = r.Length * bitsPer
	if hasRepeat(pw) {
		r.EntropyBits = r.EntropyBits * 2 / 3
	}

	low := strings.ToLower(strings.TrimSpace(pw))
	if _, ok := commonPasswords[low]; ok {
		r.Common = true
		r.Issues = append(r.Issues, "this is on the common-password list")
	}
	if r.Length < 8 {
		r.Issues = append(r.Issues, "shorter than 8 characters")
	} else if r.Length < 12 {
		r.Issues = append(r.Issues, "under 12 characters — short for 2026")
	}
	if classes < 2 {
		r.Issues = append(r.Issues, "only one character type")
	} else if classes < 3 && r.Length < 16 {
		r.Issues = append(r.Issues, "mix more character types (or make it much longer)")
	}
	if hasSequence(low) {
		r.Issues = append(r.Issues, "contains a keyboard or alphabet sequence")
	}
	if hasRepeat(pw) {
		r.Issues = append(r.Issues, "repeating characters")
	}
	if yearRe.MatchString(pw) {
		r.Issues = append(r.Issues, "looks like it contains a year")
	}
	for _, hint := range identityHints {
		hint = strings.ToLower(strings.TrimSpace(hint))
		if len(hint) >= 3 && strings.Contains(low, hint) {
			r.Issues = append(r.Issues, "contains your name or email")
			break
		}
	}

	switch {
	case r.Common || r.Length < 8:
		r.Strength = "very weak"
	case r.EntropyBits < 40 || r.Length < 10:
		r.Strength = "weak"
	case r.EntropyBits < 60 || r.Length < 12:
		r.Strength = "fair"
	case r.EntropyBits < 80:
		r.Strength = "strong"
	default:
		r.Strength = "excellent"
	}
	return r
}

func FinishRisk(r PasswordReport) PasswordReport {
	switch {
	case r.Common || (r.PwnedOK && r.PwnedCount > 10000):
		r.Risk = "critical"
	case r.PwnedOK && r.PwnedCount > 0:
		r.Risk = "high"
	case r.Strength == "very weak" || r.Strength == "weak":
		r.Risk = "high"
	case r.Strength == "fair":
		r.Risk = "medium"
	case r.PwnedOK && r.PwnedCount == 0 && (r.Strength == "strong" || r.Strength == "excellent"):
		r.Risk = "ok"
	default:
		r.Risk = "medium"
	}

	r.Advice = nil
	if r.PwnedOK && r.PwnedCount > 0 {
		r.Advice = append(r.Advice,
			"Stop using this password on every account that still has it.",
			"The dump index does not name each stolen database — treat it as public.",
			"Turn on 2-step verification wherever this password was reused.",
		)
		r.Summary = "This password appears in public breach compilations. Attackers try it first."
	} else if r.Common {
		r.Advice = append(r.Advice, "Pick something that is not on any “top passwords” list.")
		r.Summary = "Too common. Cracking tools try this immediately."
	} else if r.Risk == "ok" {
		r.Advice = append(r.Advice,
			"Still use a unique password per site (a password manager helps).",
			"Enable 2-step verification on email and banking.",
		)
		r.Summary = "Not found in the public dump index, and the shape looks solid."
	} else {
		r.Advice = append(r.Advice,
			"Length beats complexity: 16+ random characters, or 5+ random words.",
			"Never reuse it on email, banking, or Apple/Google/Microsoft.",
		)
		r.Summary = "Not a famous dump hit, but the password itself is easy to guess."
	}
	if r.Suggestions == nil {
		r.Suggestions = Suggest(3)
	}
	return r
}

func hasRepeat(s string) bool {
	if len(s) < 4 {
		return false
	}
	runes := []rune(s)
	same := 1
	for i := 1; i < len(runes); i++ {
		if runes[i] == runes[i-1] {
			same++
			if same >= 3 {
				return true
			}
		} else {
			same = 1
		}
	}
	return strings.Contains(strings.ToLower(s), "aaaa") || strings.Contains(s, "1111")
}

func hasSequence(low string) bool {
	checks := []string{seqLower, seqUpper, strings.ToLower(seqDigits), seqQwerty}
	for _, seq := range checks {
		for i := 0; i+3 < len(seq); i++ {
			chunk := seq[i : i+4]
			if strings.Contains(low, chunk) {
				return true
			}
			rev := reverseASCII(chunk)
			if strings.Contains(low, rev) {
				return true
			}
		}
	}
	return false
}

func reverseASCII(s string) string {
	b := []byte(s)
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	return string(b)
}

func IdentityHints(first, last, email string, extra []string) []string {
	var out []string
	add := func(s string) {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" {
			return
		}
		out = append(out, s)
		if i := strings.IndexByte(s, '@'); i > 2 {
			out = append(out, s[:i])
		}
	}
	add(first)
	add(last)
	add(first + last)
	add(email)
	for _, e := range extra {
		add(e)
	}
	return out
}
