CREATE TABLE IF NOT EXISTS stripe_refund_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  stripe_refund_id TEXT NOT NULL UNIQUE,
  payment_id UUID NOT NULL REFERENCES payments(id) ON DELETE CASCADE,
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  amount_minor BIGINT NOT NULL CHECK (amount_minor > 0),
  currency TEXT NOT NULL CHECK (char_length(currency) = 3 AND currency = upper(currency)),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS stripe_refund_events_payment_idx ON stripe_refund_events (payment_id);

ALTER TABLE payments
  ADD COLUMN IF NOT EXISTS refunded_amount_minor BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS last_stripe_failure_code TEXT;

ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_refunded_nonneg;
ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_refunded_lte_amount;

ALTER TABLE payments
  ADD CONSTRAINT payments_refunded_nonneg CHECK (refunded_amount_minor >= 0);

ALTER TABLE payments
  ADD CONSTRAINT payments_refunded_lte_amount CHECK (refunded_amount_minor <= amount_minor);

CREATE INDEX IF NOT EXISTS payments_stripe_payment_intent_id_idx
  ON payments (stripe_payment_intent_id)
  WHERE stripe_payment_intent_id IS NOT NULL AND BTRIM(stripe_payment_intent_id) <> '';
