package validate

import "testing"

func TestNormalizeEmail(t *testing.T) {
	if got := NormalizeEmail("  User@Example.COM "); got != "user@example.com" {
		t.Fatalf("got %q", got)
	}
}

func TestTrimString(t *testing.T) {
	if got := TrimString("  x  "); got != "x" {
		t.Fatalf("got %q", got)
	}
}
