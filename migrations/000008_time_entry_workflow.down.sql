DROP TABLE IF EXISTS time_entry_events;

ALTER TABLE time_entries
  DROP COLUMN IF EXISTS rejected_reason,
  DROP COLUMN IF EXISTS approver_user_id,
  DROP COLUMN IF EXISTS reviewed_at,
  DROP COLUMN IF EXISTS submitted_by_user_id,
  DROP COLUMN IF EXISTS submitted_at;
