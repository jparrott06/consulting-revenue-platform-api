package httpapi

import (
	"context"
	"database/sql"
	"log"

	"github.com/jparrott06/consulting-revenue-platform-api/internal/repo"
)

func logAudit(ctx context.Context, db *sql.DB, p repo.InsertAuditLogParams) {
	if db == nil {
		return
	}
	if err := repo.InsertAuditLog(ctx, db, p); err != nil {
		log.Printf("audit_log: insert failed action=%s: %v", p.Action, err)
	}
}
