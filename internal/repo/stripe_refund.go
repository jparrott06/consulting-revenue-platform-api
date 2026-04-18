package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// ErrStripeRefundExceedsRemaining is returned when a refund would exceed the invoice payment amount.
var ErrStripeRefundExceedsRemaining = errors.New("stripe refund: amount exceeds remaining refundable balance")

// StripeRefundInput is a succeeded Stripe refund tied to a PaymentIntent.
type StripeRefundInput struct {
	StripeRefundID        string
	StripePaymentIntentID string
	AmountMinor           int64
	Currency              string
}

// StripePaymentFailureInput records a terminal failure signal on a payment row.
type StripePaymentFailureInput struct {
	StripePaymentIntentID string
	FailureCode           string
}

// LookupPaymentByStripePaymentIntentID loads a payment row by Stripe PaymentIntent id (no row lock).
func LookupPaymentByStripePaymentIntentID(ctx context.Context, tx *sql.Tx, paymentIntentID string) (PaymentRecord, error) {
	paymentIntentID = strings.TrimSpace(paymentIntentID)
	if paymentIntentID == "" {
		return PaymentRecord{}, ErrPaymentNotFound
	}
	var r PaymentRecord
	err := tx.QueryRowContext(ctx, `
SELECT id, organization_id, invoice_id, stripe_payment_link_id, stripe_checkout_session_id, stripe_payment_intent_id, payment_url, amount_minor, currency, idempotency_key, refunded_amount_minor, last_stripe_failure_code, created_at, updated_at
FROM payments
WHERE stripe_payment_intent_id IS NOT NULL AND BTRIM(stripe_payment_intent_id) = BTRIM($1)`, paymentIntentID).Scan(
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
		&r.RefundedAmountMinor,
		&r.LastStripeFailureCode,
		&r.CreatedAt,
		&r.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return PaymentRecord{}, ErrPaymentNotFound
	}
	return r, err
}

func paymentIntentIDMatches(pay PaymentRecord, want string) bool {
	want = strings.TrimSpace(want)
	if want == "" || !pay.StripePaymentIntentID.Valid {
		return false
	}
	return strings.TrimSpace(pay.StripePaymentIntentID.String) == want
}

// ApplyStripeRefund applies a succeeded Stripe refund idempotently and optionally reopens a fully refunded paid invoice.
func ApplyStripeRefund(ctx context.Context, tx *sql.Tx, in StripeRefundInput) error {
	in.StripeRefundID = strings.TrimSpace(in.StripeRefundID)
	in.StripePaymentIntentID = strings.TrimSpace(in.StripePaymentIntentID)
	if in.StripeRefundID == "" || in.StripePaymentIntentID == "" || in.AmountMinor <= 0 {
		return errors.New("stripe refund: missing refund id, payment intent, or amount")
	}

	pay0, err := LookupPaymentByStripePaymentIntentID(ctx, tx, in.StripePaymentIntentID)
	if errors.Is(err, ErrPaymentNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if !paymentIntentIDMatches(pay0, in.StripePaymentIntentID) {
		return nil
	}

	inv, err := GetInvoiceByIDForUpdate(ctx, tx, pay0.InvoiceID)
	if err != nil {
		return err
	}
	pay, err := GetPaymentByInvoiceForUpdate(ctx, tx, inv.OrganizationID, inv.ID)
	if err != nil {
		return err
	}
	if !paymentIntentIDMatches(pay, in.StripePaymentIntentID) {
		return nil
	}

	cur, err := NormalizeCurrencyCode(in.Currency)
	if err != nil {
		return fmt.Errorf("stripe refund: currency: %w", err)
	}
	if cur != pay.Currency {
		return fmt.Errorf("%w: payment %s refund %s", ErrStripeReconcileCurrencyMismatch, pay.Currency, cur)
	}

	remaining := pay.AmountMinor - pay.RefundedAmountMinor
	if in.AmountMinor > remaining {
		return fmt.Errorf("%w: remaining %d refund %d", ErrStripeRefundExceedsRemaining, remaining, in.AmountMinor)
	}

	var ledgerID uuid.UUID
	err = tx.QueryRowContext(ctx, `
INSERT INTO stripe_refund_events (stripe_refund_id, payment_id, organization_id, amount_minor, currency)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (stripe_refund_id) DO NOTHING
RETURNING id`,
		in.StripeRefundID, pay.ID, pay.OrganizationID, in.AmountMinor, pay.Currency,
	).Scan(&ledgerID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}

	newRefunded := pay.RefundedAmountMinor + in.AmountMinor
	_, err = tx.ExecContext(ctx, `
UPDATE payments
SET refunded_amount_minor = $1, updated_at = NOW()
WHERE id = $2`, newRefunded, pay.ID)
	if err != nil {
		return err
	}

	if inv.Status == "paid" && newRefunded >= pay.AmountMinor {
		_, err = tx.ExecContext(ctx, `
UPDATE invoices
SET status = 'issued', updated_at = NOW()
WHERE id = $1 AND organization_id = $2 AND status = 'paid'`, inv.ID, inv.OrganizationID)
	}
	return err
}

// RecordStripePaymentFailure stores the latest Stripe failure code on the payment row when the intent matches.
func RecordStripePaymentFailure(ctx context.Context, tx *sql.Tx, in StripePaymentFailureInput) error {
	in.StripePaymentIntentID = strings.TrimSpace(in.StripePaymentIntentID)
	if in.StripePaymentIntentID == "" {
		return nil
	}
	code := strings.TrimSpace(in.FailureCode)
	if len(code) > 255 {
		code = code[:255]
	}

	pay0, err := LookupPaymentByStripePaymentIntentID(ctx, tx, in.StripePaymentIntentID)
	if errors.Is(err, ErrPaymentNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if !paymentIntentIDMatches(pay0, in.StripePaymentIntentID) {
		return nil
	}

	inv, err := GetInvoiceByIDForUpdate(ctx, tx, pay0.InvoiceID)
	if err != nil {
		return err
	}
	pay, err := GetPaymentByInvoiceForUpdate(ctx, tx, inv.OrganizationID, inv.ID)
	if err != nil {
		return err
	}
	if !paymentIntentIDMatches(pay, in.StripePaymentIntentID) {
		return nil
	}

	_, err = tx.ExecContext(ctx, `
UPDATE payments
SET last_stripe_failure_code = $1, updated_at = NOW()
WHERE id = $2`, nullStringSQL(code), pay.ID)
	return err
}
