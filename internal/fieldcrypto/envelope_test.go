package fieldcrypto

import (
	"bytes"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key := bytes.Repeat([]byte("k"), 32)
	plaintext := []byte("sensitive metadata")

	out, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, EnvelopeVersionV1) {
		t.Fatalf("expected versioned envelope, got %q", out)
	}

	got, err := Decrypt(key, out)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("plaintext mismatch: %q vs %q", got, plaintext)
	}
}

func TestDecrypt_InvalidEnvelope(t *testing.T) {
	key := bytes.Repeat([]byte("k"), 32)
	if _, err := Decrypt(key, "v2.nope"); !errors.Is(err, ErrInvalidEnvelope) {
		t.Fatalf("expected invalid envelope, got %v", err)
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	key := bytes.Repeat([]byte("k"), 32)
	out, err := Encrypt(key, []byte("data"))
	if err != nil {
		t.Fatal(err)
	}
	wrong := bytes.Repeat([]byte("z"), 32)
	if _, err := Decrypt(wrong, out); err == nil {
		t.Fatal("expected error")
	}
}

func TestDecrypt_TamperedCiphertext(t *testing.T) {
	key := bytes.Repeat([]byte("k"), 32)
	out, err := Encrypt(key, []byte("data"))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(out, EnvelopeVersionV1))
	if err != nil {
		t.Fatal(err)
	}
	const gcmNonceSize = 12
	if len(raw) <= gcmNonceSize+1 {
		t.Fatal("unexpected sealed payload size")
	}
	raw[gcmNonceSize+1] ^= 0x01
	tampered := EnvelopeVersionV1 + base64.RawStdEncoding.EncodeToString(raw)
	if _, err := Decrypt(key, tampered); !errors.Is(err, ErrDecryptFailed) {
		t.Fatalf("expected decrypt failure, got %v", err)
	}
}
