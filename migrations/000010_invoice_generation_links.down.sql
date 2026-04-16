DROP INDEX IF EXISTS time_entries_invoice_lookup_idx;
ALTER TABLE time_entries DROP COLUMN IF EXISTS invoice_id;
