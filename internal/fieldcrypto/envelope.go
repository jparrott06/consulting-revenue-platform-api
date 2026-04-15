package fieldcrypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

// EnvelopeVersionV1 is the prefix for the v1 AES-GCM envelope encoding.
const EnvelopeVersionV1 = "v1."

var (
	// ErrInvalidKeyLength is returned when the symmetric key is not 32 bytes (AES-256).
	ErrInvalidKeyLength = errors.New("encryption key must be 32 bytes for AES-256-GCM")
	// ErrInvalidEnvelope is returned when ciphertext cannot be parsed.
	ErrInvalidEnvelope = errors.New("invalid ciphertext envelope")
	// ErrDecryptFailed is returned when ciphertext fails authentication or is corrupted.
	ErrDecryptFailed = errors.New("decryption failed")
)

// Encrypt seals plaintext using AES-256-GCM and returns a versioned ASCII envelope.
func Encrypt(key []byte, plaintext []byte) (string, error) {
	if len(key) != 32 {
		return "", ErrInvalidKeyLength
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, plaintext, nil)
	encoded := base64.RawStdEncoding.EncodeToString(sealed)
	return EnvelopeVersionV1 + encoded, nil
}

// Decrypt parses a versioned envelope and returns plaintext.
func Decrypt(key []byte, envelope string) ([]byte, error) {
	if len(key) != 32 {
		return nil, ErrInvalidKeyLength
	}
	if !strings.HasPrefix(envelope, EnvelopeVersionV1) {
		return nil, ErrInvalidEnvelope
	}
	raw := strings.TrimPrefix(envelope, EnvelopeVersionV1)
	sealed, err := base64.RawStdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidEnvelope, err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(sealed) < gcm.NonceSize() {
		return nil, ErrInvalidEnvelope
	}
	nonce := sealed[:gcm.NonceSize()]
	ciphertext := sealed[gcm.NonceSize():]
	out, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrDecryptFailed
	}
	return out, nil
}
