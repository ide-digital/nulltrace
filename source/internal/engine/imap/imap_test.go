package imapx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestTrackingFromReply(t *testing.T) {
	tok := "0b7a4c1e-2f3d-4e5f-8a9b-0c1d2e3f4a5b"
	if got := trackingFromReply("<nulltrace." + tok + "@gmail.com>"); got != tok {
		t.Fatalf("got %q", got)
	}
	for _, h := range []string{"", "<CAF123@mail.gmail.com>", "<nulltrace.not-a-uuid@x.com>", tok} {
		if got := trackingFromReply(h); got != "" {
			t.Fatalf("%q should not give a token, got %q", h, got)
		}
	}
}

func TestExtractURLsDedupes(t *testing.T) {
	body := `<a href="https://spokeo.com/confirm?x=1">confirm</a> or https://spokeo.com/confirm?x=1.`
	got := extractURLs(body)
	if len(got) != 1 || got[0] != "https://spokeo.com/confirm?x=1" {
		t.Fatalf("got %v", got)
	}
}

func TestFetchConfirmationStaysOnDomain(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("Your request has been received"))
	}))
	defer ok.Close()
	okURL, _ := url.Parse(ok.URL)

	// Same server, but reached as "localhost" so the host differs.
	away := strings.Replace(ok.URL, "127.0.0.1", "localhost", 1)
	bounce := httptest.NewServer(http.RedirectHandler(away, http.StatusFound))
	defer bounce.Close()

	allow := func(host string) bool { return host == okURL.Hostname() }

	res, err := FetchConfirmation(context.Background(), nil, ok.URL+"/confirm", allow)
	if err != nil || !res.Success {
		t.Fatalf("allowed host: res=%+v err=%v", res, err)
	}
	if _, err := FetchConfirmation(context.Background(), nil, bounce.URL+"/confirm", allow); err == nil {
		t.Fatal("redirect to another host should be refused")
	}
	if _, err := FetchConfirmation(context.Background(), nil, "https://evil.test/confirm", allow); err == nil {
		t.Fatal("unknown host should be refused before any request")
	}
}
