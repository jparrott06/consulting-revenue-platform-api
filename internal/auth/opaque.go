package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

// NewRefreshToken returns a high-entropy opaque refresh token string.
func NewRefreshToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// HashRefreshToken hashes an opaque refresh token for storage.
func HashRefreshToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
