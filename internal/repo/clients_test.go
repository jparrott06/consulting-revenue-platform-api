package repo

import "testing"

func TestNormalizeBillingEmail(t *testing.T) {
	if got := NormalizeBillingEmail("  Test@Example.COM "); got != "test@example.com" {
		t.Fatalf("got %q", got)
	}
}
