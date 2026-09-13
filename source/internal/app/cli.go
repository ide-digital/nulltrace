package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/4x3/nulltrace/internal/broker"
	"github.com/4x3/nulltrace/internal/config"
	ncrypto "github.com/4x3/nulltrace/internal/crypto"
	"github.com/4x3/nulltrace/internal/engine/playbook"
	"github.com/4x3/nulltrace/internal/nterr"
	"github.com/4x3/nulltrace/internal/paths"
	"github.com/4x3/nulltrace/internal/tui"
	"github.com/4x3/nulltrace/pkg/ipc"
	"github.com/4x3/nulltrace/pkg/opsec"
)

// PassphraseFile is --passphrase-file. Do not log it.
var PassphraseFile string

func NewRoot() *cobra.Command {
	root := &cobra.Command{
		Use:           "nulltrace",
		Version:       tui.Version,
		Short:         "Local-first privacy toolkit",
		Long:          "Maps your identity against a local data-broker catalog, queues CCPA/GDPR deletion mail, and checks password dumps. The vault stays on this machine. Browser and captcha helpers stay off until you enable them.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMenu(cmd.Context(), PassphraseFile)
		},
	}
	root.PersistentFlags().StringVar(&PassphraseFile, "passphrase-file", "", "read vault passphrase from a file")

	root.AddCommand(
		cmdVault(),
		cmdIdentity(),
		cmdSecret(),
		cmdScan(),
		cmdScrub(),
		cmdAudit(),
		cmdExport(),
		cmdAction(),
		cmdPlaybook(),
		cmdFootprint(),
		cmdAutomate(),
		cmdStatus(),
		cmdExif(),
		cmdConfig(),
	)
	return root
}

func cmdAudit() *cobra.Command {
	return &cobra.Command{
		Use:   "audit",
		Short: "Open the TUI",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAudit(cmd.Context(), PassphraseFile)
		},
	}
}

func runMenu(ctx context.Context, passFile string) error {
	if cl, err := tryClient(); err == nil {
		return runInteractive(ctx, nil, cl)
	}
	if passFile != "" {
		return withRT(ctx, passFile, func(rt *Runtime) error {
			return runInteractive(ctx, rt, nil)
		})
	}

	inited, err := tui.RunBoot(ctx, func() (bool, error) {
		return vaultInitialized(ctx)
	})
	if err != nil {
		return err
	}

	if !inited {
		res, err := tui.RunLogin(ctx, true, "")
		if err != nil {
			return err
		}
		if res.Canceled {
			return nil
		}
		defer ncrypto.Zeroize(res.Password)
		return runSetupThenMenu(ctx, res.Password)
	}

	flash := ""
	for {
		res, err := tui.RunLogin(ctx, false, flash)
		if err != nil {
			return err
		}
		if res.Canceled {
			return nil
		}
		rt, err := Open(ctx, res.Password, false)
		ncrypto.Zeroize(res.Password)
		if err != nil {
			if errors.Is(err, nterr.ErrInvalidPassphrase) || errors.Is(err, nterr.ErrEmptyPassphrase) {
				flash = "wrong password"
				continue
			}
			return err
		}
		defer rt.Close()
		return runInteractive(ctx, rt, nil)
	}
}

func runAudit(ctx context.Context, passFile string) error {
	if c, err := tryClient(); err == nil {
		return tui.Run(ctx, func(ctx context.Context) (ipc.Snapshot, error) {
			rep, err := c.Call(ctx, ipc.CmdSnapshot, nil)
			if err != nil {
				return ipc.Snapshot{}, err
			}
			return ipc.UnmarshalPayload[ipc.Snapshot](rep)
		})
	}
	return withRT(ctx, passFile, func(rt *Runtime) error {
		return tui.Run(ctx, rt.Snapshot)
	})
}

