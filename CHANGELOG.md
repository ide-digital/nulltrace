# Changelog

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
- Gmail / Outlook / Yahoo linking via App Passwords — no OAuth
- Browser automation and CapSolver stay off until you turn them on
