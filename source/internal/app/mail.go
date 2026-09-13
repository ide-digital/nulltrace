package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/4x3/nulltrace/internal/broker"
	ncrypto "github.com/4x3/nulltrace/internal/crypto"
	imapx "github.com/4x3/nulltrace/internal/engine/imap"
	"github.com/4x3/nulltrace/internal/engine/mailer"
	"github.com/4x3/nulltrace/internal/nterr"
)

func (rt *Runtime) DispatchSMTP(ctx context.Context, actionID string) error {
	pw, err := rt.Store.GetSecret(ctx, SecretSMTPPassword)
	if err != nil {
		return err
	}
	defer ncrypto.Zeroize(pw)
	d := mailer.New(rt.Cfg, pw, rt.Log)
	defer d.Close()
	return rt.dispatchSMTP(ctx, d, actionID)
}

func (rt *Runtime) SendQueuedSMTP(ctx context.Context, report func(sent, fail int, name string)) (sent, fail int, err error) {
	pw, err := rt.Store.GetSecret(ctx, SecretSMTPPassword)
	if err != nil {
		return 0, 0, err
	}
	defer ncrypto.Zeroize(pw)
	d := mailer.New(rt.Cfg, pw, rt.Log)
	defer d.Close()
	if !d.Configured() {
		return 0, 0, nterr.ErrMissingSMTP
	}
	acts, err := rt.Store.ListActions(ctx)
	if err != nil {
		return 0, 0, err
	}
	for _, a := range acts {
		if err := ctx.Err(); err != nil {
			return sent, fail, err
		}
		if a.Strategy != string(broker.StrategySMTP) || a.CurrentState != string(broker.StateQueued) {
			continue
		}
		name := a.BrokerID
		if row, e := rt.Store.GetBroker(ctx, a.BrokerID); e == nil {
			name = row.Name
		}
		if e := rt.dispatchSMTP(ctx, d, a.ID); e != nil {
			fail++
			if report != nil {
				report(sent, fail, name+": "+e.Error())
			}
			continue
		}
		sent++
		if report != nil {
			report(sent, fail, name)
		}
	}
	return sent, fail, nil
}

func (rt *Runtime) dispatchSMTP(ctx context.Context, d *mailer.Dispatcher, actionID string) error {
	a, err := rt.Store.GetAction(ctx, actionID)
	if err != nil {
		return err
	}
	if a.Strategy != string(broker.StrategySMTP) {
		return fmt.Errorf("action %s is not SMTP_DEMAND", actionID)
	}
	if !d.Configured() {
		_ = rt.Store.UpdateActionState(ctx, a.ID, string(broker.StateFailed), 1, nterr.ErrMissingSMTP.Error(), nil)
		return nterr.ErrMissingSMTP
	}
	row, err := rt.Store.GetBroker(ctx, a.BrokerID)
	if err != nil {
		return err
	}
	b := brokerFromRow(row)
	if b.ContactEmail == "" {
		return fmt.Errorf("broker %s has no privacy email", b.ID)
	}
	view, _, err := LoadIdentityView(ctx, rt.Store, "")
	if err != nil {
		return err
	}
	res, err := d.Send(ctx, mailer.Demand{
		BrokerName:   b.Name,
		BrokerDomain: b.Domain,
		To:           b.ContactEmail,
		TrackingID:   a.TrackingToken,
		Identity:     view,
		Jurisdiction: rt.Cfg.Jurisdiction,
	})
	if err != nil {
		_ = rt.Store.UpdateActionState(ctx, a.ID, string(broker.StateFailed), 1, err.Error(), nil)
		rt.Audit(ctx, "smtp.failed", b.ID, map[string]any{"action_id": a.ID, "err": err.Error()})
		return err
	}
	payload, _ := json.Marshal(map[string]any{"to": res.To, "subject": res.Subject, "sent_at": res.SentAt})
	enc, _ := rt.Store.EncryptActionResponse(a.ID, payload)
	next := string(broker.StateSubmitted)
	if b.RequiresEmailConfirmation {
		next = string(broker.StateAwaitingConfirmation)
	}
	if err := rt.Store.UpdateActionState(ctx, a.ID, next, 1, "", enc); err != nil {
		return err
	}
	rt.Audit(ctx, "smtp.sent", b.ID, map[string]any{"action_id": a.ID, "tracking": a.TrackingToken})
	return nil
}

func (rt *Runtime) HandleIMAP(ctx context.Context, f imapx.Finding) error {
	rt.Audit(ctx, "imap.mail", f.BrokerID, map[string]any{
		"from":     f.From,
		"subject":  f.Subject,
		"urls":     f.URLs,
		"tracking": f.TrackingID,
	})
	var actionID string
	if f.TrackingID != "" {
		if a, err := rt.Store.GetActionByTracking(ctx, f.TrackingID); err == nil {
			actionID = a.ID
		}
	}
	if actionID == "" && f.BrokerID != "" {
		if a, err := rt.Store.LatestAwaitingByBroker(ctx, f.BrokerID); err == nil {
			actionID = a.ID
		}
	}
	if actionID != "" {
		payload, _ := json.Marshal(f)
		enc, _ := rt.Store.EncryptActionResponse(actionID, payload)
		state := string(broker.StateAwaitingConfirmation)
		if len(f.URLs) == 0 {
			state = string(broker.StateSubmitted)
		}
		_ = rt.Store.UpdateActionState(ctx, actionID, state, 0, "", enc)
	}

	if !rt.Cfg.IMAP.AutoFetch {
		return nil
	}
	for _, u := range f.URLs {
		if !imapx.LooksLikeConfirmation(u) {
			continue
		}
		res, err := imapx.FetchConfirmation(ctx, rt.HTTP, u)
		if err != nil {
			rt.Log.Warn("confirmation fetch failed", "url_host", hostOf(u), "err", err)
			continue
		}
		rt.Audit(ctx, "imap.confirm_fetch", f.BrokerID, map[string]any{
			"status":      res.Status,
			"success":     res.Success,
			"needs_human": res.NeedsHuman,
			"indicator":   res.Indicator,
		})
		if res.NeedsHuman {
			if actionID != "" {
				_ = rt.Store.UpdateActionState(ctx, actionID, string(broker.StateAwaitingManual), 0, "challenge page; finish in a browser", nil)
			}
			continue
		}
		if res.Success && actionID != "" {
			_ = rt.Store.UpdateActionState(ctx, actionID, string(broker.StateCompleted), 0, "", nil)
		}
	}
	return nil
}

func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "redacted"
	}
	return u.Host
}
