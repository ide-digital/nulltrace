# Changelog

## 0.5.0

- Weekly catalog check: every opt-out page and privacy address is probed, and broken ones open a single tracking issue
- Releases only ship when the catalog check passes
- Build provenance attestations for every download, plus `checksums.txt`, so you can verify a build without a paid signing certificate
- `make build` builds everything from source, including the Windows launcher and `app/` folder when `GOOS=windows`
- README: how to verify a download, how the catalog stays current, and how re-listing is handled

## 0.4.0

Moved to [ide-digital/nulltrace](https://github.com/ide-digital/nulltrace). The old URL redirects. If you build from source, the module path is now `github.com/ide-digital/nulltrace`.

- Downloads for Linux and macOS (amd64 + arm64) on every release, next to the Windows zip
- A queued email is claimed before it's sent, so the daemon and the CLI can't both mail the same broker
- Starting the daemon before linking a mailbox no longer marks the whole queue as failed
- Scrub retries emails that failed to send (wrong password, dropped connection) instead of skipping them forever
- IMAP only follows confirmation links on the broker's own domain, redirects included
- Broker replies are matched back to the request they answer
- A failed Pwned Passwords lookup is reported as failed instead of "not found"
- Closing the terminal (or piping in nothing) no longer counts as "yes" at the Send/Scrub prompts
- The daemon shuts down properly on Windows and cleans up if it can't bind
- Timeouts on SMTP, IMAP, the dump lookup, and CapSolver so nothing hangs forever
- Overdue and re-check dates compare as times, not strings
- In daemon mode the Email screen shows whether mail is linked
- Custom SMTP refuses an empty password
- A failed identity save no longer leaves half a record behind
- `install.sh` installs the systemd unit per user and only uses sudo for `/usr/local/bin`
- Docker image listens on the container interface so `-p 127.0.0.1:7738:7738` works

## 0.3.1

- Setup and Identity prompts check names, email, phone, date of birth, and city before saving. Blank still skips optional fields.
- Windows console buffer is kept the same size as the window so the boot/login screens don't clip or smear.

## 0.3.0

First public cut.

- Windows launcher (`NullTrace.exe`) plus a console engine and optional daemon
- Encrypted local vault (Argon2id, XChaCha20-Poly1305)
- Login screen with a locked machine-name username
- Home menu for scan, listings, scrub, send, leaks, email, footprint
- Broker catalog of 400+ people-search and marketing aggregators
- CCPA / CPRA / GDPR / state deletion mail from local templates
- Password strength plus Pwned Passwords k-anonymity (5-character SHA-1 prefix)
- Gmail / Outlook / Yahoo linking via App Passwords, no OAuth
- Browser automation and CapSolver stay off until you turn them on
