package repo

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrInvoiceNotFound indicates invoice does not exist for organization scope.
var ErrInvoiceNotFound = errors.New("invoice not found")

// ErrInvoiceNotSendable is returned when the invoice cannot be issued (for example void or paid).
var ErrInvoiceNotSendable = errors.New("invoice cannot be sent in current state")

// InvoiceRecord stores invoice header fields.
type InvoiceRecord struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	InvoiceNumber  int64
	Status         string
	Currency       string
	SubtotalMinor  int64
	TaxMinor       int64
	TotalMinor     int64
	IssuedAt       sql.NullTime
	DueAt          sql.NullTime
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// AllocateNextInvoiceNumber allocates the next monotonic invoice number per organization.
func AllocateNextInvoiceNumber(ctx context.Context, tx *sql.Tx, organizationID uuid.UUID) (int64, error) {
	var allocated int64
	err := tx.QueryRowContext(ctx, `
INSERT INTO invoice_sequences (organization_id, next_number, updated_at)
VALUES ($1, 2, NOW())
ON CONFLICT (organization_id)
DO UPDATE SET next_number = invoice_sequences.next_number + 1, updated_at = NOW()
RETURNING next_number - 1`, organizationID).Scan(&allocated)
	if err != nil {
		return 0, err
	}
	return allocated, nil
}

// CreateDraftInvoice creates a draft invoice with allocated per-org number in one transaction.
func CreateDraftInvoice(ctx context.Context, db *sql.DB, organizationID uuid.UUID, currency string, dueAt *time.Time) (InvoiceRecord, error) {
	currency, err := NormalizeCurrencyCode(currency)
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
	if dueAt != nil {
		due = sql.NullTime{Time: dueAt.UTC(), Valid: true}
	}

	var rec InvoiceRecord
	err = tx.QueryRowContext(ctx, `
INSERT INTO invoices (organization_id, invoice_number, status, currency, due_at)
VALUES ($1, $2, 'draft', $3, $4)
RETURNING id, organization_id, invoice_number, status, currency, subtotal_minor, tax_minor, total_minor, issued_at, due_at, created_at, updated_at`,
		organizationID, number, currency, due,
	).Scan(
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
	if err != nil {
		return InvoiceRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return InvoiceRecord{}, err
	}
	return rec, nil
}

// SendInvoice transitions a draft invoice to issued (sent), sets issued_at when first issued, and is
// idempotent when the invoice is already issued.
func SendInvoice(ctx context.Context, db *sql.DB, organizationID, invoiceID uuid.UUID) (InvoiceRecord, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return InvoiceRecord{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var rec InvoiceRecord
	err = tx.QueryRowContext(ctx, `
UPDATE invoices
SET status = 'issued', issued_at = COALESCE(issued_at, NOW()), updated_at = NOW()
WHERE id = $1 AND organization_id = $2 AND status = 'draft'
RETURNING id, organization_id, invoice_number, status, currency, subtotal_minor, tax_minor, total_minor, issued_at, due_at, created_at, updated_at`,
		invoiceID, organizationID,
	).Scan(
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
	if err == nil {
		if err := tx.Commit(); err != nil {
			return InvoiceRecord{}, err
		}
		return rec, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return InvoiceRecord{}, err
	}

	rec, err = loadInvoiceHeader(ctx, tx, organizationID, invoiceID)
	if errors.Is(err, sql.ErrNoRows) {
		return InvoiceRecord{}, ErrInvoiceNotFound
	}
	if err != nil {
		return InvoiceRecord{}, err
	}
	if rec.Status == "issued" {
		if err := tx.Commit(); err != nil {
			return InvoiceRecord{}, err
		}
		return rec, nil
	}
	return InvoiceRecord{}, ErrInvoiceNotSendable
}

func loadInvoiceHeader(ctx context.Context, tx *sql.Tx, organizationID, invoiceID uuid.UUID) (InvoiceRecord, error) {
	var rec InvoiceRecord
	err := tx.QueryRowContext(ctx, `
SELECT id, organization_id, invoice_number, status, currency, subtotal_minor, tax_minor, total_minor, issued_at, due_at, created_at, updated_at
FROM invoices
WHERE id = $1 AND organization_id = $2`, invoiceID, organizationID).Scan(
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
	return rec, err
}
