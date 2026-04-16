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
	sb.WriteString(fmt.Sprintf("Invoice #%d\n", h.InvoiceNumber))
	sb.WriteString(fmt.Sprintf("Status: %s  Currency: %s\n", h.Status, h.Currency))
	if h.IssuedAt != nil {
		sb.WriteString("Issued: ")
		sb.WriteString(h.IssuedAt.UTC().Format(time.RFC3339))
		sb.WriteByte('\n')
	}
	sb.WriteString("\n")
	for _, ln := range lines {
		sb.WriteString(fmt.Sprintf("%s | qty %s | unit %s | line %s\n",
			ln.Description, ln.Quantity, ln.UnitDisplay, ln.LineDisplay))
	}
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("Subtotal: %s\n", h.SubtotalDisp))
	sb.WriteString(fmt.Sprintf("Tax: %s\n", h.TaxDisp))
	sb.WriteString(fmt.Sprintf("Total: %s\n", h.TotalDisp))

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
