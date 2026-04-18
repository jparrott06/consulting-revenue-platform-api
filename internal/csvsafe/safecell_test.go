package csvsafe

import (
	"encoding/csv"
	"strings"
	"testing"
)

func TestSafeCell_FormulaPrefixes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in, want string
	}{
		{"=SUM(A1)", "'=SUM(A1)"},
		{"+123", "'+123"},
		{"-42", "'-42"},
		{"@ref", "'@ref"},
		{"\t=cmd", "'\t=cmd"},
		{"normal", "normal"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := SafeCell(tt.in); got != tt.want {
			t.Fatalf("SafeCell(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestSafeCell_EncodingCSVDoesNotStartWithEquals(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	w := csv.NewWriter(&buf)
	if err := w.Write([]string{SafeCell("=SUM(1)")}); err != nil {
		t.Fatal(err)
	}
	w.Flush()
	line := buf.String()
	if strings.Contains(line, "\n=SUM") || strings.HasPrefix(strings.TrimSpace(line), "=") {
		t.Fatalf("encoded line may still be interpreted as formula: %q", line)
	}
}
