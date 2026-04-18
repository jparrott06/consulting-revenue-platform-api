package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Typed ledger event names (append-only audit stream).
const (
	LedgerEventInvoiceIssued   = "invoice_issued"
	LedgerEventPaymentCaptured = "payment_captured"
	LedgerEventRefundApplied   = "refund_applied"
	LedgerEntityInvoice        = "invoice"
	LedgerEntityPayment        = "payment"
)

// LedgerEntry is one immutable financial event row.
type LedgerEntry struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	EventType      string
	EntityType     string
	EntityID       uuid.UUID
	AmountMinor    int64
	Currency       string
	MetadataJSON   []byte
	CreatedAt      time.Time
}

// InsertLedgerEntryTx appends a ledger row in the caller's transaction.
func InsertLedgerEntryTx(ctx context.Context, tx *sql.Tx, organizationID uuid.UUID, eventType, entityType string, entityID uuid.UUID, amountMinor int64, currency string, metadata map[string]any) error {
	currency, err := NormalizeCurrencyCode(currency)
	if err != nil {
		return err
	}
	meta := metadata
	if meta == nil {
		meta = map[string]any{}
	}
	raw, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO ledger_entries (organization_id, event_type, entity_type, entity_id, amount_minor, currency, metadata_json)
VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb)`,
		organizationID, eventType, entityType, entityID, amountMinor, currency, string(raw),
	)
	return err
}

// ListLedgerEntries returns recent ledger rows for an organization (newest first).
func ListLedgerEntries(ctx context.Context, db *sql.DB, organizationID uuid.UUID, limit int) ([]LedgerEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	rows, err := db.QueryContext(ctx, `
SELECT id, organization_id, event_type, entity_type, entity_id, amount_minor, currency, metadata_json, created_at
FROM ledger_entries
WHERE organization_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2`, organizationID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []LedgerEntry
	for rows.Next() {
		var e LedgerEntry
		if err := rows.Scan(
			&e.ID,
			&e.OrganizationID,
			&e.EventType,
			&e.EntityType,
			&e.EntityID,
			&e.AmountMinor,
			&e.Currency,
			&e.MetadataJSON,
			&e.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
