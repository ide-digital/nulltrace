// Package check probes the broker catalog: do the opt-out pages still load
// and can the privacy addresses still receive mail. It is only used by
// cmd/catalogcheck and never linked into nulltrace or nulltraced.
package check

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"
)

type Status string

const (
	OK    Status = "ok"
	Warn  Status = "warn"
	Fail  Status = "fail"
	Known Status = "known"
)

// Kind is what went wrong on the wire, if anything.
type Kind string

const (
	KindNone     Kind = ""
	KindTimeout  Kind = "timeout"
	KindNotFound Kind = "dns"
	KindRefused  Kind = "refused"
	KindTLS      Kind = "tls"
	KindOther    Kind = "network"
)

// Page is the final outcome of probing one URL, after any retry.
type Page struct {
	Code   int    // final HTTP status, 0 when there was no response
	Err    Kind   // set when there was no usable response
	Detail string // error text, only used for KindOther
	From   string // host we asked for
	To     string // host we ended up on after redirects
}

// ClassifyPage turns a probe outcome into a status and a short reason.
func ClassifyPage(p Page) (Status, string) {
	switch p.Err {
	case KindTimeout:
		return Warn, "timed out"
	case KindNotFound:
		return Fail, "domain does not resolve"
	case KindRefused:
		return Fail, "connection refused"
	case KindTLS:
		return Fail, "invalid TLS certificate"
	case KindOther:
		return Warn, "connection error: " + p.Detail
	}

	switch c := p.Code; {
	case c == 404 || c == 410:
		return Fail, fmt.Sprintf("page is gone (%d)", c)
	case c >= 500:
		return Fail, fmt.Sprintf("server error (%d)", c)
	case c == 401 || c == 403 || c == 429:
		return Warn, fmt.Sprintf("blocked (%d), probably bot protection", c)
	case c >= 200 && c < 400:
		if p.To != "" && registrable(p.To) != registrable(p.From) {
			return Warn, "redirects to " + p.To
		}
		return OK, ""
	default:
		return Warn, fmt.Sprintf("unexpected status %d", c)
	}
}

// Mail is what the DNS lookups for a privacy address's domain turned up.
type Mail struct {
	MX   bool   // at least one MX record
	Host bool   // the domain has an A/AAAA record
	Err  string // lookup failed for a reason other than "does not exist"
}

func ClassifyMail(m Mail) (Status, string) {
	switch {
	case m.MX:
		return OK, ""
	case m.Host:
		return Warn, "has an A record but no MX"
	case m.Err != "":
		return Warn, "dns lookup failed: " + m.Err
	default:
		return Fail, "domain has no MX or address records"
	}
}

// kindOf sorts a transport error into one of the buckets above.
func kindOf(err error) Kind {
	if err == nil {
		return KindNone
	}
	var dns *net.DNSError
	if errors.As(err, &dns) && dns.IsNotFound {
		return KindNotFound
	}
	var cert *tls.CertificateVerificationError
	var unknownCA x509.UnknownAuthorityError
	var badCert x509.CertificateInvalidError
	var badHost x509.HostnameError
	if errors.As(err, &cert) || errors.As(err, &unknownCA) ||
		errors.As(err, &badCert) || errors.As(err, &badHost) {
		return KindTLS
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return KindTimeout
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return KindTimeout
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return KindRefused
	}
	// Windows reports refusals with its own errno, which errors.Is won't match.
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "connection refused") || strings.Contains(msg, "actively refused") {
		return KindRefused
	}
	return KindOther
}

// registrable is a rough "which site is this" key: example.com for
// www.example.com, example.co.uk for shop.example.co.uk. It doesn't carry the
// whole public suffix list, which is fine for spotting a move to another site.
func registrable(host string) string {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	if net.ParseIP(host) != nil {
		return host
	}
	labels := strings.Split(host, ".")
	n := len(labels)
	if n <= 2 {
		return host
	}
	if len(labels[n-1]) == 2 && secondLevel[labels[n-2]] && n >= 3 {
		return strings.Join(labels[n-3:], ".")
	}
	return strings.Join(labels[n-2:], ".")
}

var secondLevel = map[string]bool{
	"co": true, "com": true, "org": true, "net": true,
	"gov": true, "edu": true, "ac": true,
}
