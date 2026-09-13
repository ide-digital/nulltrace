package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/4x3/nulltrace/internal/broker"
	"github.com/4x3/nulltrace/internal/engine/recon"
	"github.com/4x3/nulltrace/internal/storage"
	"github.com/4x3/nulltrace/pkg/ipc"
)

func (rt *Runtime) Scan(ctx context.Context, enableHIBP bool) (recon.Report, ipc.ScanResult, error) {
	if rt.Store.Locked() {
		return recon.Report{}, ipc.ScanResult{}, fmt.Errorf("vault locked")
	}
	view, identID, err := LoadIdentityView(ctx, rt.Store, "")
	if err != nil {
		return recon.Report{}, ipc.ScanResult{}, err
	}
	var hibpKey []byte
	if enableHIBP && rt.Cfg.HIBP.Enabled {
		hibpKey, err = rt.Store.GetSecret(ctx, SecretHIBPKey)
		if err != nil {
			return recon.Report{}, ipc.ScanResult{}, err
		}
	}
	coord := recon.New(rt.Reg, rt.Cfg.HIBP.APIBase, hibpKey, enableHIBP && len(hibpKey) > 0)
	if rt.HTTP != nil {
		coord.HTTP = rt.HTTP
	}
	rep, err := coord.Scan(ctx, view)
	if err != nil {
		return rep, ipc.ScanResult{}, err
	}

	persisted := 0
	var hibpBits []recon.Finding
	maxHIBP := ""
	maxConf := 0.0
	for _, f := range rep.Findings {
		if strings.HasPrefix(f.BrokerID, "hibp-") {
			hibpBits = append(hibpBits, f)
			if f.Confidence > maxConf {
				maxConf = f.Confidence
				maxHIBP = f.RiskTier
			}
			continue
		}
		raw, _ := json.Marshal(f)
		rec := storage.ExposedRecordRow{
			IdentityID: identID,
			BrokerID:   f.BrokerID,
			ProfileURL: f.ProfileURL,
			Confidence: f.Confidence,
			Status:     string(broker.StatusDiscovered),
			RiskTier:   f.RiskTier,
		}
		if _, err := rt.Store.SaveFinding(ctx, rec, raw); err != nil {
			return rep, ipc.ScanResult{}, fmt.Errorf("persist finding %s: %w", f.BrokerID, err)
		}
		persisted++
	}
	if len(hibpBits) > 0 {
		raw, _ := json.Marshal(map[string]any{
			"breaches": rep.Breaches,
			"findings": hibpBits,
		})
		if maxHIBP == "" {
			maxHIBP = string(broker.RiskHigh)
		}
		rec := storage.ExposedRecordRow{
			IdentityID: identID,
			BrokerID:   "haveibeenpwned",
			ProfileURL: "https://haveibeenpwned.com",
			Confidence: maxConf,
			Status:     string(broker.StatusDiscovered),
			RiskTier:   maxHIBP,
		}
		if _, err := rt.Store.SaveFinding(ctx, rec, raw); err != nil {
			return rep, ipc.ScanResult{}, fmt.Errorf("persist hibp: %w", err)
		}
		persisted++
	}
	rt.Audit(ctx, "scan.completed", "", map[string]any{
		"findings":  len(rep.Findings),
		"breaches":  len(rep.Breaches),
		"persisted": persisted,
		"hibp":      enableHIBP && len(hibpKey) > 0,
		"identity":  identID,
	})
	return rep, ipc.ScanResult{
		FindingCount: len(rep.Findings),
		BreachCount:  len(rep.Breaches),
		Persisted:    persisted,
	}, nil
}
