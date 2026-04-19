package repo

import (
	"context"
	"database/sql"
	"time"
)

// PurgeAuditLogsOlderThan deletes audit_logs rows strictly older than cutoff (created_at < cutoff).
func PurgeAuditLogsOlderThan(ctx context.Context, db *sql.DB, cutoff time.Time) (int64, error) {
	res, err := db.ExecContext(ctx, `DELETE FROM audit_logs WHERE created_at < $1`, cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// PurgeWebhookEventsOlderThan deletes webhook_events rows strictly older than cutoff (received_at < cutoff).
func PurgeWebhookEventsOlderThan(ctx context.Context, db *sql.DB, cutoff time.Time) (int64, error) {
	res, err := db.ExecContext(ctx, `DELETE FROM webhook_events WHERE received_at < $1`, cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