func cmdVault() *cobra.Command {
	c := &cobra.Command{Use: "vault", Short: "Create or check the encrypted vault"}
	c.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "Create the vault and seed brokers",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			rt, err := Open(ctx, nil, false)
			if err != nil {
				return err
			}
			defer rt.Close()
			ok, err := rt.Store.Initialized(ctx)
			if err != nil {
				return err
			}
			if ok {
				return nterr.ErrVaultExists
			}
			pw, err := ReadPassphrase(PassphraseFile, true)
			if err != nil {
				return err
			}
			defer ncrypto.Zeroize(pw)
			if err := rt.Store.InitVault(ctx, pw); err != nil {
				return err
			}
			if err := config.Save(rt.Dirs.ConfigFile, rt.Cfg); err != nil {
				return err
			}
			rt.Audit(ctx, "vault.initialized", "", map[string]any{"brokers": rt.Reg.Len()})
			fmt.Fprintf(cmd.OutOrStdout(), "vault initialized (%d brokers)\nconfig: %s\n", rt.Reg.Len(), rt.Dirs.ConfigFile)
			return nil
		},
	})
	c.AddCommand(&cobra.Command{
		Use:   "unlock",
		Short: "Check the passphrase",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withRT(cmd.Context(), PassphraseFile, func(rt *Runtime) error {
				fmt.Fprintln(cmd.OutOrStdout(), "ok")
				return nil
			})
		},
	})
	return c
}

func cmdIdentity() *cobra.Command {
	c := &cobra.Command{Use: "identity", Short: "Store the person you're scrubbing"}

	var first, last, middle, dob, email, phone, city string
	add := &cobra.Command{
		Use:   "add",
		Short: "Encrypt a name and optional contact fields",
		RunE: func(cmd *cobra.Command, args []string) error {
			if first == "" || last == "" {
				if err := huh.NewForm(huh.NewGroup(
					huh.NewInput().Title("First name").Value(&first),
					huh.NewInput().Title("Last name").Value(&last),
					huh.NewInput().Title("Middle name").Value(&middle),
					huh.NewInput().Title("Date of birth (optional)").Value(&dob),
					huh.NewInput().Title("Email").Value(&email),
					huh.NewInput().Title("Phone").Value(&phone),
					huh.NewInput().Title("City").Value(&city),
				)).Run(); err != nil {
					return err
				}
			}
			if first == "" || last == "" {
				return fmt.Errorf("first and last name are required")
			}
			req := ipc.IdentityAddRequest{First: first, Last: last, Middle: middle, DOB: dob}
			if email != "" {
				req.Emails = []string{email}
			}
			if phone != "" {
				req.Phones = []string{phone}
			}
			if city != "" {
				req.Cities = []string{city}
			}
			res, err := callOrLocal(cmd.Context(), ipc.CmdIdentityAdd, req, func(rt *Runtime) (ipc.IdentityAddResult, error) {
				return rt.AddIdentity(cmd.Context(), req)
			})
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), res.ID)
			return nil
		},
	}
	add.Flags().StringVar(&first, "first", "", "")
	add.Flags().StringVar(&last, "last", "", "")
	add.Flags().StringVar(&middle, "middle", "", "")
	add.Flags().StringVar(&dob, "dob", "", "")
	add.Flags().StringVar(&email, "email", "", "")
	add.Flags().StringVar(&phone, "phone", "", "")
	add.Flags().StringVar(&city, "city", "", "")
	c.AddCommand(add)

	var ident, typ, value string
	var primary bool
	attr := &cobra.Command{Use: "attr", Short: "Add EMAIL, PHONE, ADDRESS, CITY, USERNAME, ALIAS, RELATIVE"}
	attrAdd := &cobra.Command{
		Use:   "add",
		Short: "Encrypt and store one field",
		RunE: func(cmd *cobra.Command, args []string) error {
			if typ == "" || value == "" {
				return fmt.Errorf("--type and --value are required")
			}
			req := ipc.AttrAddRequest{IdentityID: ident, Type: typ, Value: value, Primary: primary}
			return doOrLocal(cmd.Context(), ipc.CmdAttrAdd, req, func(rt *Runtime) error {
				return rt.AddAttribute(cmd.Context(), req)
			})
		},
	}
	attrAdd.Flags().StringVar(&ident, "identity", "", "")
	attrAdd.Flags().StringVar(&typ, "type", "", "EMAIL|PHONE|ADDRESS|CITY|USERNAME|ALIAS|RELATIVE")
	attrAdd.Flags().StringVar(&value, "value", "", "")
	attrAdd.Flags().BoolVar(&primary, "primary", false, "")
	attr.AddCommand(attrAdd)
	c.AddCommand(attr)

	c.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "Show stored names",
		RunE: func(cmd *cobra.Command, args []string) error {
			snap, err := callOrLocal(cmd.Context(), ipc.CmdSnapshot, nil, func(rt *Runtime) (ipc.Snapshot, error) {
				return rt.Snapshot(cmd.Context())
			})
			if err != nil {
				return err
			}
			id := snap.Identity
			if id.ID == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "(none)")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s %s (%s)\n", id.First, id.Middle, id.Last, id.ID)
			return nil
		},
	})
	return c
}

