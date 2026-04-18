package webhookworker

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"time"

	"github.com/jparrott06/consulting-revenue-platform-api/internal/config"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/jobworker"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/repo"
)

// Run polls the database for pending Stripe webhook events until ctx is cancelled.
func Run(ctx context.Context, cfg config.Config, db *sql.DB) error {
	interval := cfg.WebhookWorkerPollInterval
	if interval <= 0 {
		interval = 5 * time.Second
	}
	return jobworker.Poll(ctx, interval, func(ctx context.Context) error {
		err := ProcessOne(ctx, db, cfg)
		if errors.Is(err, repo.ErrNoPendingWebhookEvent) {
			return jobworker.ErrIdle
		}
		if err != nil {
			log.Printf("webhookworker: process error: %v", err)
			return jobworker.ErrIdle
		}
		return nil
	})
}
