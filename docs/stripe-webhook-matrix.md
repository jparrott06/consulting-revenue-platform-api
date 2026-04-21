# Stripe webhook matrix

This document maps **Stripe event types** to **application behavior**. Events are first verified and stored by the HTTP handler; the **webhook worker** processes pending rows asynchronously.

## Ingestion (`POST /webhooks/stripe`)

| Step | Behavior |
|------|----------|
| Signature | `Stripe-Signature` validated with `STRIPE_WEBHOOK_SECRET`. |
| Persistence | Row inserted into `webhook_events` (idempotent on Stripe `event.id`). |
| Response | `200` with `{ "received": true, "inserted": <bool> }` — does not run invoice reconciliation. |

## Worker (`internal/webhookworker`)

Processing order for each locked row (see `handlers.go`):

| Stripe event type(s) | Effect |
|----------------------|--------|
| `refund.created`, `refund.updated` | If refund status is succeeded, apply refund to ledger/payment via `ApplyStripeRefund`. Non-succeeded or missing payment intent: no mutation. |
| `refund.failed` | Intentionally ignored (no mutation). |
| `payment_intent.payment_failed` | Record failure metadata via `RecordStripePaymentFailure`. |
| `checkout.session.completed` | If mode is `payment` and amount &gt; 0, reconcile paid amount to invoice when invoice metadata or payment link resolves (`ReconcileStripePaymentPaid`). Other modes: skip silently. |
| `payment_intent.succeeded` | Reconcile when `metadata.invoice_id` is present. |
| Other types | Skipped without error (no mutation). |

## Metrics

`stripe_webhook_worker_outcomes_total{event_category,outcome}` on the **worker** path:

- **event_category** (coarse): `checkout_session`, `payment_intent`, `refund`, `charge`, `other`, `unknown`.
- **outcome**: `success` (processed or intentional no-op), `retry` (error, will retry), `terminal` (max attempts, dead-lettered).

HTTP ingestion does not increment these counters; only the worker does.

## Related

- [Threat model](threat-model.md) — webhook secrets and tenant isolation.
- [Runbook](runbook.md) — operations and Stripe configuration.
