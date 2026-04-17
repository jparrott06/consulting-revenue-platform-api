package repo

import (
	"context"
	"database/sql"
)

// InsertStripeWebhookEvent persists a Stripe webhook payload if the event id is new.
// Returns inserted=true when a new row was written; false when the event was a duplicate.
func InsertStripeWebhookEvent(ctx context.Context, db *sql.DB, eventID, eventType string, payloadJSON []byte) (inserted bool, err error) {
	res, err := db.ExecContext(ctx, `
INSERT INTO webhook_events (provider, event_id, event_type, payload_json)
VALUES ('stripe', $1, $2, $3::jsonb)
ON CONFLICT (provider, event_id) DO NOTHING`, eventID, eventType, string(payloadJSON))
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
