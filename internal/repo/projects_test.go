package repo

import "testing"

func TestNormalizeBillingMode(t *testing.T) {
	if _, err := normalizeBillingMode("bogus"); err == nil {
		t.Fatal("expected error")
	}
	got, err := normalizeBillingMode(" Hourly ")
	if err != nil || got != "hourly" {
		t.Fatalf("got %q err %v", got, err)
	}
}
