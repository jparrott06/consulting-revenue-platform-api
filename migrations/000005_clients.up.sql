CREATE TABLE IF NOT EXISTS clients (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  name TEXT NOT NULL CHECK (char_length(trim(name)) > 0),
  billing_email CITEXT NOT NULL,
  currency_preference TEXT NOT NULL DEFAULT 'USD' CHECK (char_length(currency_preference) = 3 AND currency_preference = upper(currency_preference)),
  deleted_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS clients_org_active_idx ON clients (organization_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS clients_org_created_idx ON clients (organization_id, created_at ASC, id ASC);
