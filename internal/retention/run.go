package retention

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/jparrott06/consulting-revenue-platform-api/internal/config"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/jobworker"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/repo"
)

var retentionLog = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

// RunOnce applies configured retention windows and returns rows removed per table.
func RunOnce(ctx context.Context, db *sql.DB, cfg config.Config) (auditDeleted, webhookDeleted int64, err error) {
	auditDays := cfg.RetentionAuditLogDays
	if auditDays <= 0 {
		auditDays = 365
	}
	whDays := cfg.RetentionWebhookEventDays
	if whDays <= 0 {
		whDays = 90
	}
	now := time.Now().UTC()
	auditCutoff := now.AddDate(0, 0, -auditDays)
	whCutoff := now.AddDate(0, 0, -whDays)

	auditDeleted, err = repo.PurgeAuditLogsOlderThan(ctx, db, auditCutoff)
	if err != nil {
		return 0, 0, fmt.Errorf("purge audit_logs: %w", err)
	}
	webhookDeleted, err = repo.PurgeWebhookEventsOlderThan(ctx, db, whCutoff)
	if err != nil {
		return auditDeleted, 0, fmt.Errorf("purge webhook_events: %w", err)
	}
	return auditDeleted, webhookDeleted, nil
}

// Run polls and executes retention deletes until ctx is cancelled.
func Run(ctx context.Context, cfg config.Config, db *sql.DB) error {
	interval := cfg.RetentionWorkerPollInterval
	if interval <= 0 {
		interval = time.Hour
	}
	return jobworker.Poll(ctx, interval, func(ctx context.Context) error {
		cid := newRetentionCorrelationID()
		a, w, err := RunOnce(ctx, db, cfg)
		if err != nil {
			retentionLog.Error("retention run error", "component", "retention", "correlation_id", cid, "msg", err.Error())
			return jobworker.ErrIdle
		}
		if a > 0 || w > 0 {
			retentionLog.Info("retention purge complete", "component", "retention", "correlation_id", cid, "audit_logs_removed", a, "webhook_events_removed", w)
		}
		return jobworker.ErrIdle
	})
}

func newRetentionCorrelationID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(b)
}
