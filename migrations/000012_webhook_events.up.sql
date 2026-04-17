CREATE TABLE IF NOT EXISTS webhook_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  provider TEXT NOT NULL DEFAULT 'stripe',
  event_id TEXT NOT NULL,
  event_type TEXT NOT NULL,
  payload_json JSONB NOT NULL,
  received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  processed_at TIMESTAMPTZ,
  processing_error TEXT,
  UNIQUE (provider, event_id)
);

CREATE INDEX IF NOT EXISTS webhook_events_provider_received_idx ON webhook_events (provider, received_at DESC);
