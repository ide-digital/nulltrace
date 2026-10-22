package imapx

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type FetchResult struct {
	URL        string
	Status     int
	Success    bool
	NeedsHuman bool
	Indicator  string
	Snippet    string
}

var successPhrases = []string{
	"request confirmed",
	"record deleted",
	"opt-out successful",
	"opt out successful",
	"successfully removed",
	"your request has been received",
	"we'll process your request",
	"we will process your request",
	"confirmation complete",
}

var challengePhrases = []string{
	"captcha",
	"cf-challenge",
	"attention required",
	"verify you are human",
	"access denied",
	"datadome",
	"perimeterx",
	"just a moment",
}

// FetchConfirmation GETs a link out of a broker's mail. allow is checked on
// the link and on every redirect, since anyone can send mail to the inbox.
func FetchConfirmation(ctx context.Context, client *http.Client, rawURL string, allow func(host string) bool) (FetchResult, error) {
	if !allowedURL(rawURL, allow) {
		return FetchResult{}, fmt.Errorf("confirmation link is not on a known broker domain")
	}
	c := http.Client{Timeout: 20 * time.Second}
	if client != nil {
		c = *client
	}
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 8 {
			return fmt.Errorf("too many redirects")
		}
		if !allowedURL(req.URL.String(), allow) {
			return fmt.Errorf("redirect left the broker's domain")
		}
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return FetchResult{}, fmt.Errorf("confirmation request: %w", err)
	}
	req.Header.Set("User-Agent", "NullTrace/1.0")
	req.Header.Set("Accept", "text/html, text/plain;q=0.9, */*;q=0.8")

	resp, err := c.Do(req)
	if err != nil {
		return FetchResult{}, fmt.Errorf("confirmation fetch: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	low := strings.ToLower(string(body))
	res := FetchResult{URL: rawURL, Status: resp.StatusCode, Snippet: clip(string(body), 240)}

	for _, p := range challengePhrases {
		if strings.Contains(low, p) {
			res.NeedsHuman = true
			res.Indicator = p
			return res, nil
		}
	}
	for _, p := range successPhrases {
		if strings.Contains(low, p) {
			res.Success = true
			res.Indicator = p
			return res, nil
		}
	}
	if resp.StatusCode >= 400 {
		res.NeedsHuman = true
		res.Indicator = resp.Status
	}
	return res, nil
}

func allowedURL(raw string, allow func(string) bool) bool {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" {
		return false
	}
	return allow != nil && allow(u.Hostname())
}

func clip(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= n {
		return s
	}
	return strings.ToValidUTF8(s[:n], "") + "..."
}
