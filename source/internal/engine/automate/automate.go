package automate

import (
	"context"
	"fmt"
	"strings"

	"github.com/4x3/nulltrace/internal/engine/browser"
	"github.com/4x3/nulltrace/internal/engine/captcha"
	"github.com/4x3/nulltrace/internal/engine/mailer"
)

type Result struct {
	BrokerID string `json:"broker_id"`
	Status   string `json:"status"` // SUBMITTED | NEEDS_HUMAN | FAILED | CONFIRMED
	Detail   string `json:"detail"`
	URL      string `json:"url,omitempty"`
}

type Selectors struct {
	FirstName   []string
	LastName    []string
	Email       []string
	Phone       []string
	City        []string
	Address     []string
	ProfileURL  []string
	SearchInput []string
	SearchGo    []string
	Submit      []string
	Consent     []string
}

type Spec struct {
	URL       string
	Selectors Selectors
}

func DefaultSelectors() Selectors {
	return Selectors{
		FirstName: []string{
			"input[name='firstName']", "input[name='first_name']", "input[name='first']",
			"input[placeholder*='First' i]", "input[id*='firstName' i]", "input[id*='first_name' i]",
		},
		LastName: []string{
			"input[name='lastName']", "input[name='last_name']", "input[name='last']",
			"input[placeholder*='Last' i]", "input[id*='lastName' i]", "input[id*='last_name' i]",
		},
		Email: []string{
			"input[type='email']", "input[name='email']", "input[placeholder*='email' i]", "input[id*='email' i]",
		},
		Phone: []string{
			"input[type='tel']", "input[name='phone']", "input[placeholder*='phone' i]", "input[id*='phone' i]",
		},
		City: []string{
			"input[name='city']", "input[placeholder*='city' i]", "input[id*='city' i]",
		},
		Address: []string{
			"input[name='address']", "input[placeholder*='address' i]", "input[id*='address' i]",
		},
		ProfileURL: []string{
			"input[name='url']", "input[placeholder*='url' i]", "input[placeholder*='profile' i]", "textarea[name='url']",
		},
		SearchInput: []string{
			"input[name='q']", "input[type='search']", "input[placeholder*='search' i]", "input[placeholder*='name' i]",
		},
		SearchGo: []string{
			"button[type='submit']", "button[aria-label*='search' i]", "input[type='submit']",
		},
		Submit: []string{
			"button[type='submit']", "input[type='submit']", "button[aria-label*='Submit' i]", "button[aria-label*='Remove' i]",
		},
		Consent: []string{
			"input[type='checkbox'][name*='consent' i]", "input[type='checkbox'][name*='agree' i]",
		},
	}
}

type Engine struct {
	Browser *browser.Engine
	Solver  *captcha.Client
}

func (e *Engine) Run(ctx context.Context, spec Spec, id mailer.IdentityView) (Result, error) {
	res := Result{URL: spec.URL, Status: "FAILED"}

	if e.Browser == nil {
		return res, fmt.Errorf("automate: browser not launched")
	}
	if spec.URL == "" {
		return res, fmt.Errorf("automate: no opt-out URL")
	}

	if err := e.Browser.Navigate(spec.URL); err != nil {
		res.Detail = fmt.Sprintf("navigate: %v", err)
		return res, err
	}

	challenge, err := e.detectChallenge(ctx)
	if err != nil {
		return res, err
	}
	if challenge.detected {
		if challenge.solved {
			res.Detail = "captcha solved with CapSolver"
		} else {
			res.Status = "NEEDS_HUMAN"
			res.Detail = challenge.kind
			return res, nil
		}
	}

	if err := e.fill(spec.Selectors, id); err != nil {
		res.Detail = fmt.Sprintf("fill: %v", err)
		return res, err
	}

	if err := e.submit(spec.Selectors); err != nil {
		res.Detail = fmt.Sprintf("submit: %v", err)
		return res, err
	}

	text, _ := e.Browser.Text()
	low := text
	switch {
	case containsAny(low, successPhrases):
		res.Status = "CONFIRMED"
		res.Detail = "success indicator found"
	case containsAny(low, challengePhrases):
		res.Status = "NEEDS_HUMAN"
		res.Detail = "challenge after submit"
	default:
		res.Status = "SUBMITTED"
		res.Detail = "request submitted (unverified)"
	}
	return res, nil
}

