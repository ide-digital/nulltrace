package mailer

import (
	"context"
	"fmt"
	"time"

	"github.com/wneessen/go-mail"

	"github.com/4x3/nulltrace/internal/config"
	"github.com/4x3/nulltrace/internal/nterr"
)

// Probe dials the SMTP server and authenticates. It does not send mail.
func Probe(ctx context.Context, cfg config.SMTPConfig, password []byte) error {
	if cfg.Host == "" || cfg.Username == "" {
		return nterr.ErrMissingSMTP
	}
	if len(password) == 0 {
		return fmt.Errorf("mailbox password is empty")
	}
	opts := []mail.Option{
		mail.WithPort(cfg.Port),
		mail.WithSMTPAuth(mail.SMTPAuthAutoDiscover),
		mail.WithUsername(cfg.Username),
		mail.WithPassword(string(password)),
		mail.WithTimeout(15 * time.Second),
	}
	if cfg.STARTTLS {
		opts = append(opts, mail.WithTLSPortPolicy(mail.TLSMandatory))
	}
	client, err := mail.NewClient(cfg.Host, opts...)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer func() { _ = client.Close() }()
	if err := client.DialWithContext(ctx); err != nil {
		return fmt.Errorf("could not sign in to %s: %w", cfg.Host, err)
	}
	return nil
}
