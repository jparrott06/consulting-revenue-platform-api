package repo

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// ErrPaymentNotFound is returned when no payment row exists for the invoice.
var ErrPaymentNotFound = errors.New("payment not found for invoice")

// PaymentRecord is a persisted Stripe payment link for an invoice.
type PaymentRecord struct {
	ID                      uuid.UUID
	OrganizationID          uuid.UUID
	InvoiceID               uuid.UUID
	StripePaymentLinkID     string
	StripeCheckoutSessionID sql.NullString
	StripePaymentIntentID   sql.NullString
	PaymentURL              string
	AmountMinor             int64
	Currency                string
	IdempotencyKey          string
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

// GetInvoiceForUpdate loads an invoice row with FOR UPDATE (caller must hold an open transaction).
func GetInvoiceForUpdate(ctx context.Context, tx *sql.Tx, organizationID, invoiceID uuid.UUID) (InvoiceRecord, error) {
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

// GetPaymentByInvoiceForUpdate returns a payment row for the invoice with FOR UPDATE, or ErrPaymentNotFound.
func GetPaymentByInvoiceForUpdate(ctx context.Context, tx *sql.Tx, organizationID, invoiceID uuid.UUID) (PaymentRecord, error) {
	var r PaymentRecord
	err := tx.QueryRowContext(ctx, `
SELECT id, organization_id, invoice_id, stripe_payment_link_id, stripe_checkout_session_id, stripe_payment_intent_id, payment_url, amount_minor, currency, idempotency_key, created_at, updated_at
FROM payments
WHERE organization_id = $1 AND invoice_id = $2
FOR UPDATE`, organizationID, invoiceID).Scan(
		&r.ID,
		&r.OrganizationID,
		&r.InvoiceID,
		&r.StripePaymentLinkID,
		&r.StripeCheckoutSessionID,
		&r.StripePaymentIntentID,
		&r.PaymentURL,
		&r.AmountMinor,
		&r.Currency,
		&r.IdempotencyKey,
		&r.CreatedAt,
		&r.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return PaymentRecord{}, ErrPaymentNotFound
	}
	return r, err
}

// InsertPayment inserts a payment row. On unique violation for invoice_id (concurrent creator), returns ErrPaymentExists.
func InsertPayment(ctx context.Context, tx *sql.Tx, organizationID, invoiceID uuid.UUID, stripePaymentLinkID, paymentURL string, amountMinor int64, currency, idempotencyKey string) (PaymentRecord, error) {
	var r PaymentRecord
	err := tx.QueryRowContext(ctx, `
INSERT INTO payments (organization_id, invoice_id, stripe_payment_link_id, payment_url, amount_minor, currency, idempotency_key)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, organization_id, invoice_id, stripe_payment_link_id, stripe_checkout_session_id, stripe_payment_intent_id, payment_url, amount_minor, currency, idempotency_key, created_at, updated_at`,
		organizationID, invoiceID, stripePaymentLinkID, paymentURL, amountMinor, currency, idempotencyKey,
	).Scan(
		&r.ID,
		&r.OrganizationID,
		&r.InvoiceID,
		&r.StripePaymentLinkID,
		&r.StripeCheckoutSessionID,
		&r.StripePaymentIntentID,
		&r.PaymentURL,
		&r.AmountMinor,
		&r.Currency,
		&r.IdempotencyKey,
		&r.CreatedAt,
		&r.UpdatedAt,
	)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return PaymentRecord{}, ErrPaymentExists
		}
		return PaymentRecord{}, err
	}
	return r, nil
}

// ErrPaymentExists indicates another transaction inserted the payment for this invoice.
var ErrPaymentExists = errors.New("payment already exists for invoice")

// GetPaymentByInvoice returns a payment row without locking.
func GetPaymentByInvoice(ctx context.Context, db *sql.DB, organizationID, invoiceID uuid.UUID) (PaymentRecord, error) {
	var r PaymentRecord
	err := db.QueryRowContext(ctx, `
SELECT id, organization_id, invoice_id, stripe_payment_link_id, stripe_checkout_session_id, stripe_payment_intent_id, payment_url, amount_minor, currency, idempotency_key, created_at, updated_at
FROM payments
WHERE organization_id = $1 AND invoice_id = $2`, organizationID, invoiceID).Scan(
		&r.ID,
		&r.OrganizationID,
		&r.InvoiceID,
		&r.StripePaymentLinkID,
		&r.StripeCheckoutSessionID,
		&r.StripePaymentIntentID,
		&r.PaymentURL,
		&r.AmountMinor,
		&r.Currency,
		&r.IdempotencyKey,
		&r.CreatedAt,
		&r.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return PaymentRecord{}, ErrPaymentNotFound
	}
	return r, err
}
