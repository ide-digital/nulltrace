# Security

## What this project stores

NullTrace keeps the vault on the machine that runs it:

- Windows: `%LocalAppData%\nulltrace` (vault), `%AppData%\nulltrace` (config)
- Linux / macOS: `$HOME/.local/share/nulltrace` (vault), `$HOME/.config/nulltrace` (config)

The vault is encrypted. Do not attach `vault.db`, `config.yaml`, `daemon.token`,
or a passphrase file to an issue or a pull request.

## What leaves the machine

By default, almost nothing. Optional features that talk to the network:

- Password dump checks send a 5-character SHA-1 prefix to the public
  Pwned Passwords range API. The rest of the password stays local.
- Scan matches your identity against a local broker catalog. It does not
  scrape the open web unless you turn extra tools on.
- Email send uses the SMTP account you paste in. That is your mailbox,
  not a NullTrace server.

Browser automation and CapSolver are off until you enable them.

## Reporting a vulnerability

Please use GitHub's private vulnerability reporting on this repository
(Security tab → Report a vulnerability). Include the version (`v0.3.1`
or `git rev-parse --short HEAD`) and enough to reproduce. Do not open a
public issue for a vault-bypass or crypto bug.
