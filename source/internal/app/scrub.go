package app

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/4x3/nulltrace/internal/broker"
	"github.com/4x3/nulltrace/internal/storage"
	"github.com/4x3/nulltrace/pkg/ipc"
)

func (rt *Runtime) Scrub(ctx context.Context) (ipc.ScrubResult, error) {
	if rt.Store.Locked() {
		return ipc.ScrubResult{}, fmt.Errorf("vault locked")
	}
	recs, err := rt.Store.ListExposed(ctx)
	if err != nil {
		return ipc.ScrubResult{}, err
	}
	var res ipc.ScrubResult
	deadline := time.Now().UTC().AddDate(0, 0, 45)
	for _, rec := range recs {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		switch rec.Status {
		case string(broker.StatusIgnored), string(broker.StatusVerifiedRemoved):
			res.Skipped++
			continue
		}
		if rec.BrokerID == "haveibeenpwned" {
			res.Skipped++
			continue
		}
		row, err := rt.Store.GetBroker(ctx, rec.BrokerID)
		if err != nil {
			res.Skipped++
			continue
		}
		b := brokerFromRow(row)
		if b.ContactEmail != "" {
			n, err := rt.queueAction(ctx, rec, b, broker.StrategySMTP, string(broker.StateQueued), &deadline)
			if err != nil {
				return res, err
			}
			if n {
				res.QueuedSMTP++
			} else {
				res.Skipped++
			}
		}
		if b.RequiresHuman() {
			n, err := rt.queueAction(ctx, rec, b, broker.StrategyManual, string(broker.StateAwaitingManual), &deadline)
			if err != nil {
				return res, err
			}
			if n {
				res.QueuedManual++
			} else {
				res.Skipped++
			}
		}
		if b.ContactEmail == "" && !b.RequiresHuman() {
			res.Skipped++
		}
	}
	rt.Audit(ctx, "scrub.queued", "", map[string]any{
		"smtp":   res.QueuedSMTP,
		"manual": res.QueuedManual,
		"skip":   res.Skipped,
	})
	return res, nil
}

func (rt *Runtime) queueAction(ctx context.Context, rec storage.ExposedRecordRow, b broker.Broker, strategy broker.Strategy, state string, deadline *time.Time) (bool, error) {
	open, err := rt.Store.HasOpenAction(ctx, rec.ID, string(strategy))
	if err != nil {
		return false, err
	}
	if open {
		return false, nil
	}
	id := uuid.NewString()
	a := storage.ActionRow{
		ID:                id,
		ExposedRecordID:   rec.ID,
		BrokerID:          b.ID,
		Strategy:          string(strategy),
		CurrentState:      state,
		StatutoryDeadline: deadline,
	}
	if _, err := rt.Store.InsertAction(ctx, a); err != nil {
		return false, fmt.Errorf("queue %s for %s: %w", strategy, b.ID, err)
	}
	if rec.Status == string(broker.StatusDiscovered) || rec.Status == string(broker.StatusPendingReview) {
		_ = rt.Store.SetExposedStatus(ctx, rec.ID, string(broker.StatusApproved))
	}
	rt.Audit(ctx, "action.queued", b.ID, map[string]any{
		"action_id": id,
		"strategy":  strategy,
		"state":     state,
		"record_id": rec.ID,
	})
	return true, nil
}

func (rt *Runtime) CompleteAction(ctx context.Context, actionID string) error {
	a, err := rt.Store.GetAction(ctx, actionID)
	if err != nil {
		return err
	}
	if err := rt.Store.UpdateActionState(ctx, a.ID, string(broker.StateCompleted), 0, "", nil); err != nil {
		return err
	}
	rt.Audit(ctx, "action.completed", a.BrokerID, map[string]any{"action_id": a.ID})
	return nil
}

func (rt *Runtime) VerifyRemoved(ctx context.Context, recordID string) error {
	if err := rt.Store.SetExposedVerified(ctx, recordID); err != nil {
		return err
	}
	rt.Audit(ctx, "record.verified_removed", "", map[string]any{"record_id": recordID})
	return nil
}

func (rt *Runtime) Watchdog(ctx context.Context) (int, error) {
	days := rt.Cfg.Scheduler.RepopulationDays
	if days <= 0 {
		days = 45
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -days)
	stale, err := rt.Store.StaleVerified(ctx, cutoff)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, rec := range stale {
		if err := rt.Store.SetExposedStatus(ctx, rec.ID, string(broker.StatusPendingReview)); err != nil {
			return n, err
		}
		rt.Audit(ctx, "watchdog.pending_review", rec.BrokerID, map[string]any{
			"record_id": rec.ID,
			"reason":    "verified_removed older than watchdog window",
		})
		n++
	}
	// Also honour per-broker repopulation on completed actions.
	past, err := rt.Store.CompletedPastRepopulation(ctx, time.Now().UTC())
	if err != nil {
		return n, err
	}
	for _, a := range past {
		if err := rt.Store.SetExposedStatus(ctx, a.ExposedRecordID, string(broker.StatusPendingReview)); err != nil {
			return n, err
		}
		rt.Audit(ctx, "watchdog.pending_review", a.BrokerID, map[string]any{
			"record_id": a.ExposedRecordID,
			"action_id": a.ID,
			"reason":    "broker repopulation window elapsed",
		})
		n++
	}
	return n, nil
}
