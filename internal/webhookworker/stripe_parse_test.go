package webhookworker

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestParseStripePaidReconcileInput_CheckoutSessionCompleted(t *testing.T) {
	t.Parallel()

	inv := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	raw := map[string]any{
		"id":   "evt_cs_1",
		"type": "checkout.session.completed",
		"data": map[string]any{
			"object": map[string]any{
				"id":           "cs_test_1",
				"mode":         "payment",
				"amount_total": 5000,
				"currency":     "usd",
				"metadata": map[string]any{
					"invoice_id": inv.String(),
				},
				"payment_link":   "plink_abc",
				"payment_intent": "pi_xyz",
			},
		},
	}
	payload, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}

	in, disp, err := parseStripePaidReconcileInput(payload)
	if err != nil {
		t.Fatal(err)
	}
	if disp != stripePaidApply {
		t.Fatalf("expected apply, got %v", disp)
	}
	if in.MetadataInvoiceID != inv.String() {
		t.Fatalf("metadata invoice: got %q", in.MetadataInvoiceID)
	}
	if in.StripePaymentLinkID != "plink_abc" {
		t.Fatalf("link: got %q", in.StripePaymentLinkID)
	}
	if in.StripeCheckoutSessionID != "cs_test_1" {
		t.Fatalf("session: got %q", in.StripeCheckoutSessionID)
	}
	if in.StripePaymentIntentID != "pi_xyz" {
		t.Fatalf("pi: got %q", in.StripePaymentIntentID)
	}
	if in.AmountMinor != 5000 || in.Currency != "usd" {
		t.Fatalf("amount/currency: %+v", in)
	}
}

func TestParseStripePaidReconcileInput_PaymentIntentSucceeded(t *testing.T) {
	t.Parallel()

	inv := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	raw := map[string]any{
		"id":   "evt_pi_1",
		"type": "payment_intent.succeeded",
		"data": map[string]any{
			"object": map[string]any{
				"id":       "pi_123",
				"amount":   100,
				"currency": "usd",
				"metadata": map[string]any{
					"invoice_id": inv.String(),
				},
			},
		},
	}
	payload, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}

	in, disp, err := parseStripePaidReconcileInput(payload)
	if err != nil {
		t.Fatal(err)
	}
	if disp != stripePaidApply {
		t.Fatalf("expected apply, got %v", disp)
	}
	if in.MetadataInvoiceID != inv.String() {
		t.Fatalf("metadata: %q", in.MetadataInvoiceID)
	}
	if in.StripePaymentIntentID != "pi_123" || in.AmountMinor != 100 {
		t.Fatalf("unexpected %+v", in)
	}
}

func TestParseStripePaidReconcileInput_PaymentFailedSkips(t *testing.T) {
	t.Parallel()

	raw := map[string]any{
		"id":   "evt_pi_fail",
		"type": "payment_intent.payment_failed",
		"data": map[string]any{"object": map[string]any{"id": "pi_fail"}},
	}
	payload, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	_, disp, err := parseStripePaidReconcileInput(payload)
	if err != nil {
		t.Fatal(err)
	}
	if disp != stripePaidSkipSilent {
		t.Fatalf("expected skip, got %v", disp)
	}
}
