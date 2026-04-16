package invoicepdf

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/phpdave11/gofpdf"
)

// Line is one invoice line for rendering.
type Line struct {
	Description string
	Quantity    string
	UnitDisplay string
	LineDisplay string
}

// Header carries invoice summary fields for the PDF cover block.
type Header struct {
	InvoiceNumber int64
	Currency      string
	Status        string
	SubtotalDisp  string
	TaxDisp       string
	TotalDisp     string
	IssuedAt      *time.Time
}

// Build renders a minimal plain-text invoice PDF.
func Build(h Header, lines []Line) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetTitle("Invoice", false)
	pdf.AddPage()
	pdf.SetFont("Helvetica", "", 10)

	var sb strings.Builder
	_, _ = fmt.Fprintf(&sb, "Invoice #%d\n", h.InvoiceNumber)
	_, _ = fmt.Fprintf(&sb, "Status: %s  Currency: %s\n", h.Status, h.Currency)
	if h.IssuedAt != nil {
		_, _ = fmt.Fprintf(&sb, "Issued: %s\n", h.IssuedAt.UTC().Format(time.RFC3339))
	}
	sb.WriteByte('\n')
	for _, ln := range lines {
		_, _ = fmt.Fprintf(&sb, "%s | qty %s | unit %s | line %s\n",
			ln.Description, ln.Quantity, ln.UnitDisplay, ln.LineDisplay)
	}
	sb.WriteByte('\n')
	_, _ = fmt.Fprintf(&sb, "Subtotal: %s\n", h.SubtotalDisp)
	_, _ = fmt.Fprintf(&sb, "Tax: %s\n", h.TaxDisp)
	_, _ = fmt.Fprintf(&sb, "Total: %s\n", h.TotalDisp)

	pdf.MultiCell(0, 5, sanitizePDFText(sb.String()), "", "L", false)
	if err := pdf.Error(); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func sanitizePDFText(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '\n' || r == '\t':
			b.WriteByte('\n')
		case r >= 32 && r < 127:
			b.WriteRune(r)
		default:
			b.WriteByte('?')
		}
	}
	return b.String()
}
