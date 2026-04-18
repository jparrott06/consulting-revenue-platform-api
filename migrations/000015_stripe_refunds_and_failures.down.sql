DROP INDEX IF EXISTS payments_stripe_payment_intent_id_idx;

ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_refunded_nonneg;
ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_refunded_lte_amount;

ALTER TABLE payments
  DROP COLUMN IF EXISTS last_stripe_failure_code,
  DROP COLUMN IF EXISTS refunded_amount_minor;

DROP TABLE IF EXISTS stripe_refund_events;
