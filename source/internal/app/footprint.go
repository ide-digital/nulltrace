package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/4x3/nulltrace/internal/broker"
	"github.com/4x3/nulltrace/internal/engine/footprint"
	"github.com/4x3/nulltrace/internal/nterr"
	"github.com/4x3/nulltrace/internal/storage"
	"github.com/4x3/nulltrace/pkg/ipc"
	"github.com/4x3/nulltrace/pkg/opsec"
)

func (rt *Runtime) Footprint(ctx context.Context, username string, persist bool) (ipc.FootprintResult, error) {
	if !rt.Cfg.Footprint.Enabled {
		return ipc.FootprintResult{}, nterr.ErrFootprintDisabled
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return ipc.FootprintResult{}, fmt.Errorf("username is empty")
	}

	client := rt.HTTP
	if client == nil {
		c, err := opsec.HTTPClient(rt.Cfg.Proxy, time.Duration(rt.Cfg.Footprint.TimeoutSeconds)*time.Second)
		if err != nil {
			return ipc.FootprintResult{}, err
		}
		client = c
	}

	results := footprint.Probe(ctx, username, footprint.Sites(), client, rt.Cfg.Footprint.MaxConcurrent)

	out := ipc.FootprintResult{Username: username, Results: make([]ipc.FootprintSite, 0, len(results))}
	for _, r := range results {
		out.Results = append(out.Results, ipc.FootprintSite{
			Name:   r.Name,
			URL:    r.URL,
			Status: string(r.Status),
			Detail: r.Detail,
		})
		if r.Status == footprint.StatusFound {
			out.Found++
		}
	}
	out.Checked = len(results)

	if persist && !rt.Store.Locked() {
		if err := rt.persistFootprint(ctx, username, results); err != nil {
			return out, err
		}
	}
	rt.Audit(ctx, "footprint.run", "", map[string]any{
		"username": username, "checked": out.Checked, "found": out.Found,
	})
	return out, nil
}

func (rt *Runtime) persistFootprint(ctx context.Context, username string, results []footprint.Result) error {
	row, err := rt.Store.PrimaryIdentity(ctx)
	if err != nil {
		return err
	}
	for _, r := range results {
		if r.Status != footprint.StatusFound {
			continue
		}
		slug := slugify(r.Name)
		raw, _ := json.Marshal(map[string]any{
			"username": username, "site": r.Name, "url": r.URL,
		})
		rec := storage.ExposedRecordRow{
			IdentityID: row.ID,
			BrokerID:   "footprint-" + slug,
			ProfileURL: r.URL,
			Confidence: 0.55,
			Status:     string(broker.StatusDiscovered),
			RiskTier:   string(broker.RiskMedium),
		}
		if _, err := rt.Store.SaveFinding(ctx, rec, raw); err != nil {
			return fmt.Errorf("persist footprint %s: %w", r.Name, err)
		}
	}
	return nil
}

func slugify(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
