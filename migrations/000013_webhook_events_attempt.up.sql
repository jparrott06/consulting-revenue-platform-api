ALTER TABLE webhook_events
  ADD COLUMN IF NOT EXISTS attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0);

CREATE INDEX IF NOT EXISTS webhook_events_pending_received_idx
  ON webhook_events (received_at ASC)
  WHERE processed_at IS NULL;
