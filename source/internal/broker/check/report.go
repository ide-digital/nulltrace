package check

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Summary struct {
	Total      int `json:"total"`
	OK         int `json:"ok"`
	Warn       int `json:"warn"`
	Fail       int `json:"fail"`
	Known      int `json:"known"`
	StaleKnown int `json:"stale_known"`
}

type Report struct {
	Generated time.Time `json:"generated"`
	Summary   Summary   `json:"summary"`
	Fail      []Result  `json:"fail"`
	Warn      []Result  `json:"warn"`
	Known     []Result  `json:"known"`

	// StaleKnown entries are older than StaleAfter and fail the run.
	StaleKnown []KnownIssue `json:"stale_known"`
	// Resolved entries no longer match a failure and can be deleted.
	Resolved []KnownIssue `json:"resolved"`
}

// Build sorts results into buckets and matches failures against the known
// issues list. Anything in the list that still fails is reported as known.
func Build(results []Result, known []KnownIssue, now time.Time) Report {
	rep := Report{Generated: now.UTC()}

	byKey := map[string]KnownIssue{}
	for _, k := range known {
		byKey[k.ID+"/"+k.Field] = k
	}
	hit := map[string]bool{}

	for _, r := range results {
		rep.Summary.Total++
		switch r.Status {
		case OK:
			rep.Summary.OK++
		case Warn:
			rep.Summary.Warn++
			rep.Warn = append(rep.Warn, r)
		case Fail:
			key := r.ID + "/" + r.Field
			if k, ok := byKey[key]; ok {
				hit[key] = true
				r.Status = Known
				r.Reason += " (known: " + k.Reason + ")"
				rep.Summary.Known++
				rep.Known = append(rep.Known, r)
				continue
			}
			rep.Summary.Fail++
			rep.Fail = append(rep.Fail, r)
		}
	}

	for _, k := range known {
		if !hit[k.ID+"/"+k.Field] {
			rep.Resolved = append(rep.Resolved, k)
		}
		if added, err := k.addedAt(); err == nil && now.Sub(added) > StaleAfter {
			rep.StaleKnown = append(rep.StaleKnown, k)
		}
	}
	rep.Summary.StaleKnown = len(rep.StaleKnown)
	return rep
}

// Broken is the number that should fail the run.
func (r Report) Broken() int {
	return r.Summary.Fail + r.Summary.StaleKnown
}

func (r Report) Markdown() string {
	var b strings.Builder
	s := r.Summary
	fmt.Fprintf(&b, "# Catalog check\n\n")
	fmt.Fprintf(&b, "Run on %s.\n\n", r.Generated.Format("2006-01-02 15:04 UTC"))
	fmt.Fprintf(&b, "| checked | ok | warn | fail | known |\n|---|---|---|---|---|\n")
	fmt.Fprintf(&b, "| %d | %d | %d | %d | %d |\n\n", s.Total, s.OK, s.Warn, s.Fail, s.Known)

	if s.StaleKnown > 0 {
		fmt.Fprintf(&b, "## Known issues older than %d days\n\n", int(StaleAfter.Hours()/24))
		b.WriteString("Fix these or renew them with a fresh reason.\n\n")
		b.WriteString("| id | field | added | reason |\n|---|---|---|---|\n")
		for _, k := range r.StaleKnown {
			fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", cell(k.ID), k.Field, k.Added, cell(k.Reason))
		}
		b.WriteString("\n")
	}

	writeTable(&b, "Fail", r.Fail)
	writeTable(&b, "Warn", r.Warn)
	writeTable(&b, "Known", r.Known)

	if len(r.Resolved) > 0 {
		b.WriteString("## Known issues that pass now\n\n")
		b.WriteString("These can come out of `known_issues.yaml`.\n\n")
		for _, k := range r.Resolved {
			fmt.Fprintf(&b, "- `%s` %s\n", k.ID, k.Field)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func writeTable(b *strings.Builder, title string, rows []Result) {
	if len(rows) == 0 {
		return
	}
	fmt.Fprintf(b, "## %s (%d)\n\n", title, len(rows))
	b.WriteString("| broker | field | value | reason |\n|---|---|---|---|\n")
	for _, r := range rows {
		fmt.Fprintf(b, "| %s | %s | %s | %s |\n", cell(r.ID), r.Field, cell(r.Value), cell(r.Reason))
	}
	b.WriteString("\n")
}

// cell keeps a value from breaking out of its markdown table row.
func cell(s string) string {
	s = strings.ReplaceAll(s, "|", `\|`)
	return strings.ReplaceAll(s, "\n", " ")
}

// Write drops catalog-report.md and catalog-report.json into dir.
func (r Report) Write(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "catalog-report.json"), append(raw, '\n'), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "catalog-report.md"), []byte(r.Markdown()), 0o644)
}
