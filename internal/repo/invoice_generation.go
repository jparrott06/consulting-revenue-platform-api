package repo

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNoEligibleTimeEntries is returned when no approved uninvoiced entries exist for range.
var ErrNoEligibleTimeEntries = errors.New("no eligible time entries")

// GenerateInvoiceParams controls approved entry selection.
type GenerateInvoiceParams struct {
	FromDate time.Time
	ToDate   time.Time
	Currency string
	DueAt    *time.Time
}

func GenerateInvoiceFromApprovedEntries(ctx context.Context, db *sql.DB, organizationID uuid.UUID, params GenerateInvoiceParams) (InvoiceRecord, error) {
	currency, err := NormalizeCurrencyCode(params.Currency)
	if err != nil {
		return InvoiceRecord{}, err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return InvoiceRecord{}, err
	}
	defer func() { _ = tx.Rollback() }()

	number, err := AllocateNextInvoiceNumber(ctx, tx, organizationID)
	if err != nil {
		return InvoiceRecord{}, err
	}

	var due sql.NullTime
	if params.DueAt != nil {
		due = sql.NullTime{Time: params.DueAt.UTC(), Valid: true}
	}

	var invoice InvoiceRecord
	err = tx.QueryRowContext(ctx, `
INSERT INTO invoices (organization_id, invoice_number, status, currency, due_at)
VALUES ($1, $2, 'draft', $3, $4)
RETURNING id, organization_id, invoice_number, status, currency, subtotal_minor, tax_minor, total_minor, issued_at, due_at, created_at, updated_at`,
		organizationID, number, currency, due,
	).Scan(
		&invoice.ID,
		&invoice.OrganizationID,
		&invoice.InvoiceNumber,
		&invoice.Status,
		&invoice.Currency,
		&invoice.SubtotalMinor,
		&invoice.TaxMinor,
		&invoice.TotalMinor,
		&invoice.IssuedAt,
		&invoice.DueAt,
		&invoice.CreatedAt,
		&invoice.UpdatedAt,
	)
	if err != nil {
		return InvoiceRecord{}, err
	}

	rows, err := tx.QueryContext(ctx, `
SELECT id, work_date, minutes, hourly_rate_minor
FROM time_entries
WHERE organization_id = $1
  AND status = 'approved'
  AND invoice_id IS NULL
  AND work_date >= $2
  AND work_date <= $3
ORDER BY work_date ASC, id ASC
FOR UPDATE`, organizationID, params.FromDate, params.ToDate)
	if err != nil {
		return InvoiceRecord{}, err
	}
	defer func() { _ = rows.Close() }()

	type row struct {
		ID       uuid.UUID
		WorkDate time.Time
		Minutes  int
		Rate     int64
	}
	var selected []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.ID, &r.WorkDate, &r.Minutes, &r.Rate); err != nil {
			return InvoiceRecord{}, err
		}
		selected = append(selected, r)
	}
	if err := rows.Err(); err != nil {
		return InvoiceRecord{}, err
	}
	if len(selected) == 0 {
		if _, err := tx.ExecContext(ctx, `DELETE FROM invoices WHERE id = $1 AND organization_id = $2`, invoice.ID, organizationID); err != nil {
			return InvoiceRecord{}, err
		}
		return InvoiceRecord{}, ErrNoEligibleTimeEntries
	}

	var subtotal int64
	for _, it := range selected {
		lineTotal := int64(it.Minutes) * it.Rate / 60
		subtotal += lineTotal
		desc := it.WorkDate.Format("2006-01-02") + " approved work"
		if _, err := tx.ExecContext(ctx, `
INSERT INTO invoice_line_items (invoice_id, organization_id, source_time_entry_id, description, quantity, unit_amount_minor, line_total_minor)
VALUES ($1, $2, $3, $4, $5, $6, $7)`, invoice.ID, organizationID, it.ID, desc, float64(it.Minutes)/60.0, it.Rate, lineTotal); err != nil {
			return InvoiceRecord{}, err
		}
		if _, err := tx.ExecContext(ctx, `
UPDATE time_entries SET invoice_id = $1, status = 'invoiced', updated_at = NOW()
WHERE id = $2 AND organization_id = $3`, invoice.ID, it.ID, organizationID); err != nil {
			return InvoiceRecord{}, err
		}
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE invoices
SET subtotal_minor = $1, tax_minor = 0, total_minor = $1, updated_at = NOW()
WHERE id = $2 AND organization_id = $3`, subtotal, invoice.ID, organizationID); err != nil {
		return InvoiceRecord{}, err
	}

	invoice.SubtotalMinor = subtotal
	invoice.TotalMinor = subtotal
	if err := tx.Commit(); err != nil {
		return InvoiceRecord{}, err
	}
	return invoice, nil
}
