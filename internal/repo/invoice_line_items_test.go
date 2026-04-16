package repo

import (
	"math"
	"testing"
)

func TestParseQuantityHundredths(t *testing.T) {
	tests := []struct {
		in       string
		want     int64
		wantText string
		ok       bool
	}{
		{"1", 100, "1.00", true},
		{"1.5", 150, "1.50", true},
		{"2.25", 225, "2.25", true},
		{"0.00", 0, "", false},
		{"-1", 0, "", false},
		{"1.234", 0, "", false},
	}
	for _, tt := range tests {
		got, txt, err := parseQuantityHundredths(tt.in)
		if tt.ok {
			if err != nil {
				t.Fatalf("parseQuantityHundredths(%q) unexpected err: %v", tt.in, err)
			}
			if got != tt.want || txt != tt.wantText {
				t.Fatalf("parseQuantityHundredths(%q) = (%d,%q), want (%d,%q)", tt.in, got, txt, tt.want, tt.wantText)
			}
			continue
		}
		if err == nil {
			t.Fatalf("parseQuantityHundredths(%q) expected error", tt.in)
		}
	}
}

func TestComputeLineTotalMinor_Rounding(t *testing.T) {
	got, err := computeLineTotalMinor(150, 101)
	if err != nil || got != 152 {
		t.Fatalf("expected 152, got %d err=%v", got, err)
	}
	got, err = computeLineTotalMinor(333, 100)
	if err != nil || got != 333 {
		t.Fatalf("expected 333, got %d err=%v", got, err)
	}
}

func TestComputeLineTotalMinor_Overflow(t *testing.T) {
	_, err := computeLineTotalMinor(math.MaxInt64, 2)
	if err == nil {
		t.Fatal("expected overflow error")
	}
}
