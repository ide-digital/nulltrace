package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/4x3/nulltrace/internal/broker"
	"github.com/4x3/nulltrace/internal/config"
	ncrypto "github.com/4x3/nulltrace/internal/crypto"
	"github.com/4x3/nulltrace/internal/engine/mailer"
	"github.com/4x3/nulltrace/internal/nterr"
	"github.com/4x3/nulltrace/internal/paths"
	"github.com/4x3/nulltrace/internal/storage"
	"github.com/4x3/nulltrace/pkg/ipc"
	"github.com/4x3/nulltrace/pkg/opsec"
)

const (
	SecretSMTPPassword = "smtp.password"
	SecretIMAPPassword = "imap.password"
	SecretHIBPKey      = "hibp.api_key"
	SecretCapSolverKey = "capsolver.api_key"
)

type Runtime struct {
	Dirs    paths.Dirs
	Cfg     config.File
	DB      *sql.DB
	Store   *storage.Store
	Reg     *broker.Registry
	Log     *slog.Logger
	HTTP    *http.Client
	logFile *os.File
}

func Open(ctx context.Context, passphrase []byte, jsonLog bool) (*Runtime, error) {
	dirs, err := paths.Resolve()
	if err != nil {
		return nil, err
	}
	if err := dirs.Ensure(); err != nil {
		return nil, err
	}
	cfg, err := config.Load(dirs.ConfigFile)
	if err != nil {
		return nil, err
	}
	reg, err := broker.Default()
	if err != nil {
		return nil, fmt.Errorf("load broker registry: %w", err)
	}
	db, err := storage.Open(ctx, dirs.VaultDB)
	if err != nil {
		return nil, err
	}
	if err := storage.Migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	st := storage.New(db)
	httpClient, err := opsec.HTTPClient(cfg.Proxy, 20*time.Second)
	if err != nil {
		_ = st.Close()
		return nil, err
	}

	rt := &Runtime{
		Dirs:  dirs,
		Cfg:   cfg,
		DB:    db,
		Store: st,
		Reg:   reg,
		HTTP:  httpClient,
	}
	rt.Log = slog.Default()
	if jsonLog {
		f, err := os.OpenFile(dirs.DaemonLog, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			_ = st.Close()
			return nil, fmt.Errorf("open daemon log: %w", err)
		}
		rt.logFile = f
		rt.Log = slog.New(slog.NewJSONHandler(io.MultiWriter(os.Stderr, f), &slog.HandlerOptions{Level: slog.LevelInfo}))
	}

	if err := SeedBrokers(ctx, st, reg); err != nil {
		rt.Close()
		return nil, err
	}

	if len(passphrase) > 0 {
		ok, err := st.Initialized(ctx)
		if err != nil {
			rt.Close()
			return nil, err
		}
		if !ok {
			rt.Close()
			return nil, nterr.ErrNotInitialized
		}
		if err := st.Unlock(ctx, passphrase); err != nil {
			rt.Close()
			return nil, err
		}
	}
	return rt, nil
}

func (rt *Runtime) Close() {
	if rt == nil {
		return
	}
	if rt.Store != nil {
		rt.Store.Lock()
		_ = rt.Store.Close()
	}
	if rt.logFile != nil {
		_ = rt.logFile.Close()
	}
}

func SeedBrokers(ctx context.Context, st *storage.Store, reg *broker.Registry) error {
	if reg == nil {
		return fmt.Errorf("nil registry")
	}
	for _, b := range reg.All() {
		if err := st.UpsertBroker(ctx, toBrokerRow(b)); err != nil {
			return err
		}
	}
	return nil
}

func toBrokerRow(b broker.Broker) storage.BrokerRow {
	return storage.BrokerRow{
		ID:                        b.ID,
		Name:                      b.Name,
		Domain:                    b.Domain,
		Category:                  string(b.Category),
		Mechanism:                 string(b.Mechanism),
		OptOutURL:                 b.OptOutURL,
		ContactEmail:              b.ContactEmail,
		RequiresCaptcha:           b.RequiresCaptcha,
		RequiresEmailConfirmation: b.RequiresEmailConfirmation,
		JurisdictionCoverage:      string(b.JurisdictionCoverage),
		RepopulationPeriodDays:    b.RepopulationPeriodDays,
		Notes:                     b.Notes,
		Playbook:                  b.Playbook,
	}
}

