package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
)

// ErrNoPendingWebhookEvent is returned when no Stripe webhook row is available to process.
var ErrNoPendingWebhookEvent = errors.New("no pending webhook events")

// StripeWebhookEventRow is a locked row ready for processing.
type StripeWebhookEventRow struct {
	ID           uuid.UUID
	EventID      string
	EventType    string
	PayloadJSON  []byte
	AttemptCount int
}

// LockNextPendingStripeWebhook selects the oldest pending Stripe event with FOR UPDATE SKIP LOCKED.
// Rows with attempt_count >= maxAttempts are not eligible.
func LockNextPendingStripeWebhook(ctx context.Context, tx *sql.Tx, maxAttempts int) (StripeWebhookEventRow, error) {
	var row StripeWebhookEventRow
	err := tx.QueryRowContext(ctx, `
SELECT id, event_id, event_type, payload_json, attempt_count
FROM webhook_events
WHERE provider = 'stripe' AND processed_at IS NULL AND attempt_count < $1
ORDER BY received_at ASC
FOR UPDATE SKIP LOCKED
LIMIT 1`, maxAttempts).Scan(
		&row.ID,
		&row.EventID,
		&row.EventType,
		&row.PayloadJSON,
		&row.AttemptCount,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return StripeWebhookEventRow{}, ErrNoPendingWebhookEvent
	}
	return row, err
}

// MarkStripeWebhookProcessed sets processed_at for a successfully handled event.
func MarkStripeWebhookProcessed(ctx context.Context, tx *sql.Tx, id uuid.UUID) error {
	_, err := tx.ExecContext(ctx, `
UPDATE webhook_events SET processed_at = NOW(), processing_error = NULL WHERE id = $1`, id)
	return err
}

// RecordStripeWebhookFailure increments attempt_count, stores processing_error, and optionally
// dead-letters the event when attempt_count reaches maxAttempts (terminal failure).
// The returned terminal flag is true when the event will not be retried (dead-letter path).
func RecordStripeWebhookFailure(ctx context.Context, tx *sql.Tx, id uuid.UUID, maxAttempts int, errText string) (terminal bool, err error) {
	if len(errText) > 2000 {
		errText = errText[:2000]
	}
	var newAttempt int
	err = tx.QueryRowContext(ctx, `
UPDATE webhook_events
SET attempt_count = attempt_count + 1,
    processing_error = $2
WHERE id = $1
RETURNING attempt_count`, id, errText).Scan(&newAttempt)
	if err != nil {
		return false, err
	}
	if newAttempt < maxAttempts {
		return false, nil
	}
	dlqPayload, jerr := json.Marshal(map[string]string{"webhook_event_id": id.String()})
	if jerr != nil {
		return false, jerr
	}
	if err := InsertDeadLetterTx(ctx, tx, "stripe_webhooks", dlqPayload, newAttempt, errText); err != nil {
		return false, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE webhook_events SET processed_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return false, err
	}
	return true, nil
}
