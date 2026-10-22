package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"

	"github.com/ide-digital/nulltrace/internal/broker"
	ncrypto "github.com/ide-digital/nulltrace/internal/crypto"
	imapx "github.com/ide-digital/nulltrace/internal/engine/imap"
	"github.com/ide-digital/nulltrace/internal/engine/mailer"
	"github.com/ide-digital/nulltrace/internal/nterr"
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
		e := rt.dispatchSMTP(ctx, d, a.ID)
		if errors.Is(e, nterr.ErrAlreadyClaimed) {
			continue
		}
		if e != nil {
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
	if a.CurrentState != string(broker.StateQueued) {
		return nterr.ErrAlreadyClaimed
	}
	if !d.Configured() {
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
	claimed, err := rt.Store.ClaimAction(ctx, a.ID, string(broker.StateQueued), string(broker.StateSending))
	if err != nil {
		return err
	}
	if !claimed {
		return nterr.ErrAlreadyClaimed
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
	// The mail already went out. Losing the receipt beats re-sending it.
	enc, err := rt.Store.EncryptActionResponse(a.ID, payload)
	if err != nil {
		rt.Log.Warn("could not store send receipt", "action", a.ID, "err", err)
	}
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
		enc, err := rt.Store.EncryptActionResponse(actionID, payload)
		if err != nil {
			rt.Log.Warn("could not store imap mail", "action", actionID, "err", err)
		}
		state := string(broker.StateAwaitingConfirmation)
		if len(f.URLs) == 0 {
			state = string(broker.StateSubmitted)
		}
		rt.setState(ctx, actionID, state, "", enc)
	}

	if !rt.Cfg.IMAP.AutoFetch {
		return nil
	}
	sameBroker := func(host string) bool {
		b, ok := rt.Reg.MatchHost(host)
		return ok && (f.BrokerID == "" || b.ID == f.BrokerID)
	}
	for _, u := range f.URLs {
		if !imapx.LooksLikeConfirmation(u) {
			continue
		}
		res, err := imapx.FetchConfirmation(ctx, rt.HTTP, u, sameBroker)
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
				rt.setState(ctx, actionID, string(broker.StateAwaitingManual), "challenge page; finish in a browser", nil)
			}
			continue
		}
		if res.Success && actionID != "" {
			rt.setState(ctx, actionID, string(broker.StateCompleted), "", nil)
		}
	}
	return nil
}

// setState is for the IMAP side, where there is nobody to hand an error back to.
func (rt *Runtime) setState(ctx context.Context, actionID, state, note string, resp []byte) {
	if err := rt.Store.UpdateActionState(ctx, actionID, state, 0, note, resp); err != nil {
		rt.Log.Warn("update action", "action", actionID, "state", state, "err", err)
	}
}

func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "redacted"
	}
	return u.Host
}
