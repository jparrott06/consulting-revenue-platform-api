CREATE TABLE IF NOT EXISTS jobs_dead_letter (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  queue_name TEXT NOT NULL,
  payload BYTEA NOT NULL,
  error TEXT NOT NULL,
  attempt INT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS jobs_dead_letter_queue_created_idx ON jobs_dead_letter (queue_name, created_at DESC);
