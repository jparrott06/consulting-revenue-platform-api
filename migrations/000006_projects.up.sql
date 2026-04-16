CREATE TABLE IF NOT EXISTS projects (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  client_id UUID NOT NULL REFERENCES clients(id) ON DELETE RESTRICT,
  name TEXT NOT NULL CHECK (char_length(trim(name)) > 0),
  billing_mode TEXT NOT NULL CHECK (billing_mode IN ('hourly', 'fixed', 'non_billable')),
  default_rate_minor BIGINT NOT NULL CHECK (default_rate_minor >= 0),
  archived BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS projects_org_client_idx ON projects (organization_id, client_id);
CREATE INDEX IF NOT EXISTS projects_org_created_idx ON projects (organization_id, created_at ASC, id ASC);
