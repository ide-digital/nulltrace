package nterr

import "errors"

var (
	ErrBrokerNotFound       = errors.New("broker not found")
	ErrIdentityNotFound     = errors.New("identity not found")
	ErrRecordNotFound       = errors.New("exposed record not found")
	ErrActionNotFound       = errors.New("erasure action not found")
	ErrRateLimited          = errors.New("rate limited")
	ErrStateDrift           = errors.New("listing reappeared after verified removal")
	ErrVaultLocked          = errors.New("vault is locked")
	ErrVaultExists          = errors.New("vault already initialized")
	ErrInvalidPassphrase    = errors.New("invalid passphrase")
	ErrNotInitialized       = errors.New("vault has not been initialized")
	ErrDaemonUnavailable    = errors.New("nulltraced is not reachable")
	ErrUnauthorized         = errors.New("missing or invalid control token")
	ErrInvalidState         = errors.New("illegal state transition")
	ErrMissingSMTP          = errors.New("SMTP is not configured")
	ErrMissingIMAP          = errors.New("IMAP is not configured")
	ErrFootprintDisabled    = errors.New("footprint recon is disabled")
	ErrBrowserDisabled      = errors.New("browser automation is disabled")
	ErrNoIdentity           = errors.New("no identity is stored in the vault")
	ErrEmptyPassphrase      = errors.New("passphrase must not be empty")
	ErrCiphertextCorrupt    = errors.New("ciphertext authentication failed")
	ErrManualActionRequired = errors.New("broker requires a manual web-form opt-out")
)
