package app

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/4x3/nulltrace/pkg/ipc"
)

func (rt *Runtime) Export(ctx context.Context, format string) (ipc.ExportResult, error) {
	if rt.Store.Locked() {
		return ipc.ExportResult{}, fmt.Errorf("vault locked")
	}
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = "md"
	}
	if err := os.MkdirAll(rt.Dirs.AuditLogs, 0o700); err != nil {
		return ipc.ExportResult{}, err
	}
	stamp := time.Now().UTC().Format("20060102T150405Z")
	name := "audit-" + stamp + "." + extFor(format)
	path := filepath.Join(rt.Dirs.AuditLogs, name)

	rows, err := rt.Store.ListAudit(ctx, 5000)
	if err != nil {
		return ipc.ExportResult{}, err
	}
	type line struct {
		Timestamp string `json:"timestamp"`
		Event     string `json:"event"`
		BrokerID  string `json:"broker_id"`
		Hash      string `json:"payload_hash"`
		Details   string `json:"details"`
	}
	out := make([]line, 0, len(rows))
	for _, r := range rows {
		details, err := rt.Store.DecryptAudit(r)
		if err != nil {
			details = []byte("(decrypt failed)")
		}
		out = append(out, line{
			Timestamp: r.Timestamp.UTC().Format(time.RFC3339),
			Event:     r.EventType,
			BrokerID:  r.BrokerID,
			Hash:      r.PayloadHash,
			Details:   string(details),
		})
	}

	var raw []byte
	switch format {
	case "json":
		raw, err = json.MarshalIndent(out, "", "  ")
		if err != nil {
			return ipc.ExportResult{}, err
		}
	case "csv":
		var b strings.Builder
		w := csv.NewWriter(&b)
		_ = w.Write([]string{"timestamp", "event", "broker_id", "payload_hash", "details"})
		for _, l := range out {
			_ = w.Write([]string{l.Timestamp, l.Event, l.BrokerID, l.Hash, l.Details})
		}
		w.Flush()
		raw = []byte(b.String())
	default:
		var b strings.Builder
		b.WriteString("# NullTrace audit export\n\n")
		b.WriteString("Generated: " + time.Now().UTC().Format(time.RFC3339) + "\n\n")
		for _, l := range out {
			b.WriteString("## ")
			b.WriteString(l.Timestamp)
			b.WriteString(" — ")
			b.WriteString(l.Event)
			b.WriteByte('\n')
			if l.BrokerID != "" {
				b.WriteString("- broker: `")
				b.WriteString(l.BrokerID)
				b.WriteString("`\n")
			}
			b.WriteString("- hash: `")
			b.WriteString(l.Hash)
			b.WriteString("`\n\n```\n")
			b.WriteString(l.Details)
			b.WriteString("\n```\n\n")
		}
		raw = []byte(b.String())
		if format != "md" && format != "markdown" {
			return ipc.ExportResult{}, fmt.Errorf("unknown export format %q (md, json, csv)", format)
		}
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return ipc.ExportResult{}, err
	}
	rt.Audit(ctx, "audit.exported", "", map[string]any{"path": path, "format": format, "rows": len(out)})
	return ipc.ExportResult{Path: path}, nil
}

func extFor(format string) string {
	switch format {
	case "json":
		return "json"
	case "csv":
		return "csv"
	default:
		return "md"
	}
}
