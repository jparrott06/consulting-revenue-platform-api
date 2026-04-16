package repo

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/google/uuid"
)

// ErrInvalidTimeEntryTransition indicates the entry cannot move from current state.
var ErrInvalidTimeEntryTransition = errors.New("invalid time entry transition")

// ErrRejectReasonRequired indicates reject endpoint was called without reason.
var ErrRejectReasonRequired = errors.New("reject reason is required")

func SubmitTimeEntry(ctx context.Context, db *sql.DB, organizationID, entryID, actorUserID uuid.UUID) error {
	return applyTimeEntryTransition(ctx, db, organizationID, entryID, actorUserID, "submit", "draft", "submitted", "")
}

func ApproveTimeEntry(ctx context.Context, db *sql.DB, organizationID, entryID, actorUserID uuid.UUID) error {
	return applyTimeEntryTransition(ctx, db, organizationID, entryID, actorUserID, "approve", "submitted", "approved", "")
}

func RejectTimeEntry(ctx context.Context, db *sql.DB, organizationID, entryID, actorUserID uuid.UUID, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return ErrRejectReasonRequired
	}
	return applyTimeEntryTransition(ctx, db, organizationID, entryID, actorUserID, "reject", "submitted", "draft", reason)
}

func applyTimeEntryTransition(ctx context.Context, db *sql.DB, organizationID, entryID, actorUserID uuid.UUID, action, fromStatus, toStatus, reason string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var status string
	var ownerUserID uuid.UUID
	err = tx.QueryRowContext(ctx, `
SELECT status, user_id
FROM time_entries
WHERE id = $1 AND organization_id = $2
FOR UPDATE`, entryID, organizationID).Scan(&status, &ownerUserID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrTimeEntryNotFound
	}
	if err != nil {
		return err
	}
	if status != fromStatus {
		return ErrInvalidTimeEntryTransition
	}

	switch action {
	case "submit":
		_, err = tx.ExecContext(ctx, `
UPDATE time_entries
SET status = 'submitted', submitted_at = NOW(), submitted_by_user_id = $1, reviewed_at = NULL, approver_user_id = NULL, rejected_reason = NULL, updated_at = NOW()
WHERE id = $2 AND organization_id = $3`, actorUserID, entryID, organizationID)
	case "approve":
		_, err = tx.ExecContext(ctx, `
UPDATE time_entries
SET status = 'approved', reviewed_at = NOW(), approver_user_id = $1, rejected_reason = NULL, updated_at = NOW()
WHERE id = $2 AND organization_id = $3`, actorUserID, entryID, organizationID)
	case "reject":
		_, err = tx.ExecContext(ctx, `
UPDATE time_entries
SET status = 'draft', reviewed_at = NOW(), approver_user_id = $1, rejected_reason = $2, updated_at = NOW()
WHERE id = $3 AND organization_id = $4`, actorUserID, reason, entryID, organizationID)
	default:
		return ErrInvalidTimeEntryTransition
	}
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
INSERT INTO time_entry_events (organization_id, time_entry_id, actor_user_id, from_status, to_status, action, reason)
VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''))`,
		organizationID, entryID, actorUserID, fromStatus, toStatus, action, reason)
	if err != nil {
		return err
	}

	return tx.Commit()
}
