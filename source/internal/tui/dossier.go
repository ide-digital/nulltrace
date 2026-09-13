package tui

import (
	"fmt"
	"strings"

	"github.com/4x3/nulltrace/pkg/ipc"
)

func renderDossier(s ipc.Snapshot) string {
	id := s.Identity
	if id.ID == "" {
		return styleMuted.Render("  No identity in the vault. Run: nulltrace identity add")
	}
	line := func(k, v string) string {
		if v == "" {
			v = "—"
		}
		return fmt.Sprintf("  %-14s %s", k, v)
	}
	join := func(ss []string) string {
		if len(ss) == 0 {
			return ""
		}
		return strings.Join(ss, ", ")
	}
	name := strings.TrimSpace(id.First + " " + id.Middle + " " + id.Last)
	return strings.Join([]string{
		styleTitle.Render("  DOSSIER  (held encrypted at rest)"),
		"",
		line("id", id.ID),
		line("name", name),
		line("dob", id.DOB),
		line("emails", join(id.Emails)),
		line("phones", join(id.Phones)),
		line("cities", join(id.Cities)),
		line("addresses", join(id.Addresses)),
		line("usernames", join(id.Usernames)),
	}, "\n")
}
