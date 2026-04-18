package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Stripe paid reconciliation errors (permanent mismatches surface to webhook processing_error / DLQ).
var (
	ErrStripeReconcileNotIssued            = errors.New("stripe reconcile: invoice is not in issued state")
	ErrStripeReconcileNoPayment            = errors.New("stripe reconcile: no payment row for invoice")
	ErrStripeReconcilePaymentLinkMismatch  = errors.New("stripe reconcile: payment link id does not match stored payment")
	ErrStripeReconcileAmountMismatch       = errors.New("stripe reconcile: amount does not match stored payment")
	ErrStripeReconcileCurrencyMismatch     = errors.New("stripe reconcile: currency mismatch")
	ErrStripeReconcileMetadataLinkMismatch = errors.New("stripe reconcile: invoice metadata does not match payment link row")
	ErrStripeReconcileCannotResolveInvoice = errors.New("stripe reconcile: cannot resolve invoice from Stripe payload")
)

// StripePaidReconcileInput is a normalized successful charge for one invoice payment link.
type StripePaidReconcileInput struct {
	InvoiceID               uuid.UUID
	MetadataInvoiceID       string // Stripe metadata invoice_id (UUID string); used only before Resolve fills InvoiceID
	StripePaymentLinkID     string // when non-empty, must match payments.stripe_payment_link_id
	StripeCheckoutSessionID string
	StripePaymentIntentID   string
	AmountMinor             int64
	Currency                string // Stripe payload (typically lowercase ISO code)
}

// ResolveStripePaidInvoiceID maps Stripe metadata and/or payment link id to an invoice id within tx.
func ResolveStripePaidInvoiceID(ctx context.Context, tx *sql.Tx, metadataInvoiceID, stripePaymentLinkID string) (uuid.UUID, error) {
	metadataInvoiceID = strings.TrimSpace(metadataInvoiceID)
	stripePaymentLinkID = strings.TrimSpace(stripePaymentLinkID)

	if metadataInvoiceID != "" {
		invID, err := uuid.Parse(strings.TrimSpace(metadataInvoiceID))
		if err != nil {
			return uuid.UUID{}, fmt.Errorf("stripe reconcile: metadata invoice_id: %w", err)
		}
		if stripePaymentLinkID != "" {
			var rowInvoice uuid.UUID
			err := tx.QueryRowContext(ctx, `
SELECT invoice_id FROM payments WHERE stripe_payment_link_id = $1`, stripePaymentLinkID).Scan(&rowInvoice)
			if errors.Is(err, sql.ErrNoRows) {
				return uuid.UUID{}, ErrStripeReconcileNoPayment
			}
			if err != nil {
				return uuid.UUID{}, err
			}
			if rowInvoice != invID {
				return uuid.UUID{}, ErrStripeReconcileMetadataLinkMismatch
			}
		}
		return invID, nil
	}
	if stripePaymentLinkID != "" {
		var invID uuid.UUID
		err := tx.QueryRowContext(ctx, `
SELECT invoice_id FROM payments WHERE stripe_payment_link_id = $1`, stripePaymentLinkID).Scan(&invID)
		if errors.Is(err, sql.ErrNoRows) {
			return uuid.UUID{}, ErrStripeReconcileNoPayment
		}
		if err != nil {
			return uuid.UUID{}, err
		}
		return invID, nil
	}
	return uuid.UUID{}, ErrStripeReconcileCannotResolveInvoice
}

// ReconcileStripePaymentPaid marks an issued invoice paid when Stripe totals match the stored payment row.
func ReconcileStripePaymentPaid(ctx context.Context, tx *sql.Tx, in StripePaidReconcileInput) error {
	inv, err := GetInvoiceByIDForUpdate(ctx, tx, in.InvoiceID)
	if err != nil {
		return err
	}
	if inv.Status == "paid" {
		return nil
	}
	if inv.Status != "issued" {
		return fmt.Errorf("%w: status=%s", ErrStripeReconcileNotIssued, inv.Status)
	}

	pay, err := GetPaymentByInvoiceForUpdate(ctx, tx, inv.OrganizationID, inv.ID)
	if err != nil {
		if errors.Is(err, ErrPaymentNotFound) {
			return ErrStripeReconcileNoPayment
		}
		return err
	}

	if in.StripePaymentLinkID != "" && in.StripePaymentLinkID != pay.StripePaymentLinkID {
		return ErrStripeReconcilePaymentLinkMismatch
	}

	stripeCur, err := NormalizeCurrencyCode(in.Currency)
	if err != nil {
		return fmt.Errorf("stripe reconcile: currency: %w", err)
	}
	if stripeCur != inv.Currency {
		return fmt.Errorf("%w: invoice %s stripe %s", ErrStripeReconcileCurrencyMismatch, inv.Currency, stripeCur)
	}

	if in.AmountMinor != pay.AmountMinor {
		return fmt.Errorf("%w: expected %d got %d", ErrStripeReconcileAmountMismatch, pay.AmountMinor, in.AmountMinor)
	}

	res, err := tx.ExecContext(ctx, `
UPDATE invoices
SET status = 'paid', updated_at = NOW()
WHERE id = $1 AND organization_id = $2 AND status = 'issued'`, inv.ID, inv.OrganizationID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrStripeReconcileNotIssued
	}

	_, err = tx.ExecContext(ctx, `
UPDATE payments SET
  stripe_checkout_session_id = COALESCE(NULLIF(BTRIM(COALESCE(stripe_checkout_session_id, '')), ''), NULLIF(BTRIM($1), '')),
  stripe_payment_intent_id = COALESCE(NULLIF(BTRIM(COALESCE(stripe_payment_intent_id, '')), ''), NULLIF(BTRIM($2), '')),
  updated_at = NOW()
WHERE id = $3`,
		nullStringSQL(in.StripeCheckoutSessionID),
		nullStringSQL(in.StripePaymentIntentID),
		pay.ID,
	)
	if err != nil {
		return err
	}

	meta := map[string]any{"invoice_id": inv.ID.String()}
	if pay.StripePaymentIntentID.Valid {
		meta["stripe_payment_intent_id"] = strings.TrimSpace(pay.StripePaymentIntentID.String)
	}
	if in.StripePaymentIntentID != "" {
		meta["stripe_payment_intent_event"] = strings.TrimSpace(in.StripePaymentIntentID)
	}
	if err := InsertLedgerEntryTx(ctx, tx, inv.OrganizationID, LedgerEventPaymentCaptured, LedgerEntityPayment, pay.ID, pay.AmountMinor, inv.Currency, meta); err != nil {
		return err
	}
	orgID := inv.OrganizationID
	invID := inv.ID
	return InsertAuditLogTx(ctx, tx, InsertAuditLogParams{
		OrganizationID: &orgID,
		Action:         "invoice.paid",
		EntityType:     "invoice",
		EntityID:       &invID,
		Metadata: map[string]any{
			"amount_minor": pay.AmountMinor,
			"currency":     inv.Currency,
			"source":       "stripe",
		},
	})
}

func nullStringSQL(s string) interface{} {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return s
}
