package retention

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/jparrott06/consulting-revenue-platform-api/internal/config"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/jobworker"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/repo"
)

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
		a, w, err := RunOnce(ctx, db, cfg)
		if err != nil {
			log.Printf("retention: run error: %v", err)
			return jobworker.ErrIdle
		}
		if a > 0 || w > 0 {
			log.Printf("retention: removed audit_logs=%d webhook_events=%d", a, w)
		}
		return jobworker.ErrIdle
	})
}
