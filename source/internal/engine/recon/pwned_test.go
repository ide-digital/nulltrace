package recon

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
	"testing"
)

type fakeRT func(*http.Request) *http.Response

func (f fakeRT) RoundTrip(r *http.Request) (*http.Response, error) { return f(r), nil }

func reply(code int, body string) *http.Client {
	return &http.Client{Transport: fakeRT(func(*http.Request) *http.Response {
		return &http.Response{
			StatusCode: code,
			Status:     http.StatusText(code),
			Body:       io.NopCloser(strings.NewReader(body)),
		}
	})}
}

func TestPwnedPrefixHit(t *testing.T) {
	sum := sha1.Sum([]byte("hunter2"))
	suffix := strings.ToUpper(hex.EncodeToString(sum[:]))[5:]
	c := reply(200, "0000000000000000000000000000000000A:0\r\n"+suffix+":17043\r\n")
	n, err := PwnedPasswordPrefix(context.Background(), c, []byte("hunter2"))
	if err != nil || n != 17043 {
		t.Fatalf("n=%d err=%v", n, err)
	}
}

func TestPwnedPrefixServerErrorIsAnError(t *testing.T) {
	n, err := PwnedPasswordPrefix(context.Background(), reply(503, "busy"), []byte("hunter2"))
	if err == nil {
		t.Fatalf("503 came back as %d hits with no error", n)
	}
}
