package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// ErrInvoiceImmutable is returned when line items are mutated for non-draft invoices.
var ErrInvoiceImmutable = errors.New("invoice is not editable")

var quantityPattern = regexp.MustCompile(`^[0-9]+(?:\.[0-9]{1,2})?$`)

// LineItemUpsert represents a create or update payload.
type LineItemUpsert struct {
	ID              *uuid.UUID
	Description     string
	Quantity        string
	UnitAmountMinor int64
}

// PatchInvoiceLineItems mutates line items and recomputes invoice totals for draft invoices only.
func PatchInvoiceLineItems(ctx context.Context, db *sql.DB, organizationID, invoiceID uuid.UUID, upserts []LineItemUpsert, removeIDs []uuid.UUID) (InvoiceRecord, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return InvoiceRecord{}, err
	}
	defer func() { _ = tx.Rollback() }()

	invoice, err := getInvoiceForUpdate(ctx, tx, organizationID, invoiceID)
	if err != nil {
		return InvoiceRecord{}, err
	}
	if invoice.Status != "draft" {
		return InvoiceRecord{}, ErrInvoiceImmutable
	}

	for _, id := range removeIDs {
		if _, err := tx.ExecContext(ctx, `
DELETE FROM invoice_line_items
WHERE id = $1 AND invoice_id = $2 AND organization_id = $3`, id, invoiceID, organizationID); err != nil {
			return InvoiceRecord{}, err
		}
	}

	for _, item := range upserts {
		desc := strings.TrimSpace(item.Description)
		if desc == "" {
			return InvoiceRecord{}, errors.New("description is required")
		}
		qtyHundredths, qtyDisplay, err := parseQuantityHundredths(item.Quantity)
		if err != nil {
			return InvoiceRecord{}, err
		}
		if item.UnitAmountMinor < 0 {
			return InvoiceRecord{}, errors.New("unit_amount_minor must be non-negative")
		}
		lineTotal, err := computeLineTotalMinor(qtyHundredths, item.UnitAmountMinor)
		if err != nil {
			return InvoiceRecord{}, err
		}

		if item.ID == nil {
			if _, err := tx.ExecContext(ctx, `
INSERT INTO invoice_line_items (invoice_id, organization_id, description, quantity, unit_amount_minor, line_total_minor)
VALUES ($1, $2, $3, $4, $5, $6)`,
				invoiceID, organizationID, desc, qtyDisplay, item.UnitAmountMinor, lineTotal,
			); err != nil {
				return InvoiceRecord{}, err
			}
			continue
		}

		res, err := tx.ExecContext(ctx, `
UPDATE invoice_line_items
SET description = $1, quantity = $2, unit_amount_minor = $3, line_total_minor = $4, updated_at = NOW()
WHERE id = $5 AND invoice_id = $6 AND organization_id = $7`,
			desc, qtyDisplay, item.UnitAmountMinor, lineTotal, *item.ID, invoiceID, organizationID,
		)
		if err != nil {
			return InvoiceRecord{}, err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return InvoiceRecord{}, err
		}
		if n == 0 {
			return InvoiceRecord{}, ErrInvoiceNotFound
		}
	}

	if err := recomputeInvoiceTotals(ctx, tx, organizationID, invoiceID); err != nil {
		return InvoiceRecord{}, err
	}

	updated, err := getInvoiceForUpdate(ctx, tx, organizationID, invoiceID)
	if err != nil {
		return InvoiceRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return InvoiceRecord{}, err
	}
	return updated, nil
}

func getInvoiceForUpdate(ctx context.Context, tx *sql.Tx, organizationID, invoiceID uuid.UUID) (InvoiceRecord, error) {
	var rec InvoiceRecord
	err := tx.QueryRowContext(ctx, `
SELECT id, organization_id, invoice_number, status, currency, subtotal_minor, tax_minor, total_minor, issued_at, due_at, created_at, updated_at
FROM invoices
WHERE id = $1 AND organization_id = $2
FOR UPDATE`, invoiceID, organizationID).Scan(
		&rec.ID,
		&rec.OrganizationID,
		&rec.InvoiceNumber,
		&rec.Status,
		&rec.Currency,
		&rec.SubtotalMinor,
		&rec.TaxMinor,
		&rec.TotalMinor,
		&rec.IssuedAt,
		&rec.DueAt,
		&rec.CreatedAt,
		&rec.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return InvoiceRecord{}, ErrInvoiceNotFound
	}
	return rec, err
}

func recomputeInvoiceTotals(ctx context.Context, tx *sql.Tx, organizationID, invoiceID uuid.UUID) error {
	var subtotal int64
	if err := tx.QueryRowContext(ctx, `
SELECT COALESCE(SUM(line_total_minor), 0)
FROM invoice_line_items
WHERE invoice_id = $1 AND organization_id = $2`, invoiceID, organizationID).Scan(&subtotal); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
UPDATE invoices
SET subtotal_minor = $1, tax_minor = 0, total_minor = $1, updated_at = NOW()
WHERE id = $2 AND organization_id = $3`, subtotal, invoiceID, organizationID)
	return err
}

func parseQuantityHundredths(raw string) (int64, string, error) {
	raw = strings.TrimSpace(raw)
	if !quantityPattern.MatchString(raw) {
		return 0, "", errors.New("quantity must be a positive decimal with up to 2 digits")
	}
	parts := strings.SplitN(raw, ".", 2)
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, "", err
	}
	frac := int64(0)
	fracDisplay := ""
	if len(parts) == 2 {
		fracDisplay = parts[1]
		if len(fracDisplay) == 1 {
			fracDisplay += "0"
		}
		f, err := strconv.ParseInt(fracDisplay, 10, 64)
		if err != nil {
			return 0, "", err
		}
		frac = f
	}
	if whole == 0 && frac == 0 {
		return 0, "", errors.New("quantity must be greater than zero")
	}
	h := whole*100 + frac
	return h, fmt.Sprintf("%d.%02d", whole, frac), nil
}

func computeLineTotalMinor(quantityHundredths, unitAmountMinor int64) (int64, error) {
	if unitAmountMinor > 0 && quantityHundredths > math.MaxInt64/unitAmountMinor {
		return 0, errors.New("line amount overflow")
	}
	product := quantityHundredths * unitAmountMinor
	if product < 0 {
		return 0, errors.New("line amount overflow")
	}
	return (product + 50) / 100, nil
}