func cmdSecret() *cobra.Command {
	c := &cobra.Command{Use: "secret", Short: "Keep SMTP/IMAP/HIBP/CapSolver keys in the vault"}
	c.AddCommand(&cobra.Command{
		Use:   "set <smtp.password|imap.password|hibp.api_key|capsolver.api_key>",
		Short: "Read a secret from stdin or NULLTRACE_SECRET",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			val := os.Getenv("NULLTRACE_SECRET")
			if val == "" {
				if st, _ := os.Stdin.Stat(); st != nil && st.Mode()&os.ModeCharDevice != 0 {
					fmt.Fprint(os.Stderr, "Secret: ")
				}
				pw, err := ReadPassphrase("", false)
				if err != nil {
					return err
				}
				val = string(pw)
				ncrypto.Zeroize(pw)
			}
			req := ipc.SecretSetRequest{Key: key, Value: val}
			if err := doOrLocal(cmd.Context(), ipc.CmdSecretSet, req, func(rt *Runtime) error {
				return rt.Store.PutSecret(cmd.Context(), key, []byte(val))
			}); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "stored %s\n", key)
			return nil
		},
	})
	return c
}

func cmdScan() *cobra.Command {
	var hibp bool
	c := &cobra.Command{
		Use:   "scan",
		Short: "Map vault data onto the broker list (optional HIBP)",
		RunE: func(cmd *cobra.Command, args []string) error {
			req := ipc.ScanRequest{EnableHIBP: hibp}
			res, err := callOrLocal(cmd.Context(), ipc.CmdScan, req, func(rt *Runtime) (ipc.ScanResult, error) {
				_, r, err := rt.Scan(cmd.Context(), hibp)
				return r, err
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "findings=%d breaches=%d persisted=%d\n", res.FindingCount, res.BreachCount, res.Persisted)
			return nil
		},
	}
	c.Flags().BoolVar(&hibp, "hibp", false, "query Have I Been Pwned (needs hibp.api_key)")
	return c
}

func cmdScrub() *cobra.Command {
	return &cobra.Command{
		Use:   "scrub",
		Short: "Queue deletion emails and web-form playbooks",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := callOrLocal(cmd.Context(), ipc.CmdScrub, ipc.ScrubRequest{All: true}, func(rt *Runtime) (ipc.ScrubResult, error) {
				return rt.Scrub(cmd.Context())
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "queued smtp=%d manual=%d skipped=%d\n", res.QueuedSMTP, res.QueuedManual, res.Skipped)
			return nil
		},
	}
}

func cmdExport() *cobra.Command {
	var format string
	c := &cobra.Command{
		Use:   "export",
		Short: "Write the audit ledger (md, json, csv)",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := callOrLocal(cmd.Context(), ipc.CmdExport, ipc.ExportRequest{Format: format}, func(rt *Runtime) (ipc.ExportResult, error) {
				return rt.Export(cmd.Context(), format)
			})
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), res.Path)
			return nil
		},
	}
	c.Flags().StringVar(&format, "format", "md", "md|json|csv")
	return c
}

