package webhookworker

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/jparrott06/consulting-revenue-platform-api/internal/config"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/jobworker"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/repo"
)

var workerLog = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

// Run polls the database for pending Stripe webhook events until ctx is cancelled.
func Run(ctx context.Context, cfg config.Config, db *sql.DB) error {
	interval := cfg.WebhookWorkerPollInterval
	if interval <= 0 {
		interval = 5 * time.Second
	}
	return jobworker.Poll(ctx, interval, func(ctx context.Context) error {
		cid := newCorrelationID()
		err := ProcessOne(ctx, db, cfg)
		if errors.Is(err, repo.ErrNoPendingWebhookEvent) {
			return jobworker.ErrIdle
		}
		if err != nil {
			workerLog.Error("webhookworker process error", "component", "webhookworker", "correlation_id", cid, "msg", err.Error())
			return jobworker.ErrIdle
		}
		return nil
	})
}

func newCorrelationID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(b)
}
