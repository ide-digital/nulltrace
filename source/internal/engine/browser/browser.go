package browser

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/launcher/flags"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
)

type Options struct {
	Headless bool
	Bin      string // override path; empty auto-detects/downloads
	Stealth  bool
	Proxy    string // http(s):// or socks5:// URL
	Timeout  time.Duration
}

type Engine struct {
	Browser    *rod.Browser
	Page       *rod.Page
	profileDir string
}

func Launch(opts Options) (*Engine, error) {
	if opts.Timeout <= 0 {
		opts.Timeout = 45 * time.Second
	}
	profileDir, err := ephemeralProfile()
	if err != nil {
		return nil, err
	}

	l := launcher.New().
		Leakless(true).
		Headless(opts.Headless).
		UserDataDir(profileDir).
		Set(flags.Flag("no-first-run")).
		Set(flags.Flag("no-default-browser-check")).
		Set(flags.Flag("disable-blink-features"), "AutomationControlled").
		Set(flags.Flag("disable-dev-shm-usage")).
		Set(flags.Flag("disable-notifications"))
	if opts.Bin != "" {
		l = l.Bin(opts.Bin)
	}
	if opts.Proxy != "" {
		p, perr := normalizeProxy(opts.Proxy)
		if perr != nil {
			os.RemoveAll(profileDir)
			return nil, perr
		}
		l = l.Proxy(p)
	}

	controlURL, err := l.Launch()
	if err != nil {
		os.RemoveAll(profileDir)
		return nil, fmt.Errorf("launch browser: %w", err)
	}

	b := rod.New().ControlURL(controlURL).NoDefaultDevice().Timeout(opts.Timeout)
	if err := b.Connect(); err != nil {
		os.RemoveAll(profileDir)
		return nil, fmt.Errorf("connect to browser: %w", err)
	}

	var page *rod.Page
	if opts.Stealth {
		page, err = stealth.Page(b)
	} else {
		page, err = b.Page(proto.TargetCreateTarget{})
	}
	if err != nil {
		_ = b.Close()
		os.RemoveAll(profileDir)
		return nil, fmt.Errorf("open page: %w", err)
	}

	return &Engine{Browser: b, Page: page, profileDir: profileDir}, nil
}

func (e *Engine) Close() {
	if e == nil {
		return
	}
	if e.Browser != nil {
		_ = e.Browser.Close()
	}
	if e.profileDir != "" {
		_ = os.RemoveAll(e.profileDir)
	}
}

func (e *Engine) Navigate(raw string) error {
	if err := e.Page.Navigate(raw); err != nil {
		return fmt.Errorf("navigate: %w", err)
	}
	return e.Page.WaitLoad()
}

func (e *Engine) CurrentURL() string {
	info, err := e.Page.Info()
	if err != nil || info == nil {
		return ""
	}
	return info.URL
}

func (e *Engine) Fill(selector, value string) error {
	el, err := e.Page.Element(selector)
	if err != nil {
		return fmt.Errorf("fill %q: %w", selector, err)
	}
	if err := el.Input(value); err != nil {
		return fmt.Errorf("type into %q: %w", selector, err)
	}
	return nil
}

func (e *Engine) Click(selector string) error {
	el, err := e.Page.Element(selector)
	if err != nil {
		return fmt.Errorf("click %q: %w", selector, err)
	}
	return el.Click(proto.InputMouseButtonLeft, 1)
}

func (e *Engine) Has(selector string) bool {
	_, err := e.Page.Element(selector)
	return err == nil
}

func (e *Engine) HTML() (string, error) {
	return e.Page.HTML()
}

func (e *Engine) Text() (string, error) {
	body, err := e.Page.Element("body")
	if err != nil {
		return "", err
	}
	txt, err := body.Text()
	if err != nil {
		return "", err
	}
	return strings.ToLower(txt), nil
}

func (e *Engine) Screenshot(path string) error {
	raw, err := e.Page.Screenshot(true, nil)
	if err != nil {
		return fmt.Errorf("screenshot: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}

func ephemeralProfile() (string, error) {
	dir, err := os.MkdirTemp("", "nulltrace-browser-*")
	if err != nil {
		return "", fmt.Errorf("create browser profile: %w", err)
	}
	return dir, nil
}

func normalizeProxy(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse proxy url: %w", err)
	}
	switch u.Scheme {
	case "http", "https", "socks5", "socks5h":
	default:
		return "", fmt.Errorf("unsupported proxy scheme %q (http, https, socks5)", u.Scheme)
	}
	if u.Host == "" {
		return "", fmt.Errorf("proxy url missing host")
	}
	scheme := u.Scheme
	if scheme == "https" {
		scheme = "http"
	}
	if scheme == "socks5h" {
		scheme = "socks5"
	}
	return scheme + "://" + u.Host, nil
}
