package repo

import "testing"

func TestNormalizeCurrencyCode(t *testing.T) {
	if _, err := NormalizeCurrencyCode(""); err == nil {
		t.Fatal("expected error for empty")
	}
	got, err := NormalizeCurrencyCode("usd")
	if err != nil || got != "USD" {
		t.Fatalf("got %q err %v", got, err)
	}
	if _, err := NormalizeCurrencyCode("US"); err == nil {
		t.Fatal("expected error for short code")
	}
	if _, err := NormalizeCurrencyCode("US1"); err == nil {
		t.Fatal("expected error for non-alpha")
	}
}

func TestNormalizeBillingEmail(t *testing.T) {
	if got := NormalizeBillingEmail("  Test@Example.COM "); got != "test@example.com" {
		t.Fatalf("got %q", got)
	}
}
