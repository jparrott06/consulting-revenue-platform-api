package webhookworker

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// stripeWebhookWorkerOutcomes counts worker processing results (low-cardinality event_category).
var stripeWebhookWorkerOutcomes = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "stripe_webhook_worker_outcomes_total",
		Help: "Stripe webhook worker processing outcomes by coarse event category.",
	},
	[]string{"event_category", "outcome"},
)

// outcomeSuccess: handler completed without error (including intentional no-ops).
// outcomeRetry: processing error; row remains pending for retry.
// outcomeTerminal: max attempts exhausted; event dead-lettered.
func recordStripeWebhookOutcome(eventType string, outcome string) {
	stripeWebhookWorkerOutcomes.WithLabelValues(stripeEventCategory(eventType), outcome).Inc()
}

// stripeEventCategory maps Stripe event types to a small label set (never raw event id).
func stripeEventCategory(eventType string) string {
	s := strings.TrimSpace(eventType)
	switch {
	case strings.HasPrefix(s, "checkout.session"):
		return "checkout_session"
	case strings.HasPrefix(s, "payment_intent."):
		return "payment_intent"
	case strings.HasPrefix(s, "refund."):
		return "refund"
	case strings.HasPrefix(s, "charge."):
		return "charge"
	case s == "":
		return "unknown"
	default:
		return "other"
	}
}
