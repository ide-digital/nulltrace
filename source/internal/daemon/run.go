package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/4x3/nulltrace/internal/app"
	ncrypto "github.com/4x3/nulltrace/internal/crypto"
	imapx "github.com/4x3/nulltrace/internal/engine/imap"
	"github.com/4x3/nulltrace/internal/nterr"
	"github.com/4x3/nulltrace/pkg/ipc"
)

func Run(ctx context.Context, passphrase []byte) error {
	rt, err := app.Open(ctx, passphrase, true)
	if err != nil {
		return err
	}
	defer rt.Close()

	if rt.Store.Locked() {
		return nterr.ErrVaultLocked
	}
	slog.SetDefault(rt.Log)

	token, err := ipc.RandomToken()
	if err != nil {
		return err
	}
	if err := ipc.WriteToken(rt.Dirs.TokenFile, token); err != nil {
		return fmt.Errorf("write daemon token: %w", err)
	}
	rt.Log.Info("nulltraced starting",
		"listen", rt.Cfg.ListenAddr,
		"vault", rt.Dirs.VaultDB,
		"brokers", rt.Reg.Len(),
	)

	srv := NewServer(rt, token)
	if err := srv.Listen(); err != nil {
		return err
	}

	pool := NewPool(2, func(ctx context.Context, job Job) error {
		return rt.DispatchSMTP(ctx, job.ActionID)
	}, func(msg string, args ...any) {
		rt.Log.Warn(msg, args...)
	})
	pool.Start(ctx)
	go RunScheduler(ctx, rt, pool)
	go runIMAP(ctx, rt)

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ctx) }()

	select {
	case <-ctx.Done():
		rt.Log.Info("nulltraced shutting down")
		pool.Wait()
		return nil
	case err := <-errCh:
		return err
	}
}

func runIMAP(ctx context.Context, rt *app.Runtime) {
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		if rt.Cfg.IMAP.Host == "" || rt.Cfg.IMAP.Username == "" {
			rt.Log.Info("imap disabled: no host/username in config")
			return
		}
		pw, err := rt.Store.GetSecret(ctx, app.SecretIMAPPassword)
		if err != nil || len(pw) == 0 {
			rt.Log.Info("imap disabled: no imap.password in vault")
			return
		}
		lis := imapx.New(rt.Cfg.IMAP, pw, rt.Reg, rt.Log, rt.HandleIMAP)
		ncrypto.Zeroize(pw)
		err = lis.Run(ctx)
		lis.Close()
		if ctx.Err() != nil {
			return
		}
		rt.Log.Warn("imap listener exited, reconnecting", "err", err, "backoff", backoff.String())
		t := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
		}
		if backoff < 5*time.Minute {
			backoff *= 2
		}
	}
}
