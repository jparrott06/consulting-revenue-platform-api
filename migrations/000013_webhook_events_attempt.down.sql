DROP INDEX IF EXISTS webhook_events_pending_received_idx;

ALTER TABLE webhook_events DROP COLUMN IF EXISTS attempt_count;
