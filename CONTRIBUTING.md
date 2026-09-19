# Contributing

Build from `source/`. Go 1.23+ is enough; CGO is off.

```
cd source
go test ./...
go build -o nulltrace ./cmd/nulltrace
```

On Windows the launcher is:

```
powershell -File source/build.ps1
```

That writes `NullTrace.exe` next to this repo and the engine into `app/`.
Both paths are gitignored.

Please do not commit:

- vault databases, `daemon.token`, `.env`, passphrase files
- real names, emails, or screenshots of a live vault
- API keys

A pull request that adds a broker to `source/internal/broker/data/` should
include a source for the opt-out URL or privacy address. Catalog entries
are best-effort; they go stale.
