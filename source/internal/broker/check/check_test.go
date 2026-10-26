package check

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/ide-digital/nulltrace/internal/broker"
)

func TestClassifyPage(t *testing.T) {
	tests := []struct {
		name string
		in   Page
		want Status
	}{
		{"200", Page{Code: 200, From: "a.com", To: "a.com"}, OK},
		{"301 chain ends 200", Page{Code: 200, From: "a.com", To: "www.a.com"}, OK},
		{"3xx final", Page{Code: 302, From: "a.com", To: "a.com"}, OK},
		{"404", Page{Code: 404}, Fail},
		{"410", Page{Code: 410}, Fail},
		{"500 after retry", Page{Code: 500}, Fail},
		{"503 after retry", Page{Code: 503}, Fail},
		{"401", Page{Code: 401}, Warn},
		{"403", Page{Code: 403}, Warn},
		{"429", Page{Code: 429}, Warn},
		{"418", Page{Code: 418}, Warn},
		{"timeout", Page{Err: KindTimeout}, Warn},
		{"other network error", Page{Err: KindOther, Detail: "EOF"}, Warn},
		{"nxdomain", Page{Err: KindNotFound}, Fail},
		{"refused", Page{Err: KindRefused}, Fail},
		{"bad cert", Page{Err: KindTLS}, Fail},
		{"moved to another site", Page{Code: 200, From: "a.com", To: "b.com"}, Warn},
		{"same site, other tld label", Page{Code: 200, From: "shop.a.co.uk", To: "a.co.uk"}, OK},
		{"404 on another site is still gone", Page{Code: 404, From: "a.com", To: "b.com"}, Fail},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, why := ClassifyPage(tc.in)
			if got != tc.want {
				t.Fatalf("got %s (%s), want %s", got, why, tc.want)
			}
			if got != OK && why == "" {
				t.Fatal("non-ok result should say why")
			}
		})
	}
}

func TestClassifyMail(t *testing.T) {
	tests := []struct {
		name string
		in   Mail
		want Status
	}{
		{"mx", Mail{MX: true}, OK},
		{"mx and host", Mail{MX: true, Host: true}, OK},
		{"a record only", Mail{Host: true}, Warn},
		{"lookup trouble", Mail{Err: "i/o timeout"}, Warn},
		{"nothing", Mail{}, Fail},
	}
	for _, tc := range tests {
		if got, why := ClassifyMail(tc.in); got != tc.want {
			t.Errorf("%s: got %s (%s), want %s", tc.name, got, why, tc.want)
		}
	}
}

