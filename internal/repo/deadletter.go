package repo

import (
	"context"
	"database/sql"
)

// InsertDeadLetter persists a poisoned or exhausted job payload for later inspection.
func InsertDeadLetter(ctx context.Context, db *sql.DB, queueName string, payload []byte, attempt int, errText string) error {
	_, err := db.ExecContext(ctx, `
INSERT INTO jobs_dead_letter (queue_name, payload, error, attempt)
VALUES ($1, $2, $3, $4)`, queueName, payload, errText, attempt)
	return err
}

// InsertDeadLetterTx is the transactional variant of InsertDeadLetter.
func InsertDeadLetterTx(ctx context.Context, tx *sql.Tx, queueName string, payload []byte, attempt int, errText string) error {
	_, err := tx.ExecContext(ctx, `
INSERT INTO jobs_dead_letter (queue_name, payload, error, attempt)
VALUES ($1, $2, $3, $4)`, queueName, payload, errText, attempt)
	return err
}
