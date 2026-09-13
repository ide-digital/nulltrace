package crypto

import (
	"crypto/rand"
	"fmt"

	"golang.org/x/crypto/argon2"
)

// Argon2id. Memory is KiB (64*1024 == 64 MiB).
const (
	ArgonTime    uint32 = 3
	ArgonMemory  uint32 = 64 * 1024
	ArgonThreads uint8  = 4
	ArgonKeyLen  uint32 = 32
	SaltLen             = 32
)

// KDFParams is stored next to the wrapped DEK so we can bump costs later
// without breaking existing vaults.
type KDFParams struct {
	Time    uint32 `json:"time"`
	Memory  uint32 `json:"memory"`
	Threads uint8  `json:"threads"`
	KeyLen  uint32 `json:"key_len"`
}

func DefaultKDFParams() KDFParams {
	return KDFParams{
		Time:    ArgonTime,
		Memory:  ArgonMemory,
		Threads: ArgonThreads,
		KeyLen:  ArgonKeyLen,
	}
}

func GenerateSalt() ([]byte, error) {
	salt := make([]byte, SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("generate argon2 salt: %w", err)
	}
	return salt, nil
}

// DeriveKey returns a 32-byte KEK. Caller must Zeroize it. passphrase is []byte
// on purpose — Go strings cannot be wiped.
func DeriveKey(passphrase, salt []byte, params KDFParams) ([]byte, error) {
	if len(passphrase) == 0 {
		return nil, fmt.Errorf("derive key: empty passphrase")
	}
	if len(salt) < 16 {
		return nil, fmt.Errorf("derive key: salt must be at least 16 bytes")
	}
	if params.Time == 0 || params.Memory == 0 || params.Threads == 0 || params.KeyLen == 0 {
		return nil, fmt.Errorf("derive key: invalid KDF parameters")
	}
	key := argon2.IDKey(passphrase, salt, params.Time, params.Memory, params.Threads, params.KeyLen)
	return key, nil
}
