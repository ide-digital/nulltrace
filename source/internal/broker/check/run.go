package check

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ide-digital/nulltrace/internal/broker"
)

const (
	FieldURL   = "opt_out_url"
	FieldEmail = "contact_email"

	userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
		"(KHTML, like Gecko) Chrome/124.0 Safari/537.36"
)

type Result struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Field  string `json:"field"`
	Value  string `json:"value"`
	Status Status `json:"status"`
	Reason string `json:"reason,omitempty"`
}

// Options are all optional. The zero value does what the CLI defaults do,
// minus the network knobs tests like to swap out.
type Options struct {
	Concurrency int
	Timeout     time.Duration
	RetryDelay  time.Duration

	Client     *http.Client
	LookupMX   func(ctx context.Context, domain string) ([]*net.MX, error)
	LookupHost func(ctx context.Context, domain string) ([]string, error)
}

type runner struct {
	opt Options

	mu    sync.Mutex
	mails map[string]Mail
}

type job struct {
	b     broker.Broker
	field string
	value string
}

// Run checks every opt-out URL and privacy address in brokers.
func Run(ctx context.Context, brokers []broker.Broker, opt Options) []Result {
	if opt.Concurrency <= 0 {
		opt.Concurrency = 8
	}
	if opt.Timeout <= 0 {
		opt.Timeout = 15 * time.Second
	}
	if opt.RetryDelay <= 0 {
		opt.RetryDelay = 3 * time.Second
	}
	if opt.Client == nil {
		opt.Client = &http.Client{Timeout: opt.Timeout}
	}
	opt.Client = withRedirectLimit(opt.Client)
	if opt.LookupMX == nil {
		opt.LookupMX = net.DefaultResolver.LookupMX
	}
	if opt.LookupHost == nil {
		opt.LookupHost = net.DefaultResolver.LookupHost
	}
	r := &runner{opt: opt, mails: map[string]Mail{}}

	var jobs []job
	for _, b := range brokers {
		if b.OptOutURL != "" {
			jobs = append(jobs, job{b, FieldURL, b.OptOutURL})
		}
		if b.ContactEmail != "" {
			jobs = append(jobs, job{b, FieldEmail, b.ContactEmail})
		}
	}

	out := make([]Result, len(jobs))
	sem := make(chan struct{}, opt.Concurrency)
	var wg sync.WaitGroup
	for i, j := range jobs {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			out[i] = r.one(ctx, j)
		}()
	}
	wg.Wait()

	sort.SliceStable(out, func(a, b int) bool {
		if out[a].ID != out[b].ID {
			return out[a].ID < out[b].ID
		}
		return out[a].Field < out[b].Field
	})
	return out
}

// withRedirectLimit copies c so the caller's client is left alone.
func withRedirectLimit(c *http.Client) *http.Client {
	cp := *c
	cp.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
	return &cp
}

func (r *runner) one(ctx context.Context, j job) Result {
	res := Result{ID: j.b.ID, Name: j.b.Name, Field: j.field, Value: j.value}
	if ctx.Err() != nil {
		res.Status, res.Reason = Warn, "skipped, run was cancelled"
		return res
	}
	switch j.field {
	case FieldURL:
		res.Status, res.Reason = ClassifyPage(r.page(ctx, j.value))
	case FieldEmail:
		domain := mailDomain(j.value)
		if domain == "" {
			res.Status, res.Reason = Fail, "not a valid email address"
			break
		}
		res.Status, res.Reason = ClassifyMail(r.mail(ctx, domain))
	}
	return res
}

// page probes a URL, retrying once after a pause on network errors and 5xx.
func (r *runner) page(ctx context.Context, raw string) Page {
	var p Page
	for try := 0; try < 2; try++ {
		p = r.probe(ctx, raw)
		if p.Err == KindNone && p.Code < 500 {
			break
		}
		if try == 0 {
			select {
			case <-ctx.Done():
				return p
			case <-time.After(r.opt.RetryDelay):
			}
		}
	}
	return p
}

// probe tries HEAD first. Plenty of sites answer HEAD with 405 or a bogus
// 404, so anything that isn't a clean answer gets a GET before we judge it.
func (r *runner) probe(ctx context.Context, raw string) Page {
	p := Page{}
	if u, err := url.Parse(raw); err == nil {
		p.From = u.Host
	}

	resp, err := r.fetch(ctx, http.MethodHead, raw)
	retryWithGet := false
	switch {
	case err == nil && resp.StatusCode >= 400:
		retryWithGet = true
	case err != nil && kindOf(err) == KindOther:
		retryWithGet = true
	}
	if retryWithGet {
		if resp != nil {
			resp.Body.Close()
		}
		resp, err = r.fetch(ctx, http.MethodGet, raw)
	}
	if err != nil {
		p.Err = kindOf(err)
		p.Detail = trimErr(err)
		return p
	}
	defer resp.Body.Close()
	p.Code = resp.StatusCode
	p.To = resp.Request.URL.Host
	return p
}

func (r *runner) fetch(ctx context.Context, method, raw string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, raw, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,*/*;q=0.8")
	return r.opt.Client.Do(req)
}

func (r *runner) mail(ctx context.Context, domain string) Mail {
	r.mu.Lock()
	m, ok := r.mails[domain]
	r.mu.Unlock()
	if ok {
		return m
	}

	m = r.lookup(ctx, domain)
	r.mu.Lock()
	r.mails[domain] = m
	r.mu.Unlock()
	return m
}

func (r *runner) lookup(ctx context.Context, domain string) Mail {
	var m Mail
	mx, err := r.opt.LookupMX(ctx, domain)
	if len(mx) > 0 {
		m.MX = true
		return m
	}
	if err != nil && !isNotFound(err) {
		m.Err = trimErr(err)
		return m
	}
	// No MX. A bare A record can still take mail, so look for one.
	hosts, err := r.opt.LookupHost(ctx, domain)
	switch {
	case len(hosts) > 0:
		m.Host = true
	case err != nil && !isNotFound(err):
		m.Err = trimErr(err)
	}
	return m
}

func isNotFound(err error) bool {
	var dns *net.DNSError
	return errors.As(err, &dns) && dns.IsNotFound
}

func mailDomain(addr string) string {
	a, err := mail.ParseAddress(addr)
	if err != nil {
		// some catalog rows are a bare address with stray spaces
		a = &mail.Address{Address: strings.TrimSpace(addr)}
	}
	at := strings.LastIndex(a.Address, "@")
	if at < 1 || at == len(a.Address)-1 {
		return ""
	}
	return strings.ToLower(a.Address[at+1:])
}

func trimErr(err error) string {
	s := err.Error()
	if len(s) > 120 {
		s = s[:120] + "..."
	}
	return s
}