func cmdAction() *cobra.Command {
	c := &cobra.Command{Use: "action", Short: "Mark opt-outs done"}
	c.AddCommand(&cobra.Command{
		Use:   "complete <action-id>",
		Args:  cobra.ExactArgs(1),
		Short: "Mark a queued action completed",
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			return doOrLocal(cmd.Context(), ipc.CmdActionComplete, ipc.ActionIDRequest{ActionID: id}, func(rt *Runtime) error {
				return rt.CompleteAction(cmd.Context(), id)
			})
		},
	})
	c.AddCommand(&cobra.Command{
		Use:   "verify <exposed-record-id>",
		Args:  cobra.ExactArgs(1),
		Short: "You confirmed the listing is gone",
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			return doOrLocal(cmd.Context(), ipc.CmdActionVerify, ipc.ActionIDRequest{RecordID: id}, func(rt *Runtime) error {
				return rt.VerifyRemoved(cmd.Context(), id)
			})
		},
	})
	return c
}

func cmdPlaybook() *cobra.Command {
	return &cobra.Command{
		Use:   "playbook [broker-id]",
		Short: "Print web-form steps",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				res, err := callOrLocal(cmd.Context(), ipc.CmdPlaybook, ipc.PlaybookRequest{BrokerID: args[0]}, func(rt *Runtime) (ipc.PlaybookResult, error) {
					b, ok := rt.Reg.Get(args[0])
					if !ok {
						return ipc.PlaybookResult{}, nterr.ErrBrokerNotFound
					}
					return ipc.PlaybookResult{Text: playbook.Render(playbook.For(b))}, nil
				})
				if err == nil {
					fmt.Fprintln(cmd.OutOrStdout(), res.Text)
					return nil
				}
			}
			reg, err := broker.Default()
			if err != nil {
				return err
			}
			if len(args) == 1 {
				b, ok := reg.Get(args[0])
				if !ok {
					return nterr.ErrBrokerNotFound
				}
				fmt.Fprintln(cmd.OutOrStdout(), playbook.Render(playbook.For(b)))
				return nil
			}
			for _, e := range playbook.AllManual(reg.All()) {
				fmt.Fprintln(cmd.OutOrStdout(), playbook.Render(e))
				fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("-", 40))
			}
			return nil
		},
	}
}

