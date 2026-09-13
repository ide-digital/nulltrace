package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/4x3/nulltrace/internal/app"
	ncrypto "github.com/4x3/nulltrace/internal/crypto"
	"github.com/4x3/nulltrace/internal/daemon"
	"github.com/4x3/nulltrace/internal/nterr"
	"github.com/4x3/nulltrace/pkg/ipc"
)

func main() {
	appMode := os.Getenv("NULLTRACE_APP") == "1"
	rest := make([]string, 0, len(os.Args)-1)
	for _, a := range os.Args[1:] {
		switch a {
		case "--app":
			appMode = true
			continue
		case "--here":
			continue
		}
		rest = append(rest, a)
	}
	os.Args = append([]string{os.Args[0]}, rest...)

	interactive := len(rest) == 0
	if appMode || interactive {
		prepareConsole()
		bootLog("start appMode=%v interactive=%v terminal=%v args=%v", appMode, interactive, isStdinTerminal(), rest)
	}

	err := run(appMode)
	if err != nil {
		bootLog("exit error: %v", err)
		fmt.Fprintln(os.Stderr, err)
		if appMode || interactive {
			pause()
		}
		os.Exit(1)
	}
	bootLog("exit ok")
}

func run(appMode bool) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	root := app.NewRoot()
	root.AddCommand(cmdDaemon())
	if appMode {
		root.SilenceUsage = true
	}
	return root.ExecuteContext(ctx)
}

func pause() {
	fmt.Fprint(os.Stderr, "\nPress Enter to close...")
	if isStdinTerminal() {
		_, _ = bufio.NewReader(os.Stdin).ReadBytes('\n')
		return
	}
	showEngineError("NullTrace closed because the console had no keyboard attached.\n\nDouble-click NullTrace.exe rather than piping it.")
}

func cmdDaemon() *cobra.Command {
	c := &cobra.Command{Use: "daemon", Short: "Talk to the local nulltraced process"}

	c.AddCommand(&cobra.Command{
		Use:   "start",
		Short: "Unlock the vault and listen on 127.0.0.1:7738",
		RunE: func(cmd *cobra.Command, args []string) error {
			pw, err := app.ReadPassphrase(app.PassphraseFile, false)
			if err != nil {
				return err
			}
			defer ncrypto.Zeroize(pw)
			return daemon.Run(cmd.Context(), pw)
		},
	})

	c.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Ping the daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			cl, err := app.TryClient()
			if err != nil {
				return nterr.ErrDaemonUnavailable
			}
			rep, err := cl.Call(cmd.Context(), ipc.CmdStatus, nil)
			if err != nil {
				return err
			}
			st, err := ipc.UnmarshalPayload[ipc.StatusPayload](rep)
			if err != nil {
				return err
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(st)
		},
	})

	c.AddCommand(&cobra.Command{
		Use:   "stop",
		Short: "Ask the daemon to shut down",
		RunE: func(cmd *cobra.Command, args []string) error {
			cl, err := app.TryClient()
			if err != nil {
				return nterr.ErrDaemonUnavailable
			}
			_, err = cl.Call(cmd.Context(), ipc.CmdShutdown, nil)
			return err
		},
	})

	return c
}
