package exposure

import (
	"crypto/rand"
	"math/big"
	"strings"
)

const alphabet = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
const symbols = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789!@#$%^&*-_?"

func Suggest(n int) []string {
	if n < 1 {
		n = 3
	}
	out := make([]string, 0, n)
	out = append(out, passphrase(5))
	out = append(out, randomFrom(alphabet, 20))
	if n > 2 {
		out = append(out, randomFrom(symbols, 16))
	}
	for len(out) < n {
		out = append(out, passphrase(6))
	}
	return out[:n]
}

func passphrase(words int) string {
	if words < 4 {
		words = 4
	}
	parts := make([]string, words)
	for i := 0; i < words; i++ {
		parts[i] = wordList[randIndex(len(wordList))]
	}
	return strings.Join(parts, "-")
}

func randomFrom(set string, n int) string {
	runes := []rune(set)
	b := make([]rune, n)
	for i := 0; i < n; i++ {
		b[i] = runes[randIndex(len(runes))]
	}
	return string(b)
}

func randIndex(n int) int {
	if n <= 0 {
		return 0
	}
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0
	}
	return int(v.Int64())
}
