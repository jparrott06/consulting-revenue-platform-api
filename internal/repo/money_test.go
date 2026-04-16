package repo

import "testing"

func TestNormalizeCurrencyCode(t *testing.T) {
	if _, err := NormalizeCurrencyCode(""); err == nil {
		t.Fatal("expected error for empty")
	}
	if got, err := NormalizeCurrencyCode(" usd "); err != nil || got != "USD" {
		t.Fatalf("expected USD, got %q err=%v", got, err)
	}
	if got, err := NormalizeCurrencyCode("usd"); err != nil || got != "USD" {
		t.Fatalf("got %q err %v", got, err)
	}
	if _, err := NormalizeCurrencyCode("US"); err == nil {
		t.Fatal("expected error for short code")
	}
	if _, err := NormalizeCurrencyCode("US1"); err == nil {
		t.Fatal("expected error for non-alpha")
	}
	if _, err := NormalizeCurrencyCode("EUR"); err == nil {
		t.Fatal("expected unsupported currency error")
	}
}

func TestParseMajorToMinor(t *testing.T) {
	tests := []struct {
		currency string
		major    string
		want     int64
		ok       bool
	}{
		{"USD", "12.34", 1234, true},
		{"USD", "12", 1200, true},
		{"USD", "12.345", 0, false},
		{"JPY", "1500", 1500, true},
		{"JPY", "1500.00", 0, false},
	}
	for _, tt := range tests {
		got, err := ParseMajorToMinor(tt.currency, tt.major)
		if tt.ok {
			if err != nil || got != tt.want {
				t.Fatalf("ParseMajorToMinor(%s,%s)=%d err=%v want=%d", tt.currency, tt.major, got, err, tt.want)
			}
		} else if err == nil {
			t.Fatalf("ParseMajorToMinor(%s,%s) expected error", tt.currency, tt.major)
		}
	}
}

func TestFormatMinorForDisplay(t *testing.T) {
	if got, err := FormatMinorForDisplay("USD", 1234); err != nil || got != "12.34" {
		t.Fatalf("got %q err=%v", got, err)
	}
	if got, err := FormatMinorForDisplay("JPY", 1500); err != nil || got != "1500" {
		t.Fatalf("got %q err=%v", got, err)
	}
}
