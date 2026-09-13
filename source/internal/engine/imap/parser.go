package imapx

import (
	"net/url"
	"regexp"
	"strings"
)

var (
	hrefRe  = regexp.MustCompile(`(?i)href=["'](https?://[^"']+)["']`)
	plainRe = regexp.MustCompile(`https?://[^\s<>"']+`)
)

// ExtractURLs returns http(s) URLs from a MIME text or HTML body.
func ExtractURLs(body string) []string {
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
