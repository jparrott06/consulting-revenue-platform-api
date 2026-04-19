DROP INDEX IF EXISTS organizations_deactivated_at_idx;

ALTER TABLE organizations
  DROP COLUMN IF EXISTS deactivated_at;
