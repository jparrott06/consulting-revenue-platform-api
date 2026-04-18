DROP TRIGGER IF EXISTS ledger_entries_append_only ON ledger_entries;
DROP FUNCTION IF EXISTS forbid_ledger_entry_mutation();
DROP TABLE IF EXISTS ledger_entries;
