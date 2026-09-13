package crypto

import (
	"crypto/rand"
	"fmt"

	"golang.org/x/crypto/chacha20poly1305"

	"github.com/4x3/nulltrace/internal/nterr"
)

const (
	NonceSize = chacha20poly1305.NonceSizeX
	KeySize   = chacha20poly1305.KeySize
	Overhead  = 16
)

// Envelope is salt + wrapped DEK + a known-plaintext check.
// WrappedDEK is nonce || ciphertext || tag.
type Envelope struct {
	Salt       []byte
	WrappedDEK []byte
	Params     KDFParams
	Verifier   []byte
}

func NewDEK() ([]byte, error) {
	dek := make([]byte, KeySize)
	if _, err := rand.Read(dek); err != nil {
		return nil, fmt.Errorf("generate DEK: %w", err)
	}
	return dek, nil
}

// Encrypt returns nonce || ciphertext || tag. aad binds a blob to a row/column
// so ciphertext cannot be copied between fields.
func Encrypt(key, plaintext, aad []byte) ([]byte, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("encrypt: key must be %d bytes", KeySize)
	}
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, fmt.Errorf("encrypt: aead init: %w", err)
	}
	nonce := make([]byte, NonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("encrypt: nonce: %w", err)
	}
	sealed := aead.Seal(nil, nonce, plaintext, aad)
	out := make([]byte, 0, NonceSize+len(sealed))
	out = append(out, nonce...)
	out = append(out, sealed...)
	return out, nil
}

func Decrypt(key, blob, aad []byte) ([]byte, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("decrypt: key must be %d bytes", KeySize)
	}
	if len(blob) < NonceSize+Overhead {
		return nil, fmt.Errorf("decrypt: %w", nterr.ErrCiphertextCorrupt)
	}
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, fmt.Errorf("decrypt: aead init: %w", err)
	}
	nonce := blob[:NonceSize]
	sealed := blob[NonceSize:]
	plain, err := aead.Open(nil, nonce, sealed, aad)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", nterr.ErrCiphertextCorrupt)
	}
	return plain, nil
}

const verifierPlaintext = "nulltrace-vault-v1"

func wrapDEK(kek, dek []byte) ([]byte, error) {
	return Encrypt(kek, dek, []byte("nulltrace/dek"))
}

func unwrapDEK(kek, wrapped []byte) ([]byte, error) {
	return Decrypt(kek, wrapped, []byte("nulltrace/dek"))
}

func InitEnvelope(passphrase []byte) (*Envelope, []byte, error) {
	if len(passphrase) == 0 {
		return nil, nil, fmt.Errorf("init envelope: %w", nterr.ErrEmptyPassphrase)
	}
	params := DefaultKDFParams()
	salt, err := GenerateSalt()
	if err != nil {
		return nil, nil, err
	}
	kek, err := DeriveKey(passphrase, salt, params)
	if err != nil {
		return nil, nil, err
	}
	defer Zeroize(kek)

	dek, err := NewDEK()
	if err != nil {
		return nil, nil, err
	}
	wrapped, err := wrapDEK(kek, dek)
	if err != nil {
		Zeroize(dek)
		return nil, nil, fmt.Errorf("wrap DEK: %w", err)
	}
	verifier, err := Encrypt(dek, []byte(verifierPlaintext), []byte("nulltrace/verifier"))
	if err != nil {
		Zeroize(dek)
		return nil, nil, fmt.Errorf("seal verifier: %w", err)
	}
	env := &Envelope{
		Salt:       salt,
		WrappedDEK: wrapped,
		Params:     params,
		Verifier:   verifier,
	}
	return env, dek, nil
}

func OpenEnvelope(passphrase []byte, env *Envelope) ([]byte, error) {
	if env == nil {
		return nil, fmt.Errorf("open envelope: nil envelope")
	}
	if len(passphrase) == 0 {
		return nil, fmt.Errorf("open envelope: %w", nterr.ErrEmptyPassphrase)
	}
	kek, err := DeriveKey(passphrase, env.Salt, env.Params)
	if err != nil {
		return nil, err
	}
	defer Zeroize(kek)

	dek, err := unwrapDEK(kek, env.WrappedDEK)
	if err != nil {
		return nil, fmt.Errorf("%w", nterr.ErrInvalidPassphrase)
	}
	plain, err := Decrypt(dek, env.Verifier, []byte("nulltrace/verifier"))
	if err != nil {
		Zeroize(dek)
		return nil, fmt.Errorf("%w", nterr.ErrInvalidPassphrase)
	}
	defer Zeroize(plain)
	if string(plain) != verifierPlaintext {
		Zeroize(dek)
		return nil, fmt.Errorf("%w", nterr.ErrInvalidPassphrase)
	}
	return dek, nil
}

func AAD(table, column, rowID string) []byte {
	return []byte(table + "|" + column + "|" + rowID)
}
