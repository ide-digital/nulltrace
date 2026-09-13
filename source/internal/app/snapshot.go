package app

import (
	"context"
	"strings"
	"time"

	"github.com/4x3/nulltrace/internal/engine/playbook"
	"github.com/4x3/nulltrace/pkg/ipc"
)

func (rt *Runtime) Snapshot(ctx context.Context) (ipc.Snapshot, error) {
	var snap ipc.Snapshot
	st, err := rt.Store.Stats(ctx)
	if err != nil {
		return snap, err
	}
	snap.Status = ipc.StatusPayload{
		Unlocked:        !rt.Store.Locked(),
		ListenAddr:      rt.Cfg.ListenAddr,
		BrokerCount:     st.BrokerCount,
		ActiveRemovals:  st.ActiveRemovals,
		VerifiedRemoved: st.VerifiedRemoved,
		ExposedRecords:  st.ExposedRecords,
		FailedActions:   st.FailedActions,
		AwaitingConfirm: st.AwaitingConfirm,
		ManualPending:   st.ManualPending,
		Overdue:         st.Overdue,
	}
	if rt.Store.Locked() {
		return snap, nil
	}
	view, id, err := LoadIdentityView(ctx, rt.Store, "")
	if err == nil {
		snap.Identity = ipc.IdentityView{
			ID:        id,
			First:     view.First,
			Middle:    view.Middle,
			Last:      view.Last,
			DOB:       view.DOB,
			Emails:    view.Emails,
			Phones:    view.Phones,
			Addresses: view.Addresses,
			Cities:    view.Cities,
			Usernames: view.Usernames,
		}
		snap.Status.IdentityName = view.FullName()
	}

	acts, err := rt.Store.ListActions(ctx)
	if err != nil {
		return snap, err
	}
	byRecord := map[string]ipc.ActionView{}
	for _, a := range acts {
		name := a.BrokerID
		if b, err := rt.Store.GetBroker(ctx, a.BrokerID); err == nil {
			name = b.Name
		}
		deadline := ""
		if a.StatutoryDeadline != nil {
			deadline = a.StatutoryDeadline.UTC().Format(time.RFC3339)
		}
		av := ipc.ActionView{
			ID:         a.ID,
			RecordID:   a.ExposedRecordID,
			BrokerID:   a.BrokerID,
			BrokerName: name,
			Strategy:   a.Strategy,
			State:      a.CurrentState,
			Attempts:   a.Attempts,
			Tracking:   a.TrackingToken,
			LastError:  a.LastError,
			Deadline:   deadline,
			Updated:    a.UpdatedAt.UTC().Format(time.RFC3339),
		}
		snap.Actions = append(snap.Actions, av)
		byRecord[a.ExposedRecordID] = av
	}

	recs, err := rt.Store.ListExposed(ctx)
	if err != nil {
		return snap, err
	}
	for _, r := range recs {
		ev := ipc.ExposedView{
			ID:         r.ID,
			BrokerID:   r.BrokerID,
			BrokerName: r.BrokerID,
			Status:     r.Status,
			RiskTier:   r.RiskTier,
			Confidence: r.Confidence,
			ProfileURL: r.ProfileURL,
			Detected:   r.FirstDetectedAt.UTC().Format(time.RFC3339),
		}
		if !r.LastVerifiedAt.IsZero() {
			ev.LastVerified = r.LastVerifiedAt.UTC().Format(time.RFC3339)
		}
		if b, err := rt.Store.GetBroker(ctx, r.BrokerID); err == nil {
			ev.BrokerName = b.Name
			ev.Domain = b.Domain
			ev.Category = b.Category
			ev.Mechanism = b.Mechanism
			ev.OptOutURL = b.OptOutURL
			ev.ContactEmail = b.ContactEmail
			ev.Notes = strings.TrimSpace(b.Notes)
		}
		if ai, ok := byRecord[r.ID]; ok {
			ev.ActionState = ai.State
			ev.ActionStrategy = ai.Strategy
		}
		snap.Exposed = append(snap.Exposed, ev)
	}

	audit, err := rt.Store.ListAudit(ctx, 80)
	if err != nil {
		return snap, err
	}
	for _, r := range audit {
		details, _ := rt.Store.DecryptAudit(r)
		if len(details) > 240 {
			details = details[:240]
		}
		snap.Audit = append(snap.Audit, ipc.AuditView{
			Timestamp: r.Timestamp.UTC().Format(time.RFC3339),
			EventType: r.EventType,
			BrokerID:  r.BrokerID,
			Details:   string(details),
		})
	}
	for _, b := range rt.Reg.All() {
		if b.RequiresHuman() {
			snap.Playbooks = append(snap.Playbooks, playbook.Render(playbook.For(b)))
		}
	}
	return snap, nil
}

func (rt *Runtime) Status(ctx context.Context) (ipc.StatusPayload, error) {
	snap, err := rt.Snapshot(ctx)
	if err != nil {
		return ipc.StatusPayload{}, err
	}
	return snap.Status, nil
}
