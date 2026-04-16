ALTER TABLE time_entries
  ADD COLUMN IF NOT EXISTS invoice_id UUID REFERENCES invoices(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS time_entries_invoice_lookup_idx ON time_entries (organization_id, invoice_id, status, work_date);
