package webhookworker

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jparrott06/consulting-revenue-platform-api/internal/config"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/repo"
)

// ProcessOne locks and processes at most one pending Stripe webhook event.
func ProcessOne(ctx context.Context, db *sql.DB, cfg config.Config) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	max := cfg.WebhookWorkerMaxAttempts
	if max < 2 {
		max = 2
	}

	row, err := repo.LockNextPendingStripeWebhook(ctx, tx, max)
	if errors.Is(err, repo.ErrNoPendingWebhookEvent) {
		return err
	}
	if err != nil {
		return err
	}

	if err := handleStripeWebhook(ctx, tx, row); err != nil {
		if rerr := repo.RecordStripeWebhookFailure(ctx, tx, row.ID, max, err.Error()); rerr != nil {
			return rerr
		}
		return tx.Commit()
	}

	if err := repo.MarkStripeWebhookProcessed(ctx, tx, row.ID); err != nil {
		return err
	}
	return tx.Commit()
}
