package daemon

import (
	"context"
	"time"

	"github.com/4x3/nulltrace/internal/app"
	"github.com/4x3/nulltrace/internal/broker"
)

func RunScheduler(ctx context.Context, rt *app.Runtime, pool *Pool) {
	enqueue := time.NewTicker(30 * time.Second)
	watch := time.NewTicker(1 * time.Hour)
	defer enqueue.Stop()
	defer watch.Stop()

	drainQueued(ctx, rt, pool)
	if n, err := rt.Watchdog(ctx); err != nil {
		rt.Log.Warn("watchdog", "err", err)
	} else if n > 0 {
		rt.Log.Info("watchdog marked pending_review", "count", n)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-enqueue.C:
			drainQueued(ctx, rt, pool)
		case <-watch.C:
			n, err := rt.Watchdog(ctx)
			if err != nil {
				rt.Log.Warn("watchdog", "err", err)
				continue
			}
			if n > 0 {
				rt.Log.Info("watchdog marked pending_review", "count", n)
			}
		}
	}
}

func drainQueued(ctx context.Context, rt *app.Runtime, pool *Pool) {
	if rt.Store.Locked() {
		return
	}
	acts, err := rt.Store.ActionsByState(ctx, string(broker.StateQueued))
	if err != nil {
		rt.Log.Warn("list queued actions", "err", err)
		return
	}
	for _, a := range acts {
		if a.Strategy != string(broker.StrategySMTP) {
			continue
		}
		pool.Submit(ctx, Job{ActionID: a.ID})
	}
}
