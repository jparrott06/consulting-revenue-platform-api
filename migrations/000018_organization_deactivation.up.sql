ALTER TABLE organizations
  ADD COLUMN IF NOT EXISTS deactivated_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS organizations_deactivated_at_idx ON organizations (deactivated_at)
  WHERE deactivated_at IS NOT NULL;