func TestKindOf(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want Kind
	}{
		{"nil", nil, KindNone},
		{"nxdomain", &net.DNSError{Err: "no such host", IsNotFound: true}, KindNotFound},
		{"dns timeout", &net.DNSError{Err: "i/o timeout", IsTimeout: true}, KindTimeout},
		{"deadline", context.DeadlineExceeded, KindTimeout},
		{"refused", &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}, KindRefused},
		{"refused text", errors.New("dial tcp: connectex: No connection could be made because the target machine actively refused it"), KindRefused},
		{"bad host cert", x509.HostnameError{Host: "a.com"}, KindTLS},
		{"unknown ca", x509.UnknownAuthorityError{}, KindTLS},
		{"reset", errors.New("read: connection reset by peer"), KindOther},
	}
	for _, tc := range tests {
		if got := kindOf(tc.err); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestRegistrable(t *testing.T) {
	for in, want := range map[string]string{
		"example.com":         "example.com",
		"www.example.com":     "example.com",
		"a.b.example.com:443": "example.com",
		"shop.example.co.uk":  "example.co.uk",
		"EXAMPLE.com.":        "example.com",
		"127.0.0.1:8080":      "127.0.0.1",
		"localhost":           "localhost",
	} {
		if got := registrable(in); got != want {
			t.Errorf("registrable(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRunAgainstLocalServer(t *testing.T) {
	var flaky atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	mux.HandleFunc("/moved", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ok", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/gone", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	mux.HandleFunc("/blocked", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(403) })
	mux.HandleFunc("/down", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) })
	// the first attempt is a HEAD plus a GET fallback, so fail the first two
	mux.HandleFunc("/flaky", func(w http.ResponseWriter, r *http.Request) {
		if flaky.Add(1) <= 2 {
			w.WriteHeader(500)
			return
		}
		w.WriteHeader(200)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	var brokers []broker.Broker
	want := map[string]Status{
		"ok": OK, "moved": OK, "gone": Fail, "blocked": Warn, "down": Fail, "flaky": OK,
	}
	for id := range want {
		brokers = append(brokers, broker.Broker{ID: id, OptOutURL: srv.URL + "/" + id})
	}

	brokers = append(brokers,
		broker.Broker{ID: "mail-ok", ContactEmail: "privacy@good.test"},
		broker.Broker{ID: "mail-a", ContactEmail: "privacy@bare.test"},
		broker.Broker{ID: "mail-none", ContactEmail: "privacy@nothing.test"},
		broker.Broker{ID: "mail-dupe", ContactEmail: "Legal <legal@good.test>"},
	)
	want["mail-ok"], want["mail-a"], want["mail-none"], want["mail-dupe"] = OK, Warn, Fail, OK

	notFound := &net.DNSError{Err: "no such host", IsNotFound: true}
	res := Run(context.Background(), brokers, Options{
		Concurrency: 4,
		RetryDelay:  time.Millisecond,
		Client:      srv.Client(),
		LookupMX: func(_ context.Context, d string) ([]*net.MX, error) {
			if d == "good.test" {
				return []*net.MX{{Host: "mx.good.test."}}, nil
			}
			return nil, notFound
		},
		LookupHost: func(_ context.Context, d string) ([]string, error) {
			if d == "bare.test" {
				return []string{"192.0.2.1"}, nil
			}
			return nil, notFound
		},
	})

	if len(res) != len(want) {
		t.Fatalf("got %d results, want %d", len(res), len(want))
	}
	for _, r := range res {
		if r.Status != want[r.ID] {
			t.Errorf("%s: got %s (%s), want %s", r.ID, r.Status, r.Reason, want[r.ID])
		}
	}
	if flaky.Load() != 3 {
		t.Errorf("flaky endpoint was hit %d times, want 3 (one retry)", flaky.Load())
	}
}

func TestRunNeverTouchesTheNetworkWithoutFields(t *testing.T) {
	res := Run(context.Background(), []broker.Broker{{ID: "empty"}}, Options{
		Client: &http.Client{Transport: failTransport{}},
	})
	if len(res) != 0 {
		t.Fatalf("expected nothing to check, got %d results", len(res))
	}
}

type failTransport struct{}

func (failTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("network should not be used")
}

func TestBuildKnownAndStale(t *testing.T) {
	now := time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)
	results := []Result{
		{ID: "a", Field: FieldURL, Status: Fail, Reason: "page is gone (404)"},
		{ID: "b", Field: FieldURL, Status: Fail, Reason: "page is gone (404)"},
		{ID: "c", Field: FieldURL, Status: Warn},
		{ID: "d", Field: FieldURL, Status: OK},
	}
	known := []KnownIssue{
		{ID: "a", Field: FieldURL, Reason: "moved, waiting", Added: "2026-10-01"},
		{ID: "d", Field: FieldURL, Reason: "was down", Added: "2026-10-01"},
		{ID: "e", Field: FieldEmail, Reason: "old", Added: "2026-06-01"},
	}
	rep := Build(results, known, now)

	if rep.Summary.Fail != 1 || rep.Summary.Known != 1 || rep.Summary.Warn != 1 || rep.Summary.OK != 1 {
		t.Fatalf("summary: %+v", rep.Summary)
	}
	if len(rep.Resolved) != 2 {
		t.Errorf("expected d and e to show as resolved, got %v", rep.Resolved)
	}
	if rep.Summary.StaleKnown != 1 || rep.StaleKnown[0].ID != "e" {
		t.Errorf("expected e to be stale, got %v", rep.StaleKnown)
	}
	if rep.Broken() != 2 {
		t.Errorf("broken = %d, want 2 (one fail, one stale)", rep.Broken())
	}
	if md := rep.Markdown(); md == "" {
		t.Error("empty markdown")
	}
}

func TestLoadKnownRejectsBadEntries(t *testing.T) {
	dir := t.TempDir()
	write := func(body string) string {
		p := dir + "/k.yaml"
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	if _, err := LoadKnown(write("[]\n")); err != nil {
		t.Fatalf("empty list: %v", err)
	}
	if _, err := LoadKnown(write("- {id: a, field: opt_out_url, reason: x, added: 2026-10-01}\n")); err != nil {
		t.Fatalf("good entry: %v", err)
	}
	if _, err := LoadKnown(write("- {id: a, field: opt_out_url, reason: x, added: last week}\n")); err == nil {
		t.Fatal("bad date should be rejected")
	}
	if _, err := LoadKnown(write("- {id: a, field: opt_out_url, added: 2026-10-01}\n")); err == nil {
		t.Fatal("missing reason should be rejected")
	}
}

// the file we ship has to parse, or the weekly job dies before it checks anything
func TestShippedKnownIssuesParse(t *testing.T) {
	if _, err := LoadKnown("../data/known_issues.yaml"); err != nil {
		t.Fatal(err)
	}
}
