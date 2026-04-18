package webhookworker

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jparrott06/consulting-revenue-platform-api/internal/repo"
	"github.com/stripe/stripe-go/v81"
)

type stripePaidDispatch int

const (
	stripePaidNone stripePaidDispatch = iota
	stripePaidApply
	stripePaidSkipSilent
)

type checkoutSessionDTO struct {
	ID            string            `json:"id"`
	Mode          string            `json:"mode"`
	Metadata      map[string]string `json:"metadata"`
	AmountTotal   int64             `json:"amount_total"`
	Currency      string            `json:"currency"`
	PaymentIntent json.RawMessage   `json:"payment_intent"`
	PaymentLink   json.RawMessage   `json:"payment_link"`
}

type paymentIntentDTO struct {
	ID             string            `json:"id"`
	Metadata       map[string]string `json:"metadata"`
	Amount         int64             `json:"amount"`
	AmountReceived int64             `json:"amount_received"`
	Currency       string            `json:"currency"`
}

func stripeJSONRefID(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	if raw[0] == '"' {
		var s string
		_ = json.Unmarshal(raw, &s)
		return s
	}
	var wrap struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(raw, &wrap)
	return wrap.ID
}

func metadataInvoiceID(meta map[string]string) string {
	if meta == nil {
		return ""
	}
	return meta["invoice_id"]
}

// parseStripePaidReconcileInput extracts a successful charge for invoice reconciliation, or skip for unrelated events.
func parseStripePaidReconcileInput(payload []byte) (repo.StripePaidReconcileInput, stripePaidDispatch, error) {
	var ev stripe.Event
	if err := json.Unmarshal(payload, &ev); err != nil {
		return repo.StripePaidReconcileInput{}, stripePaidNone, fmt.Errorf("stripe webhook: decode event: %w", err)
	}

	switch ev.Type {
	case stripe.EventTypePaymentIntentPaymentFailed:
		return repo.StripePaidReconcileInput{}, stripePaidSkipSilent, nil

	case stripe.EventTypeCheckoutSessionCompleted:
		var sess checkoutSessionDTO
		if err := json.Unmarshal(ev.Data.Raw, &sess); err != nil {
			return repo.StripePaidReconcileInput{}, stripePaidNone, fmt.Errorf("stripe webhook: checkout session: %w", err)
		}
		if sess.Mode != string(stripe.CheckoutSessionModePayment) {
			return repo.StripePaidReconcileInput{}, stripePaidSkipSilent, nil
		}
		if sess.AmountTotal <= 0 {
			return repo.StripePaidReconcileInput{}, stripePaidNone, errors.New("stripe webhook: checkout session has no charge amount")
		}
		meta := metadataInvoiceID(sess.Metadata)
		linkID := stripeJSONRefID(sess.PaymentLink)
		piID := stripeJSONRefID(sess.PaymentIntent)
		if meta == "" && linkID == "" {
			return repo.StripePaidReconcileInput{}, stripePaidNone, fmt.Errorf("%w", repo.ErrStripeReconcileCannotResolveInvoice)
		}
		return repo.StripePaidReconcileInput{
			MetadataInvoiceID:       meta,
			StripePaymentLinkID:     linkID,
			StripeCheckoutSessionID: sess.ID,
			StripePaymentIntentID:   piID,
			AmountMinor:             sess.AmountTotal,
			Currency:                sess.Currency,
		}, stripePaidMetadata(meta, linkID), nil

	case stripe.EventTypePaymentIntentSucceeded:
		var pi paymentIntentDTO
		if err := json.Unmarshal(ev.Data.Raw, &pi); err != nil {
			return repo.StripePaidReconcileInput{}, stripePaidNone, fmt.Errorf("stripe webhook: payment_intent: %w", err)
		}
		amt := pi.AmountReceived
		if amt <= 0 {
			amt = pi.Amount
		}
		if amt <= 0 {
			return repo.StripePaidReconcileInput{}, stripePaidNone, errors.New("stripe webhook: payment_intent has no amount")
		}
		meta := metadataInvoiceID(pi.Metadata)
		if meta == "" {
			return repo.StripePaidReconcileInput{}, stripePaidNone, fmt.Errorf("%w", repo.ErrStripeReconcileCannotResolveInvoice)
		}
		return repo.StripePaidReconcileInput{
			MetadataInvoiceID:       meta,
			StripePaymentLinkID:     "",
			StripeCheckoutSessionID: "",
			StripePaymentIntentID:   pi.ID,
			AmountMinor:             amt,
			Currency:                pi.Currency,
		}, stripePaidMetadata(meta, ""), nil

	default:
		return repo.StripePaidReconcileInput{}, stripePaidSkipSilent, nil
	}
}

func stripePaidMetadata(metaInvoiceID, linkID string) stripePaidDispatch {
	if metaInvoiceID == "" && linkID == "" {
		return stripePaidNone
	}
	return stripePaidApply
}