func cmdFootprint() *cobra.Command {
	var persist bool
	c := &cobra.Command{
		Use:   "footprint <username>",
		Short: "See where a username already exists",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			req := ipc.FootprintRequest{Username: args[0], Persist: persist}
			res, err := callOrLocal(cmd.Context(), ipc.CmdFootprint, req, func(rt *Runtime) (ipc.FootprintResult, error) {
				return rt.Footprint(cmd.Context(), args[0], persist)
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %d checked, %d found\n", res.Username, res.Checked, res.Found)
			for _, r := range res.Results {
				mark := "-"
				switch r.Status {
				case "FOUND":
					mark = "+"
				case "ERROR":
					mark = "!"
				}
				detail := ""
				if r.Detail != "" {
					detail = " (" + r.Detail + ")"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %s %-7s %s%s\n", mark, r.Status, r.Name, detail)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&persist, "persist", false, "save found profiles as findings")
	return c
}

func cmdAutomate() *cobra.Command {
	var headful bool
	c := &cobra.Command{
		Use:   "automate <broker-id>",
		Short: "Fill one published opt-out form in Chromium",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			req := ipc.AutomateRequest{BrokerID: args[0], Headful: headful}
			res, err := callOrLocal(cmd.Context(), ipc.CmdAutomate, req, func(rt *Runtime) (ipc.AutomateResult, error) {
				return rt.Automate(cmd.Context(), args[0], headful)
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %s %s\n", res.BrokerName, res.Status, res.Detail)
			return nil
		},
	}
	c.Flags().BoolVar(&headful, "headful", false, "show the browser window")
	return c
}

func cmdStatus() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Print vault and queue counts",
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := callOrLocal(cmd.Context(), ipc.CmdStatus, nil, func(rt *Runtime) (ipc.StatusPayload, error) {
				return rt.Status(cmd.Context())
			})
			if err != nil {
				return err
			}
			return writeJSON(cmd.OutOrStdout(), st)
		},
	}
}

func cmdExif() *cobra.Command {
	var out string
	c := &cobra.Command{
		Use:   "exif <file>",
		Short: "Strip JPEG/PNG metadata",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dst := out
			if dst == "" {
				dst = args[0]
			}
			return opsec.StripFile(args[0], dst)
		},
	}
	c.Flags().StringVar(&out, "out", "", "write here instead of in place")
	return c
}

func cmdConfig() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Show config and data paths",
		RunE: func(cmd *cobra.Command, args []string) error {
			dirs, err := paths.Resolve()
			if err != nil {
				return err
			}
			cfg, err := config.Load(dirs.ConfigFile)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "config: %s\ndata: %s\nvault: %s\naudit: %s\nlisten: %s\n",
				dirs.ConfigFile, dirs.Data, dirs.VaultDB, dirs.AuditLogs, cfg.ListenAddr)
			return nil
		},
	}
}

func withRT(ctx context.Context, passFile string, fn func(*Runtime) error) error {
	for {
		pw, err := ReadPassphrase(passFile, false)
		if err != nil {
			if passFile == "" && os.Getenv("NULLTRACE_APP") == "1" && os.Getenv("NULLTRACE_PASSPHRASE") == "" {
				fmt.Fprintln(os.Stderr, err)
				continue
			}
			return err
		}
		rt, err := Open(ctx, pw, false)
		ncrypto.Zeroize(pw)
		if err != nil {
			if passFile == "" && (errors.Is(err, nterr.ErrInvalidPassphrase) || errors.Is(err, nterr.ErrEmptyPassphrase)) {
				fmt.Fprintln(os.Stderr, "wrong password — try again")
				continue
			}
			return err
		}
		defer rt.Close()
		return fn(rt)
	}
}

func callOrLocal[T any](ctx context.Context, cmd string, req any, local func(*Runtime) (T, error)) (T, error) {
	var zero T
	if cl, err := tryClient(); err == nil {
		rep, err := cl.Call(ctx, cmd, req)
		if err != nil {
			return zero, err
		}
		return ipc.UnmarshalPayload[T](rep)
	}
	var out T
	err := withRT(ctx, PassphraseFile, func(rt *Runtime) error {
		v, err := local(rt)
		out = v
		return err
	})
	return out, err
}

func doOrLocal(ctx context.Context, cmd string, req any, local func(*Runtime) error) error {
	if cl, err := tryClient(); err == nil {
		_, err := cl.Call(ctx, cmd, req)
		return err
	}
	return withRT(ctx, PassphraseFile, local)
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func TryClient() (*ipc.Client, error) {
	return tryClient()
}

func tryClient() (*ipc.Client, error) {
	dirs, err := paths.Resolve()
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(dirs.ConfigFile)
	if err != nil {
		return nil, err
	}
	tok, err := ipc.LoadToken(dirs.TokenFile)
	if err != nil {
		return nil, err
	}
	c := ipc.NewClient(cfg.ListenAddr, tok)
	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()
	if err := c.Ping(ctx); err != nil {
		return nil, err
	}
	return c, nil
}
