package crypto

import (
	"crypto/subtle"
	"runtime"
)

// Zeroize overwrites b in place. KeepAlive stops the compiler from dropping
// the wipe if b is about to go out of scope.
func Zeroize(b []byte) {
	if len(b) == 0 {
		return
	}
	zeros := make([]byte, len(b))
	subtle.ConstantTimeCopy(1, b, zeros)
	runtime.KeepAlive(b)
	runtime.KeepAlive(zeros)
}
