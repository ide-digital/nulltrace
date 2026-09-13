package imapx

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	idle "github.com/emersion/go-imap-idle"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"

	"github.com/4x3/nulltrace/internal/broker"
	"github.com/4x3/nulltrace/internal/config"
	ncrypto "github.com/4x3/nulltrace/internal/crypto"
)

var urlRe = regexp.MustCompile(`https?://[^\s<>"']+`)

type Finding struct {
	From       string
	Subject    string
	Date       time.Time
	BrokerID   string
	URLs       []string
	TrackingID string
	Preview    string
}

type Listener struct {
	cfg      config.IMAPConfig
	password []byte
	reg      *broker.Registry
	log      *slog.Logger
	handler  func(context.Context, Finding) error
}

func New(cfg config.IMAPConfig, password []byte, reg *broker.Registry, log *slog.Logger, handler func(context.Context, Finding) error) *Listener {
	if log == nil {
		log = slog.Default()
	}
	return &Listener{
		cfg:      cfg,
		password: append([]byte(nil), password...),
		reg:      reg,
		log:      log,
		handler:  handler,
	}
}

func (l *Listener) Close() { ncrypto.Zeroize(l.password) }

func (l *Listener) Configured() bool {
	return l != nil && l.cfg.Host != "" && l.cfg.Username != ""
}

func (l *Listener) Run(ctx context.Context) error {
	if !l.Configured() {
		return fmt.Errorf("imap: missing host or username")
	}
	addr := fmt.Sprintf("%s:%d", l.cfg.Host, l.cfg.Port)

	var (
		c   *client.Client
		err error
	)
	if l.cfg.TLS {
		c, err = client.DialTLS(addr, nil)
	} else {
		c, err = client.Dial(addr)
	}
	if err != nil {
		return fmt.Errorf("imap dial %s: %w", addr, err)
	}
	defer func() { _ = c.Logout() }()

	if err := c.Login(l.cfg.Username, string(l.password)); err != nil {
		return fmt.Errorf("imap login: %w", err)
	}

	mbox := l.cfg.Mailbox
	if mbox == "" {
		mbox = "INBOX"
	}
	if _, err := c.Select(mbox, false); err != nil {
		return fmt.Errorf("imap select %s: %w", mbox, err)
	}

	l.log.Info("imap connected", "host", l.cfg.Host, "mailbox", mbox)

	idleClient := idle.NewClient(c)
	updates := make(chan client.Update, 8)
	c.Updates = updates

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := l.drainUnseen(ctx, c); err != nil {
			l.log.Warn("imap drain", "err", err)
		}

		idleCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		idleDone := make(chan error, 1)
		go func() {
			idleDone <- idleClient.IdleWithFallback(idleCtx.Done(), 5*time.Minute)
		}()

		select {
		case <-ctx.Done():
			cancel()
			<-idleDone
			return ctx.Err()
		case err := <-idleDone:
			cancel()
			if err != nil && ctx.Err() == nil {
				return fmt.Errorf("imap idle: %w", err)
			}
		case <-updates:
			cancel()
			<-idleDone
		}
	}
}

func (l *Listener) drainUnseen(ctx context.Context, c *client.Client) error {
	criteria := imap.NewSearchCriteria()
	criteria.WithoutFlags = []string{imap.SeenFlag}
	ids, err := c.Search(criteria)
	if err != nil {
		return fmt.Errorf("imap search: %w", err)
	}
	if len(ids) == 0 {
		return nil
	}
	seq := new(imap.SeqSet)
	seq.AddNum(ids...)
	section := &imap.BodySectionName{}
	items := []imap.FetchItem{section.FetchItem(), imap.FetchEnvelope, imap.FetchUid}
	ch := make(chan *imap.Message, 8)
	done := make(chan error, 1)
	go func() { done <- c.Fetch(seq, items, ch) }()
	for msg := range ch {
		if err := ctx.Err(); err != nil {
			return err
		}
		finding, err := l.parseMessage(msg, section)
		if err != nil {
			l.log.Warn("imap parse", "err", err)
			continue
		}
		if finding == nil {
			continue
		}
		if l.handler != nil {
			if err := l.handler(ctx, *finding); err != nil {
				l.log.Warn("imap handler", "err", err)
			}
		}
	}
	return <-done
}

func (l *Listener) parseMessage(msg *imap.Message, section *imap.BodySectionName) (*Finding, error) {
	if msg == nil {
		return nil, nil
	}
	r := msg.GetBody(section)
	if r == nil {
		return nil, nil
	}
	mr, err := mail.CreateReader(r)
	if err != nil {
		return nil, err
	}
	defer mr.Close()

	header := mr.Header
	subject, _ := header.Subject()
	from := ""
	if addrs, err := header.AddressList("From"); err == nil && len(addrs) > 0 {
		from = addrs[0].Address
	}
	date, _ := header.Date()
	tracking := header.Get("X-NullTrace-Tracking-ID")
	if tracking == "" {
		tracking = header.Get("In-Reply-To")
	}

	var bodies []string
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch h := p.Header.(type) {
		case *mail.InlineHeader:
			ct, _, _ := h.ContentType()
			if strings.HasPrefix(ct, "text/") {
				b, _ := io.ReadAll(io.LimitReader(p.Body, 256*1024))
				bodies = append(bodies, string(b))
			}
		default:
			_, _ = io.Copy(io.Discard, io.LimitReader(p.Body, 64*1024))
		}
	}
	joined := strings.Join(bodies, "\n")
	urls := unique(extractURLs(joined))
	if len(urls) == 0 && tracking == "" {
		return nil, nil
	}

	brokerID := ""
	if l.reg != nil {
		if b, ok := l.reg.MatchURL(joined + " " + from + " " + strings.Join(urls, " ")); ok {
			brokerID = b.ID
		}
	}
	preview := joined
	if len(preview) > 400 {
		preview = preview[:400]
	}
	return &Finding{
		From:       from,
		Subject:    subject,
		Date:       date,
		BrokerID:   brokerID,
		URLs:       urls,
		TrackingID: tracking,
		Preview:    preview,
	}, nil
}

func extractURLs(s string) []string {
	found := urlRe.FindAllString(s, -1)
	var out []string
	for _, u := range found {
		u = strings.TrimRight(u, ".,);[]")
		parsed, err := url.Parse(u)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			continue
		}
		out = append(out, u)
	}
	return out
}

func unique(in []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
