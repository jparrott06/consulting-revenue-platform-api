package invoicepdf

import (
	"testing"
	"time"
)

func TestBuild_MinimalInvoice(t *testing.T) {
	issued := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	b, err := Build(Header{
		InvoiceNumber: 42,
		Currency:      "USD",
		Status:        "issued",
		SubtotalDisp:  "10.00",
		TaxDisp:       "0.00",
		TotalDisp:     "10.00",
		IssuedAt:      &issued,
	}, []Line{
		{Description: "Work", Quantity: "1.00", UnitDisplay: "10.00", LineDisplay: "10.00"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(b) < 100 || string(b[:5]) != "%PDF-" {
		t.Fatalf("unexpected pdf prefix: len=%d prefix=%q", len(b), string(b[:min(20, len(b))]))
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
