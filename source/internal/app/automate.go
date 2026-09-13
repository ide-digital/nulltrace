package app

import (
	"context"
	"fmt"
	"time"

	ncrypto "github.com/4x3/nulltrace/internal/crypto"
	"github.com/4x3/nulltrace/internal/engine/automate"
	"github.com/4x3/nulltrace/internal/engine/browser"
	"github.com/4x3/nulltrace/internal/engine/captcha"
	"github.com/4x3/nulltrace/internal/nterr"
	"github.com/4x3/nulltrace/pkg/ipc"
)

func (rt *Runtime) Automate(ctx context.Context, brokerID string, headful bool) (ipc.AutomateResult, error) {
	if !rt.Cfg.Browser.Enabled {
		return ipc.AutomateResult{}, nterr.ErrBrowserDisabled
	}
	b, ok := rt.Reg.Get(brokerID)
	if !ok {
		return ipc.AutomateResult{}, nterr.ErrBrokerNotFound
	}
	if b.OptOutURL == "" {
		return ipc.AutomateResult{}, fmt.Errorf("broker %s has no opt-out URL", b.ID)
	}
	headless := rt.Cfg.Browser.Headless && !headful

	var solver *captcha.Client
	if rt.Cfg.CapSolver.Enabled {
		key, err := rt.Store.GetSecret(ctx, SecretCapSolverKey)
		if err != nil {
			return ipc.AutomateResult{}, err
		}
		if len(key) > 0 {
			solver = captcha.New(string(key), rt.Cfg.CapSolver.APIBase, rt.HTTP)
		}
		ncrypto.Zeroize(key)
	}

	eng, err := browser.Launch(browser.Options{
		Headless: headless,
		Stealth:  rt.Cfg.Browser.Stealth,
		Bin:      rt.Cfg.Browser.Bin,
		Proxy:    rt.Cfg.Proxy,
		Timeout:  time.Duration(rt.Cfg.Browser.TimeoutSeconds) * time.Second,
	})
	if err != nil {
		return ipc.AutomateResult{}, err
	}
	defer eng.Close()

	view, _, err := LoadIdentityView(ctx, rt.Store, "")
	if err != nil {
		return ipc.AutomateResult{}, err
	}

	spec := automate.Spec{URL: b.OptOutURL, Selectors: automate.DefaultSelectors()}
	run := &automate.Engine{Browser: eng, Solver: solver}
	res, err := run.Run(ctx, spec, view)
	if err != nil {
		return ipc.AutomateResult{}, err
	}

	rt.Audit(ctx, "automate.optout", b.ID, map[string]any{
		"status": res.Status, "detail": res.Detail, "url": res.URL,
	})

	return ipc.AutomateResult{
		BrokerID:   b.ID,
		BrokerName: b.Name,
		Status:     res.Status,
		Detail:     res.Detail,
		URL:        res.URL,
	}, nil
}
