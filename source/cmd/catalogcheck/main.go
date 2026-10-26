// Command catalogcheck probes every opt-out page and privacy address in the
// broker catalog. It's a maintainer tool: CI runs it weekly and before a
// release, and it isn't part of the NullTrace download.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/ide-digital/nulltrace/internal/broker"
	"github.com/ide-digital/nulltrace/internal/broker/check"
)

func main() {
	reportDir := flag.String("report-dir", ".", "where to write catalog-report.md and .json")
	concurrency := flag.Int("concurrency", 8, "requests in flight at once")
	timeout := flag.Duration("timeout", 15*time.Second, "per-request timeout")
	only := flag.String("only", "", "check a single broker id")
	knownPath := flag.String("known", "internal/broker/data/known_issues.yaml", "known issues file")
	flag.Parse()

	if err := run(*reportDir, *concurrency, *timeout, *only, *knownPath); err != nil {
		fmt.Fprintln(os.Stderr, "catalogcheck:", err)
		os.Exit(2)
	}
}

func run(reportDir string, concurrency int, timeout time.Duration, only, knownPath string) error {
	reg, err := broker.Default()
	if err != nil {
		return err
	}
	brokers := reg.All()
	if only != "" {
		b, ok := reg.Get(only)
		if !ok {
			return fmt.Errorf("no broker with id %q", only)
		}
		brokers = []broker.Broker{b}
	}

	known, err := check.LoadKnown(knownPath)
	if err != nil {
		return err
	}
	if only != "" {
		known = filterKnown(known, only)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	fmt.Printf("checking %d brokers...\n", len(brokers))
	results := check.Run(ctx, brokers, check.Options{
		Concurrency: concurrency,
		Timeout:     timeout,
	})
	rep := check.Build(results, known, time.Now())
	if err := rep.Write(reportDir); err != nil {
		return err
	}

	s := rep.Summary
	fmt.Printf("checked %d: %d ok, %d warn, %d fail, %d known\n", s.Total, s.OK, s.Warn, s.Fail, s.Known)
	for _, r := range rep.Fail {
		fmt.Printf("  FAIL %-28s %-14s %s (%s)\n", r.ID, r.Field, r.Value, r.Reason)
	}
	for _, k := range rep.StaleKnown {
		fmt.Printf("  STALE %s %s, added %s\n", k.ID, k.Field, k.Added)
	}
	fmt.Println("reports in", reportDir)

	if rep.Broken() > 0 {
		os.Exit(1)
	}
	return nil
}

// with -only, entries for other brokers would all show up as "pass now"
func filterKnown(all []check.KnownIssue, id string) []check.KnownIssue {
	var out []check.KnownIssue
	for _, k := range all {
		if strings.EqualFold(k.ID, id) {
			out = append(out, k)
		}
	}
	return out
}
