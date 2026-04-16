CREATE TABLE IF NOT EXISTS time_entries (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  work_date DATE NOT NULL,
  minutes INT NOT NULL CHECK (minutes > 0 AND minutes <= 1440),
  hourly_rate_minor BIGINT NOT NULL CHECK (hourly_rate_minor >= 0),
  status TEXT NOT NULL CHECK (status IN ('draft', 'submitted', 'approved', 'invoiced')),
  notes TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS time_entries_org_date_id_idx ON time_entries (organization_id, work_date ASC, id ASC);
CREATE INDEX IF NOT EXISTS time_entries_org_status_id_idx ON time_entries (organization_id, status, id ASC);
CREATE INDEX IF NOT EXISTS time_entries_org_user_idx ON time_entries (organization_id, user_id);
