package webhookworker

import (
	"context"
	"database/sql"

	"github.com/jparrott06/consulting-revenue-platform-api/internal/repo"
)

// handleStripeWebhook applies business logic for a locked webhook row.
// E-04 will reconcile invoices/payments from payload; until then this is a durable no-op.
func handleStripeWebhook(ctx context.Context, tx *sql.Tx, row repo.StripeWebhookEventRow) error {
	_ = ctx
	_ = tx
	_ = row
	return nil
}
