package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// InsertAuditLogParams describes one append-only audit record (metadata is redacted before storage).
type InsertAuditLogParams struct {
	OrganizationID *uuid.UUID
	ActorUserID    *uuid.UUID
	Action         string
	EntityType     string
	EntityID       *uuid.UUID
	Metadata       map[string]any
}

// auditExec supports inserting from *sql.DB or *sql.Tx.
type auditExec interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

// InsertAuditLog appends an audit row using the default database connection executor.
func InsertAuditLog(ctx context.Context, db *sql.DB, p InsertAuditLogParams) error {
	return insertAuditLog(ctx, db, p)
}

// InsertAuditLogTx appends an audit row within an existing transaction.
func InsertAuditLogTx(ctx context.Context, tx *sql.Tx, p InsertAuditLogParams) error {
	return insertAuditLog(ctx, tx, p)
}

func insertAuditLog(ctx context.Context, exec auditExec, p InsertAuditLogParams) error {
	if strings.TrimSpace(p.Action) == "" {
		return errors.New("audit log: action is required")
	}
	meta := RedactAuditMetadata(cloneMetadata(p.Metadata))
	b, err := json.Marshal(meta)
	if err != nil {
		return err
	}

	var org any
	if p.OrganizationID != nil {
		org = *p.OrganizationID
	}
	var actor any
	if p.ActorUserID != nil {
		actor = *p.ActorUserID
	}
	var entity any
	if p.EntityID != nil {
		entity = *p.EntityID
	}
	et := strings.TrimSpace(p.EntityType)

	_, err = exec.ExecContext(ctx, `
INSERT INTO audit_logs (organization_id, actor_user_id, action, entity_type, entity_id, metadata_json)
VALUES ($1, $2, $3, $4, $5, $6::jsonb)`,
		org, actor, p.Action, et, entity, b,
	)
	return err
}

func cloneMetadata(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// RedactAuditMetadata removes or masks nested values that must not appear in audit storage.
func RedactAuditMetadata(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			lk := strings.ToLower(strings.TrimSpace(k))
			if sensitiveJSONKey(lk) {
				out[k] = "[REDACTED]"
				continue
			}
			out[k] = RedactAuditMetadata(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = RedactAuditMetadata(val)
		}
		return out
	default:
		return v
	}
}

func sensitiveJSONKey(lk string) bool {
	if lk == "" {
		return false
	}
	if strings.Contains(lk, "password") || strings.Contains(lk, "passwd") {
		return true
	}
	suffixes := []string{"_token", "_secret", "_signature", "_hash", "_cookie"}
	for _, suf := range suffixes {
		if strings.HasSuffix(lk, suf) {
			return true
		}
	}
	switch lk {
	case "password", "token", "refresh_token", "access_token", "authorization",
		"secret", "api_key", "apikey", "webhook_secret", "stripe_signature",
		"signature", "cookie", "cookies", "bearer":
		return true
	default:
		return false
	}
}

// AuditLogRecord is one row from audit_logs.
type AuditLogRecord struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	ActorUserID    sql.NullString
	EntityID       sql.NullString
	Action         string
	EntityType     string
	MetadataJSON   []byte
	CreatedAt      time.Time
}

// ListAuditLogs returns recent audit rows for one organization (newest first).
func ListAuditLogs(ctx context.Context, db *sql.DB, organizationID uuid.UUID, limit int) ([]AuditLogRecord, error) {
	if limit < 1 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}
	rows, err := db.QueryContext(ctx, `
SELECT id, organization_id, actor_user_id, action, entity_type, entity_id, metadata_json, created_at
FROM audit_logs
WHERE organization_id = $1
ORDER BY created_at DESC
LIMIT $2`, organizationID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []AuditLogRecord
	for rows.Next() {
		var rec AuditLogRecord
		if err := rows.Scan(
			&rec.ID,
			&rec.OrganizationID,
			&rec.ActorUserID,
			&rec.Action,
			&rec.EntityType,
			&rec.EntityID,
			&rec.MetadataJSON,
			&rec.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}