func brokerFromRow(r storage.BrokerRow) broker.Broker {
	return broker.Broker{
		ID:                        r.ID,
		Name:                      r.Name,
		Domain:                    r.Domain,
		Category:                  broker.ParseCategory(r.Category),
		Mechanism:                 broker.ParseMechanism(r.Mechanism),
		OptOutURL:                 r.OptOutURL,
		ContactEmail:              r.ContactEmail,
		RequiresCaptcha:           r.RequiresCaptcha,
		RequiresEmailConfirmation: r.RequiresEmailConfirmation,
		JurisdictionCoverage:      broker.ParseJurisdiction(r.JurisdictionCoverage),
		RepopulationPeriodDays:    r.RepopulationPeriodDays,
		Notes:                     r.Notes,
		Playbook:                  r.Playbook,
	}
}

func LoadIdentityView(ctx context.Context, st *storage.Store, identityID string) (mailer.IdentityView, string, error) {
	var row storage.IdentityRow
	var err error
	if identityID == "" {
		row, err = st.PrimaryIdentity(ctx)
	} else {
		row, err = st.GetIdentity(ctx, identityID)
	}
	if err != nil {
		return mailer.IdentityView{}, "", err
	}
	first, last, middle, dob, err := st.DecryptIdentity(row)
	if err != nil {
		return mailer.IdentityView{}, "", err
	}
	defer ncrypto.Zeroize(first)
	defer ncrypto.Zeroize(last)
	defer ncrypto.Zeroize(middle)
	defer ncrypto.Zeroize(dob)

	v := mailer.IdentityView{
		First:  string(first),
		Last:   string(last),
		Middle: string(middle),
		DOB:    string(dob),
	}
	attrs, err := st.ListAttributes(ctx, row.ID)
	if err != nil {
		return mailer.IdentityView{}, "", err
	}
	for _, a := range attrs {
		val, err := st.DecryptAttribute(a)
		if err != nil {
			return mailer.IdentityView{}, "", err
		}
		s := strings.TrimSpace(string(val))
		ncrypto.Zeroize(val)
		if s == "" {
			continue
		}
		switch broker.AttributeType(strings.ToUpper(a.Type)) {
		case broker.AttrEmail:
			v.Emails = append(v.Emails, s)
		case broker.AttrPhone:
			v.Phones = append(v.Phones, s)
		case broker.AttrAddress:
			v.Addresses = append(v.Addresses, s)
		case broker.AttrCity:
			v.Cities = append(v.Cities, s)
		case broker.AttrUsername:
			v.Usernames = append(v.Usernames, s)
		case broker.AttrAlias:
			v.Aliases = append(v.Aliases, s)
			v.Usernames = append(v.Usernames, s)
		case broker.AttrRelative:
			v.Relatives = append(v.Relatives, s)
		default:
			v.Aliases = append(v.Aliases, s)
		}
	}
	return v, row.ID, nil
}

func (rt *Runtime) AddIdentity(ctx context.Context, req ipc.IdentityAddRequest) (ipc.IdentityAddResult, error) {
	id, err := rt.Store.InsertIdentity(ctx, []byte(req.First), []byte(req.Last), []byte(req.Middle), []byte(req.DOB))
	if err != nil {
		return ipc.IdentityAddResult{}, err
	}
	for i, e := range req.Emails {
		if _, err := rt.Store.AddAttribute(ctx, id, string(broker.AttrEmail), []byte(e), i == 0); err != nil {
			return ipc.IdentityAddResult{}, err
		}
	}
	for _, p := range req.Phones {
		if _, err := rt.Store.AddAttribute(ctx, id, string(broker.AttrPhone), []byte(p), false); err != nil {
			return ipc.IdentityAddResult{}, err
		}
	}
	for _, c := range req.Cities {
		if _, err := rt.Store.AddAttribute(ctx, id, string(broker.AttrCity), []byte(c), false); err != nil {
			return ipc.IdentityAddResult{}, err
		}
	}
	rt.Audit(ctx, "identity.created", "", map[string]any{"id": id})
	return ipc.IdentityAddResult{ID: id}, nil
}

func (rt *Runtime) AddAttribute(ctx context.Context, req ipc.AttrAddRequest) error {
	ident := req.IdentityID
	if ident == "" {
		row, err := rt.Store.PrimaryIdentity(ctx)
		if err != nil {
			return err
		}
		ident = row.ID
	}
	_, err := rt.Store.AddAttribute(ctx, ident, req.Type, []byte(req.Value), req.Primary)
	return err
}

func (rt *Runtime) Audit(ctx context.Context, event, brokerID string, payload any) {
	if rt == nil || rt.Store == nil || rt.Store.Locked() {
		return
	}
	var raw []byte
	switch v := payload.(type) {
	case nil:
		raw = []byte("{}")
	case string:
		raw = []byte(v)
	case []byte:
		raw = v
	default:
		raw, _ = json.Marshal(v)
	}
	if err := rt.Store.AppendAudit(ctx, event, brokerID, raw); err != nil && rt.Log != nil {
		rt.Log.Warn("audit append failed", "event", event, "err", err)
	}
}
