package imapx

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/ide-digital/nulltrace/internal/engine/mailer"
)

var (
	hrefRe  = regexp.MustCompile(`(?i)href=["'](https?://[^"']+)["']`)
	plainRe = regexp.MustCompile(`https?://[^\s<>"']+`)
	uuidRe  = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

func extractURLs(body string) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(raw string) {
		raw = strings.TrimRight(raw, ".,);[]")
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return
		}
		if _, ok := seen[raw]; ok {
			return
		}
		seen[raw] = struct{}{}
		out = append(out, raw)
	}
	for _, m := range hrefRe.FindAllStringSubmatch(body, -1) {
		if len(m) > 1 {
			add(m[1])
		}
	}
	for _, m := range plainRe.FindAllString(body, -1) {
		add(m)
	}
	return out
}

func urlHost(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

func addrHost(addr string) string {
	if i := strings.LastIndexByte(addr, '@'); i >= 0 {
		return addr[i+1:]
	}
	return ""
}

// trackingFromReply pulls our token back out of an In-Reply-To like
// <nulltrace.UUID@example.com>. Anything else is ignored.
func trackingFromReply(h string) string {
	h = strings.Trim(strings.TrimSpace(h), "<>")
	rest, ok := strings.CutPrefix(h, mailer.MessageIDPrefix)
	if !ok {
		return ""
	}
	tok, _, _ := strings.Cut(rest, "@")
	if !uuidRe.MatchString(tok) {
		return ""
	}
	return tok
}

func LooksLikeConfirmation(u string) bool {
	low := strings.ToLower(u)
	keys := []string{"confirm", "opt-out", "optout", "unsubscribe", "verify", "removal", "privacy", "ccpa", "gdpr"}
	for _, k := range keys {
		if strings.Contains(low, k) {
			return true
		}
	}
	return false
}
