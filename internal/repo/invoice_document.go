package repo

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

// InvoiceLineItemPDF is a line row for PDF rendering (quantity as display text from DB).
type InvoiceLineItemPDF struct {
	Description     string
	QuantityText    string
	UnitAmountMinor int64
	LineTotalMinor  int64
}

// ListInvoiceLineItemsForPDF returns line items for an invoice in stable order.
func ListInvoiceLineItemsForPDF(ctx context.Context, db *sql.DB, organizationID, invoiceID uuid.UUID) ([]InvoiceLineItemPDF, error) {
	rows, err := db.QueryContext(ctx, `
SELECT description, quantity::text, unit_amount_minor, line_total_minor
FROM invoice_line_items
WHERE invoice_id = $1 AND organization_id = $2
ORDER BY created_at ASC, id ASC`, invoiceID, organizationID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []InvoiceLineItemPDF
	for rows.Next() {
		var r InvoiceLineItemPDF
		if err := rows.Scan(&r.Description, &r.QuantityText, &r.UnitAmountMinor, &r.LineTotalMinor); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