func (e *Engine) fill(s Selectors, id mailer.IdentityView) error {
	set := func(selectors []string, value string) {
		if value == "" {
			return
		}
		for _, sel := range selectors {
			if e.Browser.Has(sel) {
				_ = e.Browser.Fill(sel, value)
				return
			}
		}
	}
	set(s.FirstName, id.First)
	set(s.LastName, id.Last)
	set(s.Email, id.PrimaryEmail())
	if len(id.Phones) > 0 {
		set(s.Phone, id.Phones[0])
	}
	if len(id.Cities) > 0 {
		set(s.City, id.Cities[0])
	}
	if len(id.Addresses) > 0 {
		set(s.Address, id.Addresses[0])
	}
	return nil
}

func (e *Engine) submit(s Selectors) error {
	for _, sel := range s.Consent {
		if !e.Browser.Has(sel) {
			continue
		}
		_ = e.Browser.Click(sel)
	}
	for _, sel := range s.Submit {
		if e.Browser.Has(sel) {
			if err := e.Browser.Click(sel); err != nil {
				return err
			}
			return nil
		}
	}
	return fmt.Errorf("no submit control found")
}

type challenge struct {
	detected bool
	solved   bool
	kind     string
}

var successPhrases = []string{
	"request confirmed", "record deleted", "opt-out successful", "opt out successful",
	"successfully removed", "your request has been received", "we'll process your request",
	"confirmation complete", "thank you", "submitted",
}

var challengePhrases = []string{
	"captcha", "cf-challenge", "attention required", "verify you are human",
	"access denied", "datadome", "perimeterx", "just a moment", "checking your browser",
}

func (e *Engine) detectChallenge(ctx context.Context) (challenge, error) {
	htmlText, err := e.Browser.HTML()
	if err != nil {
		return challenge{}, err
	}
	low := strings.ToLower(htmlText)

	out := challenge{}
	if strings.Contains(low, "g-recaptcha") || strings.Contains(low, "recaptcha") {
		out.detected = true
		out.kind = "recaptcha"
		return e.trySolve(ctx, low, out, "recaptcha")
	}
	if strings.Contains(low, "h-captcha") || strings.Contains(low, "hcaptcha") {
		out.detected = true
		out.kind = "hcaptcha"
		return e.trySolve(ctx, low, out, "hcaptcha")
	}
	for _, p := range challengePhrases {
		if p != "captcha" && strings.Contains(low, p) {
			out.detected = true
			out.kind = p
			return out, nil
		}
	}
	return out, nil
}

func (e *Engine) trySolve(ctx context.Context, html string, c challenge, kind string) (challenge, error) {
	if e.Solver == nil {
		return c, nil
	}
	siteKey := extractSiteKey(html)
	if siteKey == "" {
		return c, nil
	}
	url := e.Browser.CurrentURL()
	var token string
	var err error
	switch kind {
	case "recaptcha":
		token, err = e.Solver.SolveRecaptchaV2(ctx, url, siteKey)
	default:
		token, err = e.Solver.SolveHCaptcha(ctx, url, siteKey)
	}
	if err != nil {
		return c, err
	}
	if token == "" {
		return c, nil
	}
	if err := e.injectToken(kind, token); err != nil {
		return c, err
	}
	c.solved = true
	return c, nil
}

func (e *Engine) injectToken(kind, token string) error {
	var selectors []string
	switch kind {
	case "recaptcha":
		selectors = []string{"#g-recaptcha-response", "textarea[name='g-recaptcha-response']"}
	default:
		selectors = []string{"textarea[name='h-captcha-response']", "textarea[name='g-recaptcha-response']"}
	}
	for _, sel := range selectors {
		if e.Browser.Has(sel) {
			return e.Browser.Fill(sel, token)
		}
	}
	return nil
}

func extractSiteKey(html string) string {
	for _, marker := range []string{"data-sitekey=\"", "data-sitekey='"} {
		i := strings.Index(html, marker)
		if i < 0 {
			continue
		}
		rest := html[i+len(marker):]
		end := strings.IndexAny(rest, "\"'")
		if end > 0 {
			return rest[:end]
		}
	}
	return ""
}

func containsAny(haystack string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(haystack, n) {
			return true
		}
	}
	return false
}
