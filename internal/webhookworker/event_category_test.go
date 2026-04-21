package webhookworker

import "testing"

func TestStripeEventCategory(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"checkout.session.completed", "checkout_session"},
		{"payment_intent.succeeded", "payment_intent"},
		{"payment_intent.payment_failed", "payment_intent"},
		{"refund.created", "refund"},
		{"charge.succeeded", "charge"},
		{"customer.created", "other"},
		{"", "unknown"},
	}
	for _, tt := range tests {
		if got := stripeEventCategory(tt.in); got != tt.want {
			t.Fatalf("stripeEventCategory(%q)=%q want %q", tt.in, got, tt.want)
		}
	}
}
