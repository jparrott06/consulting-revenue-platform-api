package httpapi

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/config"
)

func TestMintParseInvoicePDFToken(t *testing.T) {
	cfg := config.Config{JWTSigningKey: "test-secret-key-for-hmac"}
	org := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	inv := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	tok, exp, err := mintInvoicePDFToken(cfg, org, inv, 2*time.Minute)
	if err != nil || tok == "" {
		t.Fatalf("mint: err=%v tok empty=%v", err, tok == "")
	}
	if !exp.After(time.Now().UTC()) {
		t.Fatalf("exp not in future: %v", exp)
	}
	gotOrg, gotInv, err := parseInvoicePDFToken(cfg, tok)
	if err != nil {
		t.Fatal(err)
	}
	if gotOrg != org || gotInv != inv {
		t.Fatalf("got org=%v inv=%v", gotOrg, gotInv)
	}
}

func TestParseInvoicePDFToken_BadSignature(t *testing.T) {
	cfg := config.Config{JWTSigningKey: "secret-a"}
	tok, _, err := mintInvoicePDFToken(cfg, uuid.New(), uuid.New(), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	cfg2 := config.Config{JWTSigningKey: "secret-b"}
	_, _, err = parseInvoicePDFToken(cfg2, tok)
	if !errors.Is(err, errPDFTokenBadSig) {
		t.Fatalf("expected errPDFTokenBadSig, got %v", err)
	}
}

func TestParseInvoicePDFToken_Expired(t *testing.T) {
	cfg := config.Config{JWTSigningKey: "test-secret-key-for-hmac"}
	org := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	inv := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	tok, _, err := mintInvoicePDFToken(cfg, org, inv, -time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = parseInvoicePDFToken(cfg, tok)
	if !errors.Is(err, errPDFTokenExpired) {
		t.Fatalf("expected errPDFTokenExpired, got %v", err)
	}
}
