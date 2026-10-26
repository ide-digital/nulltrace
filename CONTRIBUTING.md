# Contributing

Build from `source/`. Go 1.23+ is enough; CGO is off.

```
cd source
go test ./...
go build -o nulltrace ./cmd/nulltrace
```

Or run `make build` from the repo root. Everything lands in `bin/`, which is
gitignored. With `GOOS=windows` you get the zip layout (`NullTrace.exe` plus
`app/`).

Please do not commit:

- vault databases, `daemon.token`, `.env`, passphrase files
- real names, emails, or screenshots of a live vault
- API keys

A pull request that adds a broker to `source/internal/broker/data/` should
include a source for the opt-out URL or privacy address. Catalog entries
are best-effort; they go stale.

## Checking the catalog

A weekly job probes every `opt_out_url` and `contact_email` and opens an
issue when something breaks. You can run the same check on one broker:

```
cd source
go run ./cmd/catalogcheck -only spokeo -report-dir /tmp/cc
```

Leave out `-only` to check everything, which takes a minute or two. Other
flags are `-concurrency` (default 8) and `-timeout` (default 15s). The
reports land in `catalog-report.md` and `catalog-report.json`. The command
exits 1 if anything fails that isn't in the known issues list.

Pages that answer 401, 403 or 429 only get a warning, since that is usually
bot protection and not a dead link. A 404, a domain that doesn't resolve, or
a bad certificate is a failure.

## Known issues

If an entry is broken and you can't find the new page yet, list it in
`source/internal/broker/data/known_issues.yaml` instead of leaving the check
red:

```
- id: spokeo
  field: opt_out_url
  reason: site is down, waiting on their status page
  added: 2026-10-26
```

`field` is `opt_out_url` or `contact_email`, and `added` is a date. Entries
older than 60 days fail the check, so either fix the entry or renew it with
a fresh date and reason. Delete an entry once the check passes again.
