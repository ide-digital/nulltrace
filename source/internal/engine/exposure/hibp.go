package exposure

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/4x3/nulltrace/internal/engine/recon"
)

// CheckPassword scores the password locally, then asks the public Pwned
// Passwords range API whether that SHA-1 prefix appears in known dumps.
// The full password never leaves the machine. No API key is used.
func CheckPassword(ctx context.Context, httpc *http.Client, pw string, hints []string) PasswordReport {
	r := Analyze(pw, hints)
	count, err := recon.PwnedPasswordPrefix(ctx, httpc, []byte(pw))
	if err != nil {
		r.PwnedOK = false
		r.Advice = append(r.Advice, "Could not reach the dump index: "+err.Error())
	} else {
		r.PwnedOK = true
		r.PwnedCount = count
	}
	return FinishRisk(r)
}

func Comma(n int) string {
	s := strconv.Itoa(n)
	if n < 0 {
		s = strconv.Itoa(-n)
	}
	var b strings.Builder
	pre := len(s) % 3
	if pre == 0 {
		pre = 3
	}
	b.WriteString(s[:pre])
	for i := pre; i < len(s); i += 3 {
		b.WriteByte(',')
		b.WriteString(s[i : i+3])
	}
	if n < 0 {
		return "-" + b.String()
	}
	return b.String()
}
