package mailer

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/wneessen/go-mail"
	"golang.org/x/time/rate"

	"github.com/4x3/nulltrace/internal/config"
	ncrypto "github.com/4x3/nulltrace/internal/crypto"
	"github.com/4x3/nulltrace/internal/nterr"
)

//go:embed templates/*.txt
var templateFS embed.FS

const trackingHeader = "X-NullTrace-Tracking-ID"

type Demand struct {
	BrokerName   string
	BrokerDomain string
	To           string
	TrackingID   string
	Identity     IdentityView
	Jurisdiction string
}

type Result struct {
	Subject string
	Body    string
	To      string
	SentAt  time.Time
}

type Dispatcher struct {
	cfg      config.SMTPConfig
	from     string
	limiter  *rate.Limiter
	log      *slog.Logger
	password []byte
}

func New(cfg config.File, smtpPassword []byte, log *slog.Logger) *Dispatcher {
	perMin := cfg.Mailer.PerMinute
	if perMin <= 0 {
		perMin = 5
	}
	if log == nil {
		log = slog.Default()
	}
	pw := append([]byte(nil), smtpPassword...)
	return &Dispatcher{
		cfg:      cfg.SMTP,
		from:     cfg.SMTP.From,
		limiter:  rate.NewLimiter(rate.Every(time.Minute/time.Duration(perMin)), 1),
		log:      log,
		password: pw,
	}
}

func (d *Dispatcher) Close() {
	ncrypto.Zeroize(d.password)
}

func (d *Dispatcher) Configured() bool {
	return d != nil && d.cfg.Host != "" && d.from != ""
}

func (d *Dispatcher) Render(ctx context.Context, demand Demand) (subject, body string, err error) {
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	name, statute, file := selectTemplate(demand.Jurisdiction)
	raw, err := templateFS.ReadFile(file)
	if err != nil {
		return "", "", fmt.Errorf("load template %s: %w", file, err)
	}
	f := viewToFields(demand.BrokerName, demand.BrokerDomain, demand.TrackingID, demand.Identity, statute)
	return render(name, string(raw), f)
}

func selectTemplate(jurisdiction string) (name, statute, file string) {
	switch strings.ToUpper(strings.TrimSpace(jurisdiction)) {
	case "GDPR":
		return "gdpr", "GDPR Article 17", "templates/gdpr.txt"
	case "VCDPA":
		return "state", "Virginia Consumer Data Protection Act (VCDPA)", "templates/state.txt"
	case "CPA":
		return "state", "Colorado Privacy Act (CPA)", "templates/state.txt"
	case "CTDPA":
		return "state", "Connecticut Data Privacy Act (CTDPA)", "templates/state.txt"
	default:
		return "ccpa", "California Consumer Privacy Act, Cal. Civ. Code § 1798.105", "templates/ccpa.txt"
	}
}

func (d *Dispatcher) Send(ctx context.Context, demand Demand) (Result, error) {
	if !d.Configured() {
		return Result{}, nterr.ErrMissingSMTP
	}
	if demand.To == "" {
		return Result{}, fmt.Errorf("send demand: empty recipient")
	}
	if err := d.limiter.Wait(ctx); err != nil {
		return Result{}, fmt.Errorf("%w: %v", nterr.ErrRateLimited, err)
	}
	subject, body, err := d.Render(ctx, demand)
	if err != nil {
		return Result{}, err
	}

	msg := mail.NewMsg()
	if err := msg.From(d.from); err != nil {
		return Result{}, fmt.Errorf("smtp from: %w", err)
	}
	if err := msg.To(demand.To); err != nil {
		return Result{}, fmt.Errorf("smtp to: %w", err)
	}
	msg.Subject(subject)
	msg.SetBodyString(mail.TypeTextPlain, body)
	msg.SetGenHeader(mail.Header(trackingHeader), demand.TrackingID)
	msg.SetMessageID()

	opts := []mail.Option{
		mail.WithPort(d.cfg.Port),
		mail.WithSMTPAuth(mail.SMTPAuthAutoDiscover),
		mail.WithUsername(d.cfg.Username),
		mail.WithPassword(string(d.password)),
	}
	if d.cfg.STARTTLS {
		opts = append(opts, mail.WithTLSPortPolicy(mail.TLSMandatory))
	}
	client, err := mail.NewClient(d.cfg.Host, opts...)
	if err != nil {
		return Result{}, fmt.Errorf("smtp client: %w", err)
	}
	defer func() { _ = client.Close() }()

	if err := client.DialAndSendWithContext(ctx, msg); err != nil {
		return Result{}, fmt.Errorf("failed to dispatch legal demand to %s: %w", demand.To, err)
	}
	d.log.Info("smtp demand sent",
		"broker", demand.BrokerName,
		"to", demand.To,
		"tracking", demand.TrackingID,
	)
	return Result{
		Subject: subject,
		Body:    body,
		To:      demand.To,
		SentAt:  time.Now().UTC(),
	}, nil
}
