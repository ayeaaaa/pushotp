package pushotp

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"math/big"
)

func generateCode(length int) (string, error) {
	if length < 4 || length > 8 {
		return "", fmt.Errorf("%w: code length %d out of range [4,8]", ErrConfig, length)
	}
	buf := make([]byte, length)
	for i := range buf {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		buf[i] = "0123456789"[n.Int64()]
	}
	return string(buf), nil
}

func randomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

func hashCode(salt []byte, code string) []byte {
	h := sha256.New()
	h.Write(salt)
	h.Write([]byte(code))
	return h.Sum(nil)
}

func equalHash(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}

func newID() (string, error) {
	b, err := randomBytes(16)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
